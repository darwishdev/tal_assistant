//go:build windows

package go_recording

import (
	"fmt"
	"os/exec"
	"strings"
)

// Screen describes a Windows display monitor.
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
