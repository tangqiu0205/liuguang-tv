package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"time"
)

// Window-local only: do not intercept a text editor, native menu, or mpv HWND.
// Showing search disables these bindings, even before its input gains focus.
func (a *application) playbackShortcuts(c *ui.Context, root ui.Element) ui.Element {
	if a.showSettings || a.showSearch || a.transferBusy || a.busy {
		return root
	}
	bind := func(key ui.Key, fn func()) {
		root = root.OnShortcut(0, key, fn)
		c.OnShortcut(0, key, fn) // Also works before any element receives focus.
	}
	bind(ui.KeyUp, func() { a.stepChannel(-1) })
	bind(ui.KeyDown, func() { a.stepChannel(1) })
	if !a.supportsPlaybackControls() || a.now == "" || a.player == nil {
		return root
	}
	bind(ui.KeySpace, a.togglePause)
	bind(ui.KeyLeft, func() { a.changeVolume(-5) })
	bind(ui.KeyRight, func() { a.changeVolume(5) })
	return root
}
func (a *application) togglePause() {
	if a.now == "" || a.player == nil {
		return
	}
	a.controls.Paused = !a.controls.Paused
	a.lastProgress = time.Now()
	a.syncPlaybackControls()
}
func (a *application) changeVolume(delta float64) {
	a.controls.Volume = max(0, min(100, a.controls.Volume+delta))
	a.syncPlaybackControls()
}

// Idle controls update preferences without eagerly creating a playback engine.
func (a *application) syncPlaybackControls() {
	if a.player != nil {
		a.player.SetControls(a.controls)
	}
}
func (a *application) stepChannel(direction int) {
	channels := a.filteredChannels()
	if channels.Len() == 0 {
		return
	}
	index := -1
	for i := 0; i < channels.Len(); i++ {
		if playlist.ChannelKey(channels.At(i)) == playlist.ChannelKey(a.activeChannel) {
			index = i
			break
		}
	}
	if index < 0 {
		index = 0
		if direction < 0 {
			index = channels.Len() - 1
		}
	} else {
		index = (index + direction + channels.Len()) % channels.Len()
	}
	a.play(channels.At(index))
}
