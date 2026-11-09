package store

import (
	"path/filepath"
	"testing"
)

func TestPlaybackEnginePersistsAndTransfers(t *testing.T) {
	for _, engine := range []string{"", "mpv", "webview"} {
		t.Run(engine, func(t *testing.T) {
			state := State{PlaybackEngine: engine, Player: "keep-mpv-path", Favorites: map[string]bool{}}
			file := filepath.Join(t.TempDir(), "state.json")
			if err := Save(file, state); err != nil {
				t.Fatal(err)
			}
			loaded, err := Load(file)
			if err != nil || loaded.PlaybackEngine != engine || loaded.Player != state.Player {
				t.Fatalf("settings lost: %+v %v", loaded, err)
			}
			backup := filepath.Join(t.TempDir(), "backup.json")
			if err := Export(backup, file, state); err != nil {
				t.Fatal(err)
			}
			restored, err := ReadBackup(backup)
			if err != nil || restored.PlaybackEngine != engine || restored.Player != state.Player {
				t.Fatalf("backup lost engine: %+v %v", restored, err)
			}
		})
	}
	if err := validateTransfer(State{PlaybackEngine: "vlc"}); err == nil {
		t.Fatal("unknown engine accepted")
	}
}
