package store

import (
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripAndOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	s, err := Load(path)
	if err != nil || s.Favorites == nil {
		t.Fatalf("%+v %v", s, err)
	}
	s.Channels = []playlist.Channel{{Name: "Test", URL: "https://example.org/live"}}
	s.Favorites[s.Channels[0].URL] = true
	for _, player := range []string{"mpv", "C:\\tools\\mpv.exe"} {
		s.Player = player
		if err := Save(path, s); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil || got.Player != player || !got.Favorites[s.Channels[0].URL] {
			t.Fatalf("%+v %v", got, err)
		}
	}
	if err := os.WriteFile(path, []byte("invalid json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid config error")
	}
}
