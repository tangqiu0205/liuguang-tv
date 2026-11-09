package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPortableModeMarker(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	if err := os.MkdirAll(app, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(app, "liuguangtv-portable.mode")
	for _, tc := range []struct{ goos, exe string }{
		{"windows", filepath.Join(app, "LiuguangTV.exe")},
		{"linux", filepath.Join(app, "LiuguangTV")},
		{"darwin", filepath.Join(app, "LiuguangTV.app", "Contents", "MacOS", "LiuguangTV")},
	} {
		if err := os.WriteFile(marker, nil, 0600); err != nil {
			t.Fatal(err)
		}
		if got, err := portableMode(tc.exe, tc.goos); err != nil || !got {
			t.Fatalf("%s: got %v, %v", tc.goos, got, err)
		}
		if err := os.Remove(marker); err != nil {
			t.Fatal(err)
		}
		if got, err := portableMode(tc.exe, tc.goos); err != nil || got {
			t.Fatalf("%s without marker: got %v, %v", tc.goos, got, err)
		}
	}
	if err := os.Mkdir(marker, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := portableMode(filepath.Join(app, "LiuguangTV"), "linux"); err == nil {
		t.Fatal("accepted non-file marker")
	}
}

func TestMissingMarkerDoesNotHidePortableData(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	if err := os.MkdirAll(app, 0700); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(root, "data")
	if err := os.Mkdir(data, 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ goos, exe string }{
		{"windows", filepath.Join(app, "LiuguangTV.exe")},
		{"linux", filepath.Join(app, "liuguangtv")},
		{"darwin", filepath.Join(app, "LiuguangTV.app", "Contents", "MacOS", "LiuguangTV")},
	} {
		if _, err := portableMode(tc.exe, tc.goos); err == nil {
			t.Errorf("%s: missing marker silently accepted despite existing data", tc.goos)
		}
	}
	if err := os.Remove(data); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(data, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := portableMode(filepath.Join(app, "liuguangtv"), "linux"); err == nil {
		t.Fatal("data path collision silently accepted")
	}
}

func TestRootWindowsPortableLayout(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "LiuguangTV.exe")
	marker := filepath.Join(root, "portable.flag")
	if err := os.WriteFile(marker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if portable, err := portableMode(exe, "windows"); err != nil || !portable {
		t.Fatalf("new Windows portable mode: %v, %v", portable, err)
	}
	got, err := portablePath(exe, "windows")
	if err != nil || got != filepath.Join(root, "data", "state.json") {
		t.Fatalf("new Windows portable path: %q, %v", got, err)
	}
	if err := os.MkdirAll(filepath.Dir(got), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if _, err := portableMode(exe, "windows"); err == nil {
		t.Fatal("missing new marker silently switched to installed data")
	}
	if err := os.Mkdir(marker, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := portableMode(exe, "windows"); err == nil {
		t.Fatal("accepted a directory as portable.flag")
	}
}

func TestConflictingPortableMarkersFail(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"portable.flag", "liuguangtv-portable.mode"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := portableMode(filepath.Join(root, "LiuguangTV.exe"), "windows"); err == nil {
		t.Fatal("ambiguous data layout accepted")
	}
}

func TestLegacyPortableLayoutSurvivesExecutableUpdate(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	data := filepath.Join(root, "data")
	for _, dir := range []string{app, data} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	state := filepath.Join(data, "state.json")
	if err := os.WriteFile(state, []byte("old user data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "liuguangtv-portable.mode"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(app, "LiuguangTV.exe")
	if err := os.WriteFile(exe, []byte("updated executable"), 0700); err != nil {
		t.Fatal(err)
	}
	if portable, err := portableMode(exe, "windows"); err != nil || !portable {
		t.Fatalf("legacy marker lost after update: %v", err)
	}
	if got, err := portablePath(exe, "windows"); err != nil || got != state {
		t.Fatalf("legacy data redirected: %q, %v", got, err)
	}
	if contents, err := os.ReadFile(state); err != nil || string(contents) != "old user data" {
		t.Fatal("legacy data changed")
	}
}

func TestPortablePath(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct{ os, exe string }{
		{"windows", filepath.Join(root, "app", "LiuguangTV.exe")},
		{"linux", filepath.Join(root, "app", "LiuguangTV")},
		{"darwin", filepath.Join(root, "app", "LiuguangTV.app", "Contents", "MacOS", "LiuguangTV")},
	} {
		t.Run(tc.os, func(t *testing.T) {
			got, err := portablePath(tc.exe, tc.os)
			if err != nil || got != filepath.Join(root, "data", "state.json") {
				t.Fatalf("portablePath = %q, %v", got, err)
			}
		})
	}
	if _, err := portablePath(filepath.Join(root, "LiuguangTV"), "linux"); err == nil {
		t.Fatal("accepted executable outside app directory")
	}
}
