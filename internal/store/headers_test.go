package store

import (
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"path/filepath"
	"testing"
)

func TestHeadersKeepActiveSubscriptionOwnership(t *testing.T) {
	shared := "https://example.org/live"
	a := playlist.Channel{Name: "Demo", URL: shared, Headers: playlist.StreamHeaders{UserAgent: "disabled"}}
	b := a
	b.Headers.UserAgent = "active"
	s := State{Subscriptions: []Subscription{
		{ID: "a", Disabled: true, Channels: []playlist.Channel{a}},
		{ID: "b", Channels: []playlist.Channel{b}},
	}}
	s.RebuildChannels()
	if s.Channels[0].Lines[0].Headers.UserAgent != "active" || s.SourceChannels["b"][0].Lines[0].Headers.UserAgent != "active" {
		t.Fatal("disabled subscription headers leaked to active route")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.Channels[0].Lines[0].Headers.UserAgent != "active" {
		t.Fatalf("headers lost in cache: %+v %v", loaded, err)
	}
}
