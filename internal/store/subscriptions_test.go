package store

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func TestSubscriptionMergeEditFailureAndDelete(t *testing.T) {
	s := State{Favorites: map[string]bool{"https://media.example/a": true}}
	a, err := s.UpsertSubscription("", "A", "https://example.org/a.m3u")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.UpsertSubscription("", "B", "https://example.org/b.m3u")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Second)
	ac := []playlist.Channel{{Name: "A", URL: "https://media.example/a"}, {Name: "Shared", URL: "https://media.example/shared"}}
	bc := []playlist.Channel{{Name: "Shared B", URL: "https://media.example/shared"}, {Name: "B", URL: "https://media.example/b"}}
	if !s.ApplyRefresh(a, 1, ac, nil, at) || !s.ApplyRefresh(b, 1, bc, nil, at) || len(s.Channels) != 3 {
		t.Fatalf("merge: %+v", s.Channels)
	}
	s.ApplyRefresh(a, 1, nil, fmt.Errorf("HTTP 403"), at.Add(time.Hour))
	if len(s.Channels) != 3 || len(s.FindSubscription(a).Channels) != 2 || s.FindSubscription(a).UpdatedAt != at || s.FindSubscription(a).LastError == "" {
		t.Fatal("failure erased usable cache")
	}
	if _, err := s.UpsertSubscription(a, "Edited", "https://example.org/new.m3u"); err != nil {
		t.Fatal(err)
	}
	if s.ApplyRefresh(a, 1, nil, nil, at) {
		t.Fatal("accepted stale response after edit")
	}
	if len(s.FindSubscription(a).Channels) != 2 {
		t.Fatal("edit prematurely erased cache")
	}
	if !s.DeleteSubscription(a) || len(s.Channels) != 2 || !s.Favorites["https://media.example/a"] {
		t.Fatal("delete did not preserve other subscriptions or favorites")
	}
	if s.ApplyRefresh(a, 2, ac, nil, at) {
		t.Fatal("deleted subscription was resurrected")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || len(loaded.Subscriptions) != 1 || len(loaded.Channels) != 2 || loaded.Subscriptions[0].ID != b {
		t.Fatalf("roundtrip: %+v %v", loaded, err)
	}
}

func TestMultiLineCatalogAndLegacyFavorite(t *testing.T) {
	s := State{Favorites: map[string]bool{"https://media.example/one": true}}
	a, _ := s.UpsertSubscription("", "Source A", "https://example.org/a")
	b, _ := s.UpsertSubscription("", "Source B", "https://example.org/b")
	s.ApplyRefresh(a, 1, []playlist.Channel{{Name: "Demo", URL: "https://media.example/one"}}, nil, time.Now())
	s.ApplyRefresh(b, 1, []playlist.Channel{{Name: "Demo", TVGID: "demo", URL: "https://media.example/two"}}, nil, time.Now())
	if len(s.Channels) != 1 || len(s.Channels[0].Lines) != 2 || s.Channels[0].Lines[1].Label != "Source B" {
		t.Fatal("same channel was not merged into labeled routes")
	}
	key := playlist.ChannelKey(s.Channels[0])
	s.SetSubscriptionEnabled(a, false)
	if len(s.Channels) != 1 || len(s.Channels[0].Lines) != 1 || !s.Favorites["channel:"+key] {
		t.Fatal("legacy favorite was lost when a route disappeared")
	}
	s.SetSubscriptionEnabled(a, true)
	s.SetSubscriptionEnabled(b, false)
	if playlist.ChannelKey(s.Channels[0]) != key {
		t.Fatal("disabling the ID-bearing source changed channel identity")
	}
}

func TestEnableAndOrdering(t *testing.T) {
	s := State{}
	a, _ := s.UpsertSubscription("", "A", "https://example.org/a")
	b, _ := s.UpsertSubscription("", "B", "https://example.org/b")
	for _, id := range []string{a, b} {
		s.ApplyRefresh(id, 1, []playlist.Channel{{Name: id, URL: "https://example.org/shared"}}, nil, time.Now())
	}
	if s.Channels[0].Name != a {
		t.Fatal("initial priority incorrect")
	}
	if s.MoveSubscription(a, -1) || s.MoveSubscription(b, 1) {
		t.Fatal("accepted move beyond boundary")
	}
	if !s.MoveSubscription(b, -1) || s.Channels[0].Name != b {
		t.Fatal("ordering did not update deduplication priority")
	}
	if !s.SetSubscriptionEnabled(b, false) || s.Channels[0].Name != a || len(s.FindSubscription(b).Channels) != 1 {
		t.Fatal("disable should preserve cache and use next enabled source")
	}
	if s.ApplyRefresh(b, s.FindSubscription(b).Revision, nil, nil, time.Now()) {
		t.Fatal("disabled subscription accepted a response")
	}
	if !s.SetSubscriptionEnabled(b, true) || s.Channels[0].Name != b {
		t.Fatal("enable did not restore cached channels")
	}
}

func TestLegacyMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	old := State{Channels: []playlist.Channel{{Name: "Old", URL: "https://example.org/live"}}, Favorites: map[string]bool{"https://example.org/live": true}, Player: "mpv"}
	if err := Save(path, old); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil || len(s.Subscriptions) != 1 || s.Subscriptions[0].Source != "" || len(s.Channels) != 1 || !s.Favorites[old.Channels[0].URL] || s.Player != "mpv" {
		t.Fatalf("migration lost data: %+v %v", s, err)
	}
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	s, err = Load(path)
	if err != nil || len(s.Subscriptions) != 1 {
		t.Fatal("migration duplicated entries")
	}
}

func TestSubscriptionValidation(t *testing.T) {
	s := State{}
	for _, source := range []string{"", "ftp://example.org/list", "https:///missing"} {
		if _, err := s.UpsertSubscription("", "Test", source); err == nil {
			t.Fatalf("accepted %q", source)
		}
	}
	id, err := s.UpsertSubscription("", "", "https://example.org/a.m3u#fragment")
	if err != nil {
		t.Fatal(err)
	}
	if s.FindSubscription(id).Name != "example.org" {
		t.Fatal("default name missing")
	}
	if _, err := s.UpsertSubscription("", "Duplicate", "https://example.org/a.m3u"); err == nil {
		t.Fatal("accepted duplicate")
	}
	if _, err := s.UpsertSubscription("missing", "Name", "https://example.org/new"); err == nil {
		t.Fatal("accepted nonexistent edit")
	}
}

func TestMultipleHTTPSubscriptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			http.Error(w, "Unavailable", 503)
			return
		}
		fmt.Fprintf(w, "#EXTM3U\n#EXTINF:-1 group-title=\"Test\",%s\nhttps://media.example%s/live.m3u8\n", r.URL.Path, r.URL.Path)
	}))
	defer server.Close()
	s := State{}
	for _, path := range []string{"/a", "/b"} {
		id, err := s.UpsertSubscription("", path, server.URL+path)
		if err != nil {
			t.Fatal(err)
		}
		ch, err := playlist.Load(context.Background(), server.URL+path)
		s.ApplyRefresh(id, 1, ch, err, time.Now())
	}
	if len(s.Channels) != 2 {
		t.Fatal(s.Channels)
	}
	sub := s.Subscriptions[0]
	_, err := s.UpsertSubscription(sub.ID, "Broken", server.URL+"/fail")
	if err != nil {
		t.Fatal(err)
	}
	ch, loadErr := playlist.Load(context.Background(), server.URL+"/fail")
	s.ApplyRefresh(sub.ID, 2, ch, loadErr, time.Now())
	if len(s.Channels) != 2 || s.FindSubscription(sub.ID).LastError == "" {
		t.Fatal("failed source wiped other streams or cache")
	}
}
