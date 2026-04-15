//go:build windows

package go_recording

import (
	"encoding/binary"
	"io"
	"math"
	"time"
)

// AudioMode controls which audio sources are captured .
type AudioMode int

const (
	// AudioModeSeparate captures speaker and mic to separate outputs.
	AudioModeSeparate AudioMode = iota

	// AudioModeMerged captures speaker (left) and mic (right) into one stereo stream.
	// Only safe when the active speaker has IsPersonal == true; otherwise the mic
	// will pick up room audio from the speaker and both channels will overlap.
	AudioModeMerged

	// AudioModeMergedMono mixes speaker and mic into a single mono stream.
	// Use this when the speaker is a loud device (IsPersonal == false) to avoid
	// channel overlap issues and let downstream STT handle diarization.
	AudioModeMergedMono

	// AudioModeSpeakerOnly captures only the speaker loopback.
	AudioModeSpeakerOnly

	// AudioModeMicOnly captures only the microphone.
	AudioModeMicOnly
)

func (m AudioMode) String() string {
	switch m {
	case AudioModeSeparate:
		return "separate"
	case AudioModeMerged:
		return "merged"
	case AudioModeSpeakerOnly:
		return "speaker-only"
	case AudioModeMicOnly:
		return "mic-only"
	default:
		return "unknown"
	}
}

// Source identifies which capture source produced a StreamChunk.
type Source int

const (
	SourceSpeaker Source = iota
	SourceMic
	SourceMerged // interleaved stereo (L=speaker, R=mic)
)

// StreamChunk carries a batch of raw float32 PCM audio samples from one
// WASAPI poll cycle.
type StreamChunk struct {
	// Data holds interleaved float32 samples; len == NumFrames * int(Channels).
	Data       []float32
	NumFrames  uint32
	SampleRate uint32
	Channels   uint16
	Source     Source
	Timestamp  time.Time
}

// FileSink configures file-based recording output.
type FileSink struct {
	// Dir is the output directory. Defaults to the current directory.
	Dir string

	// Bitrate is the ffmpeg MP3 audio bitrate (e.g. "192k", "320k").
	// Defaults to "192k".
	Bitrate string

	// VideoPreset is the x264 preset used for screen recordings
	// (e.g. "ultrafast", "fast", "medium"). Defaults to "ultrafast".
	VideoPreset string

	// VideoQuality is the x264 CRF value (0–51; lower = better quality /
	// larger file). Defaults to 23.
	VideoQuality int
}

// ChannelSink receives streaming audio chunks over Go channels.
// Set only the channels you need; nil channels are skipped.
//
// For AudioModeSeparate, set both Speaker and Mic.
// For AudioModeMerged,   set Merged.
// For AudioModeSpeakerOnly, set Speaker.
// For AudioModeMicOnly,     set Mic.
type ChannelSink struct {
	Speaker chan<- StreamChunk
	Mic     chan<- StreamChunk
	Merged  chan<- StreamChunk // interleaved stereo (L=speaker, R=mic)
}

// chanWriter implements io.Writer by decoding raw f32le bytes into float32
// samples and forwarding them as StreamChunk values to a Go channel.
// Chunks are dropped (non-blocking send) when the channel is full.
type chanWriter struct {
	ch         chan<- StreamChunk
	sampleRate uint32
	channels   uint16
	source     Source
}

func (cw *chanWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n := len(p) / 4 // number of float32 values
	floats := make([]float32, n)
	for i := 0; i < n; i++ {
		floats[i] = math.Float32frombits(binary.LittleEndian.Uint32(p[i*4 : i*4+4]))
	}
	numFrames := uint32(n) / uint32(cw.channels)
	if numFrames == 0 {
		return len(p), nil
	}
	chunk := StreamChunk{
		Data:       floats,
		NumFrames:  numFrames,
		SampleRate: cw.sampleRate,
		Channels:   cw.channels,
		Source:     cw.source,
		Timestamp:  time.Now(),
	}
	select {
	case cw.ch <- chunk:
	default: // drop: channel full
	}
	return len(p), nil
}

// buildWriter returns an io.Writer that writes to filePipe and/or a chanWriter
// depending on which are non-nil.  If both are nil, returns io.Discard.
func buildWriter(
	filePipe io.Writer,
	ch chan<- StreamChunk,
	sampleRate uint32,
	channels uint16,
	source Source,
) io.Writer {
	if filePipe == nil && ch == nil {
		return io.Discard
	}
	if filePipe == nil {
		return &chanWriter{ch: ch, sampleRate: sampleRate, channels: channels, source: source}
	}
	if ch == nil {
		return filePipe
	}
	cw := &chanWriter{ch: ch, sampleRate: sampleRate, channels: channels, source: source}
	return io.MultiWriter(filePipe, cw)
}
