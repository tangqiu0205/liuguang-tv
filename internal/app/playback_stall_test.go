package app

import (
	"errors"
	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
	"os"
	"testing"
	"time"
)

func TestStallProgressPauseAndBufferGrace(t *testing.T) {
	a, p, _, ch := playbackFixture(t)
	t.Cleanup(a.cancelStallTimer)
	a.play(ch)
	gen := a.generation
	a.markLineReady(gen)
	now := time.Now()
	pos := 10.0
	a.observePlayback(gen, player.PlaybackStatus{Position: &pos}, now)
	// Repeated positions do not falsely keep a stalled stream alive.
	a.observePlayback(gen, player.PlaybackStatus{Position: &pos}, now.Add(20*time.Second))
	if !a.lastProgress.Equal(now) {
		t.Fatal("unchanged position counted as progress")
	}
	a.checkPlaybackStall(gen, now.Add(29*time.Second))
	if len(p.streams) != 1 {
		t.Fatal("normal delay triggered failover")
	}
	yes, no := true, false
	a.observePlayback(gen, player.PlaybackStatus{Paused: &yes}, now.Add(30*time.Second))
	a.checkPlaybackStall(gen, now.Add(10*time.Minute))
	if len(p.streams) != 1 {
		t.Fatal("intentional pause triggered failover")
	}
	resumed := now.Add(10 * time.Minute)
	a.observePlayback(gen, player.PlaybackStatus{Paused: &no, Buffering: &yes}, resumed)
	a.checkPlaybackStall(gen, resumed.Add(59*time.Second))
	if len(p.streams) != 1 {
		t.Fatal("buffering grace ignored")
	}
	pos = 11
	a.observePlayback(gen, player.PlaybackStatus{Position: &pos}, resumed.Add(59*time.Second))
	a.checkPlaybackStall(gen, resumed.Add(61*time.Second))
	if len(p.streams) != 1 {
		t.Fatal("real progress ignored")
	}
	a.checkPlaybackStall(gen, resumed.Add(119*time.Second))
	if len(p.streams) != 2 {
		t.Fatal("sustained buffering did not fail over")
	}
	a.observePlayback(gen, player.PlaybackStatus{Position: &pos}, resumed.Add(time.Hour))
	if a.hasPosition {
		t.Fatal("old session progress contaminated new route")
	}
	a.stop()
	a.checkPlaybackStall(a.generation-1, resumed.Add(time.Hour))
	if len(p.streams) != 2 {
		t.Fatal("stale timeout restarted playback")
	}
}
func TestStallFallbackBoundedAndSourceScoped(t *testing.T) {
	a, p, _, ch := playbackFixture(t)
	t.Cleanup(a.cancelStallTimer)
	a.play(ch)
	for range 3 {
		a.markLineReady(a.generation)
		a.checkPlaybackStall(a.generation, a.lastProgress.Add(stallTimeout))
	}
	if len(p.streams) != 3 || a.now != "" || a.stallTimer != nil {
		t.Fatal("stall recovery was not bounded")
	}
	b, engine, _, _ := subscriptionPlaybackFixture(t)
	b.markLineReady(b.generation)
	b.state.UpdateSubscriptionEnabled("b", false)
	b.checkPlaybackStall(b.generation, b.lastProgress.Add(stallTimeout))
	if len(engine.streams) != 1 || b.now != "" {
		t.Fatal("stall recovery used disabled source")
	}
}
func TestRequestHeadersFollowSelectedRoute(t *testing.T) {
	a, _, _, _ := playbackFixture(t)
	ch := playlist.Merge([]playlist.Channel{
		{Name: "Demo", URL: "https://example.org/a", Headers: playlist.StreamHeaders{UserAgent: "one"}},
		{Name: "Demo", URL: "https://example.org/b", Headers: playlist.StreamHeaders{Referer: "https://example.org/"}},
	})[0]
	a.play(ch)
	// startLine must select line headers, not retain the first channel's headers.
	// A recording engine receives the selected Channel, as the real player does.
	recorder := &headerEngine{}
	a.stop()
	a.player = recorder
	a.startLine(ch, 1, map[string]bool{})
	if recorder.channel.Headers != ch.Lines[1].Headers {
		t.Fatalf("wrong route headers: %+v", recorder.channel)
	}
	a.stop()
}

type headerEngine struct {
	channel playlist.Channel
	done    chan error
}

func (p *headerEngine) Play(_ string, ch playlist.Channel, _ ...player.Options) (<-chan error, error) {
	p.channel = ch
	p.done = make(chan error)
	return p.done, nil
}
func (p *headerEngine) Stop() {
	if p.done != nil {
		close(p.done)
		p.done = nil
	}
}
func (p *headerEngine) SetControls(player.Controls) {}
func TestStartupLoadFailureBlocksAllSaves(t *testing.T) {
	a, _, _, _ := playbackFixture(t)
	a.configError = errors.New("restore failed")
	original := store.State{Favorites: map[string]bool{"saved": true}}
	if err := store.Save(a.path, original); err != nil {
		t.Fatal(err)
	}
	a.state = store.State{}
	a.save()
	loaded, err := store.Load(a.path)
	if err != nil || !loaded.Favorites["saved"] {
		t.Fatal("read-failure session overwrote saved data")
	}
	if _, err := os.Stat(a.path); err != nil {
		t.Fatal(err)
	}
}
