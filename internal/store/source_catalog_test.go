package store

import (
	"path/filepath"
	"testing"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func TestSourceCatalogScopesRoutesAndSharedURLs(t *testing.T) {
	shared := "https://example.org/shared"
	s := State{Subscriptions: []Subscription{
		{ID: "a", Name: "Source A", Channels: []playlist.Channel{{Name: "Demo", TVGID: "demo", Group: "Group A", URL: shared}, {Name: "Demo", TVGID: "demo", Group: "Group A", URL: "https://example.org/a"}}},
		{ID: "b", Name: "Source B", Channels: []playlist.Channel{{Name: "Demo B", TVGID: "demo", Group: "Group B", URL: shared}, {Name: "Demo B", TVGID: "demo", Group: "Group B", URL: "https://example.org/b"}}},
	}}
	s.RebuildChannels()
	if len(s.Channels) != 1 || len(s.Channels[0].Lines) != 3 {
		t.Fatal("all-source catalog incorrect")
	}
	for _, id := range []string{"a", "b"} {
		view := s.SourceChannels[id]
		if len(view) != 1 || len(view[0].Lines) != 2 || view[0].Key != s.Channels[0].Key || view[0].Lines[0].URL != shared {
			t.Fatalf("source %s lost a shared URL or identity: %+v", id, view)
		}
		for _, line := range view[0].Lines {
			if line.Label != "Source "+map[string]string{"a": "A", "b": "B"}[id] {
				t.Fatal("route contains another source's label")
			}
		}
	}
	if s.SourceChannels["b"][0].Group != "Group B" {
		t.Fatal("source-specific groups were lost")
	}
	s.SelectedSource = "b"
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.SelectedSource != "b" || len(loaded.SourceChannels["b"][0].Lines) != 2 {
		t.Fatal("source selection or derived catalog did not restore")
	}
	s.SetSubscriptionEnabled("b", false)
	if _, exists := s.SourceChannels["b"]; exists {
		t.Fatal("disabled source should not be selectable")
	}
}
