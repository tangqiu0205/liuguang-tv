package diagnostic

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestRedactionAndConcurrentRecords(t *testing.T) {
	dir := t.TempDir()
	log, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	secret := "https://account:password@example.org/private/path?token=credential"
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Record(PlaylistDownload, secret, errors.New(secret+" Referer: secret-header"))
		}()
	}
	wg.Wait()
	log.Record(PlaylistParse, secret, context.Canceled)
	if err := log.Err(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "diagnostic.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"account", "password", "example.org", "private", "credential", "secret-header", "Referer"} {
		if strings.Contains(string(data), s) {
			t.Fatalf("diagnostic leaked %q", s)
		}
	}
	if strings.Count(string(data), "\n") != 11 || !strings.Contains(string(data), "canceled") {
		t.Fatal(string(data))
	}
}
func TestRotationBoundsAndWriteFailure(t *testing.T) {
	dir := t.TempDir()
	log, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "diagnostic.log")
	for range 4 {
		if err := os.WriteFile(path, []byte(strings.Repeat("x", MaxFileBytes)), 0600); err != nil {
			t.Fatal(err)
		}
		log.Record(PlayerExit, "", nil)
		if err := log.Err(); err != nil {
			t.Fatal(err)
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != FileCount {
		t.Fatalf("%v %v", files, err)
	}
	for _, f := range files {
		info, err := f.Info()
		if err != nil || info.Size() > MaxFileBytes {
			t.Fatalf("%v %v", info, err)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	log.Record(PlayerStart, "", nil)
	if log.Err() == nil {
		t.Fatal("write error was hidden")
	}
}
