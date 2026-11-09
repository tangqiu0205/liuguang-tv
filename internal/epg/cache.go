package epg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var ErrExpired = errors.New("节目单没有保留范围内的有效节目")

func CachedRetained(dir, source string, now time.Time) (*Guide, time.Time, error) {
	f, err := os.Open(cachePath(dir, source))
	if err != nil {
		return nil, time.Time{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, time.Time{}, err
	}
	g, err := Parse(f)
	if err != nil {
		return nil, time.Time{}, err
	}
	g = g.Retained(now)
	if len(g.Programmes) == 0 {
		return nil, info.ModTime(), ErrExpired
	}
	return g, info.ModTime(), nil
}

var cacheName = regexp.MustCompile(`^[0-9a-f]{64}\.xml$`)
var temporaryName = regexp.MustCompile(`^epg-[0-9]+\.tmp$`)

// Cleanup only touches files owned by this package. Keep recent orphan caches
// for seven days so temporarily disabled subscriptions can recover offline.
// Old active caches expire after 30 days; abandoned staging files after a day.
func Cleanup(ctx context.Context, dir string, sources []string, now time.Time) (int, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	keep := make(map[string]bool, len(sources))
	for _, source := range sources {
		keep[filepath.Base(cachePath(dir, source))] = true
	}
	removed := 0
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		if !entry.Type().IsRegular() {
			continue
		}
		age := 30 * 24 * time.Hour
		switch {
		case temporaryName.MatchString(entry.Name()):
			age = 24 * time.Hour
		case cacheName.MatchString(entry.Name()):
			if !keep[entry.Name()] {
				age = 7 * 24 * time.Hour
			}
		default:
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return removed, err
		}
		if !info.ModTime().Before(now.Add(-age)) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, err
		}
		removed++
	}
	return removed, nil
}
