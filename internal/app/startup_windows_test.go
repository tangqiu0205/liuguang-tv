//go:build windows

package app

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/tangqiu0205/LiuguangTV/internal/embedhost"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

// Opt in: isolated native windows, no user state, network or media. Each child
// runs one App loop so a global native event loop is never reused by tests.
func init() {
	// Native App.Run must execute on the process's initial goroutine/thread,
	// not testing.T's worker goroutine.
	if mode := os.Getenv("LIUGUANG_TV_STARTUP_CHILD"); mode != "" {
		if err := nativeStartupChild(mode); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("transparent first frame rendered and revealed with correct theme")
		os.Exit(0)
	}
}
func TestNativeStartupWaitsForFirstFrame(t *testing.T) {
	if os.Getenv("LIUGUANG_TV_TEST_STARTUP") == "" {
		t.Skip("set LIUGUANG_TV_TEST_STARTUP=1 to test native startup")
	}
	for _, mode := range []string{"dark", "light"} {
		for _, gpu := range []string{"1", "0"} {
			t.Run(mode+"/gpu="+gpu, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNativeStartupWaitsForFirstFrame$", "-test.v")
				cmd.Env = append(os.Environ(), "LIUGUANG_TV_STARTUP_CHILD="+mode, "MYGO_GPU="+gpu)
				output, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("native startup failed: %v\n%s", err, output)
				}
				t.Log(string(output))
			})
		}
	}
}
func nativeStartupChild(mode string) error {
	a := &application{state: store.State{ThemeMode: mode}, group: "全部分组", settle: 3}
	mygo.Theme.SetSource(nativeThemeSource(mode))
	ready, wasHidden, shown := false, false, false
	var issue string
	var screenshot []byte
	mygo.App.WhenReady(func() {
		a.window = mygo.NewWindow(a.mainWindowOptions())
		wasHidden = !a.window.IsVisible()
		var err error
		a.host, err = embedhost.New(a.window.NativeHandle())
		if err != nil {
			issue = err.Error()
			mygo.App.Quit()
			return
		}
		a.window.OnReadyToShow(func() {
			ready = true
			if a.window.IsVisible() && a.window.Opacity() > 0 {
				issue = "window became opaque before ready callback"
			}
			screenshot, err = a.window.CapturePage()
			if err != nil {
				issue = err.Error()
			}
			a.showPreparedWindow()
			shown = a.window.IsVisible() && a.window.Opacity() == 1
			getStyle := syscall.NewLazyDLL("user32.dll").NewProc("GetWindowLongW")
			style, _, _ := getStyle.Call(a.window.NativeHandle(), ^uintptr(19)) // GWL_EXSTYLE = -20
			if style&0x00080000 != 0 {
				issue = "startup left WS_EX_LAYERED enabled after restoring opacity"
			}
			mygo.App.Quit()
		})
		a.window.OnClose(func(*mygo.CloseEvent) { a.cleanup() })
		a.window.Invalidate()
		prepareInitialFrame(a.window)
	})
	timeout := time.AfterFunc(4*time.Second, func() { mygo.App.Quit() })
	defer timeout.Stop()
	mygo.App.OnQuit(a.cleanup)
	if err := mygo.App.Run(); err != nil {
		return err
	}
	if issue != "" {
		return fmt.Errorf("%s", issue)
	}
	if !ready || !wasHidden || !shown {
		return fmt.Errorf("startup did not reveal a prepared frame: ready=%v initiallyHidden=%v shown=%v", ready, wasHidden, shown)
	}
	img, err := png.Decode(bytes.NewReader(screenshot))
	if err != nil {
		return err
	}
	r, g, b, _ := img.At(img.Bounds().Min.X+2, img.Bounds().Min.Y+200).RGBA()
	expected := panelBackground(themeForMode(mode, false))
	if uint8(r>>8) != expected.R || uint8(g>>8) != expected.G || uint8(b>>8) != expected.B {
		return fmt.Errorf("wrong first-frame background: got=%d,%d,%d want=%+v", r>>8, g>>8, b>>8, expected)
	}
	return nil
}
