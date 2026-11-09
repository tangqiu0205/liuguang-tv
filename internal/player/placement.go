package player

import "fmt"

// WindowPlacement describes an initial standalone window in physical pixels,
// relative to the selected display's top-left corner. It is not tracked later.
type WindowPlacement struct {
	X, Y, Width, Height int
	Screen              int
	ScreenName          string // macOS NSScreen name, rather than a CGDisplay ID.
}

func (p WindowPlacement) arguments(goos string) []string {
	// Never affect the existing Windows embedded or standalone playback paths.
	if (goos != "darwin" && goos != "linux") || p.Width <= 0 || p.Height <= 0 || p.X < 0 || p.Y < 0 || p.Screen < 0 {
		return nil
	}
	args := []string{
		fmt.Sprintf("--geometry=%dx%d+%d+%d", p.Width, p.Height, p.X, p.Y),
		"--border=no", "--keepaspect-window=no", "--keepaspect=yes", "--hidpi-window-scale=no",
	}
	if goos == "darwin" {
		// Match the whole-screen coordinate system, not the Dock/menu work area.
		args = append(args, "--macos-geometry-calculation=whole")
		if p.ScreenName != "" {
			return append(args, "--screen-name="+p.ScreenName)
		}
	}
	return append(args, fmt.Sprintf("--screen=%d", p.Screen))
}
