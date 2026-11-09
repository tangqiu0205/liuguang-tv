package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/embedhost"
	"github.com/tangqiu0205/LiuguangTV/internal/epg"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestNarrowEPGControlsAndNativeMoreMenu(t *testing.T) {
	a, engine, _, ch := playbackFixture(t)
	a.group = "全部分组"
	a.state.Channels = []playlist.Channel{ch}
	a.play(ch)
	a.epg.show = true
	tt := ui.NewTester(a.view, 1000, 620)
	tt.SetTitleBar(ui.TitleBar{Height: 40, Right: 138})
	for _, scale := range []float32{1, 1.5} {
		tt.SetSize(1000, 620)
		tt.SetScale(scale)
		bar := requiredRect(t, tt, "播放控制栏")
		for _, label := range []string{"暂停", "停止", "更多播放操作", "窗口全屏", "全屏"} {
			contained(t, label, bar, requiredRect(t, tt, label))
		}
	}
	if err := tt.Click("更多播放操作"); err != nil {
		t.Fatal(err)
	}
	previousControls := a.controls
	err := tt.ChooseMenuItem("音量 75%")
	if embedhost.Supported {
		if err != nil {
			t.Fatal(err)
		}
		if a.controls.Volume != 75 {
			t.Fatal("volume preset did not apply")
		}
	} else {
		if err == nil || !strings.Contains(err.Error(), `the item "音量 75%" is disabled`) {
			t.Fatalf("expected disabled volume preset without embedded controls, got %v", err)
		}
		if a.controls != previousControls {
			t.Fatal("disabled volume preset changed playback controls")
		}
		// A rejected selection leaves the menu open; close it before testing routes.
		tt.CloseMenu()
	}
	if err := tt.Click("更多播放操作"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(lineLabels(ch)[1]); err != nil {
		t.Fatal(err)
	}
	if len(engine.streams) != 2 || a.nowURL != ch.Lines[1].URL {
		t.Fatal("compact menu could not switch routes")
	}
	tt.SetScale(1)
	tt.SetSize(1280, 820)
	bar := requiredRect(t, tt, "播放控制栏")
	for _, label := range []string{"播放线路", "静音", "音量", "窗口全屏", "全屏"} {
		contained(t, label, bar, requiredRect(t, tt, label))
	}
}

func subscriptionPlaybackFixture(t *testing.T) (*application, *fakeEngine, chan func(), playlist.Channel) {
	a, engine, queue, ch := playbackFixture(t)
	lines := playlist.Lines(ch)
	a.state.Subscriptions = []store.Subscription{
		{ID: "a", Name: "A", Channels: []playlist.Channel{{Name: ch.Name, URL: lines[0].URL}}},
		{ID: "b", Name: "B", Channels: []playlist.Channel{{Name: ch.Name, URL: lines[1].URL}}},
	}
	a.state.RebuildChannels()
	a.play(a.state.Channels[0])
	t.Cleanup(a.cleanup)
	return a, engine, queue, ch
}
func TestDisabledAndDeletedRoutesNeverUsedForFallback(t *testing.T) {
	for _, operation := range []string{"disable", "delete", "delete-all", "pending-rebuild"} {
		t.Run(operation, func(t *testing.T) {
			a, engine, _, ch := subscriptionPlaybackFixture(t)
			switch operation {
			case "disable":
				a.setSubscriptionEnabled("b", false)
			case "delete":
				a.deleteSubscription("b")
			case "delete-all":
				a.deleteSubscription("a")
				a.deleteSubscription("b")
			case "pending-rebuild":
				a.state.UpdateSubscriptionEnabled("b", false) // derived catalog is deliberately stale
			}
			if a.now == "" || len(engine.streams) != 1 {
				t.Fatal("metadata change interrupted current playback")
			}
			select {
			case <-engine.streams[0].done:
				t.Fatal("current stream was stopped")
			default:
			}
			for _, line := range playlist.Lines(a.playbackRoutes()) {
				if line.URL == ch.Lines[1].URL {
					t.Fatal("disabled route remains manually selectable")
				}
			}
			a.lineTimedOut(a.generation)
			if len(engine.streams) != 1 || a.now != "" {
				t.Fatal("fallback reused a disabled/deleted route")
			}
		})
	}
}
func TestFallbackUsesRefreshedRoutesBeforeCatalogIsApplied(t *testing.T) {
	a, engine, _, _ := subscriptionPlaybackFixture(t)
	// Model a refresh whose raw cache has arrived but whose catalog build is pending.
	a.state.Subscriptions[1].Channels = []playlist.Channel{{Name: "Demo", URL: "https://example.com/new"}}
	a.lineTimedOut(a.generation)
	if len(engine.streams) != 2 || a.nowURL != "https://example.com/new" {
		t.Fatal("fallback ignored the newest raw route cache")
	}
}
func TestSingleSourceScopeSurvivesLibrarySwitchAndProviderRemoval(t *testing.T) {
	a, engine, _, _ := subscriptionPlaybackFixture(t)
	a.stop()
	a.play(a.state.SourceChannels["b"][0])
	a.state.SelectedSource = "a"
	a.setSubscriptionEnabled("b", false)
	before := len(engine.streams)
	a.lineTimedOut(a.generation)
	if len(engine.streams) != before || a.now != "" {
		t.Fatal("single-source playback escaped its source scope")
	}
}
func TestSharedURLKeepsEPGSubscriptionOwnership(t *testing.T) {
	a, _, queue, _ := playbackFixture(t)
	now := time.Now()
	ch := playlist.Channel{Name: "Demo", TVGID: "one", URL: "https://example.com/shared"}
	a.state.Subscriptions = []store.Subscription{
		{ID: "a", Name: "A", Channels: []playlist.Channel{ch}, EPGURLs: []string{"epg-a"}},
		{ID: "b", Name: "B", Channels: []playlist.Channel{ch}, EPGURLs: []string{"epg-b"}},
	}
	a.state.RebuildChannels()
	guide := func(title string) *epg.Guide {
		return &epg.Guide{Programmes: map[string][]epg.Programme{"one": {{Start: now.Add(-time.Hour), Stop: now.Add(time.Hour), Title: title}}}}
	}
	a.epg.jobs = map[string]*epgJob{
		"epg-a": {guide: guide("节目 A"), nextTry: now.Add(time.Hour)},
		"epg-b": {guide: guide("节目 B"), nextTry: now.Add(time.Hour)},
	}
	t.Cleanup(a.cleanup)
	a.syncEPG(false)
	b := a.state.SourceChannels["b"][0]
	if b.SourceID != "b" || b.Lines[0].SourceID != "b" {
		t.Fatal("derived catalog lost source ownership")
	}
	if ps := a.scheduleAt(b, now); len(ps) == 0 || ps[0].Title != "节目 B" {
		t.Fatalf("source B got wrong EPG: %v", ps)
	}
	a.play(b)
	a.state.SelectedSource = "a"
	if a.activeSourceID != "b" || a.scheduleAt(a.playingEPGChannel(), now)[0].Title != "节目 B" {
		t.Fatal("library selection changed playing stream EPG")
	}
	pumpPlayback(t, queue, func() bool { return !a.epg.cleanupBusy })
}
func TestCalendarBuildIsAsyncCancellableAndRejectsOldResults(t *testing.T) {
	now := time.Now()
	a := v2CalendarApp(now, 10000)
	a.path = filepath.Join(t.TempDir(), "state.json")
	queue := make(chan func(), 16)
	a.dispatch = func(fn func()) { queue <- fn }
	t.Cleanup(a.cleanup)
	a.prepareEPGCalendar(now)
	firstToken := a.epg.calendarRequest
	if !a.epg.calendarLoading || len(a.epg.calendar.Dates) != 0 {
		t.Fatal("large calendar was built synchronously")
	}
	old := epg.CalendarFor(v2Guide(now, 3).Programmes["one"], now)
	a.activeChannel.Key = "another-channel"
	a.prepareEPGCalendar(now)
	a.applyEPGCalendar(firstToken, old, now.Format("2006-01-02"))
	if len(a.epg.calendar.Dates) != 0 {
		t.Fatal("late calendar result replaced newer channel")
	}
	pumpPlayback(t, queue, func() bool { return !a.epg.calendarLoading })
	if len(a.epg.calendar.Days[a.epg.date]) != 10000 {
		t.Fatal("new calendar was not applied")
	}
	// Closing releases indexed data and invalidates already-posted completion callbacks.
	token := a.epg.calendarRequest
	a.closeEPGPanel()
	a.applyEPGCalendar(token, old, now.Format("2006-01-02"))
	if a.epg.show || len(a.epg.calendar.Dates) != 0 {
		t.Fatal("closed panel was revived by a stale result")
	}
}
func TestStaleChannelClickCannotReviveRemovedSubscription(t *testing.T) {
	a, engine, _, _ := subscriptionPlaybackFixture(t)
	stale := a.state.Channels[0]
	a.state.Subscriptions = nil // authoritative metadata changed; catalog is stale
	a.stop()
	a.play(stale)
	if len(engine.streams) != 1 || a.now != "" {
		t.Fatal("stale UI channel revived deleted subscription")
	}
}
func TestAggregateLineSwitchUpdatesPlayingEPGOwner(t *testing.T) {
	a, _, queue, _ := playbackFixture(t)
	now := time.Now()
	a.state.Subscriptions = []store.Subscription{
		{ID: "a", Name: "A", EPGURLs: []string{"guide-a"}, Channels: []playlist.Channel{{Name: "Demo", TVGID: "one", URL: "https://example.com/a"}}},
		{ID: "b", Name: "B", EPGURLs: []string{"guide-b"}, Channels: []playlist.Channel{{Name: "Demo", TVGID: "one", URL: "https://example.com/b"}}},
	}
	a.state.RebuildChannels()
	a.epg.jobs = map[string]*epgJob{
		"guide-a": {guide: v2Guide(now, 2), nextTry: now.Add(time.Hour)},
		"guide-b": {guide: v2Guide(now, 3), nextTry: now.Add(time.Hour)},
	}
	t.Cleanup(a.cleanup)
	a.syncEPG(false)
	ch := a.state.Channels[0]
	a.play(ch)
	if a.activeSourceID != "a" {
		t.Fatal("initial source not recorded")
	}
	a.startLine(ch, 1, map[string]bool{})
	if a.activeSourceID != "b" || len(a.scheduleAt(a.playingEPGChannel(), now)) != 9 {
		t.Fatal("EPG did not follow active provider")
	}
	pumpPlayback(t, queue, func() bool { return !a.epg.cleanupBusy })
}

func TestCalendarCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := epg.CalendarForContext(ctx, nil, time.Now()); err != context.Canceled {
		t.Fatalf("cancelled build returned %v", err)
	}
}
