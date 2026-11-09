package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *application) mainWindowOptions() mygo.WindowOptions {
	return mygo.WindowOptions{
		Title: tr("流光 TV"), Width: 1280, Height: 820, MinWidth: 1000, MinHeight: 620,
		Hidden:        true, // Configure first, then prime the renderer without exposing a blank window.
		TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 40,
		Vibrancy:        mygo.VibrancySidebar,
		BackgroundColor: "light-dark(#f5f5f7, #141416)", Content: ui.View(a.view),
	}
}

// Hidden native surfaces may not receive paint events. Make the configured
// window paintable at zero opacity; the first-frame callback reveals it. No
// fixed sleep and no permanent layered/transparent video window are needed.
func prepareInitialFrame(w *mygo.Window) {
	if w == nil {
		return
	}
	w.SetOpacity(0)
	w.ShowInactive()
	w.Invalidate()
}

func (a *application) showPreparedWindow() {
	if a.closed || a.window == nil {
		return
	}
	a.window.SetOpacity(1)
	a.window.Show()
	a.settle = 3
	a.window.Invalidate()
}
