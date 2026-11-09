package app

import (
	"bytes"
	"github.com/egoist/mygo/ui"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigImportSerializesWriterAndAppliesOnlyAfterCommit(t *testing.T) {
	a, p, queue, ch := playbackFixture(t)
	a.state.Favorites = map[string]bool{"old": true}
	if err := store.Save(a.path, a.state); err != nil {
		t.Fatal(err)
	}
	a.startBackground()
	t.Cleanup(a.cleanup)
	a.state.Player = "old-pending"
	a.save()
	a.play(ch)
	imported := store.State{Favorites: map[string]bool{"new": true}, Subscriptions: []store.Subscription{{ID: "new", Name: "New", Source: "https://example.org/subscription"}}}
	imported.RebuildChannels()
	a.commitConfigImport(imported)
	if a.state.Favorites["new"] || a.now == "" {
		t.Fatal("import changed session before durable commit")
	}
	pumpPlayback(t, queue, func() bool { return !a.transferBusy })
	if !a.state.Favorites["new"] || a.state.Favorites["old"] || a.now != "" || len(a.state.Channels) != 0 {
		t.Fatalf("import not installed: %+v", a.state)
	}
	select {
	case <-p.streams[len(p.streams)-1].done:
	default:
		t.Fatal("old playback not stopped")
	}
	disk, err := store.Load(a.path)
	if err != nil || !disk.Favorites["new"] {
		t.Fatalf("stale writer replaced import: %+v %v", disk, err)
	}
	backup, err := store.Load(a.path + ".bak")
	if err != nil || !backup.Favorites["old"] {
		t.Fatalf("previous settings not backed up: %+v %v", backup, err)
	}
	if a.lastImportSnapshot == "" || !strings.Contains(a.status, a.lastImportSnapshot) {
		t.Fatalf("import did not show actual recovery path: %s", a.status)
	}
	recovery, err := store.Load(a.lastImportSnapshot)
	if err != nil || !recovery.Favorites["old"] || recovery.Player != "old-pending" {
		t.Fatalf("snapshot does not include the flushed previous settings: %+v %v", recovery, err)
	}
}
func TestImportRecoverySnapshotSurvivesAutomaticEPGSave(t *testing.T) {
	a, _, queue, _ := playbackFixture(t)
	a.state = store.State{
		Favorites:     map[string]bool{"old": true},
		Subscriptions: []store.Subscription{{ID: "old", Name: "Old", Channels: []playlist.Channel{{Name: "Original", URL: "https://example.org/original"}}}},
	}
	if err := store.Save(a.path, a.state); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(a.path)
	if err != nil {
		t.Fatal(err)
	}
	a.startBackground()
	t.Cleanup(a.cleanup)
	guide := filepath.Join(t.TempDir(), "guide.xml")
	if err := os.WriteFile(guide, []byte("<tv></tv>"), 0600); err != nil {
		t.Fatal(err)
	}
	a.commitConfigImport(store.State{Favorites: map[string]bool{"imported": true}, EPGSource: guide})
	pumpPlayback(t, queue, func() bool {
		job := a.epg.jobs[guide]
		return !a.transferBusy && job != nil && !job.loading
	})
	// EPG completion submits a normal save even when an empty guide is rejected.
	if err := a.background.Close(); err != nil {
		t.Fatal(err)
	}
	rolling, err := store.Load(a.path + ".bak")
	if err != nil || rolling.Favorites["old"] || !rolling.Favorites["imported"] {
		t.Fatalf("automatic save did not exercise backup rotation: %+v %v", rolling, err)
	}
	if a.lastImportSnapshot == "" {
		t.Fatal("missing independent snapshot path")
	}
	recoveredBytes, err := os.ReadFile(a.lastImportSnapshot)
	if err != nil || !bytes.Equal(recoveredBytes, before) {
		t.Fatalf("automatic EPG save changed the original snapshot: %v", err)
	}
	recovered, err := store.Load(a.lastImportSnapshot)
	if err != nil || !recovered.Favorites["old"] || len(recovered.Subscriptions) != 1 {
		t.Fatalf("original favorites/subscriptions cannot be recovered: %+v %v", recovered, err)
	}
}

func TestJSONConfigImportRefetchesNetworkChannels(t *testing.T) {
	a, _, queue, _ := playbackFixture(t)
	t.Cleanup(a.cleanup)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("#EXTM3U\n#EXTINF:-1,Fresh\nhttps://example.org/fresh\n"))
	}))
	defer server.Close()
	filename := filepath.Join(t.TempDir(), "backup.json")
	state := store.State{Subscriptions: []store.Subscription{{ID: "remote", Name: "Remote", Source: server.URL, CheckedAt: time.Now(), Channels: []playlist.Channel{{Name: "Stale", URL: "https://example.org/stale"}}}}}
	if err := store.Export(filename, a.path, state); err != nil {
		t.Fatal(err)
	}
	plan, err := store.ReadBackup(filename)
	if err != nil {
		t.Fatal(err)
	}
	a.commitConfigImport(plan)
	pumpPlayback(t, queue, func() bool { return !a.transferBusy })
	if len(a.state.FindSubscription("remote").Channels) != 0 {
		t.Fatal("network cache was migrated")
	}
	a.autoRefresh = true
	_ = ui.NewTester(a.view, 1280, 820)
	pumpPlayback(t, queue, func() bool { return len(a.state.FindSubscription("remote").Channels) == 1 })
	if a.state.FindSubscription("remote").Channels[0].Name != "Fresh" {
		t.Fatal("network channels not fetched after import")
	}
}

func TestFailedConfigImportKeepsSessionAndPlayback(t *testing.T) {
	a, p, queue, ch := playbackFixture(t)
	a.state.Favorites = map[string]bool{"old": true}
	if err := store.Save(a.path, a.state); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(a.path + ".bak"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(a.path+".bak", 0700); err != nil {
		t.Fatal(err)
	}
	a.play(ch)
	a.commitConfigImport(store.State{Favorites: map[string]bool{"new": true}})
	pumpPlayback(t, queue, func() bool { return !a.transferBusy })
	if !a.state.Favorites["old"] || a.state.Favorites["new"] || a.now == "" || !strings.Contains(a.status, "导入失败") {
		t.Fatal("failed import replaced session")
	}
	select {
	case <-p.streams[len(p.streams)-1].done:
		t.Fatal("failed import stopped playback")
	default:
	}
}
