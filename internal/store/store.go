package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

type State struct {
	ThemeMode      string                        `json:"themeMode,omitempty"` // Empty follows the system.
	EPGSources     []EPGSourceEntry              `json:"epgSources,omitempty"`
	EPGSource      string                        `json:"epgSource,omitempty"`
	EPGChecks      map[string]time.Time          `json:"epgChecks,omitempty"`
	Channels       []playlist.Channel            `json:"channels,omitempty"`
	Subscriptions  []Subscription                `json:"subscriptions,omitempty"`
	Favorites      map[string]bool               `json:"favorites"`
	Player         string                        `json:"player"`
	PlaybackEngine string                        `json:"playbackEngine,omitempty"` // Empty keeps the historical mpv default.
	PreferredLines map[string]string             `json:"preferredLines,omitempty"`
	FavoriteURLs   map[string][]string           `json:"-"`
	SourceChannels map[string][]playlist.Channel `json:"-"`
	SelectedSource string                        `json:"selectedSource,omitempty"`
}

func Load(path string) (State, error) {
	s := State{Favorites: map[string]bool{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		backup, backupErr := os.ReadFile(path + ".bak")
		if errors.Is(backupErr, os.ErrNotExist) {
			return s, nil
		}
		if backupErr != nil {
			return s, fmt.Errorf("读取配置备份失败，已禁止覆盖: %w", backupErr)
		}
		if err := decodeState(backup, &s); err != nil {
			return State{Favorites: map[string]bool{}}, fmt.Errorf("主配置缺失且备份无效，已禁止覆盖；请保留 %s.bak: %w", path, err)
		}
		if err := atomicWrite(path, backup); err != nil {
			return State{Favorites: map[string]bool{}}, fmt.Errorf("从备份恢复配置失败，已保留 %s.bak: %w", path, err)
		}
		err = &RecoveryNotice{}
	} else if err != nil {
		return s, err
	} else {
		err = decodeState(data, &s)
		if err != nil {
			backup, backupErr := os.ReadFile(path + ".bak")
			var recovered State
			if backupErr == nil {
				backupErr = decodeState(backup, &recovered)
			}
			if backupErr != nil {
				return State{Favorites: map[string]bool{}}, fmt.Errorf("配置损坏，已禁止覆盖；请保留 %s 并检查 %s.bak: %w", path, path, errors.Join(err, backupErr))
			}
			archive, archiveErr := writeSnapshot(filepath.Dir(path), "state-corrupt-*.json", data)
			if archiveErr != nil {
				return State{Favorites: map[string]bool{}}, fmt.Errorf("保留损坏配置失败，已禁止覆盖: %w", archiveErr)
			}
			if restoreErr := atomicWrite(path, backup); restoreErr != nil {
				return State{Favorites: map[string]bool{}}, fmt.Errorf("恢复配置失败，原文件副本位于 %s: %w", archive, restoreErr)
			}
			s = recovered
			err = &RecoveryNotice{Archive: archive}
		}
	}
	normalizeState(&s)
	return s, err
}

func normalizeState(s *State) {
	if s.Favorites == nil {
		s.Favorites = map[string]bool{}
	}
	if len(s.Subscriptions) == 0 && len(s.Channels) > 0 {
		s.Subscriptions = []Subscription{{ID: "legacy", Name: "历史导入（可编辑绑定订阅）", Channels: s.Channels}}
	}
	s.RebuildChannels()
}
func Save(path string, s State) error {
	// Subscription caches are authoritative. Do not serialize the same lines
	// again as a derived aggregate; legacy states without subscriptions retain it.
	if len(s.Subscriptions) > 0 {
		s.Channels = nil
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	previous, readErr := os.ReadFile(path)
	if readErr == nil {
		var valid State
		if err := decodeState(previous, &valid); err != nil {
			return fmt.Errorf("原配置损坏，拒绝覆盖 %s；请先从 .bak 恢复: %w", path, err)
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	if readErr == nil {
		if err := atomicWrite(path+".bak", previous); err != nil {
			return fmt.Errorf("配置备份失败，未修改原配置: %w", err)
		}
	} else {
		// The first successful save also has a recovery copy.
		if _, err := os.Stat(path + ".bak"); errors.Is(err, os.ErrNotExist) {
			if err := atomicWrite(path+".bak", data); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	return atomicWrite(path, data)
}

// RecoveryNotice reports successful recovery without hiding that data was damaged.
type RecoveryNotice struct{ Archive string }

func (n *RecoveryNotice) Error() string {
	if n.Archive == "" {
		return "主配置缺失，已从 state.json.bak 恢复配置"
	}
	return "已从 state.json.bak 恢复配置；损坏原文件保留于 " + n.Archive
}
func decodeState(data []byte, s *State) error {
	if trimmed := bytes.TrimSpace(data); len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("配置必须是 JSON 对象")
	}
	return json.Unmarshal(data, s)
}

// writeSnapshot creates a unique, private recovery copy and reports success
// only after its complete contents have been synced and the file closed.
func writeSnapshot(dir, pattern string, data []byte) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	err = f.Chmod(0600)
	if err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}
func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "state-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
