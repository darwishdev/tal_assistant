package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ScreenSource struct {
	ID         string `json:"ID"`
	Name       string `json:"Name"`
	OffsetX    int    `json:"OffsetX"`
	OffsetY    int    `json:"OffsetY"`
	Width      int    `json:"Width"`
	Height     int    `json:"Height"`
	Screenshot string `json:"Screenshot"`
}

// Source represents an input source for FFmpeg (microphone, speaker, or screen).
type AudioSource struct {
	ID   string // Device ID used by FFmpeg (e.g., "hw:0,0", "default", ":0.0")
	Name string // Human-readable name
	Type string // "mic", "speaker"
}

// FFMPEGService captures mic + speaker and merges them into a single
// stereo PCM stream suitable for Google Speech v2 diarization.
//
// Channel layout:
//
//	Left  (ch 1) = mic      → Speaker 1 in diarization
//	Right (ch 2) = speaker  → Speaker 2 in diarization
type FFMPEGService struct {
	cmd            *exec.Cmd
	screenCmd      *exec.Cmd
	screenCmdStdin io.WriteCloser
	ffmpegPath     string
}

// getFFmpegPath returns the path to ffmpeg executable.
// Priority:
//  1. Bundled ffmpeg.exe next to the executable (production)
//  2. System ffmpeg in PATH (development)
func getFFmpegPath() string {
	// Try bundled ffmpeg first
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		bundledFFmpeg := filepath.Join(exeDir, "ffmpeg.exe")
		if _, err := os.Stat(bundledFFmpeg); err == nil {
			log.Printf("[ffmpeg] Using bundled ffmpeg: %s", bundledFFmpeg)
			return bundledFFmpeg
		}
	}

	// Fall back to system ffmpeg
	log.Printf("[ffmpeg] Using system ffmpeg from PATH")
	return "ffmpeg"
}

func NewFFMPEGService() *FFMPEGService {
	return &FFMPEGService{
		ffmpegPath: getFFmpegPath(),
	}
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
		f.ffmpegPath,
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

// StartScreenRecording starts a screen capture using gdigrab.
// screen is a *ScreenSource — if nil, records the full desktop.
// outputDir is where the mp4 will be saved (same dir as srt/signals).
// Returns the full output video path.
func (f *FFMPEGService) StartScreenRecording(
	screen *ScreenSource,
	micDevice, speakerDevice string,
	outputDir string,
) (string, error) {
	if f.screenCmd != nil {
		return "", fmt.Errorf("screen recording already running")
	}

	ts := time.Now().Format("2006-01-02_15-04-05")
	// use filepath.Join so separators are correct on Windows
	videoPath := filepath.Join(outputDir, "recording_"+ts+".mp4")

	gdigrabArgs := []string{
		"-f", "gdigrab",
		"-framerate", "15",
	}

	if screen != nil {
		gdigrabArgs = append(gdigrabArgs,
			"-offset_x", fmt.Sprintf("%d", screen.OffsetX),
			"-offset_y", fmt.Sprintf("%d", screen.OffsetY),
			"-video_size", fmt.Sprintf("%dx%d", screen.Width, screen.Height),
		)
	}

	gdigrabArgs = append(gdigrabArgs, "-i", "desktop")

	args := []string{"-y"}
	args = append(args, gdigrabArgs...)
	args = append(args,
		"-f", "dshow", "-i", "audio="+micDevice,
		"-f", "dshow", "-i", "audio="+speakerDevice,
		"-filter_complex", "[1:a][2:a]amix=inputs=2:duration=first[aout]",
		"-map", "0:v",
		"-map", "[aout]",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-pix_fmt", "yuv420p", // ← fixes 4:4:4 compatibility issue
		"-crf", "28",
		"-c:a", "aac",
		"-b:a", "128k",
		videoPath,
	)

	cmd := exec.Command(f.ffmpegPath, args...)
	cmd.Stderr = nil

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("screen stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("screen ffmpeg start: %w", err)
	}

	f.screenCmd = cmd
	f.screenCmdStdin = stdin

	log.Printf("[screen-rec] started pid=%d output=%s", cmd.Process.Pid, videoPath)
	return videoPath, nil
}

// StopScreenRecording sends 'q' to ffmpeg so it finalises the MP4 container
// cleanly, then waits up to 5 seconds before force-killing.
func (f *FFMPEGService) StopScreenRecording() {
	if f.screenCmd == nil {
		return
	}

	if f.screenCmdStdin != nil {
		_, _ = f.screenCmdStdin.Write([]byte("q\n"))
		f.screenCmdStdin.Close()
		f.screenCmdStdin = nil
	}

	done := make(chan error, 1)
	go func() { done <- f.screenCmd.Wait() }()

	select {
	case <-done:
		// clean exit
	case <-time.After(5 * time.Second):
		f.screenCmd.Process.Kill()
		<-done
	}

	f.screenCmd = nil
}

func (f *FFMPEGService) Stop() {
	if f.cmd != nil {
		f.cmd.Process.Kill()
		f.cmd.Wait() // reap the process
		f.cmd = nil
	}
	if f.cmd != nil {
		f.cmd.Process.Kill()
		f.cmd.Wait() // reap the process
		f.cmd = nil
	}
}

// GetSpeakers retrieves a list of available speaker sources.
func (s *FFMPEGService) AudioDeviceList(ctx context.Context) ([]AudioSource, error) {
	ps := `
Get-PnpDevice -Class AudioEndpoint |
Where-Object {
    $_.Status -eq "OK" -and
    $_.InstanceId -match "{0\.0\.1"
} |
Select-Object FriendlyName, InstanceId
`
	fmt.Printf("getting adudio defices \n")
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", ps)

	out, err := cmd.Output()

	if err != nil {
		fmt.Printf("errrrrrr : %v \n", err)
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	var sources []AudioSource

	// Skip header line
	for _, line := range lines[2:] { // first 2 lines are header in default PowerShell table
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// PowerShell table has multiple spaces separating columns
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// FriendlyName can have spaces, so InstanceId is last field
		// instanceID := fields[len(fields)-1]
		name := strings.Join(fields[:len(fields)-1], " ")

		devType := "mic"
		if strings.Contains(name, "Stereo Mix") || strings.Contains(name, "CABLE Output") {
			devType = "speaker"
		}

		sources = append(sources, AudioSource{
			ID:   fmt.Sprintf(`audio=%q`, instanceID), // use InstanceId directly
			Name: name,
			Type: devType,
		})
	}
	fmt.Printf("getting adudio sources %s \n", sources)

	return sources, nil
}
func (s *FFMPEGService) ScreenDeviceList(ctx context.Context) ([]ScreenSource, error) {
	ps := `
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$screens = [System.Windows.Forms.Screen]::AllScreens
foreach ($screen in $screens) {
    $bounds = $screen.Bounds
    
    # capture screenshot of this screen
    $bmp = New-Object System.Drawing.Bitmap($bounds.Width, $bounds.Height)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.CopyFromScreen($bounds.Location, [System.Drawing.Point]::Empty, $bounds.Size)
    $g.Dispose()
    
    # scale down to thumbnail (320px wide) for faster transfer
    $thumbW = 320
    $thumbH = [int]($bounds.Height * 320 / $bounds.Width)
    $thumb = New-Object System.Drawing.Bitmap($thumbW, $thumbH)
    $tg = [System.Drawing.Graphics]::FromImage($thumb)
    $tg.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $tg.DrawImage($bmp, 0, 0, $thumbW, $thumbH)
    $tg.Dispose()
    $bmp.Dispose()
    
    # encode to base64 jpeg
    $ms = New-Object System.IO.MemoryStream
    $thumb.Save($ms, [System.Drawing.Imaging.ImageFormat]::Jpeg)
    $thumb.Dispose()
    $b64 = [Convert]::ToBase64String($ms.ToArray())
    $ms.Dispose()
    
    "{0}|{1}|{2}|{3}|{4}|{5}" -f $screen.DeviceName, $bounds.X, $bounds.Y, $bounds.Width, $bounds.Height, $b64
}
`

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", ps)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("screen list: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var sources []ScreenSource

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// split on first 5 pipes only — b64 may contain + and = but not |
		parts := strings.SplitN(line, "|", 6)
		if len(parts) != 6 {
			log.Printf("[screen-list] skipping malformed line %d: %q", i, line[:min(40, len(line))])
			continue
		}

		var x, y, w, h int
		fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &x)
		fmt.Sscanf(strings.TrimSpace(parts[2]), "%d", &y)
		fmt.Sscanf(strings.TrimSpace(parts[3]), "%d", &w)
		fmt.Sscanf(strings.TrimSpace(parts[4]), "%d", &h)

		// clean up device name — \\.\DISPLAY1 → Display 1
		rawName := strings.TrimSpace(parts[0])
		friendlyName := cleanDisplayName(rawName, i)

		sources = append(sources, ScreenSource{
			ID:         rawName,
			Name:       friendlyName,
			OffsetX:    x,
			OffsetY:    y,
			Width:      w,
			Height:     h,
			Screenshot: "data:image/jpeg;base64," + strings.TrimSpace(parts[5]),
		})
	}

	return sources, nil
}

// cleanDisplayName turns "\\.\DISPLAY1" into "Display 1 (2560×1440)"
func cleanDisplayName(raw string, idx int) string {
	// extract trailing number from \\.\DISPLAY1 etc.
	upper := strings.ToUpper(raw)
	num := ""
	for i := len(upper) - 1; i >= 0; i-- {
		if upper[i] >= '0' && upper[i] <= '9' {
			num = string(upper[i]) + num
		} else {
			break
		}
	}
	if num == "" {
		return fmt.Sprintf("Display %d", idx+1)
	}
	return fmt.Sprintf("Display %s", num)
}
