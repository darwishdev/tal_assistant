package ffmpeg

import (
	"fmt"
	"io"
	"os/exec"
)

// FFMPEGService captures mic + speaker and merges them into a single
// stereo PCM stream suitable for Google Speech v2 diarization.
//
// Channel layout:
//
//	Left  (ch 1) = mic      → Speaker 1 in diarization
//	Right (ch 2) = speaker  → Speaker 2 in diarization
type FFMPEGService struct {
	cmd *exec.Cmd
}

func NewFFMPEGService() *FFMPEGService {
	return &FFMPEGService{}
}

// Start launches a single ffmpeg process that:
//  1. Reads mic and speaker as two separate mono inputs
//  2. Merges them with amerge into one 2-channel (stereo) stream
//  3. Writes raw s16le PCM at 16 kHz to stdout
//
// Returns an io.ReadCloser over that stdout — pass it directly to
// stt.STTService.StreamDiarized.  Call Stop() to terminate ffmpeg.
func (f *FFMPEGService) Start(micDevice, speakerDevice string) (io.ReadCloser, error) {
	if f.cmd != nil {
		return nil, fmt.Errorf("already recording")
	}

	f.cmd = exec.Command(
		"ffmpeg",
		"-f", "dshow", "-i", "audio="+micDevice,
		"-f", "dshow", "-i", "audio="+speakerDevice,
		"-filter_complex", "[0:a]aformat=channel_layouts=mono[m];[1:a]aformat=channel_layouts=mono[s];[m][s]amerge=inputs=2[aout]",
		"-map", "[aout]",
		"-ar", "16000",
		"-ac", "2",
		"-f", "s16le",
		"pipe:1",
	)

	f.cmd.Stderr = nil

	pipe, err := f.cmd.StdoutPipe()
	if err != nil {
		f.cmd = nil
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := f.cmd.Start(); err != nil {
		f.cmd = nil
		return nil, fmt.Errorf("ffmpeg start: %w", err)
	}

	return pipe, nil
}

// Stop kills the ffmpeg process. Safe to call multiple times.
func (f *FFMPEGService) Stop() {
	if f.cmd != nil {
		f.cmd.Process.Kill()
		f.cmd.Wait() // reap the process
		f.cmd = nil
	}
}
