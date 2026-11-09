package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/diagnostic"
	"github.com/tangqiu0205/LiuguangTV/internal/embedhost"
	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/webplayer"
)

type playbackEngine interface {
	Play(string, playlist.Channel, ...player.Options) (<-chan error, error)
	SetControls(player.Controls)
	Stop()
}

const startupTimeout = 20 * time.Second

func lineLabels(ch playlist.Channel) []string {
	var labels []string
	for i, line := range playlist.Lines(ch) {
		label := fmt.Sprintf(tr("线路 %d"), i+1)
		if line.Label != "" {
			label += " · " + line.Label
		}
		labels = append(labels, label)
	}
	return labels
}
func (a *application) play(ch playlist.Channel) {
	owned := ch.SourceID != "" || len(a.state.Subscriptions) > 0
	for _, line := range playlist.Lines(ch) {
		owned = owned || line.SourceID != ""
	}
	ch = a.routesFor(ch, owned)
	if len(playlist.Lines(ch)) == 0 {
		a.status = tr("该频道没有已启用的播放线路")
		return
	}
	index := playlist.PreferredLine(ch, a.state.PreferredLines[playlist.ChannelKey(ch)])
	a.startLine(ch, index, map[string]bool{})
}
func (a *application) startLine(ch playlist.Channel, index int, tried map[string]bool) bool {
	if !a.webPlayback() && embedhost.Supported && a.host == nil && a.dispatch == nil {
		a.status = tr("视频子窗口未就绪，请重新启动应用")
		return false
	}
	lines := playlist.Lines(ch)
	if index < 0 || index >= len(lines) {
		return false
	}
	if tried == nil {
		tried = map[string]bool{}
	}
	// Unsupported addresses are route-local, not engine startup failures.
	// Keep original line indices/labels and record skips in the same attempt set.
	for a.webPlayback() && !webplayer.SupportsURL(lines[index].URL) {
		tried[lines[index].URL] = true
		index = playlist.NextLine(ch, tried)
		if index < 0 {
			a.status = tr("WebView 没有剩余的兼容 HTTP(S) 线路，请改用 mpv 或重新选择频道")
			return false
		}
	}
	if err := a.ensurePlaybackEngine(); err != nil {
		a.status = err.Error()
		return false
	}
	gen := a.generation + 1
	controls := a.controls
	controls.Paused = false
	stream := ch
	stream.URL = lines[index].URL
	stream.Headers = lines[index].Headers
	done, err := a.player.Play(strings.TrimSpace(a.state.Player), stream, player.Options{
		Window: a.host.Handle(), Controls: controls, Placement: a.standalonePlacement(),
		OnReady: func() { a.update(func() { a.markLineReady(gen) }) },
		OnPlayback: func(status player.PlaybackStatus) {
			a.update(func() { a.observePlayback(gen, status, time.Now()) })
		},
		OnControlError: func(err error) {
			a.update(func() {
				if gen == a.generation {
					a.status = err.Error()
				}
			})
		},
	})
	if err != nil {
		a.status = err.Error()
		return false
	}
	a.cancelStartupTimer()
	a.generation = gen
	a.controls = controls
	a.activeChannel = ch
	a.playbackCatalogBound = a.playbackCatalogBound || len(a.state.Subscriptions) > 0
	a.activeSourceID = lines[index].SourceID
	if a.activeSourceID == "" {
		a.activeSourceID = ch.SourceID
	}
	a.now = ch.Name
	a.nowURL = stream.URL
	a.lineIndex = index
	a.lineReady = false
	a.cancelStallTimer()
	a.hasPosition, a.playerPaused, a.buffering = false, false, false
	a.lastProgress = time.Time{}
	a.selectedLine = lineLabels(ch)[index]
	a.settle = 3
	a.triedLines = tried
	a.triedLines[stream.URL] = true
	a.status = tr("正在连接：") + ch.Name + " · " + a.selectedLine
	if a.supportsPlaybackReady() {
		a.armStartupTimer(gen, startupTimeout)
	}
	go func() {
		err := <-done
		a.update(func() {
			if gen != a.generation || a.now == "" {
				return
			}
			if a.webPlayback() && err != nil {
				a.advanceLine(err.Error())
				return
			}
			a.lineEnded(gen)
		})
	}()
	return true
}
func (a *application) cancelStartupTimer() {
	if a.startupTimer != nil {
		a.startupTimer.Stop()
		a.startupTimer = nil
	}
}
func (a *application) armStartupTimer(gen int, d time.Duration) {
	a.cancelStartupTimer()
	a.startupTimer = time.AfterFunc(d, func() { a.update(func() { a.lineTimedOut(gen) }) })
}
func (a *application) markLineReady(gen int) {
	if a.generation != gen || a.now == "" || a.lineReady {
		return
	}
	a.lineReady = true
	a.lastProgress = time.Now()
	if a.supportsPlaybackReady() {
		a.armStallTimer(gen, stallTimeout)
	}
	a.cancelStartupTimer()
	a.status = tr("正在播放：") + a.now + " · " + a.selectedLine
	key := playlist.ChannelKey(a.activeChannel)
	if a.state.PreferredLines == nil {
		a.state.PreferredLines = map[string]string{}
	}
	if a.state.PreferredLines[key] != a.nowURL {
		a.state.PreferredLines[key] = a.nowURL
		a.save()
	}
}
func (a *application) lineTimedOut(gen int) {
	if gen != a.generation || a.lineReady || a.now == "" {
		return
	}
	if a.controls.Paused {
		a.armStartupTimer(gen, 5*time.Second)
		return
	}
	a.diagnostics.Record(diagnostic.PlaybackTimeout, a.nowURL, nil)
	a.player.Stop()
	a.advanceLine(tr("连接超时"))
}
func (a *application) lineEnded(gen int) {
	if gen != a.generation || a.now == "" {
		return
	}
	a.advanceLine(tr("播放结束或连接失败"))
}
func (a *application) advanceLine(reason string) {
	ch := a.playbackRoutes()
	next := playlist.NextLine(ch, a.triedLines)
	if next >= 0 {
		if a.startLine(ch, next, a.triedLines) {
			a.status = reason + tr("，尝试 ") + a.selectedLine
			return
		}
		// Executable/startup failures are not source failures: do not spin through routes.
		message := a.status
		a.endPlayback(message)
		return
	}
	a.endPlayback(reason + tr("；该频道的所有线路均已尝试，可重新点击频道重试"))
}
func (a *application) endPlayback(status string) {
	a.generation++
	if a.player != nil {
		a.player.Stop()
	}
	a.cancelStartupTimer()
	a.cancelStallTimer()
	a.now = ""
	a.nowURL = ""
	a.activeChannel = playlist.Channel{}
	a.activeSourceID = ""
	a.playbackCatalogBound = false
	a.selectedLine = ""
	a.triedLines = nil
	a.controls.Paused = false
	a.host.Show(false)
	a.status = status
}
func (a *application) stop() {
	a.endPlayback(tr("已停止播放"))
}
func (a *application) isFavorite(ch playlist.Channel) bool {
	if a.state.Favorites["channel:"+playlist.ChannelKey(ch)] {
		return true
	}
	for _, line := range playlist.Lines(ch) {
		if a.state.Favorites[line.URL] {
			return true
		}
	}
	return false
}
func (a *application) toggleFavorite(ch playlist.Channel) {
	if a.state.Favorites == nil {
		a.state.Favorites = map[string]bool{}
	}
	key := "channel:" + playlist.ChannelKey(ch)
	if a.isFavorite(ch) {
		delete(a.state.Favorites, key)
		for _, rawURL := range a.state.FavoriteURLs[playlist.ChannelKey(ch)] {
			delete(a.state.Favorites, rawURL)
		}
		for _, line := range playlist.Lines(ch) {
			delete(a.state.Favorites, line.URL)
		}
	} else {
		a.state.Favorites[key] = true
		a.state.Favorites[ch.URL] = true
	}
	a.favoritesRevision++
	a.save()
}
