package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	fullscreenControlsDelay         = 3 * time.Second
	floatingControlsHeight  float32 = 56
	floatingControlsBottom  float32 = 16
	floatingControlsRadius  float32 = 16
)

func floatingControlsRect(width, height float32) ui.Rect {
	w := min(float32(760), max(float32(0), width-32))
	return ui.Rect{X: (width - w) / 2, Y: height - floatingControlsBottom - floatingControlsHeight, W: w, H: floatingControlsHeight}
}

type fullscreenControls struct {
	until        time.Time
	x, y         int32
	pointerValid bool
	visible      bool
	initialized  bool
}

// update tracks screen-space pointer movement even over the native video child.
func (s *fullscreenControls) update(now time.Time, x, y int32, inside, hold bool) bool {
	if !s.initialized {
		s.initialized = true
		s.until = now.Add(fullscreenControlsDelay)
	}
	if inside && (!s.pointerValid || s.x != x || s.y != y) {
		s.until = now.Add(fullscreenControlsDelay)
	}
	s.x, s.y, s.pointerValid = x, y, inside
	if hold {
		s.until = now.Add(fullscreenControlsDelay)
	}
	s.visible = now.Before(s.until)
	return s.visible
}

func (a *application) fullscreenBarsVisible(now time.Time) bool {
	if !a.cinema && !a.windowCinema {
		return true
	}
	poll := a.fullscreenPointer
	if poll == nil {
		if a.window == nil || !fullscreenPointerSupported {
			return true
		}
		poll = func() (int32, int32, bool, error) { return fullscreenPointerPosition(a.window.NativeHandle()) }
	}
	x, y, inside, err := poll()
	if err != nil {
		a.status = err.Error()
		return true
	}
	previous := a.fullscreenUI.visible
	initialized := a.fullscreenUI.initialized
	visible := a.fullscreenUI.update(now, x, y, inside, a.now == "" || a.epg.show || (inside && a.fullscreenControlsHovered))
	if initialized && visible != previous {
		a.settle = 3
	}
	return visible
}
