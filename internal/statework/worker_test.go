package statework

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

func TestLatestSnapshotFlushAndOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	var callbacks atomic.Int32
	w := New(path, func(Result) { callbacks.Add(1) })
	state := store.State{Favorites: map[string]bool{}, Subscriptions: []store.Subscription{{ID: "one", Name: "Initial", Channels: []playlist.Channel{{Name: "Demo", URL: "https://example.org/live"}}}}}
	for i := 1; i <= 100; i++ {
		state.Player = fmt.Sprintf("mpv-%d", i)
		state.Subscriptions[0].Name = fmt.Sprintf("Source %d", i)
		state.Favorites[fmt.Sprint(i)] = true
		if !w.Submit(Request{Sequence: uint64(i), State: state.Snapshot(), Build: true}) {
			t.Fatal("submit rejected")
		}
	}
	if w.Submit(Request{Sequence: 50, State: state.Snapshot()}) {
		t.Fatal("accepted an older sequence")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(path)
	if err != nil || loaded.Player != "mpv-100" || loaded.Subscriptions[0].Name != "Source 100" || len(loaded.Channels) != 1 || len(loaded.Favorites) != 100 {
		t.Fatalf("latest snapshot was not flushed: %+v %v", loaded, err)
	}
	if w.Submit(Request{Sequence: 101}) {
		t.Fatal("accepted after close")
	}
	t.Logf("100 submissions coalesced into %d completed writes", callbacks.Load())
}
func TestWriteErrorAndNoUICallbackDependency(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	results := make(chan Result, 1)
	w := New(filepath.Join(file, "state.json"), func(result Result) { results <- result })
	w.Submit(Request{Sequence: 1, State: store.State{Player: "mpv"}})
	if err := w.Close(); err == nil {
		t.Fatal("expected a write error")
	}
	if result := <-results; result.Err == nil {
		t.Fatal("UI was not notified of failure")
	}
}
