package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

type EPGSourceEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
}

const legacyEPGID = "legacy-epg"

// The old single fallback remains readable until a manual source is edited.
func (s State) ManualEPGSources() []EPGSourceEntry {
	entries := append([]EPGSourceEntry(nil), s.EPGSources...)
	if s.EPGSource != "" {
		for _, entry := range entries {
			if entry.Source == s.EPGSource {
				return entries
			}
		}
		entries = append([]EPGSourceEntry{{ID: legacyEPGID, Name: "备用 EPG", Source: s.EPGSource}}, entries...)
	}
	return entries
}

func (s *State) UpsertEPGSource(id, name, raw string) (string, error) {
	source, err := NormalizeSource(raw)
	if err != nil {
		return "", err
	}
	entries := s.ManualEPGSources()
	index := -1
	for i, entry := range entries {
		if entry.ID == id {
			index = i
		} else if entry.Source == source {
			return "", fmt.Errorf("该 EPG 地址已存在，请编辑或刷新原来源")
		}
	}
	if id != "" && index < 0 {
		return "", fmt.Errorf("EPG 来源已不存在")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(source)
		if u, err := url.Parse(source); err == nil && u.Hostname() != "" {
			name = u.Hostname()
		}
	}
	if id == "" {
		var bytes [16]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			return "", err
		}
		id = hex.EncodeToString(bytes[:])
		entries = append(entries, EPGSourceEntry{ID: id, Name: name, Source: source})
	} else {
		entries[index] = EPGSourceEntry{ID: id, Name: name, Source: source}
	}
	s.EPGSources, s.EPGSource = entries, ""
	return id, nil
}

func (s *State) MoveEPGSource(id string, delta int) bool {
	if delta != -1 && delta != 1 {
		return false
	}
	entries := s.ManualEPGSources()
	for i, entry := range entries {
		if entry.ID != id {
			continue
		}
		j := i + delta
		if j < 0 || j >= len(entries) {
			return false
		}
		entries[i], entries[j] = entries[j], entries[i]
		s.EPGSources, s.EPGSource = entries, ""
		return true
	}
	return false
}

func (s *State) RemoveEPGSource(id string) bool {
	entries := s.ManualEPGSources()
	for i, entry := range entries {
		if entry.ID == id {
			s.EPGSources = append(entries[:i], entries[i+1:]...)
			s.EPGSource = ""
			return true
		}
	}
	return false
}
