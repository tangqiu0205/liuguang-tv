package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func TestSnapshotMetadataIsolation(t *testing.T) {
	s := benchmarkState(20)
	s.Favorites["first"] = true
	s.PreferredLines["channel"] = "https://example.org/first"
	snapshot := s.Snapshot()
	oldName := snapshot.Subscriptions[0].Channels[0].Name
	s.Subscriptions[0].Name = "Edited"
	s.Subscriptions[0].Channels = []playlist.Channel{{Name: "Replaced", URL: "https://example.org/new"}}
	s.Favorites["second"] = true
	s.PreferredLines["channel"] = "https://example.org/second"
	if snapshot.Subscriptions[0].Name == "Edited" || snapshot.Subscriptions[0].Channels[0].Name != oldName || snapshot.Favorites["second"] || snapshot.PreferredLines["channel"] != "https://example.org/first" {
		t.Fatal("snapshot shared mutable metadata")
	}
}
func TestDerivedCatalogNotDuplicatedOnDisk(t *testing.T) {
	s := benchmarkState(40)
	s.RebuildChannels()
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if _, present := document["channels"]; present {
		t.Fatal("derived catalog was serialized a second time")
	}
	loaded, err := Load(path)
	if err != nil || len(loaded.Channels) != len(s.Channels) || len(loaded.Channels[0].Lines) != 4 {
		t.Fatal("deduplicated storage lost source caches or routes")
	}
}
func TestLargeConfigStorage(t *testing.T) {
	s := benchmarkState(50000)
	s.RebuildChannels()
	previous, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	compact, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(compact) >= len(previous) {
		t.Fatal("source-only compact storage should be smaller")
	}
	t.Logf("50000 source lines: previous-style JSON %d bytes, compact source-only JSON %d bytes", len(previous), len(compact))
}

func TestDisabledDuplicateDoesNotChangeEnabledPriority(t *testing.T) {
	s := State{Favorites: map[string]bool{}, Subscriptions: []Subscription{
		{ID: "disabled", Name: "Disabled", Disabled: true, Channels: []playlist.Channel{{Name: "Demo", TVGID: "demo", URL: "https://example.org/x"}}},
		{ID: "first", Name: "First", Channels: []playlist.Channel{{Name: "Other", URL: "https://example.org/other"}, {Name: "Demo", TVGID: "demo", URL: "https://example.org/y"}}},
		{ID: "second", Name: "Second", Channels: []playlist.Channel{{Name: "Demo", TVGID: "demo", URL: "https://example.org/x"}}},
	}}
	s.RebuildChannels()
	if len(s.Channels) != 2 || s.Channels[0].Name != "Other" || s.Channels[1].URL != "https://example.org/y" || s.Channels[1].Lines[1].Label != "Second" {
		t.Fatalf("enabled ordering or source labels changed: %+v", s.Channels)
	}
	for _, sub := range s.Subscriptions {
		if sub.ChannelCount != len(playlist.Merge(sub.Channels)) {
			t.Fatalf("cached card count incorrect for %s", sub.ID)
		}
	}
}
