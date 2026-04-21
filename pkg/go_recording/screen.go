//go:build windows

package go_recording

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Screen describes a Windows display monitor or application window.
type Screen struct {
	// Index is the zero-based enumeration order (matches --screen flag).
	Index int
	// Name is the GDI device name, e.g. \\.\DISPLAY1.
	Name string
	// X and Y are the top-left position of this monitor in the virtual desktop.
	X, Y int32
	// Width and Height are the resolution in pixels.
	Width, Height int32
	// IsPrimary is true for the Windows primary display.
	IsPrimary bool
	// Screenshot is a base64 encoded JPEG thumbnail of the screen.
	Screenshot string
	// WindowTitle, when set, captures a specific window by title instead of a screen region.
	// Takes precedence over screen coordinates when non-empty.
	WindowTitle string
}

// String returns a short human-readable description.
func (s Screen) String() string {
	primary := ""
	if s.IsPrimary {
		primary = " [primary]"
	}
	return fmt.Sprintf("%d: %s  %dx%d at (%d,%d)%s",
		s.Index, s.Name, s.Width, s.Height, s.X, s.Y, primary)
}

// appProcessNames lists the process names to search for, dev first then production.
var appProcessNames = []string{"tal-interview-assistant-dev", "tal-interview-assistant"}

// windowBoundsPS is the PowerShell script that resolves the app window bounds
// via Get-Process → MainWindowHandle → GetWindowRect.
// It tries each process name in appProcessNames and exits 1 if none is found.
// On success it prints "Left,Top,Width,Height".
const windowBoundsPS = `
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class WinAPI {
    [DllImport("user32.dll")]
    public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }
}
"@ -ErrorAction SilentlyContinue

$names = @('tal-interview-assistant-dev','tal-interview-assistant')
$proc  = $null
foreach ($n in $names) {
    $p = Get-Process $n -ErrorAction SilentlyContinue |
         Where-Object { $_.MainWindowHandle -ne [IntPtr]::Zero } |
         Select-Object -First 1
    if ($p) { $proc = $p; break }
}
if (-not $proc) { exit 1 }

$r = New-Object WinAPI+RECT
[WinAPI]::GetWindowRect($proc.MainWindowHandle, [ref]$r) | Out-Null

$x = $r.Left;   if ($x -lt 0) { $x = 0 }
$y = $r.Top;    if ($y -lt 0) { $y = 0 }
$w = $r.Right  - $r.Left
$h = $r.Bottom - $r.Top
"{0},{1},{2},{3}" -f $x,$y,$w,$h
`

// FindWindowBounds locates the running Tal Assistant window via its process
// MainWindowHandle and returns its current screen coordinates.
// Region-based desktop capture (offset_x/offset_y) is used because gdigrab
// "title=" capture fails for DWM-composited transparent/frameless windows.
func FindWindowBounds() (*Screen, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", windowBoundsPS)
	rawOut, err := cmd.Output()
	if err != nil {
		fmt.Printf("[FindWindowBounds] ERROR: PowerShell failed: %v\n", err)
		return nil, fmt.Errorf("FindWindowBounds: process not found or GetWindowRect failed: %w", err)
	}
	trimmed := strings.TrimSpace(string(rawOut))
	fmt.Printf("[FindWindowBounds] raw output: %q\n", trimmed)
	s, parseErr := parseScreenBounds(trimmed)
	if parseErr != nil {
		fmt.Printf("[FindWindowBounds] ERROR: parse failed: %v\n", parseErr)
		return nil, parseErr
	}
	fmt.Printf("[FindWindowBounds] OK — X=%d Y=%d W=%d H=%d\n", s.X, s.Y, s.Width, s.Height)
	return s, nil
}

// TrackWindowBounds calls FindWindowBounds every interval and sends the result
// to ch whenever the position or size has changed by more than 2 pixels.
// The goroutine exits when quit is closed. ch must be buffered (capacity ≥ 1).
func TrackWindowBounds(interval time.Duration, ch chan<- Screen, quit <-chan struct{}) {
	var last Screen
	for {
		select {
		case <-quit:
			return
		case <-time.After(interval):
		}
		s, err := FindWindowBounds()
		if err != nil {
			continue
		}
		if abs32(s.X-last.X) > 2 || abs32(s.Y-last.Y) > 2 ||
			abs32(s.Width-last.Width) > 2 || abs32(s.Height-last.Height) > 2 {
			last = *s
			select {
			case ch <- *s:
			default: // receiver not keeping up; skip this update
			}
		}
	}
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func parseScreenBounds(line string) (*Screen, error) {
	parts := strings.SplitN(line, ",", 4)
	if len(parts) != 4 {
		return nil, fmt.Errorf("FindWindowBounds: unexpected output %q", line)
	}
	var x, y, w, h int32
	fmt.Sscanf(parts[0], "%d", &x)
	fmt.Sscanf(parts[1], "%d", &y)
	fmt.Sscanf(parts[2], "%d", &w)
	fmt.Sscanf(parts[3], "%d", &h)
	return &Screen{X: x, Y: y, Width: w, Height: h}, nil
}

// ListScreens returns all active Windows monitors in enumeration order.
func ListScreens() ([]Screen, error) {
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
    
    "{0}|{1}|{2}|{3}|{4}|{5}|{6}" -f $screen.DeviceName, $bounds.X, $bounds.Y, $bounds.Width, $bounds.Height, $screen.Primary, $b64
}
`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", ps)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("screen list: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var screens []Screen

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 7)
		if len(parts) != 7 {
			continue
		}

		var x, y, w, h int
		fmt.Sscanf(parts[1], "%d", &x)
		fmt.Sscanf(parts[2], "%d", &y)
		fmt.Sscanf(parts[3], "%d", &w)
		fmt.Sscanf(parts[4], "%d", &h)

		isPrimary := strings.EqualFold(parts[5], "True")

		screens = append(screens, Screen{
			Index:      i,
			Name:       parts[0],
			X:          int32(x),
			Y:          int32(y),
			Width:      int32(w),
			Height:     int32(h),
			IsPrimary:  isPrimary,
			Screenshot: "data:image/jpeg;base64," + parts[6],
		})
	}

	return screens, nil
}
