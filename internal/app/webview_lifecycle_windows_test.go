//go:build windows

package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func init() {
	if os.Getenv("LIUGUANG_TV_WEBVIEW_LIFECYCLE_CHILD") != "" {
		if err := nativeWebViewLifecycleChild(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("WebView destroyed, service closed and fresh engine recreated for three cycles")
		os.Exit(0)
	}
}

func TestNativeWebViewReleaseAndRecreate(t *testing.T) {
	if os.Getenv("LIUGUANG_TV_TEST_WEBVIEW_LIFECYCLE") == "" {
		t.Skip("set LIUGUANG_TV_TEST_WEBVIEW_LIFECYCLE=1 to test native WebView lifecycle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNativeWebViewReleaseAndRecreate$", "-test.v")
	cmd.Env = append(os.Environ(), "LIUGUANG_TV_WEBVIEW_LIFECYCLE_CHILD=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native WebView lifecycle failed: %v\n%s", err, output)
	}
	t.Log(string(output))
}

func nativeWebViewLifecycleChild() error {
	dir, err := os.MkdirTemp("", "liuguang-webview-lifecycle-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	a := &application{state: store.State{PlaybackEngine: "webview"}, path: dir + "/state.json", group: "全部分组"}
	completed := false
	var issue error
	fail := func(err error) { issue = err; mygo.App.Quit() }
	round := 0
	var create func()
	create = func() {
		round++
		if err := a.ensurePlaybackEngine(); err != nil {
			fail(err)
			return
		}
		oldView, oldPlayer := a.webView, a.webPlayer
		handled := false
		oldView.Page().OnDOMReady(func() {
			if handled {
				return
			}
			handled = true
			a.setPlaybackEngine("mpv")
			if !oldView.IsDestroyed() || a.webView != nil || a.webPlayer != nil || a.player != nil {
				fail(fmt.Errorf("cycle %d: WebView was not destroyed", round))
				return
			}
			if _, err := oldPlayer.Play("", playlist.Channel{URL: "http://127.0.0.1:1/demo.mp4"}); err == nil {
				fail(fmt.Errorf("cycle %d: closed service accepted playback", round))
				return
			}
			a.releaseWebPlayback()
			a.setPlaybackEngine("webview")
			if a.webView != nil || a.webPlayer != nil || a.player != nil {
				fail(fmt.Errorf("cycle %d: selection eagerly recreated WebView", round))
				return
			}
			if round == 3 {
				completed = true
				mygo.App.Quit()
				return
			}
			create()
			if a.webView == oldView || a.webPlayer == oldPlayer {
				fail(fmt.Errorf("destroyed instance reused"))
			}
		})
		if err := oldView.Page().LoadURL("about:blank"); err != nil {
			fail(err)
		}
	}
	mygo.App.WhenReady(func() {
		a.window = mygo.NewWindow(a.mainWindowOptions())
		a.window.OnReadyToShow(func() { a.showPreparedWindow(); create() })
		a.window.OnClose(func(*mygo.CloseEvent) { a.cleanup() })
		prepareInitialFrame(a.window)
	})
	timer := time.AfterFunc(20*time.Second, func() { mygo.App.Quit() })
	defer timer.Stop()
	mygo.App.OnQuit(a.cleanup)
	if err := mygo.App.Run(); err != nil {
		return err
	}
	if issue != nil {
		return issue
	}
	if !completed {
		return fmt.Errorf("native lifecycle test did not complete")
	}
	return nil
}
