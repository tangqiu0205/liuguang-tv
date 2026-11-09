package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

const MaxExportBytes = 64 << 20
const exportFormat = "liuguangtv-config"
const exportVersion = 3

type configExport struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	State   *State `json:"state"`
}

func validateTransfer(s State) error {
	if s.PlaybackEngine != "" && s.PlaybackEngine != "mpv" && s.PlaybackEngine != "webview" {
		return fmt.Errorf("配置包含无效播放引擎")
	}
	switch s.ThemeMode {
	case "", "system", "light", "dark":
	default:
		return fmt.Errorf("配置包含无效主题")
	}
	ids := map[string]bool{}
	for _, sub := range s.Subscriptions {
		if sub.ID == "" || ids[sub.ID] {
			return fmt.Errorf("配置包含缺失或重复的订阅 ID")
		}
		ids[sub.ID] = true
		if sub.Source != "" {
			if _, err := NormalizeSource(sub.Source); err != nil {
				return fmt.Errorf("配置包含无效订阅地址")
			}
		}
		if err := validateChannels(sub.Channels); err != nil {
			return err
		}
		for _, source := range sub.EPGURLs {
			if !validEPGURL(source) {
				return fmt.Errorf("配置包含无效订阅 EPG 地址")
			}
		}
	}
	if s.SelectedSource != "" && !ids[s.SelectedSource] {
		return fmt.Errorf("配置选中的订阅不存在")
	}
	if s.EPGSource != "" {
		if _, err := NormalizeSource(s.EPGSource); err != nil {
			return fmt.Errorf("配置包含无效备用 EPG 地址")
		}
	}
	epgIDs, epgURLs := map[string]bool{}, map[string]bool{}
	for _, entry := range s.EPGSources {
		if entry.ID == "" || epgIDs[entry.ID] || entry.Source == "" || epgURLs[entry.Source] {
			return fmt.Errorf("配置包含缺失或重复的 EPG 来源")
		}
		if _, err := NormalizeSource(entry.Source); err != nil {
			return fmt.Errorf("配置包含无效 EPG 地址")
		}
		epgIDs[entry.ID], epgURLs[entry.Source] = true, true
	}
	if err := validateChannels(s.Channels); err != nil {
		return err
	}
	for _, raw := range s.PreferredLines {
		if raw != "" && !playlist.ValidStream(raw) {
			return fmt.Errorf("配置包含无效偏好线路")
		}
	}
	return nil
}
func validateChannels(channels []playlist.Channel) error {
	for _, ch := range channels {
		lines := playlist.Lines(ch)
		if ch.Name == "" || len(lines) == 0 {
			return fmt.Errorf("配置包含无名称或无线路的频道")
		}
		if err := ch.Headers.Validate(); err != nil {
			return err
		}
		for _, line := range lines {
			if !playlist.ValidStream(line.URL) {
				return fmt.Errorf("配置包含不安全或无效的播放地址")
			}
			if err := line.Headers.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}
func validEPGURL(raw string) bool {
	return (strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://")) && playlist.ValidStream(raw)
}

// Import replaces configuration after preserving the previous file in a unique
// recovery snapshot. Callers must serialize this with other live configuration
// writes. Snapshots are raw state JSON, separate from the rolling .bak file.
// A returned snapshot remains usable even if the subsequent Save fails.
func Import(path string, s State) (string, error) {
	if err := validateTransfer(s); err != nil {
		return "", err
	}
	previous, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if _, backupErr := os.Stat(path + ".bak"); backupErr == nil {
			return "", fmt.Errorf("主配置缺失但恢复备份存在，请先恢复配置再导入")
		} else if !errors.Is(backupErr, os.ErrNotExist) {
			return "", backupErr
		}
		return "", Save(path, s)
	}
	if err != nil {
		return "", fmt.Errorf("读取导入前配置失败，未覆盖配置: %w", err)
	}
	var valid State
	if err := decodeState(previous, &valid); err != nil {
		return "", fmt.Errorf("原配置损坏，拒绝覆盖；请先恢复配置: %w", err)
	}
	dir := filepath.Join(filepath.Dir(path), "import-backups")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("创建导入恢复快照目录失败，未覆盖配置: %w", err)
	}
	snapshot, err := writeSnapshot(dir, "state-before-import-*.json", previous)
	if err != nil {
		return "", fmt.Errorf("创建导入恢复快照失败，未覆盖配置: %w", err)
	}
	if err := Save(path, s); err != nil {
		return snapshot, fmt.Errorf("写入导入配置失败，导入前配置快照保留于 %s: %w", snapshot, err)
	}
	return snapshot, nil
}

func samePath(a, b string) (bool, error) {
	a, err := filepath.Abs(a)
	if err != nil {
		return false, err
	}
	b, err = filepath.Abs(b)
	if err != nil {
		return false, err
	}
	if a == b || (runtime.GOOS == "windows" && strings.EqualFold(a, b)) {
		return true, nil
	}
	ai, ae := os.Stat(a)
	bi, be := os.Stat(b)
	if ae == nil && be == nil {
		return os.SameFile(ai, bi), nil
	}
	return false, nil
}
