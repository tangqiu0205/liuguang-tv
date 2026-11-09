package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestDailyRefreshBoundary(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		last time.Time
		due  bool
	}{
		{time.Time{}, true}, {now, false}, {now.Add(-23 * time.Hour), false}, {now.Add(-24 * time.Hour), true}, {now.Add(-25 * time.Hour), true}, {now.Add(time.Hour), true},
	} {
		if got := dailyRefreshDue(tc.last, now); got != tc.due {
			t.Fatalf("last=%v due=%v", tc.last, got)
		}
	}
}
func TestSubscriptionDailyScheduleAndManualOverride(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail.Load() {
			w.WriteHeader(503)
			return
		}
		fmt.Fprint(w, "#EXTM3U\n#EXTINF:-1,Demo\nhttps://example.com/live\n")
	}))
	defer server.Close()
	queue := make(chan func(), 32)
	now := time.Now()
	a := &application{path: filepath.Join(t.TempDir(), "state.json"), state: store.State{Favorites: map[string]bool{}, Subscriptions: []store.Subscription{{ID: "one", Name: "Demo", Source: server.URL, Revision: 1, UpdatedAt: now}}}}
	a.dispatch = func(fn func()) { queue <- fn }
	t.Cleanup(a.cleanup)
	a.pollSubscriptions()
	if len(a.refreshing) != 0 || calls.Load() != 0 {
		t.Fatal("fresh legacy cache was downloaded again")
	}
	a.state.Subscriptions[0].UpdatedAt = now.Add(-25 * time.Hour)
	a.pollSubscriptions()
	pumpPlayback(t, queue, func() bool { return len(a.refreshing) == 0 && !a.epg.cleanupBusy })
	if calls.Load() != 1 || a.state.Subscriptions[0].CheckedAt.IsZero() {
		t.Fatal("daily refresh did not complete")
	}
	a.pollSubscriptions()
	if len(a.refreshing) != 0 {
		t.Fatal("subscription refreshed twice in a day")
	}
	a.refreshAll() // explicit button bypasses the daily interval
	pumpPlayback(t, queue, func() bool { return len(a.refreshing) == 0 && !a.epg.cleanupBusy })
	if calls.Load() != 2 {
		t.Fatal("manual refresh did not bypass schedule")
	}
	fail.Store(true)
	a.state.Subscriptions[0].CheckedAt = now.Add(-25 * time.Hour)
	a.pollSubscriptions()
	pumpPlayback(t, queue, func() bool { return len(a.refreshing) == 0 })
	if calls.Load() != 3 || a.state.Subscriptions[0].LastError == "" || len(a.state.Subscriptions[0].Channels) != 1 {
		t.Fatal("failure did not retain cache")
	}
	a.pollSubscriptions()
	loaded, err := store.Load(a.path)
	if err != nil || dailyRefreshDue(loaded.Subscriptions[0].CheckedAt, time.Now()) || len(a.refreshing) != 0 {
		t.Fatalf("failed attempt was not persisted for daily scheduling: %v", err)
	}
}
func TestEPGDailyScheduleSurvivesRestartAndKeepsManualRefresh(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	now := time.Now().UTC()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail.Load() {
			w.WriteHeader(503)
			return
		}
		fmt.Fprintf(w, `<tv><programme channel="one" start="%s +0000" stop="%s +0000"><title>Demo</title></programme></tv>`, now.Add(-time.Hour).Format("20060102150405"), now.Add(time.Hour).Format("20060102150405"))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "state.json")
	initial := store.State{Favorites: map[string]bool{}, Subscriptions: []store.Subscription{{ID: "one", Name: "Demo", EPGURLs: []string{server.URL}, Channels: []playlist.Channel{{Name: "Demo", TVGID: "one", URL: "https://example.com/live"}}}}}
	start := func(state store.State) (*application, chan func()) {
		queue := make(chan func(), 32)
		a := &application{path: path, state: state}
		a.dispatch = func(fn func()) { queue <- fn }
		t.Cleanup(a.cleanup)
		a.syncEPG(false)
		pumpPlayback(t, queue, func() bool { return !a.epg.jobs[server.URL].loading && !a.epg.cleanupBusy })
		return a, queue
	}
	a, _ := start(initial)
	if calls.Load() != 1 || a.state.EPGChecks[server.URL].IsZero() {
		t.Fatal("initial EPG check missing")
	}
	a.cleanup()
	state, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b, queue := start(state)
	if calls.Load() != 1 || b.epg.jobs[server.URL].guide == nil {
		t.Fatal("restart downloaded fresh EPG instead of restoring cache")
	}
	b.loadEPG(server.URL)
	pumpPlayback(t, queue, func() bool { return !b.epg.jobs[server.URL].loading })
	if calls.Load() != 2 {
		t.Fatal("manual EPG refresh did not bypass daily interval")
	}
	fail.Store(true)
	b.loadEPG(server.URL)
	pumpPlayback(t, queue, func() bool { return !b.epg.jobs[server.URL].loading })
	if calls.Load() != 3 || b.epg.jobs[server.URL].guide == nil || b.epg.jobs[server.URL].error == "" {
		t.Fatal("failed manual refresh lost cached guide")
	}
	b.pollEPG()
	if b.epg.jobs[server.URL].loading {
		t.Fatal("failed source retried before next daily check")
	}
	b.cleanup()
	state, err = store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := start(state)
	if calls.Load() != 3 || c.epg.jobs[server.URL].guide == nil {
		t.Fatal("restart retried a recently failed EPG request")
	}
	// Old versions only recorded the cache file timestamp, which must also work.
	c.cleanup()
	state.EPGChecks = nil
	d, _ := start(state)
	if calls.Load() != 3 || d.epg.jobs[server.URL].guide == nil {
		t.Fatal("legacy EPG cache was not reused")
	}
}
func TestEditedSourceDoesNotInheritOldDailyCooldown(t *testing.T) {
	now := time.Now()
	state := store.State{Subscriptions: []store.Subscription{{ID: "one", Source: "https://example.com/old", UpdatedAt: now, CheckedAt: now, Revision: 1, Channels: []playlist.Channel{{URL: "https://example.com/live"}}}}}
	if _, err := state.UpsertSubscription("one", "Demo", "https://example.com/new"); err != nil {
		t.Fatal(err)
	}
	sub := state.Subscriptions[0]
	if !sub.CheckedAt.IsZero() || !sub.UpdatedAt.IsZero() || len(sub.Channels) != 1 {
		t.Fatal("new address inherited old cooldown or lost cached channels")
	}
}

func TestDailyMetadataSnapshotIsolation(t *testing.T) {
	now := time.Now()
	original := store.State{EPGChecks: map[string]time.Time{"source": now}, Subscriptions: []store.Subscription{{ID: "one", CheckedAt: now}}}
	snapshot := original.Snapshot()
	original.EPGChecks["source"] = now.Add(time.Hour)
	original.Subscriptions[0].CheckedAt = now.Add(time.Hour)
	if snapshot.EPGChecks["source"] != now || snapshot.Subscriptions[0].CheckedAt != now {
		t.Fatal("daily schedule metadata aliases background snapshot")
	}
}
