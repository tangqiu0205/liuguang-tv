package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/player"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

type fakeStream struct {
	done chan error
	once sync.Once
	url  string
	opt  player.Options
}

func (s *fakeStream) finish() { s.once.Do(func() { close(s.done) }) }

type fakeEngine struct {
	streams   []*fakeStream
	failStart bool
}

func (p *fakeEngine) Play(_ string, ch playlist.Channel, options ...player.Options) (<-chan error, error) {
	if p.failStart {
		return nil, fmt.Errorf("找不到播放器")
	}
	p.Stop()
	s := &fakeStream{done: make(chan error), url: ch.URL, opt: options[0]}
	p.streams = append(p.streams, s)
	return s.done, nil
}
func (p *fakeEngine) Stop() {
	if len(p.streams) > 0 {
		p.streams[len(p.streams)-1].finish()
	}
}
func (p *fakeEngine) SetControls(player.Controls) {}

func playbackFixture(t *testing.T) (*application, *fakeEngine, chan func(), playlist.Channel) {
	t.Helper()
	ch := playlist.Merge([]playlist.Channel{
		{Name: "Demo", URL: "https://example.org/a"},
		{Name: "Demo", URL: "https://example.org/b"},
		{Name: "Demo", URL: "https://example.org/c"},
	})[0]
	queue := make(chan func(), 64)
	engine := &fakeEngine{}
	a := &application{player: engine, path: filepath.Join(t.TempDir(), "state.json"), state: store.State{Favorites: map[string]bool{}}, controls: player.Controls{Volume: 70}}
	a.dispatch = func(fn func()) { queue <- fn }
	t.Cleanup(func() { a.closed = true; a.cancelStartupTimer(); a.cancelStallTimer(); engine.Stop() })
	return a, engine, queue, ch
}
func pumpPlayback(t *testing.T, queue chan func(), until func() bool) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for !until() {
		select {
		case fn := <-queue:
			fn()
		case <-deadline:
			t.Fatal("timed out waiting for playback event")
		}
	}
}
func TestPreferredLineSuccessAndBoundedFallback(t *testing.T) {
	a, p, queue, ch := playbackFixture(t)
	key := playlist.ChannelKey(ch)
	a.state.PreferredLines = map[string]string{key: ch.Lines[1].URL}
	if err := store.Save(a.path, a.state); err != nil {
		t.Fatal(err)
	}
	a.play(ch)
	if p.streams[0].url != ch.Lines[1].URL || a.lineReady {
		t.Fatal("preferred route was not tried first")
	}
	gen := a.generation
	p.streams[0].opt.OnReady()
	pumpPlayback(t, queue, func() bool { return a.lineReady })
	loaded, err := store.Load(a.path)
	if err != nil || loaded.PreferredLines[key] != ch.Lines[1].URL {
		t.Fatal("successful route was not persisted")
	}
	p.streams[0].finish()
	pumpPlayback(t, queue, func() bool { return len(p.streams) == 2 })
	if p.streams[1].url != ch.Lines[0].URL {
		t.Fatal("fallback order incorrect")
	}
	a.markLineReady(gen)
	if a.lineReady {
		t.Fatal("old route readiness contaminated new route")
	}
	p.streams[1].opt.OnReady()
	pumpPlayback(t, queue, func() bool { return a.lineReady })
	if a.state.PreferredLines[key] != ch.Lines[0].URL {
		t.Fatal("new successful route was not remembered")
	}
	p.streams[1].finish()
	pumpPlayback(t, queue, func() bool { return len(p.streams) == 3 })
	p.streams[2].finish()
	pumpPlayback(t, queue, func() bool { return a.now == "" })
	if len(p.streams) != 3 || !strings.Contains(a.status, "所有线路") {
		t.Fatal("fallback did not stop after exhausting candidates")
	}
}
func TestStartupTimeoutManualSwitchAndStop(t *testing.T) {
	a, p, queue, ch := playbackFixture(t)
	a.play(ch)
	firstGen := a.generation
	a.controls.Paused = true
	a.lineTimedOut(firstGen)
	if len(p.streams) != 1 {
		t.Fatal("paused playback should not fail over")
	}
	a.controls.Paused = false
	a.lineTimedOut(firstGen)
	if len(p.streams) != 2 || p.streams[1].url != ch.Lines[1].URL {
		t.Fatal("startup timeout did not switch route")
	}
	// A manual selection starts a fresh, bounded set of attempts.
	a.startLine(ch, 2, map[string]bool{})
	if p.streams[2].url != ch.Lines[2].URL || len(a.triedLines) != 1 {
		t.Fatal("manual route switch failed")
	}
	a.stop()
	// Ignore delayed exit callbacks from every preceding route.
	for i := 0; i < 3; i++ {
		select {
		case fn := <-queue:
			fn()
		case <-time.After(100 * time.Millisecond):
		}
	}
	if len(p.streams) != 3 || a.now != "" {
		t.Fatal("manual stop restarted playback")
	}
	if a.state.PreferredLines[playlist.ChannelKey(ch)] != "" {
		t.Fatal("a merely launched route was marked successful")
	}
}
func TestLineSelectorUsesNativeMenu(t *testing.T) {
	a, p, _, ch := playbackFixture(t)
	a.group = "全部分组"
	a.state.Channels = []playlist.Channel{ch}
	a.play(ch)
	tt := ui.NewTester(a.view, 1000, 620)
	if err := tt.Click("播放线路"); err != nil {
		t.Fatal(err)
	}
	labels := lineLabels(ch)
	items := tt.Menu()
	if len(items) != len(labels) {
		t.Fatalf("expected native menu with %d routes, got %v", len(labels), items)
	}
	if len(p.streams) != 1 || a.now == "" {
		t.Fatal("opening route menu interrupted playback")
	}
	if err := tt.ChooseMenuItem(labels[1]); err != nil {
		t.Fatal(err)
	}
	if len(p.streams) != 2 || a.lineIndex != 1 || p.streams[1].url != ch.Lines[1].URL {
		t.Fatal("native menu did not switch route")
	}
	if err := tt.Click("播放线路"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(labels[1]); err != nil {
		t.Fatal(err)
	}
	if len(p.streams) != 2 {
		t.Fatal("choosing current route should not restart playback")
	}
	if err := tt.Click("播放线路"); err != nil {
		t.Fatal(err)
	}
	tt.CloseMenu()
	if len(p.streams) != 2 || a.now == "" {
		t.Fatal("dismissing menu interrupted playback")
	}
}

func TestMultiLineFavoritesAndSelectorLayout(t *testing.T) {
	a, p, _, ch := playbackFixture(t)
	a.group = "全部分组"
	a.state.Channels = []playlist.Channel{ch}
	a.toggleFavorite(ch)
	reversed := ch
	reversed.URL = ch.Lines[1].URL
	if !a.isFavorite(reversed) {
		t.Fatal("favorite was tied to first route order")
	}
	a.play(ch)
	tt := ui.NewTester(a.view, 1000, 620)
	for _, size := range [][2]int{{1000, 620}, {1280, 820}} {
		tt.SetSize(size[0], size[1])
		contained(t, "播放线路", requiredRect(t, tt, "播放控制栏"), requiredRect(t, tt, "播放线路"))
		contained(t, "音量", requiredRect(t, tt, "播放控制栏"), requiredRect(t, tt, "音量"))
		if !tt.HasText(" · 3 条线路") {
			t.Fatal("line count missing from channel row")
		}
	}
	a.toggleFavorite(reversed)
	if a.isFavorite(ch) {
		t.Fatal("favorite was not cleared for all routes")
	}
	p.Stop()
}
