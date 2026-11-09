package app

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/statework"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestBackgroundCatalogLatestWins(t *testing.T) {
	queue := make(chan func(), 64)
	a := &application{path: filepath.Join(t.TempDir(), "state.json"), group: "Old", state: store.State{Favorites: map[string]bool{}, Subscriptions: []store.Subscription{{ID: "one", Name: "One", Revision: 1}}}}
	a.dispatch = func(fn func()) { queue <- fn }
	a.state.Subscriptions[0].Channels = []playlist.Channel{{Name: "Initial", Group: "Old", URL: "https://example.org/initial"}}
	a.state.RebuildChannels()
	a.startBackground()
	tt := ui.NewTester(a.view, 1280, 820)
	t.Cleanup(a.cleanup)
	a.state.Subscriptions[0].Channels = []playlist.Channel{{Name: "Old", URL: "https://example.org/old"}}
	a.catalogChanged()
	a.state.Subscriptions[0].Channels = []playlist.Channel{{Name: "Latest", Group: "New", URL: "https://example.org/new"}}
	a.catalogChanged()
	a.state.Favorites["https://example.org/new"] = true
	a.favoritesRevision++
	a.save()
	if err := a.background.Close(); err != nil {
		t.Fatal(err)
	}
	pumpPlayback(t, queue, func() bool { return !a.catalogDirty })
	if len(a.state.Channels) != 1 || a.state.Channels[0].Name != "Latest" || a.state.Subscriptions[0].ChannelCount != 1 || a.group != "全部分组" {
		t.Fatal("newest catalog result was not installed")
	}
	tt.Frame()
	if !tt.HasText("Latest") {
		t.Fatal("background result did not update the live view")
	}
	if !a.isFavorite(a.state.Channels[0]) {
		t.Fatal("a favorite change was overwritten by background merge")
	}
	a.applyBackground(statework.Result{Request: statework.Request{Sequence: 1, Build: true}}, channelIndex{})
	if len(a.state.Channels) != 1 {
		t.Fatal("old worker result replaced current catalog")
	}
	loaded, err := store.Load(a.path)
	if err != nil || loaded.Channels[0].Name != "Latest" || !loaded.Favorites["https://example.org/new"] {
		t.Fatal("latest disk state incorrect")
	}
}
func TestCloseFlushesPendingCatalog(t *testing.T) {
	queue := make(chan func(), 16)
	a := &application{path: filepath.Join(t.TempDir(), "state.json"), state: store.State{Favorites: map[string]bool{}, Subscriptions: []store.Subscription{{ID: "one", Name: "One", Channels: []playlist.Channel{{Name: "Demo", URL: "https://example.org/live"}}}}}}
	a.dispatch = func(fn func()) { queue <- fn }
	a.startBackground()
	a.catalogChanged()
	a.cleanup()
	loaded, err := store.Load(a.path)
	if err != nil || len(loaded.Channels) != 1 {
		t.Fatalf("window close lost pending writes: %v", err)
	}
	for len(queue) > 0 {
		(<-queue)()
	}
	if !a.closed {
		t.Fatal("worker callback reopened closed app")
	}
}
func sameSelection(a, b channelSelection) bool {
	return sameChannels(a.channels, b.channels) && a.all == b.all && len(a.rows) == len(b.rows) && (len(a.rows) == 0 || &a.rows[0] == &b.rows[0])
}

func TestFilterCacheInvalidation(t *testing.T) {
	a := largeChannelApp()
	a.path = filepath.Join(t.TempDir(), "state.json")
	first := a.filteredChannels()
	if !sameSelection(first, a.filteredChannels()) {
		t.Fatal("unchanged filtering should reuse results")
	}
	a.query = "0001"
	filtered := a.filteredChannels()
	if filtered.Len() != 1 || !sameSelection(filtered, a.filteredChannels()) {
		t.Fatal("search results were not cached")
	}
	a.query = ""
	a.favoritesOnly = true
	if a.filteredChannels().Len() != 0 {
		t.Fatal("empty favorites should show no channels")
	}
	a.toggleFavorite(a.state.Channels[0])
	if a.filteredChannels().Len() != 1 {
		t.Fatal("favorite change did not invalidate filter")
	}
	a.state.Channels = []playlist.Channel{{Name: "Replacement", URL: "https://example.org/replacement"}}
	a.favoritesOnly = false
	if got := a.filteredChannels(); got.Len() != 1 || got.At(0).Name != "Replacement" {
		t.Fatal("catalog replacement did not invalidate filter")
	}
}
