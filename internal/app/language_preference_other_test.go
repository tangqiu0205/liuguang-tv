//go:build !darwin

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMyGoLanguagePreference(t *testing.T) {
	dir := t.TempDir()
	if got, err := readLanguagePreference(dir); err != nil || got != "" {
		t.Fatalf("new profile: %q, %v", got, err)
	}
	path := filepath.Join(dir, "locale")
	for _, tc := range []struct{ saved, want string }{{"en", "en"}, {"zh-Hans", "zh-Hans"}, {" en-US\n", "en"}, {"", ""}} {
		if err := os.WriteFile(path, []byte(tc.saved), 0600); err != nil {
			t.Fatal(err)
		}
		if got, err := readLanguagePreference(dir); err != nil || got != tc.want {
			t.Fatalf("saved %q: got %q, %v; want %q", tc.saved, got, err, tc.want)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := readLanguagePreference(dir); err == nil {
		t.Fatal("unreadable preference reported as system-following mode")
	}
}
