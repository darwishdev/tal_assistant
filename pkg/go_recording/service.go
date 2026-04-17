//go:build windows

package go_recording

import (
	"context"
	"fmt"
	"io"
)

// ScreenSource mirrors the user's request for screen metadata.
type ScreenSource struct {
	Index      int    `json:"Index"`
	ID         string `json:"ID"`
	Name       string `json:"Name"`
	OffsetX    int    `json:"OffsetX"`
	OffsetY    int    `json:"OffsetY"`
	Width      int    `json:"Width"`
	Height     int    `json:"Height"`
	Screenshot string `json:"Screenshot"`
}

// AudioSource mirrors the user's request for audio device metadata.
type AudioSource struct {
	ID         string // Device ID used by WASAPI
	Name       string // Human-readable name
	Type       string // "mic", "speaker"
	IsDefault  bool   // Whether it is the default device
	IsPersonal bool   // True for headphones/headsets, false for room speakers
}

// RecordingService provides a clean entry point for recording tasks,
// wrapping the underlying WASAPI Recorder and Screen capture logic.
type RecordingService struct {
	audioRecorder  *Recorder // STT audio stream
	screenRecorder *Recorder // screen + audio file
	ffmpegLog      io.Writer
}

func NewRecordingService() *RecordingService {
	return &RecordingService{}
}

// SetFFmpegLog directs all ffmpeg stderr output to w instead of os.Stderr.
// Must be called before Start or StartScreenRecording.
func (s *RecordingService) SetFFmpegLog(w io.Writer) {
	s.ffmpegLog = w
}

// AudioDeviceList returns all active speakers and microphones.
func (s *RecordingService) AudioDeviceList(ctx context.Context) ([]AudioSource, error) {
	devices, err := ListDevices()
	if err != nil {
		return nil, err
	}

	var sources []AudioSource
	for _, d := range devices {
		sources = append(sources, AudioSource{
			ID:         d.ID,
			Name:       d.Name,
			Type:       string(d.Flow),
			IsDefault:  d.IsDefault,
			IsPersonal: d.IsPersonal,
		})
	}
	return sources, nil
}

// ScreenDeviceList returns all active monitors with screenshots.
func (s *RecordingService) ScreenDeviceList(ctx context.Context) ([]ScreenSource, error) {
	screens, err := ListScreens()
	if err != nil {
		return nil, err
	}

	var sources []ScreenSource
	for _, sc := range screens {
		sources = append(sources, ScreenSource{
			Index:      sc.Index,
			ID:         sc.Name,
			Name:       fmt.Sprintf("Display %d", sc.Index+1),
			OffsetX:    int(sc.X),
			OffsetY:    int(sc.Y),
			Width:      int(sc.Width),
			Height:     int(sc.Height),
			Screenshot: sc.Screenshot,
		})
	}
	return sources, nil
}

// Start launches a WASAPI recording that merges mic and speaker into a stereo
// or mono s16le 16kHz stream on stdout, suitable for STT.
// Returns the audio stream and the number of channels (1 or 2).
func (s *RecordingService) Start(micID, speakerID string) (io.ReadCloser, int, error) {
	if s.audioRecorder != nil {
		return nil, 0, fmt.Errorf("already recording")
	}

	// Resolve devices
	mics, _ := ListMics()
	var mic *Device
	for _, d := range mics {
		if d.ID == micID {
			copied := d
			mic = &copied
			break
		}
	}

	speakers, _ := ListSpeakers()
	var speaker *Device
	for _, d := range speakers {
		if d.ID == speakerID {
			copied := d
			speaker = &copied
			break
		}
	}

	mode := AudioModeMerged
	channels := 2
	if speaker != nil && !speaker.IsPersonal {
		mode = AudioModeMergedMono
		channels = 1
	}

	pr, pw := io.Pipe()

	opts := Options{
		Mic:         mic,
		Speaker:     speaker,
		AudioMode:   mode,
		AudioStream: pw,
		FFmpegLog:   s.ffmpegLog,
	}

	r, err := New(opts)
	if err != nil {
		pw.Close()
		pr.Close()
		return nil, 0, err
	}

	if err := r.Start(); err != nil {
		pw.Close()
		pr.Close()
		return nil, 0, err
	}

	s.audioRecorder = r

	// Wrap pr to close recorder on close
	return &recorderReadCloser{
		ReadCloser: pr,
		recorder:   r,
		pw:         pw,
		service:    s,
	}, channels, nil
}

type recorderReadCloser struct {
	io.ReadCloser
	recorder *Recorder
	pw       *io.PipeWriter
	service  *RecordingService
}

func (r *recorderReadCloser) Close() error {
	r.pw.Close()
	_, err := r.recorder.Stop()
	r.service.audioRecorder = nil
	return err
}

// StartScreenRecording starts a screen capture using WASAPI audio and gdigrab video.
func (s *RecordingService) StartScreenRecording(
	screen *ScreenSource,
	micID, speakerID string,
	outputDir string,
) (string, error) {
	if s.screenRecorder != nil {
		return "", fmt.Errorf("screen recording already running")
	}

	// Resolve devices
	mics, _ := ListMics()
	var mic *Device
	for _, d := range mics {
		if d.ID == micID {
			copied := d
			mic = &copied
			break
		}
	}

	speakers, _ := ListSpeakers()
	var speaker *Device
	for _, d := range speakers {
		if d.ID == speakerID {
			copied := d
			speaker = &copied
			break
		}
	}
	
	var sc *Screen
	if screen != nil {
		sc = &Screen{
			Name:   screen.ID,
			X:      int32(screen.OffsetX),
			Y:      int32(screen.OffsetY),
			Width:  int32(screen.Width),
			Height: int32(screen.Height),
		}
	}

	mode := AudioModeMerged
	if speaker != nil && !speaker.IsPersonal {
		mode = AudioModeMergedMono
	}

	opts := Options{
		Mic:       mic,
		Speaker:   speaker,
		AudioMode: mode,
		Screen:    sc,
		FFmpegLog: s.ffmpegLog,
		File: &FileSink{
			Dir: outputDir,
		},
	}

	r, err := New(opts)
	if err != nil {
		return "", err
	}

	if err := r.Start(); err != nil {
		return "", err
	}

	s.screenRecorder = r

	// Let's assume there's one file.
	if len(r.files) > 0 {
		return r.files[0], nil
	}

	return "", fmt.Errorf("no file created")
}

func (s *RecordingService) StopScreenRecording() {
	if s.screenRecorder == nil {
		return
	}
	_, _ = s.screenRecorder.Stop()
	s.screenRecorder = nil
}

func (s *RecordingService) Stop() {
	if s.audioRecorder == nil {
		return
	}
	_, _ = s.audioRecorder.Stop()
	s.audioRecorder = nil
}
