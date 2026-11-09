package app

import (
	"github.com/tangqiu0205/LiuguangTV/internal/diagnostic"
	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"time"
)

const (
	stallTimeout     = 30 * time.Second
	bufferingTimeout = 60 * time.Second
)

func (a *application) cancelStallTimer() {
	if a.stallTimer != nil {
		a.stallTimer.Stop()
		a.stallTimer = nil
	}
}
func (a *application) armStallTimer(gen int, d time.Duration) {
	a.cancelStallTimer()
	a.stallTimer = time.AfterFunc(d, func() { a.update(func() { a.checkPlaybackStall(gen, time.Now()) }) })
}
func (a *application) observePlayback(gen int, status player.PlaybackStatus, now time.Time) {
	if gen != a.generation || a.now == "" {
		return
	}
	if status.Paused != nil {
		if a.playerPaused != *status.Paused {
			a.lastProgress = now
		}
		a.playerPaused = *status.Paused
	}
	if status.Buffering != nil {
		a.buffering = *status.Buffering
	}
	if status.Position != nil && (!a.hasPosition || *status.Position != a.lastPosition) {
		a.lastPosition = *status.Position
		a.hasPosition = true
		a.lastProgress = now
	}
}
func (a *application) checkPlaybackStall(gen int, now time.Time) {
	if gen != a.generation || a.now == "" || !a.lineReady {
		return
	}
	// Intentional pause is never a source failure. Resume gets a fresh grace period.
	if a.controls.Paused || a.playerPaused {
		a.lastProgress = now
		a.armStallTimer(gen, stallTimeout)
		return
	}
	limit := stallTimeout
	if a.buffering {
		limit = bufferingTimeout
	}
	elapsed := now.Sub(a.lastProgress)
	if elapsed < limit {
		a.armStallTimer(gen, limit-elapsed)
		return
	}
	a.diagnostics.Record(diagnostic.PlaybackTimeout, a.nowURL, nil)
	a.player.Stop()
	a.advanceLine(tr("播放长时间无进展"))
}
