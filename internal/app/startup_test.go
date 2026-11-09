package app

import (
	"github.com/egoist/mygo"
	"testing"
)

func TestMainWindowStartsHiddenWithThemeBackground(t *testing.T) {
	a := &application{}
	options := a.mainWindowOptions()
	if !options.Hidden || options.Content == nil {
		t.Fatal("main window would expose a blank native frame")
	}
	if options.BackgroundColor != "light-dark(#f5f5f7, #141416)" {
		t.Fatal("startup background does not follow effective theme")
	}
	if options.Vibrancy != mygo.VibrancySidebar {
		t.Fatal("sidebar material is not enabled")
	}
	if options.Transparent {
		t.Fatal("native video window must not use layered transparency")
	}
	if options.TitleBarStyle != mygo.TitleBarHidden || options.MinWidth != 1000 || options.MinHeight != 620 {
		t.Fatal("startup changed window layout constraints")
	}
}
func TestReadyCallbackDoesNotReviveClosedWindow(t *testing.T) {
	a := &application{}
	a.showPreparedWindow() // Window not created.
	a.closed = true
	a.window = &mygo.Window{}
	a.showPreparedWindow() // Must not touch native APIs after shutdown.
	prepareInitialFrame(nil)
}
