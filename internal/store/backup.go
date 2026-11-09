package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"
)

// ExportSummary makes omitted data explicit before the user saves a backup.
type ExportSummary struct {
	Network int
	Local   int
	History int
}

func remoteSource(source string) bool {
	u, err := url.Parse(source)
	return err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

// PrepareExport copies preferences and network subscriptions, never reads local
// files, and leaves the running state and its full caches untouched.
func PrepareExport(state State) (State, ExportSummary, error) {
	s := state.Snapshot()
	normalizeState(&s)
	// Preserve favorite identity even if a name/ID alias changes after refresh.
	for key, urls := range s.FavoriteURLs {
		if s.Favorites["channel:"+key] {
			for _, raw := range urls {
				s.Favorites[raw] = true
			}
		}
	}
	var summary ExportSummary
	subscriptions := make([]Subscription, 0, len(s.Subscriptions))
	selected := false
	for _, sub := range s.Subscriptions {
		if sub.Source == "" {
			summary.History++
			continue
		}
		if !remoteSource(sub.Source) {
			summary.Local++
			continue
		}
		sub.Channels, sub.EPGURLs = nil, nil
		sub.UpdatedAt, sub.CheckedAt, sub.LastError = time.Time{}, time.Time{}, ""
		sub.ChannelCount = 0
		subscriptions = append(subscriptions, sub)
		summary.Network++
		selected = selected || sub.ID == s.SelectedSource
	}
	s.Subscriptions = subscriptions
	s.Channels, s.SourceChannels, s.FavoriteURLs, s.EPGChecks = nil, nil, nil, nil
	if !selected {
		s.SelectedSource = ""
	}
	if err := validateLightBackup(s); err != nil {
		return State{}, summary, err
	}
	return s, summary, nil
}

func validateLightBackup(s State) error {
	if len(s.Channels) != 0 {
		return fmt.Errorf("备份不能包含频道缓存")
	}
	for _, sub := range s.Subscriptions {
		if !remoteSource(sub.Source) {
			return fmt.Errorf("备份仅支持 HTTP(S) 订阅链接，不支持本地或无链接订阅")
		}
		if len(sub.Channels) != 0 || len(sub.EPGURLs) != 0 {
			return fmt.Errorf("备份不能包含频道或订阅 EPG 缓存")
		}
	}
	return validateTransfer(s)
}

// Export writes only the current lightweight JSON format. Old ZIP/JSON formats
// are intentionally not supported by the importer.
func Export(target, livePath string, state State) error {
	protected := []string{livePath, livePath + ".bak"}
	for _, sub := range state.Subscriptions {
		if sub.Source != "" && !remoteSource(sub.Source) {
			protected = append(protected, sub.Source)
		}
	}
	for _, entry := range state.ManualEPGSources() {
		if !remoteSource(entry.Source) {
			protected = append(protected, entry.Source)
		}
	}
	for _, filename := range protected {
		if same, err := samePath(target, filename); err != nil {
			return err
		} else if same {
			return fmt.Errorf("导出目标不能覆盖当前配置、恢复备份或本地订阅源文件")
		}
	}
	s, _, err := PrepareExport(state)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(configExport{Format: exportFormat, Version: exportVersion, State: &s}, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > MaxExportBytes {
		return fmt.Errorf("导出配置超过 64 MiB")
	}
	return atomicWrite(target, data)
}

// ReadBackup validates without writing configuration or accessing subscriptions.
func ReadBackup(filename string) (State, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return State{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxExportBytes {
		return State{}, fmt.Errorf("备份必须是普通 JSON 文件且不超过 64 MiB")
	}
	f, err := os.Open(filename)
	if err != nil {
		return State{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxExportBytes+1))
	if err != nil {
		return State{}, err
	}
	if len(data) > MaxExportBytes {
		return State{}, fmt.Errorf("导入配置超过 64 MiB")
	}
	var doc configExport
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return State{}, fmt.Errorf("仅支持当前版本的轻量 JSON 备份，不支持 ZIP 或旧版配置")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return State{}, fmt.Errorf("JSON 备份包含多余内容")
	}
	if doc.Format != exportFormat || doc.Version != exportVersion || doc.State == nil {
		return State{}, fmt.Errorf("备份格式或版本不支持，请选择当前版本导出的 JSON 文件")
	}
	s := *doc.State
	if err := validateLightBackup(s); err != nil {
		return State{}, err
	}
	s.EPGChecks = nil
	for i := range s.Subscriptions {
		sub := &s.Subscriptions[i]
		sub.UpdatedAt, sub.CheckedAt, sub.LastError = time.Time{}, time.Time{}, ""
	}
	normalizeState(&s)
	return s, nil
}
