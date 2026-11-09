package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func (a *application) playbackLineMenu(menu *ui.Menu) {
	ch := a.playbackRoutes()
	for i, label := range lineLabels(ch) {
		selected := playlist.Lines(ch)[i].URL == a.nowURL
		if menu.Item(label).Checked(selected).Chosen() && !selected {
			a.startLine(ch, i, map[string]bool{})
		}
	}
}
func (a *application) morePlaybackMenu(menu *ui.Menu) {
	a.playbackLineMenu(menu)
	menu.Separator()
	label := tr("静音")
	if a.controls.Muted {
		label = tr("取消静音")
	}
	if menu.Item(label).Disabled(!a.supportsPlaybackControls()).Chosen() {
		a.controls.Muted = !a.controls.Muted
		a.syncPlaybackControls()
	}
	for _, volume := range []float64{0, 25, 50, 75, 100} {
		if menu.Item(fmt.Sprintf(tr("音量 %.0f%%"), volume)).Checked(a.controls.Volume == volume).Disabled(!a.supportsPlaybackControls()).Chosen() {
			a.controls.Volume = volume
			a.syncPlaybackControls()
		}
	}
}
