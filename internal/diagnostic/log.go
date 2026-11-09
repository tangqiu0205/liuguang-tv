// Package diagnostic records bounded, allowlisted events, never raw errors or URLs.
package diagnostic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const MaxFileBytes = 1 << 20
const FileCount = 3

type Event uint8

const (
	AppStarted Event = iota
	PlaylistDownload
	PlaylistParse
	PlayerStart
	PlayerExit
	PlayerStop
	PlaybackTimeout
	StateSave
	ConfigImport
	ConfigExport
	PlaylistSkippedReferers
)

var names = [...]string{"app_started", "playlist_download", "playlist_parse", "player_start", "player_exit", "player_stop", "playback_timeout", "state_save", "config_import", "config_export", "playlist_skipped_referers"}

type Logger struct {
	mu      sync.Mutex
	path    string
	lastErr error
}

func Open(dir string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "diagnostic.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	return &Logger{path: path}, nil
}
func (l *Logger) Err() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastErr
}

// Record only stores an event name, source hash, error category and numeric code.
// Headers, paths, channel names, arguments and err.Error() never reach the file.
// Failures remain observable through Err(), shown persistently in settings.
func (l *Logger) Record(event Event, source string, err error, code ...int) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if int(event) >= len(names) {
		l.lastErr = fmt.Errorf("unknown diagnostic event")
		return
	}
	record := struct {
		At     time.Time `json:"at"`
		Event  string    `json:"event"`
		Source string    `json:"source,omitempty"`
		Result string    `json:"result"`
		Code   int       `json:"code,omitempty"`
	}{At: time.Now().UTC(), Event: names[event], Result: "ok"}
	if source != "" {
		sum := sha256.Sum256([]byte(source))
		record.Source = hex.EncodeToString(sum[:8])
	}
	if err != nil {
		record.Result = "error"
		var exit *exec.ExitError
		switch {
		case errors.Is(err, context.Canceled):
			record.Result = "canceled"
		case errors.Is(err, context.DeadlineExceeded):
			record.Result = "deadline"
		case errors.Is(err, os.ErrNotExist):
			record.Result = "not_found"
		case errors.Is(err, os.ErrPermission):
			record.Result = "permission"
		case errors.As(err, &exit):
			record.Result = "process_exit"
			record.Code = exit.ExitCode()
		}
	}
	if len(code) > 0 {
		record.Code = code[0]
	}
	data, marshalErr := json.Marshal(record)
	if marshalErr != nil {
		l.lastErr = marshalErr
		return
	}
	data = append(data, '\n')
	if info, statErr := os.Stat(l.path); statErr == nil {
		if info.Size()+int64(len(data)) > MaxFileBytes {
			if err := l.rotate(); err != nil {
				l.lastErr = err
				return
			}
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		l.lastErr = statErr
		return
	}
	f, writeErr := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if writeErr != nil {
		l.lastErr = writeErr
		return
	}
	_, writeErr = f.Write(data)
	l.lastErr = errors.Join(writeErr, f.Close())
}
func (l *Logger) rotate() error {
	if err := os.Remove(l.path + ".2"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for i := 1; i >= 0; i-- {
		from := l.path
		if i > 0 {
			from += fmt.Sprint(".", i)
		}
		if err := os.Rename(from, l.path+fmt.Sprint(".", i+1)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
