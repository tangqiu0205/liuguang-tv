package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDataLockAcrossProcesses(t *testing.T) {
	if path := os.Getenv("LIUGUANG_TEST_LOCK_PATH"); path != "" {
		lock, err := LockData(path)
		if err != nil {
			os.Exit(7)
		}
		_ = lock.Close()
		os.Exit(0)
	}
	path := filepath.Join(t.TempDir(), "data", "state.json")
	lock, err := LockData(path)
	if err != nil {
		t.Fatal(err)
	}
	child := func() error {
		cmd := exec.Command(os.Args[0], "-test.run=^TestDataLockAcrossProcesses$")
		cmd.Env = append(os.Environ(), "LIUGUANG_TEST_LOCK_PATH="+path)
		return cmd.Run()
	}
	if err := child(); err == nil {
		t.Fatal("second process acquired a held lock")
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 7 {
		t.Fatalf("child failed unexpectedly: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if err := child(); err != nil {
		t.Fatalf("lock not released: %v", err)
	}
}
