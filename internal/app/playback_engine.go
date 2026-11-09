package app

import (
	"errors"
	"github.com/egoist/mygo"
	"github.com/tangqiu0205/LiuguangTV/internal/embedhost"
	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"github.com/tangqiu0205/LiuguangTV/internal/webplayer"
)

func (a *application) webPlayback() bool              { return a.state.PlaybackEngine == "webview" }
func (a *application) supportsPlaybackControls() bool { return a.webPlayback() || embedhost.Supported }
func (a *application) supportsPlaybackReady() bool    { return a.webPlayback() || player.SupportsReady }

func (a *application) ensurePlaybackEngine() error {
	if a.player != nil {
		return nil
	}
	if a.state.PlaybackEngine != "" && a.state.PlaybackEngine != "mpv" && !a.webPlayback() {
		return errors.New(tr("播放引擎设置无效，请重新选择播放器"))
	}
	if !a.webPlayback() {
		a.player = &player.Player{Logger: a.diagnostics}
		return nil
	}
	if a.webPlayer != nil {
		a.player = a.webPlayer
		return nil
	}
	if a.window == nil {
		return errors.New(tr("内置播放窗口未就绪"))
	}
	if !mygo.WebViewAvailable() {
		return errors.New(tr("系统 WebView 不可用：Windows 请安装 WebView2 Runtime，Linux 请安装 WebKitGTK，或切换 mpv"))
	}
	view, err := a.window.NewWebView(mygo.WebViewOptions{BackgroundColor: "#080808"})
	if err != nil {
		return errors.New(tr("无法创建 WebView 播放区域，请检查系统运行库或切换 mpv"))
	}
	view.Page().OnWillNavigate(func(e *mygo.NavigateEvent) { e.PreventDefault() })
	engine, err := webplayer.New(view.Page().LoadURL)
	if err != nil {
		view.Destroy()
		return err
	}
	a.webView, a.webPlayer, a.player = view, engine, engine
	return nil
}

// releaseWebPlayback relinquishes the complete optional engine, not just its
// current stream. The next WebView playback goes through lazy creation again.
func (a *application) releaseWebPlayback() {
	if a.webPlayer != nil {
		if a.player == a.webPlayer {
			a.player = nil
		}
		a.webPlayer.Close()
		a.webPlayer = nil
	}
	if a.webView != nil {
		a.webView.Destroy()
		a.webView = nil
	}
}

// A deliberate engine change stops the old session; it never silently falls
// back to another engine or interprets a non-mpv binary as mpv-compatible.
func (a *application) setPlaybackEngine(engine string) {
	if engine == a.state.PlaybackEngine || (engine == "mpv" && a.state.PlaybackEngine == "") {
		return
	}
	a.stop()
	a.player = nil
	a.state.PlaybackEngine = engine
	if !a.webPlayback() {
		a.releaseWebPlayback()
	}
	a.status = tr("播放引擎已切换，设置已提交保存，请重新选择频道")
	a.pendingSettingsSuccess = tr("播放引擎已切换，设置已保存，请重新选择频道")
	a.save()
}
