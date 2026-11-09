package store

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func TestJSONBackupExcludesLocalFilesHistoryAndCaches(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "already-deleted.m3u")
	local := filepath.Join(dir, "local.txt")
	localData := []byte("Local,https://example.org/local-file\n")
	if err := os.WriteFile(local, localData, 0600); err != nil {
		t.Fatal(err)
	}
	remote := playlist.Channel{Name: "Demo", TVGID: "demo", URL: "https://example.org/network", Headers: playlist.StreamHeaders{UserAgent: "cached-private-agent"}}
	s := State{
		Favorites: map[string]bool{remote.URL: true, "local-favorite": true},
		Player:    "mpv", ThemeMode: "dark", SelectedSource: "local",
		EPGSource:      "https://example.org/guide.xml",
		EPGChecks:      map[string]time.Time{"https://example.org/guide.xml": time.Now()},
		PreferredLines: map[string]string{playlist.ChannelKey(remote): remote.URL},
		Subscriptions: []Subscription{
			{ID: "remote", Name: "Network", Source: "https://example.org/list?token=secret", CheckedAt: time.Now(), UpdatedAt: time.Now(), LastError: "old error", Channels: []playlist.Channel{remote}, EPGURLs: []string{"https://example.org/old-guide"}},
			{ID: "missing", Name: "Deleted file", Source: missing},
			{ID: "local", Name: "Local file", Source: local, Channels: []playlist.Channel{{Name: "Local cached", URL: "https://example.org/local-cache"}}},
			{ID: "disabled", Name: "Disabled network", Source: "https://example.org/disabled", Disabled: true},
			{ID: "history", Name: "History", Channels: []playlist.Channel{{Name: "Only history", URL: "https://example.org/history"}}},
		},
	}
	prepared, summary, err := PrepareExport(s)
	if err != nil || summary.Network != 2 || summary.Local != 2 || summary.History != 1 {
		t.Fatalf("wrong summary: %+v %v", summary, err)
	}
	if len(prepared.Subscriptions) != 2 || prepared.Subscriptions[0].ID != "remote" || prepared.Subscriptions[1].ID != "disabled" || !prepared.Subscriptions[1].Disabled || prepared.SelectedSource != "" {
		t.Fatalf("wrong exported sources: %+v", prepared)
	}
	if len(s.Subscriptions[0].Channels) != 1 || s.Favorites["channel:"+playlist.ChannelKey(remote)] || s.SelectedSource != "local" {
		t.Fatal("export preparation changed caller state")
	}
	target := filepath.Join(dir, "backup.json")
	if err := Export(target, filepath.Join(dir, "state.json"), s); err != nil {
		t.Fatal(err)
	}
	data := mustRead(t, target)
	if !json.Valid(data) {
		t.Fatal("export is not JSON")
	}
	for _, excluded := range []string{local, missing, "Local cached", "Only history", "cached-private-agent", "old error", "old-guide"} {
		if bytes.Contains(data, []byte(excluded)) {
			t.Fatalf("excluded content leaked: %q", excluded)
		}
	}
	got, err := ReadBackup(target)
	if err != nil || len(got.Subscriptions) != 2 || len(got.Channels) != 0 {
		t.Fatalf("backup did not round-trip: %+v %v", got, err)
	}
	if !got.Favorites["local-favorite"] || !got.Favorites["channel:"+playlist.ChannelKey(remote)] || got.Player != "mpv" || got.ThemeMode != "dark" || got.EPGSource != s.EPGSource || got.PreferredLines[playlist.ChannelKey(remote)] != remote.URL {
		t.Fatal("favorites/settings lost")
	}
	if len(got.EPGChecks) != 0 || len(got.Subscriptions[0].Channels) != 0 || !got.Subscriptions[0].CheckedAt.IsZero() || !got.Subscriptions[0].UpdatedAt.IsZero() {
		t.Fatal("runtime cache/checks were migrated")
	}
	if !bytes.Equal(mustRead(t, local), localData) {
		t.Fatal("export changed local file")
	}
}

func TestJSONBackupAllowsSettingsOnlyAndExcludesLegacyChannels(t *testing.T) {
	s := State{Favorites: map[string]bool{"saved": true}, Channels: []playlist.Channel{{Name: "Legacy", URL: "https://example.org/history"}}}
	prepared, summary, err := PrepareExport(s)
	if err != nil || summary.Network != 0 || summary.History != 1 || len(prepared.Subscriptions) != 0 || len(prepared.Channels) != 0 {
		t.Fatalf("unexpected legacy export: %+v %+v %v", prepared, summary, err)
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := Export(path, filepath.Join(t.TempDir(), "state.json"), s); err != nil {
		t.Fatal(err)
	}
	got, err := ReadBackup(path)
	if err != nil || len(got.Subscriptions) != 0 || !got.Favorites["saved"] {
		t.Fatalf("settings-only backup failed: %+v %v", got, err)
	}
}

func TestJSONExportCannotOverwriteExcludedLocalSource(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.json")
	before := []byte("Local,https://example.org/live\n")
	if err := os.WriteFile(source, before, 0600); err != nil {
		t.Fatal(err)
	}
	s := State{Subscriptions: []Subscription{{ID: "local", Name: "Local", Source: source}}}
	if err := Export(source, filepath.Join(t.TempDir(), "state.json"), s); err == nil {
		t.Fatal("export overwrote excluded source")
	}
	if !bytes.Equal(mustRead(t, source), before) {
		t.Fatal("local source modified")
	}
}

func TestReadBackupRejectsOldFormatsZIPAndTrailingData(t *testing.T) {
	s := State{Favorites: map[string]bool{}}
	for _, version := range []int{1, 2} {
		data, err := json.Marshal(configExport{Format: exportFormat, Version: version, State: &s})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "old.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadBackup(path); err == nil {
			t.Fatalf("old format %d accepted", version)
		}
	}
	valid, err := json.Marshal(configExport{Format: exportFormat, Version: exportVersion, State: &s})
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{[]byte("PK\x03\x04old ZIP"), []byte("{}"), []byte("null"), []byte("not json"), append(valid, []byte(" {}")...)} {
		path := filepath.Join(t.TempDir(), "invalid.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadBackup(path); err == nil {
			t.Fatalf("invalid backup accepted: %q", data)
		}
	}
}

func TestReadBackupRejectsUnknownFieldsAndOversize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.json")
	data := []byte(`{"format":"liuguangtv-config","version":3,"state":{},"localFiles":{"local":"source.txt"}}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBackup(path); err == nil {
		t.Fatal("unknown local-file payload accepted")
	}
	if err := os.Truncate(path, MaxExportBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBackup(path); err == nil || !strings.Contains(err.Error(), "64 MiB") {
		t.Fatalf("oversize not rejected: %v", err)
	}
}
