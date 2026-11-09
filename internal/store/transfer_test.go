package store

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExportRoundTripAndProtectedPaths(t *testing.T) {
	dir := t.TempDir()
	live, target := filepath.Join(dir, "state.json"), filepath.Join(dir, "export.json")
	s := recoveryState()
	if err := Save(live, s); err != nil {
		t.Fatal(err)
	}
	before := mustRead(t, live)
	if err := Export(target, live, s); err != nil {
		t.Fatal(err)
	}
	imported, err := ReadBackup(target)
	if err != nil || !imported.Favorites["saved"] || len(imported.Subscriptions) != 1 || len(imported.Channels) != 0 || len(imported.Subscriptions[0].Channels) != 0 || imported.Subscriptions[0].Source != s.Subscriptions[0].Source {
		t.Fatalf("%+v %v", imported, err)
	}
	for _, filename := range []string{live, live + ".bak"} {
		if err := Export(filename, live, s); err == nil {
			t.Fatal("export overwrote configuration or recovery backup")
		}
	}
	if !bytes.Equal(mustRead(t, live), before) {
		t.Fatal("export changed live configuration")
	}
	if _, err := Import(live, imported); err != nil {
		t.Fatal(err)
	}
	backup, err := Load(live + ".bak")
	if err != nil || !backup.Favorites["saved"] {
		t.Fatal("import lost previous backup")
	}
}

func TestRejectInvalidTransferBeforeApplying(t *testing.T) {
	original := recoveryState()
	original.Subscriptions[0].Channels = nil
	duplicate := original.Snapshot()
	duplicate.Subscriptions = append(duplicate.Subscriptions, duplicate.Subscriptions[0])
	local := original.Snapshot()
	local.Subscriptions[0].Source = filepath.Join(t.TempDir(), "channels.m3u")
	history := original.Snapshot()
	history.Subscriptions[0].Source = ""
	cached := recoveryState()
	selected := original.Snapshot()
	selected.SelectedSource = "missing"
	for _, doc := range []configExport{
		{Format: exportFormat, Version: exportVersion + 1, State: &original},
		{Format: "other", Version: exportVersion, State: &original},
		{Format: exportFormat, Version: exportVersion},
		{Format: exportFormat, Version: exportVersion, State: &duplicate},
		{Format: exportFormat, Version: exportVersion, State: &local},
		{Format: exportFormat, Version: exportVersion, State: &history},
		{Format: exportFormat, Version: exportVersion, State: &cached},
		{Format: exportFormat, Version: exportVersion, State: &selected},
	} {
		data, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		filename := filepath.Join(t.TempDir(), "invalid.json")
		if err := os.WriteFile(filename, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadBackup(filename); err == nil {
			t.Fatalf("invalid backup accepted: %s", data)
		}
	}
}
