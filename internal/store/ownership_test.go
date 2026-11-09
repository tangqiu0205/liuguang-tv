package store

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func TestRouteOwnershipIsDerivedAndSurvivesReload(t *testing.T) {
	ch := playlist.Channel{Name: "Demo", TVGID: "one", URL: "https://example.com/shared"}
	s := State{Favorites: map[string]bool{}, Subscriptions: []Subscription{
		{ID: "a", Name: "A", Channels: []playlist.Channel{ch}},
		{ID: "b", Name: "B", Channels: []playlist.Channel{ch}},
	}}
	s.RebuildChannels()
	if s.Channels[0].SourceID != "" || s.Channels[0].Lines[0].SourceID != "a" {
		t.Fatal("aggregate ownership is incorrect")
	}
	for _, id := range []string{"a", "b"} {
		scoped := s.SourceChannels[id][0]
		if scoped.SourceID != id || scoped.Lines[0].SourceID != id {
			t.Fatal("shared URL lost source-scoped ownership")
		}
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SourceChannels["b"][0].Lines[0].SourceID != "b" {
		t.Fatal("source ownership was not reconstructed")
	}
	raw, err := json.Marshal(playlist.Channel{SourceID: "never-persist-this", Lines: []playlist.Line{{SourceID: "never-persist-this", URL: ch.URL}}})
	if err != nil || strings.Contains(string(raw), "never-persist-this") {
		t.Fatal("derived ownership leaked into source cache format")
	}
}
