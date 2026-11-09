package store

import (
	"path/filepath"
	"testing"
)

func TestManualEPGSourcesLegacyMigrationAndCRUD(t *testing.T) {
	s := State{EPGSource: "https://example.org/old.xml"}
	entries := s.ManualEPGSources()
	if len(entries) != 1 || entries[0].ID != legacyEPGID {
		t.Fatal("legacy EPG is not exposed")
	}
	if _, err := s.UpsertEPGSource("", "duplicate", s.EPGSource); err == nil || s.EPGSource == "" {
		t.Fatal("duplicate source mutated legacy configuration")
	}
	id, err := s.UpsertEPGSource("", "节目单", "https://example.org/new.xml.gz")
	if err != nil || id == "" || s.EPGSource != "" || len(s.EPGSources) != 2 {
		t.Fatalf("migration failed: %+v %v", s, err)
	}
	if _, err := s.UpsertEPGSource(legacyEPGID, "旧节目单", "https://example.org/edited.xml"); err != nil {
		t.Fatal(err)
	}
	before := s.Snapshot()
	if _, err := s.UpsertEPGSource("missing", "", "https://example.org/missing.xml"); err == nil {
		t.Fatal("missing source edit was accepted")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || len(loaded.ManualEPGSources()) != 2 {
		t.Fatalf("round trip: %+v %v", loaded, err)
	}
	if !s.RemoveEPGSource(id) || len(s.ManualEPGSources()) != 1 || len(before.EPGSources) != 2 {
		t.Fatal("delete modified snapshot or failed")
	}
	if err := validateTransfer(loaded); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "backup.json")
	if err := Export(backup, path, loaded); err != nil {
		t.Fatal(err)
	}
	imported, err := ReadBackup(backup)
	if err != nil || len(imported.ManualEPGSources()) != 2 {
		t.Fatalf("backup lost manual EPG: %+v %v", imported, err)
	}
}

func TestMoveEPGSourcePreservesLegacyAndPersistsOrder(t *testing.T) {
	s := State{EPGSource: "https://example.org/legacy.xml"}
	id, err := s.UpsertEPGSource("", "second", "https://example.org/second.xml")
	if err != nil {
		t.Fatal(err)
	}
	if s.MoveEPGSource(legacyEPGID, -1) || s.MoveEPGSource(id, 2) || s.MoveEPGSource("missing", 1) {
		t.Fatal("invalid move succeeded")
	}
	if !s.MoveEPGSource(id, -1) || s.ManualEPGSources()[0].ID != id {
		t.Fatal("move did not reorder sources")
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || len(loaded.ManualEPGSources()) != 2 || loaded.ManualEPGSources()[0].ID != id || loaded.ManualEPGSources()[1].ID != legacyEPGID {
		t.Fatalf("EPG order was not preserved: %+v %v", loaded.EPGSources, err)
	}
}

func TestManualEPGValidationAndLocalFileProtection(t *testing.T) {
	s := State{}
	for _, source := range []string{"", "ftp://example.org/guide.xml"} {
		if _, err := s.UpsertEPGSource("", "", source); err == nil {
			t.Fatalf("accepted invalid source %q", source)
		}
	}
	local := filepath.Join(t.TempDir(), "guide.xml")
	if _, err := s.UpsertEPGSource("", "", local); err != nil {
		t.Fatal(err)
	}
	if err := Export(local, filepath.Join(t.TempDir(), "state.json"), s); err == nil {
		t.Fatal("export could overwrite local EPG source")
	}
	copy := s.Snapshot()
	copy.EPGSources[0].Name = "changed"
	if s.EPGSources[0].Name == "changed" {
		t.Fatal("snapshot shares mutable EPG metadata")
	}
}
