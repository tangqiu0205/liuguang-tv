package app

import (
	"math"
	"os"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/player"
)

func standalonePositioningSupported(goos, session, wayland, backend string) bool {
	if goos == "darwin" {
		return true
	}
	if goos != "linux" {
		return false
	}
	// Be conservative in Wayland sessions, including mixed XWayland setups:
	// selecting GDK's X11 backend does not force mpv to use X11 as well.
	return wayland == "" && !strings.EqualFold(strings.TrimSpace(session), "wayland") && !strings.Contains(strings.ToLower(backend), "wayland")
}

func (a *application) standalonePlacement() *player.WindowPlacement {
	// In particular, do not query screen/window APIs on the Windows path.
	if !standalonePositioningSupported(runtime.GOOS, os.Getenv("XDG_SESSION_TYPE"), os.Getenv("WAYLAND_DISPLAY"), os.Getenv("GDK_BACKEND")) || a.window == nil || a.showSettings {
		return nil
	}
	// Read the window origin now, not when the viewport was last laid out: the
	// user may have moved the window without triggering another layout frame.
	return placementForViewport(a.window.ContentBounds(), a.videoViewport, mygo.Screen.Displays(), runtime.GOOS)
}

func placementForViewport(content mygo.Rectangle, viewport ui.Rect, displays []mygo.Display, goos string) *player.WindowPlacement {
	if (goos != "darwin" && goos != "linux") || viewport.W <= 0 || viewport.H <= 0 || content.Width <= 0 || content.Height <= 0 {
		return nil
	}
	x, y := float64(content.X)+float64(viewport.X), float64(content.Y)+float64(viewport.Y)
	w, h := float64(viewport.W), float64(viewport.H)
	if math.IsNaN(x+y+w+h) || math.IsInf(x+y+w+h, 0) {
		return nil
	}
	// Select the display with the greatest intersection. Clip a straddling or
	// partially off-screen region to it, avoiding mpv's negative-edge syntax.
	bestArea := 0.0
	var result *player.WindowPlacement
	for index, display := range displays {
		b := display.Bounds
		left, top := math.Max(x, float64(b.X)), math.Max(y, float64(b.Y))
		right, bottom := math.Min(x+w, float64(b.X+b.Width)), math.Min(y+h, float64(b.Y+b.Height))
		if right <= left || bottom <= top {
			continue
		}
		area := (right - left) * (bottom - top)
		if area <= bestArea {
			continue
		}
		scale := display.ScaleFactor
		if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
			scale = 1
		}
		screen := index
		if goos == "linux" {
			// MyGo exposes the original GDK monitor index as ID, even though
			// its public display list is reordered to put the primary first.
			screen = int(display.ID)
		}
		result = &player.WindowPlacement{
			X:      int(math.Round((left - float64(b.X)) * scale)),
			Y:      int(math.Round((top - float64(b.Y)) * scale)),
			Width:  int(math.Round((right - left) * scale)),
			Height: int(math.Round((bottom - top) * scale)),
			Screen: screen, ScreenName: display.Label,
		}
		bestArea = area
	}
	return result
}
