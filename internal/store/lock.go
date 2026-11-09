package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// Lock holds an operating-system lock for as long as the data directory is in use.
// The lock file must remain in place: removing it could let two processes lock
// different files with the same name.
type Lock struct{ file *os.File }

func LockData(statePath string) (*Lock, error) {
	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	f, err := os.OpenFile(filepath.Join(dir, ".liuguangtv.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("打开数据目录锁失败: %w", err)
	}
	if err := lockFile(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("无法锁定数据目录（可能已有程序使用该目录）: %w", err)
	}
	return &Lock{file: f}, nil
}

func (l *Lock) Close() error { return l.file.Close() }
