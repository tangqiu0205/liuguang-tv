package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestImportSnapshotSurvivesRollingBackupsAndRepeatedImports(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	original := recoveryState()
	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}
	before := mustRead(t, path)
	incoming := State{Favorites: map[string]bool{"imported": true}}
	snapshot, err := Import(path, incoming)
	if err != nil || snapshot == "" {
		t.Fatalf("import failed to create snapshot: %s %v", snapshot, err)
	}
	if filepath.Dir(snapshot) != filepath.Join(filepath.Dir(path), "import-backups") || !bytes.Equal(mustRead(t, snapshot), before) {
		t.Fatal("snapshot did not preserve the original file")
	}
	for range 2 {
		if err := Save(path, incoming); err != nil {
			t.Fatal(err)
		}
	}
	rolling, err := Load(path + ".bak")
	if err != nil || !rolling.Favorites["imported"] || rolling.Favorites["saved"] {
		t.Fatalf("expected rolling backup to advance: %+v %v", rolling, err)
	}
	if !bytes.Equal(mustRead(t, snapshot), before) {
		t.Fatal("ordinary saves changed the recovery snapshot")
	}
	second, err := Import(path, State{Favorites: map[string]bool{"second": true}})
	if err != nil || second == snapshot || second == "" {
		t.Fatalf("second import reused snapshot: %s %v", second, err)
	}
	if !bytes.Equal(mustRead(t, snapshot), before) {
		t.Fatal("later import changed the first recovery snapshot")
	}
	previous, err := Load(second)
	if err != nil || !previous.Favorites["imported"] {
		t.Fatalf("second snapshot has wrong configuration: %+v %v", previous, err)
	}
}

func TestImportSnapshotFailureDoesNotOverwriteConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, recoveryState()); err != nil {
		t.Fatal(err)
	}
	before, backup := mustRead(t, path), mustRead(t, path+".bak")
	// A file in place of the directory deterministically prevents snapshots,
	// including on Windows where chmod does not make a directory read-only.
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "import-backups"), []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := Import(path, State{Favorites: map[string]bool{"new": true}})
	if err == nil || snapshot != "" {
		t.Fatalf("snapshot failure was not reported: %s %v", snapshot, err)
	}
	if !bytes.Equal(mustRead(t, path), before) || !bytes.Equal(mustRead(t, path+".bak"), backup) {
		t.Fatal("snapshot failure changed configuration or rolling backup")
	}
}

func TestImportSaveFailureKeepsRecoverySnapshot(t *testing.T) {
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
	snapshot, err := Import(path, State{})
	if err == nil || snapshot == "" {
		t.Fatalf("expected write failure and usable snapshot: %s %v", snapshot, err)
	}
	if !bytes.Equal(mustRead(t, path), before) || !bytes.Equal(mustRead(t, snapshot), before) {
		t.Fatal("failed import lost original data")
	}
}

func TestFirstImportDoesNotInventPreviousConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	snapshot, err := Import(path, State{Favorites: map[string]bool{"new": true}})
	if err != nil || snapshot != "" {
		t.Fatalf("unexpected first-import snapshot: %s %v", snapshot, err)
	}
	got, err := Load(path)
	if err != nil || !got.Favorites["new"] {
		t.Fatalf("first import failed: %+v %v", got, err)
	}
}
