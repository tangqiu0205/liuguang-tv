package store

import (
	"bytes"
	"errors"
	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func recoveryState() State {
	return State{Player: "mpv", Favorites: map[string]bool{"saved": true},
		Subscriptions: []Subscription{{ID: "one", Name: "Saved", Source: "https://example.org/list", Channels: []playlist.Channel{{Name: "Demo", URL: "https://example.org/live"}}}}}
}
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestRecoverPreservesOriginalAndPreferences(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	original := recoveryState()
	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}
	first := mustRead(t, path)
	changed := original.Snapshot()
	changed.Player = "changed"
	if err := Save(path, changed); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mustRead(t, path+".bak"), first) {
		t.Fatal("backup is not last valid config")
	}
	broken := []byte("{broken")
	if err := os.WriteFile(path, broken, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	var notice *RecoveryNotice
	if !errors.As(err, &notice) {
		t.Fatalf("recovery not reported: %v", err)
	}
	if got.Player != original.Player || !got.Favorites["saved"] || len(got.Subscriptions) != 1 || len(got.Channels) != 1 {
		t.Fatalf("lost saved state: %+v", got)
	}
	if !bytes.Equal(mustRead(t, notice.Archive), broken) || !bytes.Equal(mustRead(t, path), first) || !bytes.Equal(mustRead(t, path+".bak"), first) {
		t.Fatal("recovery overwrote evidence or backup")
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
}
func TestMissingPrimaryRestoresValidBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	original := recoveryState()
	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}
	backup := mustRead(t, path+".bak")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	var notice *RecoveryNotice
	if !errors.As(err, &notice) || notice.Archive != "" {
		t.Fatalf("missing-primary recovery not reported: %v", err)
	}
	if got.Player != original.Player || !got.Favorites["saved"] || len(got.Subscriptions) != 1 || len(got.Channels) != 1 {
		t.Fatalf("backup state not restored: %+v", got)
	}
	if !bytes.Equal(mustRead(t, path), backup) || !bytes.Equal(mustRead(t, path+".bak"), backup) {
		t.Fatal("restore changed or consumed the backup")
	}
	if err := Save(path, got); err != nil {
		t.Fatalf("save after recovery failed: %v", err)
	}
}

func TestMissingPrimaryWithInvalidBackupDoesNotCreateState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	backup := []byte("not valid config")
	if err := os.WriteFile(path+".bak", backup, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err == nil || len(got.Subscriptions) != 0 || len(got.Favorites) != 0 {
		t.Fatalf("invalid backup accepted: %+v %v", got, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing primary unexpectedly created: %v", err)
	}
	if !bytes.Equal(mustRead(t, path+".bak"), backup) {
		t.Fatal("invalid backup was changed")
	}
}

func TestCorruptConfigCannotBeOverwritten(t *testing.T) {
	for _, broken := range []string{"null", "{broken", `{"subscriptions":[],"favorites":"bad"}`} {
		for _, backup := range []string{"missing", "invalid"} {
			t.Run(broken+backup, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "state.json")
				if err := os.WriteFile(path, []byte(broken), 0600); err != nil {
					t.Fatal(err)
				}
				if backup == "invalid" {
					if err := os.WriteFile(path+".bak", []byte("bad backup"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				got, err := Load(path)
				if err == nil || len(got.Subscriptions) != 0 || len(got.Favorites) != 0 {
					t.Fatalf("partial/invalid state exposed: %+v %v", got, err)
				}
				if err := Save(path, State{}); err == nil {
					t.Fatal("corrupt config overwritten")
				}
				if string(mustRead(t, path)) != broken {
					t.Fatal("original lost")
				}
				if backup == "invalid" && string(mustRead(t, path+".bak")) != "bad backup" {
					t.Fatal("backup overwritten")
				}
			})
		}
	}
}
func TestBackupWriteFailurePreservesState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, recoveryState()); err != nil {
		t.Fatal(err)
	}
	before := mustRead(t, path)
	if err := os.Remove(path + ".bak"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".bak", 0700); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, State{}); err == nil {
		t.Fatal("backup failure hidden")
	}
	if !bytes.Equal(mustRead(t, path), before) {
		t.Fatal("failed backup replaced valid config")
	}
	got, err := Load(path)
	if err != nil || !got.Favorites["saved"] || len(got.Subscriptions) != 1 {
		t.Fatalf("saved data lost: %+v %v", got, err)
	}
}
func TestWindowsRestoreAndSaveFailure(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows open file prevents rename")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, recoveryState()); err != nil {
		t.Fatal(err)
	}
	before := mustRead(t, path)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(path, State{}); err == nil {
		f.Close()
		t.Fatal("expected locked target write failure")
	}
	f.Close()
	if !bytes.Equal(mustRead(t, path), before) {
		t.Fatal("write failure lost original")
	}
	broken := []byte("{broken")
	if err := os.WriteFile(path, broken, 0600); err != nil {
		t.Fatal(err)
	}
	f, err = os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var notice *RecoveryNotice
	if _, err := Load(path); err == nil || errors.As(err, &notice) {
		t.Fatalf("restore failure hidden: %v", err)
	}
	if !bytes.Equal(mustRead(t, path), broken) || !bytes.Equal(mustRead(t, path+".bak"), before) {
		t.Fatal("failed restore lost original or backup")
	}
	archives, err := filepath.Glob(filepath.Join(filepath.Dir(path), "state-corrupt-*.json"))
	if err != nil || len(archives) != 1 || !bytes.Equal(mustRead(t, archives[0]), broken) {
		t.Fatal("failed restore did not preserve corrupt evidence")
	}
}
