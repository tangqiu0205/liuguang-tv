package app

import "github.com/egoist/mygo/ui"

func (a *application) toggleSearch() {
	a.showSearch = !a.showSearch
	if !a.showSearch {
		a.query = ""
	}
	a.channelCache.valid = false
	a.list.ScrollTo(0, ui.Start)
}
