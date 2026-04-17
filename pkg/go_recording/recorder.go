//go:build windows

package go_recording

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/cyberxnomad/wasapi/audioclient"
	"github.com/cyberxnomad/wasapi/com"
	"github.com/cyberxnomad/wasapi/mmdevice"
	"golang.org/x/sys/windows"
)

const clsctxAll = windows.CLSCTX_INPROC_SERVER | windows.CLSCTX_INPROC_HANDLER |
	windows.CLSCTX_LOCAL_SERVER | windows.CLSCTX_REMOTE_SERVER

const reftimesPerSec = 10_000_000

// Options configures a Recorder before Start is called.
// At least one of File or Channel must be non-nil.
type Options struct {
	// Speaker is the render device to loopback-capture.
	// nil = Windows default render device.
	// Ignored when AudioMode is AudioModeMicOnly.
	Speaker *Device

	// Mic is the capture device to record from.
	// nil = Windows default capture device.
	// Ignored when AudioMode is AudioModeSpeakerOnly.
	Mic *Device

	// AudioMode selects how audio sources are captured and combined.
	// Defaults to AudioModeSeparate.
	AudioMode AudioMode

	// Screen, when non-nil, adds a screen recording track to the file output.
	// Requires File to be non-nil; ignored otherwise.
	// Use ListScreens to get a Screen value, or leave nil to capture the full
	// virtual desktop.
	Screen *Screen

	// Framerate is the screen recording frame rate. Defaults to 30.
	Framerate int

	// File configures file-based output (MP3 for audio-only, MP4 with screen).
	// nil = no files written.
	File *FileSink

	// Channel configures streaming audio output to Go channels.
	// nil = no channel output.
	Channel *ChannelSink

	// AudioStream, if non-nil, receives raw audio (converted/merged by ffmpeg).
	// This is mutually exclusive with Screen (ffmpeg can't stream MP4).
	AudioStream io.Writer

	// FFmpegLog, if non-nil, receives all ffmpeg stderr output.
	// Defaults to os.Stderr when nil.
	FFmpegLog io.Writer
}

// Recording is returned by Stop.
type Recording struct {
	Files    []string
	Duration time.Duration
	// Speaker and Mic are the devices that were actually used.
	Speaker *Device
	Mic     *Device
}

// Recorder captures audio (and optionally screen) and encodes via ffmpeg.
//
// Typical usage:
//
//	r, err := New(opts)
//	r.Start()
//	// ... wait ...
//	result, err := r.Stop()
type Recorder struct {
	opts Options

	// resolved at Start time
	speaker *Device
	mic     *Device

	speakerFrames atomic.Uint64
	micFrames     atomic.Uint64
	startedAt     time.Time

	quit  chan struct{}
	wg    sync.WaitGroup
	procs []*ffmpegProc
	files []string

	mu     sync.Mutex
	active bool
}

type ffmpegProc struct {
	pipe      io.WriteCloser
	stdinPipe io.WriteCloser
	cmd       *exec.Cmd
}

// ffmpegExe returns the path to the ffmpeg executable.
// Priority:
//  1. Bundled ffmpeg.exe next to the running executable (production)
//  2. "ffmpeg" from PATH (development)
func ffmpegExe() string {
	if exePath, err := os.Executable(); err == nil {
		bundled := filepath.Join(filepath.Dir(exePath), "ffmpeg.exe")
		if _, err := os.Stat(bundled); err == nil {
			return bundled
		}
	}
	return "ffmpeg"
}

// New validates options and returns a Recorder ready to Start.
// No WASAPI sessions are opened yet.
func New(opts Options) (*Recorder, error) {
	if opts.File == nil && opts.Channel == nil && opts.AudioStream == nil {
		return nil, fmt.Errorf("at least one of File, Channel or AudioStream sink must be configured")
	}
	if opts.File != nil || opts.AudioStream != nil {
		if _, err := exec.LookPath(ffmpegExe()); err != nil {
			return nil, fmt.Errorf("ffmpeg not found — install from https://ffmpeg.org/download.html or bundle ffmpeg.exe next to the application")
		}
	}
	if opts.File != nil {
		// Copy FileSink so we can set defaults without mutating the caller's struct.
		fc := *opts.File
		if fc.Dir == "" {
			fc.Dir = "."
		}
		if fc.Bitrate == "" {
			fc.Bitrate = "192k"
		}
		if fc.VideoPreset == "" {
			fc.VideoPreset = "ultrafast"
		}
		if fc.VideoQuality == 0 {
			fc.VideoQuality = 23
		}
		opts.File = &fc
	}
	if opts.Framerate <= 0 {
		opts.Framerate = 30
	}
	return &Recorder{opts: opts}, nil
}

// Start opens WASAPI sessions, launches ffmpeg (if a FileSink is configured),
// and begins recording. Returns immediately; capture runs in background
// goroutines. Call Stop to end the recording.
func (r *Recorder) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active {
		return fmt.Errorf("recorder is already running")
	}

	err := com.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	if err != nil {
		// Ignore S_FALSE (0x00000001), which just means COM is already initialized
		if err.Error() != "com::CoInitializeEx failed with code: 0x00000001" {
			return fmt.Errorf("CoInitializeEx: %w", err)
		}
	}

	clsid := mmdevice.CLSID_MMDeviceEnumerator()
	iid := mmdevice.IID_IMMDeviceEnumerator()
	v, err := com.CoCreateInstance(&clsid, nil, clsctxAll, &iid)
	if err != nil {
		com.CoUninitialize()
		return fmt.Errorf("CoCreateInstance: %w", err)
	}
	enum := mmdevice.ToType[mmdevice.IMMDeviceEnumerator](v)
	defer enum.Release()

	if err = r.resolveDevices(enum); err != nil {
		com.CoUninitialize()
		return err
	}

	ts := time.Now().Format("2006-01-02_15-04-05")
	r.quit = make(chan struct{})
	r.procs = nil
	r.files = nil
	r.speakerFrames.Store(0)
	r.micFrames.Store(0)
	r.startedAt = time.Now()

	switch r.opts.AudioMode {
	case AudioModeSeparate:
		err = r.startSeparate(enum, ts)
	case AudioModeMerged, AudioModeMergedMono:
		err = r.startMerged(enum, ts)
	case AudioModeSpeakerOnly:
		err = r.startSpeakerOnly(enum, ts)
	case AudioModeMicOnly:
		err = r.startMicOnly(enum, ts)
	}

	if err != nil {
		r.stopFFmpeg()
		com.CoUninitialize()
		return err
	}

	r.active = true
	return nil
}

// Stop ends the recording, finalises all output files and returns the result.
func (r *Recorder) Stop() (*Recording, error) {
	r.mu.Lock()
	if !r.active {
		r.mu.Unlock()
		return nil, fmt.Errorf("recorder is not running")
	}
	r.active = false
	r.mu.Unlock()

	close(r.quit)
	
	// Close pipes and tell ffmpeg to stop so any blocked writes in capture goroutines are interrupted
	r.stopFFmpeg()
	
	r.wg.Wait()

	com.CoUninitialize()

	return &Recording{
		Files:    r.files,
		Duration: time.Since(r.startedAt),
		Speaker:  r.speaker,
		Mic:      r.mic,
	}, nil
}

// SpeakerFrames returns the number of speaker frames captured so far.
func (r *Recorder) SpeakerFrames() uint64 { return r.speakerFrames.Load() }

// MicFrames returns the number of mic frames captured so far.
func (r *Recorder) MicFrames() uint64 { return r.micFrames.Load() }

// --- mode-specific start routines -------------------------------------------

// startSeparate records speaker and mic independently.
// With a FileSink:
//   - speaker → MP3 (audio-only) or MP4 (with screen video)
//   - mic     → MP3 (audio-only, no screen track)
//
// With a ChannelSink: chunks are sent to Channel.Speaker and Channel.Mic.
func (r *Recorder) startSeparate(enum *mmdevice.IMMDeviceEnumerator, ts string) error {
	spkClient, spkFmt, err := r.activateDevice(enum, r.speaker, true)
	if err != nil {
		return fmt.Errorf("speaker activate: %w", err)
	}
	micClient, micFmt, err := r.activateDevice(enum, r.mic, false)
	if err != nil {
		spkClient.Release()
		return fmt.Errorf("mic activate: %w", err)
	}
	spkCapture, err := captureClientFor(spkClient)
	if err != nil {
		spkClient.Release()
		micClient.Release()
		return err
	}
	micCapture, err := captureClientFor(micClient)
	if err != nil {
		spkClient.Release()
		micClient.Release()
		spkCapture.Release()
		return err
	}

	// Build writers ------------------------------------------------------------
	var spkPipe, micPipe io.Writer
	if r.opts.File != nil {
		ext := "mp3"
		if r.opts.Screen != nil {
			ext = "mp4"
		}
		spkFile := filepath.Join(r.opts.File.Dir, fmt.Sprintf("speaker_%s.%s", ts, ext))
		micFile := filepath.Join(r.opts.File.Dir, fmt.Sprintf("mic_%s.mp3", ts))
		r.files = append(r.files, spkFile, micFile)

		spkProc := r.launchAudioFFmpeg(spkFmt, r.opts.Screen, spkFile, nil)
		micProc := r.launchAudioFFmpeg(micFmt, nil, micFile, nil) // mic never gets screen track
		r.procs = append(r.procs, spkProc, micProc)
		spkPipe = spkProc.pipe
		micPipe = micProc.pipe
	}

	var spkChanCh, micChanCh chan<- StreamChunk
	if r.opts.Channel != nil {
		spkChanCh = r.opts.Channel.Speaker
		micChanCh = r.opts.Channel.Mic
	}

	spkOut := buildWriter(spkPipe, spkChanCh, spkFmt.Format.SamplesPerSec, spkFmt.Format.Channels, SourceSpeaker)
	micOut := buildWriter(micPipe, micChanCh, micFmt.Format.SamplesPerSec, micFmt.Format.Channels, SourceMic)

	// Launch goroutines -------------------------------------------------------
	if err = spkClient.Start(); err != nil {
		return fmt.Errorf("speaker Start: %w", err)
	}
	if err = micClient.Start(); err != nil {
		return fmt.Errorf("mic Start: %w", err)
	}
	spkBuf, _ := spkClient.GetBufferSize()
	micBuf, _ := micClient.GetBufferSize()

	r.wg.Add(2)
	go func() {
		defer r.wg.Done()
		defer spkCapture.Release()
		defer spkClient.Release()
		defer spkClient.Stop()
		captureRaw("speaker", spkCapture, spkFmt, spkBuf, spkOut, &r.speakerFrames, r.quit)
	}()
	go func() {
		defer r.wg.Done()
		defer micCapture.Release()
		defer micClient.Release()
		defer micClient.Stop()
		captureRaw("mic", micCapture, micFmt, micBuf, micOut, &r.micFrames, r.quit)
	}()
	return nil
}

// startMerged records speaker and mic into a merged stream (stereo or mono).
func (r *Recorder) startMerged(enum *mmdevice.IMMDeviceEnumerator, ts string) error {
	spkClient, spkFmt, err := r.activateDevice(enum, r.speaker, true)
	if err != nil {
		return fmt.Errorf("speaker activate: %w", err)
	}
	micClient, micFmt, err := r.activateDevice(enum, r.mic, false)
	if err != nil {
		spkClient.Release()
		return fmt.Errorf("mic activate: %w", err)
	}
	spkCapture, err := captureClientFor(spkClient)
	if err != nil {
		spkClient.Release()
		micClient.Release()
		return err
	}
	micCapture, err := captureClientFor(micClient)
	if err != nil {
		spkClient.Release()
		micClient.Release()
		spkCapture.Release()
		return err
	}

	isMono := r.opts.AudioMode == AudioModeMergedMono
	var outChannels uint16 = 2
	if isMono {
		outChannels = 1
	}

	mergedFmt := spkFmt
	mergedFmt.Format.Channels = outChannels
	mergedFmt.Format.BlockAlign = outChannels * 4
	mergedFmt.Format.AvgBytesPerSec = spkFmt.Format.SamplesPerSec * uint32(outChannels) * 4

	// Build writers ------------------------------------------------------------
	var filePipe io.Writer
	if r.opts.File != nil {
		ext := "mp3"
		if r.opts.Screen != nil {
			ext = "mp4"
		}
		outFile := filepath.Join(r.opts.File.Dir, fmt.Sprintf("merged_%s.%s", ts, ext))
		r.files = append(r.files, outFile)
		proc := r.launchAudioFFmpeg(mergedFmt, r.opts.Screen, outFile, nil, isMono)
		r.procs = append(r.procs, proc)
		filePipe = proc.pipe
	}

	if r.opts.AudioStream != nil {
		proc := r.launchAudioFFmpeg(mergedFmt, nil, "", r.opts.AudioStream, isMono)
		r.procs = append(r.procs, proc)
		if filePipe != nil {
			filePipe = io.MultiWriter(filePipe, proc.pipe)
		} else {
			filePipe = proc.pipe
		}
	}

	var mergedCh chan<- StreamChunk
	if r.opts.Channel != nil {
		mergedCh = r.opts.Channel.Merged
	}
	mixOut := buildWriter(filePipe, mergedCh, mergedFmt.Format.SamplesPerSec, outChannels, SourceMerged)

	// Frame queues for the mixer goroutine.
	spkCh := make(chan []float32, 32)
	micCh := make(chan []float32, 32)

	if err = spkClient.Start(); err != nil {
		return fmt.Errorf("speaker Start: %w", err)
	}
	if err = micClient.Start(); err != nil {
		return fmt.Errorf("mic Start: %w", err)
	}
	spkBuf, _ := spkClient.GetBufferSize()
	micBuf, _ := micClient.GetBufferSize()

	r.wg.Add(3)
	go func() {
		defer r.wg.Done()
		defer close(spkCh)
		defer spkCapture.Release()
		defer spkClient.Release()
		defer spkClient.Stop()
		captureMono("speaker", spkCapture, spkFmt, spkBuf, spkCh, &r.speakerFrames, r.quit)
	}()
	go func() {
		defer r.wg.Done()
		defer close(micCh)
		defer micCapture.Release()
		defer micClient.Release()
		defer micClient.Stop()
		captureMono("mic", micCapture, micFmt, micBuf, micCh, &r.micFrames, r.quit)
	}()
	go func() {
		defer r.wg.Done()
		mixAudio(spkCh, micCh, mixOut, isMono)
	}()
	return nil
}

func (r *Recorder) startSpeakerOnly(enum *mmdevice.IMMDeviceEnumerator, ts string) error {
	client, wfmt, err := r.activateDevice(enum, r.speaker, true)
	if err != nil {
		return fmt.Errorf("speaker activate: %w", err)
	}
	cc, err := captureClientFor(client)
	if err != nil {
		client.Release()
		return err
	}

	var filePipe io.Writer
	if r.opts.File != nil {
		ext := "mp3"
		if r.opts.Screen != nil {
			ext = "mp4"
		}
		outFile := filepath.Join(r.opts.File.Dir, fmt.Sprintf("speaker_%s.%s", ts, ext))
		r.files = append(r.files, outFile)
		proc := r.launchAudioFFmpeg(wfmt, r.opts.Screen, outFile, nil)
		r.procs = append(r.procs, proc)
		filePipe = proc.pipe
	}

	if r.opts.AudioStream != nil {
		proc := r.launchAudioFFmpeg(wfmt, nil, "", r.opts.AudioStream)
		r.procs = append(r.procs, proc)
		if filePipe != nil {
			filePipe = io.MultiWriter(filePipe, proc.pipe)
		} else {
			filePipe = proc.pipe
		}
	}

	var spkCh chan<- StreamChunk
	if r.opts.Channel != nil {
		spkCh = r.opts.Channel.Speaker
	}
	out := buildWriter(filePipe, spkCh, wfmt.Format.SamplesPerSec, wfmt.Format.Channels, SourceSpeaker)

	if err = client.Start(); err != nil {
		return fmt.Errorf("speaker Start: %w", err)
	}
	buf, _ := client.GetBufferSize()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer cc.Release()
		defer client.Release()
		defer client.Stop()
		captureRaw("speaker", cc, wfmt, buf, out, &r.speakerFrames, r.quit)
	}()
	return nil
}

func (r *Recorder) startMicOnly(enum *mmdevice.IMMDeviceEnumerator, ts string) error {
	client, wfmt, err := r.activateDevice(enum, r.mic, false)
	if err != nil {
		return fmt.Errorf("mic activate: %w", err)
	}
	cc, err := captureClientFor(client)
	if err != nil {
		client.Release()
		return err
	}

	var filePipe io.Writer
	if r.opts.File != nil {
		ext := "mp3"
		if r.opts.Screen != nil {
			ext = "mp4"
		}
		outFile := filepath.Join(r.opts.File.Dir, fmt.Sprintf("mic_%s.%s", ts, ext))
		r.files = append(r.files, outFile)
		proc := r.launchAudioFFmpeg(wfmt, r.opts.Screen, outFile, nil)
		r.procs = append(r.procs, proc)
		filePipe = proc.pipe
	}

	if r.opts.AudioStream != nil {
		proc := r.launchAudioFFmpeg(wfmt, nil, "", r.opts.AudioStream)
		r.procs = append(r.procs, proc)
		if filePipe != nil {
			filePipe = io.MultiWriter(filePipe, proc.pipe)
		} else {
			filePipe = proc.pipe
		}
	}

	var micCh chan<- StreamChunk
	if r.opts.Channel != nil {
		micCh = r.opts.Channel.Mic
	}
	out := buildWriter(filePipe, micCh, wfmt.Format.SamplesPerSec, wfmt.Format.Channels, SourceMic)

	if err = client.Start(); err != nil {
		return fmt.Errorf("mic Start: %w", err)
	}
	buf, _ := client.GetBufferSize()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer cc.Release()
		defer client.Release()
		defer client.Stop()
		captureRaw("mic", cc, wfmt, buf, out, &r.micFrames, r.quit)
	}()
	return nil
}

// --- WASAPI helpers ----------------------------------------------------------

func (r *Recorder) resolveDevices(enum *mmdevice.IMMDeviceEnumerator) error {
	needsSpeaker := r.opts.AudioMode != AudioModeMicOnly
	needsMic := r.opts.AudioMode != AudioModeSpeakerOnly

	if needsSpeaker && r.opts.Speaker == nil {
		dev, err := enum.GetDefaultAudioEndpoint(mmdevice.ERender, mmdevice.EConsole)
		if err != nil {
			return fmt.Errorf("default speaker: %w", err)
		}
		d, err := deviceFromRaw(dev, FlowSpeaker, true)
		dev.Release()
		if err != nil {
			return err
		}
		r.speaker = &d
	} else {
		r.speaker = r.opts.Speaker
	}

	if needsMic && r.opts.Mic == nil {
		dev, err := enum.GetDefaultAudioEndpoint(mmdevice.ECapture, mmdevice.EConsole)
		if err != nil {
			return fmt.Errorf("default mic: %w", err)
		}
		d, err := deviceFromRaw(dev, FlowMic, true)
		dev.Release()
		if err != nil {
			return err
		}
		r.mic = &d
	} else {
		r.mic = r.opts.Mic
	}
	return nil
}

func (r *Recorder) activateDevice(enum *mmdevice.IMMDeviceEnumerator, device *Device, loopback bool) (
	*audioclient.IAudioClient, audioclient.WAVEFORMATEXTENSIBLE, error,
) {
	var zero audioclient.WAVEFORMATEXTENSIBLE

	dataFlow := mmdevice.ERender
	if !loopback {
		dataFlow = mmdevice.ECapture
	}
	col, err := enum.EnumAudioEndpoints(dataFlow, mmdevice.DEVICE_STATE_ACTIVE)
	if err != nil {
		return nil, zero, fmt.Errorf("EnumAudioEndpoints: %w", err)
	}
	defer col.Release()

	cnt, _ := col.GetCount()
	var raw *mmdevice.IMMDevice
	for i := uint32(0); i < uint32(cnt); i++ {
		d, err := col.Item(i)
		if err != nil {
			continue
		}
		id, _ := d.GetId()
		if id == device.ID {
			raw = d
			break
		}
		d.Release()
	}
	if raw == nil {
		return nil, zero, fmt.Errorf("device not found: %s", device.ID)
	}
	defer raw.Release()

	iidAC := audioclient.IID_IAudioClient()
	v, err := raw.Activate(iidAC, clsctxAll, nil)
	if err != nil {
		return nil, zero, fmt.Errorf("Activate: %w", err)
	}
	client := audioclient.ToType[audioclient.IAudioClient](v)

	format, err := client.GetMixFormat()
	if err != nil {
		client.Release()
		return nil, zero, fmt.Errorf("GetMixFormat: %w", err)
	}

	// Force 48000 Hz to ensure mic and speaker sync perfectly
	if format.Format.SamplesPerSec != 48000 {
		format.Format.SamplesPerSec = 48000
		format.Format.AvgBytesPerSec = 48000 * uint32(format.Format.Channels) * uint32(format.Format.BitsPerSample/8)
	}

	var flags uint32
	if loopback {
		flags = audioclient.AUDCLNT_STREAMFLAGS_LOOPBACK
	}
	// 0x80000000 = AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM
	// 0x08000000 = AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY
	if err = client.Initialize(
		audioclient.AUDCLNT_SHAREMODE_SHARED,
		flags|0x80000000|0x08000000,
		reftimesPerSec, 0, &format, nil,
	); err != nil {
		client.Release()
		return nil, zero, fmt.Errorf("Initialize: %w", err)
	}
	return client, format, nil
}

func captureClientFor(client *audioclient.IAudioClient) (*audioclient.IAudioCaptureClient, error) {
	iid := audioclient.IID_IAudioCaptureClient()
	v, err := client.GetService(&iid)
	if err != nil {
		return nil, fmt.Errorf("GetService(IAudioCaptureClient): %w", err)
	}
	return audioclient.ToType[audioclient.IAudioCaptureClient](v), nil
}

// --- ffmpeg helpers ----------------------------------------------------------

// launchAudioFFmpeg launches ffmpeg to encode audio (and optionally add a screen
// capture video track) to output.
//
// When screen is nil, it produces an MP3 (audio-only).
// When screen is non-nil, it produces an MP4 with a gdigrab video track and the
// piped audio as the audio track.  ffmpeg stops when the audio pipe is closed
// (-shortest flag).
func (r *Recorder) launchAudioFFmpeg(
	audioFmt audioclient.WAVEFORMATEXTENSIBLE,
	screen *Screen,
	output string,
	stream io.Writer,
	isMono ...bool,
) *ffmpegProc {
	var args []string
	args = append(args, "-y")

	if screen != nil {
		// Video input: gdigrab desktop (or a specific monitor region).
		args = append(args, "-f", "gdigrab",
			"-framerate", fmt.Sprintf("%d", r.opts.Framerate))
		if screen.Width > 0 && screen.Height > 0 {
			args = append(args,
				"-offset_x", fmt.Sprintf("%d", screen.X),
				"-offset_y", fmt.Sprintf("%d", screen.Y),
				"-video_size", fmt.Sprintf("%dx%d", screen.Width, screen.Height),
			)
		}
		args = append(args, "-i", "desktop")
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(fmt.Sprintf("tcp listen failed: %v", err))
	}
	addr := l.Addr().String()

	// Audio input from TCP socket.
	args = append(args,
		"-f", "f32le",
		"-ar", fmt.Sprintf("%d", audioFmt.Format.SamplesPerSec),
		"-ac", fmt.Sprintf("%d", audioFmt.Format.Channels),
		"-i", "tcp://"+addr,
	)

	if stream != nil {
		// Streaming audio output: raw s16le, 16kHz
		outChannels := "2"
		if len(isMono) > 0 && isMono[0] {
			outChannels = "1"
		}
		args = append(args,
			"-ar", "16000",
			"-ac", outChannels,
			"-f", "s16le",
			"pipe:1",
		)
	} else if screen != nil {
		sink := r.opts.File
		preset := sink.VideoPreset
		quality := sink.VideoQuality
		args = append(args,
			"-map", "0:v", "-map", "1:a",
			"-c:v", "libx264", "-preset", preset,
			"-crf", fmt.Sprintf("%d", quality),
			"-pix_fmt", "yuv420p", // Ensure compatibility with standard players
			"-c:a", "aac", "-b:a", sink.Bitrate,
			// Force a keyframe every second so fragment boundaries are
			// frequent and each flush covers at most ~1 s of video.
			"-g", fmt.Sprintf("%d", r.opts.Framerate),
			"-keyint_min", fmt.Sprintf("%d", r.opts.Framerate),
			// frag_keyframe: new fragment at every keyframe (decodable on its own).
			// empty_moov:    write a valid moov atom at file-open time.
			// default_base_moof: improves compatibility with strict players.
			"-movflags", "frag_keyframe+empty_moov+default_base_moof",
			// Flush each completed fragment to the muxer immediately so
			// it reaches disk even if ffmpeg is killed shortly after.
			"-fflags", "+flush_packets",
			"-shortest",
			output,
		)
	} else {
		args = append(args,
			"-b:a", r.opts.File.Bitrate,
			output,
		)
	}

	cmd := exec.Command(ffmpegExe(), args...)
	if r.opts.FFmpegLog != nil {
		cmd.Stderr = r.opts.FFmpegLog
	} else {
		cmd.Stderr = os.Stderr
	}
	if stream != nil {
		cmd.Stdout = stream
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(fmt.Sprintf("StdinPipe: %v", err))
	}
	if err = cmd.Start(); err != nil {
		panic(fmt.Sprintf("ffmpeg start: %v", err))
	}

	pr, pw := io.Pipe()
	go func() {
		defer l.Close()
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, pr)
	}()

	return &ffmpegProc{
		pipe:      pw,
		stdinPipe: stdin,
		cmd:       cmd,
	}
}

func (r *Recorder) stopFFmpeg() {
	for _, p := range r.procs {
		// Close audio pipe to signal EOF to the TCP listener
		p.pipe.Close()

		// Send 'q' to gracefully stop ffmpeg and finalise the MP4
		if p.stdinPipe != nil {
			_, _ = p.stdinPipe.Write([]byte("q\n"))
			p.stdinPipe.Close()
		}

		// Wait for ffmpeg to finalise, but don't block forever.
		// gdigrab (live video) can delay the shutdown by up to a frame
		// interval; if ffmpeg still hasn't exited after 8 s, force-kill it.
		// With frag_keyframe+empty_moov the fragments already flushed
		// remain playable even after a forced kill.
		done := make(chan struct{})
		go func(cmd *exec.Cmd) {
			cmd.Wait()
			close(done)
		}(p.cmd)
		select {
		case <-done:
		case <-time.After(8 * time.Second):
			p.cmd.Process.Kill()
			<-done
		}
	}
	r.procs = nil
}

// --- capture goroutines ------------------------------------------------------

// captureRaw polls WASAPI and writes raw float32 LE frames to out.
func captureRaw(
	label string,
	client *audioclient.IAudioCaptureClient,
	format audioclient.WAVEFORMATEXTENSIBLE,
	bufFrames uint32,
	out io.Writer,
	counter *atomic.Uint64,
	quit <-chan struct{},
) {
	poll := pollInterval(bufFrames, format)
	blockAlign := int(format.Format.BlockAlign)

	for {
		select {
		case <-quit:
			return
		case <-time.After(poll):
		}

		for {
			packetLen, err := client.GetNextPacketSize()
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%s] GetNextPacketSize: %v\n", label, err)
				return
			}
			if packetLen == 0 {
				break
			}

			data, numFrames, flags, _, _, err := client.GetBuffer()
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%s] GetBuffer: %v\n", label, err)
				return
			}

			frameBytes := int(numFrames) * blockAlign
			if flags&audioclient.AUDCLNT_BUFFERFLAGS_SILENT != 0 {
				out.Write(make([]byte, frameBytes))
			} else {
				// Library bug: GetBuffer slices to numFrames bytes (frame count)
				// instead of numFrames*blockAlign bytes. Re-slice correctly.
				out.Write(unsafe.Slice(&data[0], frameBytes))
			}
			counter.Add(uint64(numFrames))

			if err = client.ReleaseBuffer(numFrames); err != nil {
				fmt.Fprintf(os.Stderr, "[%s] ReleaseBuffer: %v\n", label, err)
				return
			}
		}
	}
}

// captureMono polls WASAPI, downmixes each packet to mono float32 and sends to ch.
func captureMono(
	label string,
	client *audioclient.IAudioCaptureClient,
	format audioclient.WAVEFORMATEXTENSIBLE,
	bufFrames uint32,
	ch chan<- []float32,
	counter *atomic.Uint64,
	quit <-chan struct{},
) {
	poll := pollInterval(bufFrames, format)
	blockAlign := int(format.Format.BlockAlign)
	channels := int(format.Format.Channels)

	for {
		select {
		case <-quit:
			return
		case <-time.After(poll):
		}

		for {
			packetLen, err := client.GetNextPacketSize()
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%s] GetNextPacketSize error: %v\n", label, err)
				return
			}
			if packetLen == 0 {
				break
			}

			data, numFrames, flags, _, _, err := client.GetBuffer()
			if err != nil {
				fmt.Fprintf(os.Stderr, "[%s] GetBuffer error: %v\n", label, err)
				return
			}

			frameBytes := int(numFrames) * blockAlign
			mono := make([]float32, numFrames)

			if flags&audioclient.AUDCLNT_BUFFERFLAGS_SILENT == 0 {
				src := unsafe.Slice(&data[0], frameBytes)
				for i := uint32(0); i < numFrames; i++ {
					var sum float32
					for c := 0; c < channels; c++ {
						off := (int(i)*channels + c) * 4
						sum += math.Float32frombits(binary.LittleEndian.Uint32(src[off : off+4]))
					}
					mono[i] = sum / float32(channels)
				}
			}

			counter.Add(uint64(numFrames))
			client.ReleaseBuffer(numFrames)

			select {
			case ch <- mono:
			case <-quit:
				return
			}
		}
	}
}

// mixAudio interleaves mono frames from spkCh and micCh as 2-ch f32le,
// or averages them into a 1-ch f32le if isMono is true.
func mixAudio(spkCh, micCh <-chan []float32, out io.Writer, isMono bool) {
	var buf []byte
	if isMono {
		buf = make([]byte, 4) // 1 × float32
	} else {
		buf = make([]byte, 8) // 2 × float32
	}
	var spkBuf []float32

	for {
		mic, micOk := <-micCh
		if !micOk {
			return
		}

		// Pull all available speaker frames into our buffer without blocking
	drainLoop:
		for {
			select {
			case s, ok := <-spkCh:
				if ok {
					spkBuf = append(spkBuf, s...)
				} else {
					return // Speaker channel closed
				}
			default:
				break drainLoop
			}
		}

		// Output loop: we use len(mic) as the true frame count
		for i := 0; i < len(mic); i++ {
			var spkSample float32 = 0
			if len(spkBuf) > 0 {
				spkSample = spkBuf[0]
				spkBuf = spkBuf[1:]
			}

			if isMono {
				// Mix to single channel (average)
				mixed := (spkSample + mic[i]) / 2.0
				binary.LittleEndian.PutUint32(buf[0:4], math.Float32bits(mixed))
			} else {
				// Interleave for stereo
				binary.LittleEndian.PutUint32(buf[0:4], math.Float32bits(spkSample))
				binary.LittleEndian.PutUint32(buf[4:8], math.Float32bits(mic[i]))
			}
			out.Write(buf)
		}

		// Prevent unbounded growth of speaker buffer if it's producing faster than mic
		// (Should not happen since we forced both to 48000Hz, but just in case)
		if len(spkBuf) > 48000 {
			// keep the last 48000 samples
			spkBuf = spkBuf[len(spkBuf)-48000:]
		}
	}
}

// --- misc helpers ------------------------------------------------------------

func pollInterval(bufFrames uint32, format audioclient.WAVEFORMATEXTENSIBLE) time.Duration {
	ms := uint64(bufFrames) * 1000 / uint64(format.Format.SamplesPerSec) / 2
	if ms < 1 {
		ms = 1
	}
	return time.Duration(ms) * time.Millisecond
}
