package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

type Subscription struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Source       string             `json:"source"`
	Channels     []playlist.Channel `json:"channels,omitempty"`
	EPGURLs      []string           `json:"epgUrls,omitempty"`
	UpdatedAt    time.Time          `json:"updatedAt,omitempty"`
	CheckedAt    time.Time          `json:"checkedAt,omitempty"`
	LastError    string             `json:"lastError,omitempty"`
	Revision     uint64             `json:"revision"`
	Disabled     bool               `json:"disabled,omitempty"`
	ChannelCount int                `json:"-"`
}

func NormalizeSource(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("请填写订阅地址或本地文件路径")
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return "", fmt.Errorf("订阅仅支持 HTTP(S) 地址或本地文件")
		}
		u.Fragment = ""
		return u.String(), nil
	}
	path, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("本地文件路径无效")
	}
	return path, nil
}
func (s *State) FindSubscription(id string) *Subscription {
	for i := range s.Subscriptions {
		if s.Subscriptions[i].ID == id {
			return &s.Subscriptions[i]
		}
	}
	return nil
}
func (s *State) UpsertSubscription(id, name, source string) (string, error) {
	source, err := NormalizeSource(source)
	if err != nil {
		return "", err
	}
	for _, sub := range s.Subscriptions {
		if sub.ID != id && sub.Source == source {
			return "", fmt.Errorf("该订阅地址已存在，请编辑或刷新原订阅")
		}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		if u, err := url.Parse(source); err == nil && u.Host != "" {
			name = u.Hostname()
		} else {
			name = filepath.Base(source)
		}
	}
	if id != "" {
		sub := s.FindSubscription(id)
		if sub == nil {
			return "", fmt.Errorf("订阅已不存在")
		}
		sub.Name = name
		if sub.Source != source {
			sub.CheckedAt = time.Time{}
			sub.UpdatedAt = time.Time{} // The old cache does not establish the new address's schedule.
			sub.EPGURLs = nil
		}
		sub.Source = source
		sub.Revision++
		sub.LastError = ""
	} else {
		var bytes [16]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			return "", err
		}
		id = hex.EncodeToString(bytes[:])
		s.Subscriptions = append(s.Subscriptions, Subscription{ID: id, Name: name, Source: source, Revision: 1})
	}
	return id, nil
}

// The metadata-only operations below let desktop callers build/save a snapshot
// on a worker. Synchronous wrappers are retained for headless/domain callers.
func (s *State) DeleteSubscription(id string) bool {
	if !s.RemoveSubscription(id) {
		return false
	}
	s.RebuildChannels()
	return true
}
func (s *State) RemoveSubscription(id string) bool {
	for i, sub := range s.Subscriptions {
		if sub.ID == id {
			s.Subscriptions = append(s.Subscriptions[:i], s.Subscriptions[i+1:]...)
			return true
		}
	}
	return false
}
func (s *State) ApplyRefresh(id string, revision uint64, channels []playlist.Channel, loadErr error, at time.Time) bool {
	if !s.UpdateSubscriptionCache(id, revision, channels, loadErr, at) {
		return false
	}
	s.RebuildChannels()
	return true
}
func (s *State) UpdateSubscriptionCache(id string, revision uint64, channels []playlist.Channel, loadErr error, at time.Time) bool {
	sub := s.FindSubscription(id)
	if sub == nil || sub.Disabled || sub.Revision != revision {
		return false
	}
	sub.CheckedAt = at
	if loadErr != nil {
		sub.LastError = subscriptionErrorMessage(loadErr)
	} else {
		sub.Channels = channels
		sub.UpdatedAt = at
		sub.LastError = ""
	}
	return true
}

// Network and file errors include the requested URL/path in Error(). Keep the
// underlying reason without exposing subscription credentials or local paths.
func subscriptionErrorMessage(err error) string {
	for {
		var requestErr *url.Error
		if errors.As(err, &requestErr) {
			err = requestErr.Err
			continue
		}
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			err = pathErr.Err
			continue
		}
		return "更新失败：" + err.Error()
	}
}

func (s *State) SetSubscriptionEnabled(id string, enabled bool) bool {
	if !s.UpdateSubscriptionEnabled(id, enabled) {
		return false
	}
	s.RebuildChannels()
	return true
}
func (s *State) UpdateSubscriptionEnabled(id string, enabled bool) bool {
	sub := s.FindSubscription(id)
	if sub == nil || sub.Disabled == !enabled {
		return false
	}
	sub.Disabled = !enabled
	sub.Revision++
	return true
}
func (s *State) MoveSubscription(id string, delta int) bool {
	if !s.ReorderSubscription(id, delta) {
		return false
	}
	s.RebuildChannels()
	return true
}
func (s *State) ReorderSubscription(id string, delta int) bool {
	if delta != 1 && delta != -1 {
		return false
	}
	for i, sub := range s.Subscriptions {
		if sub.ID != id {
			continue
		}
		j := i + delta
		if j < 0 || j >= len(s.Subscriptions) {
			return false
		}
		s.Subscriptions[i], s.Subscriptions[j] = s.Subscriptions[j], s.Subscriptions[i]
		return true
	}
	return false
}

func (s *State) RebuildChannels() {
	type activeRoute struct {
		channel  *playlist.Channel
		order    int
		label    string
		sourceID string
		headers  playlist.StreamHeaders
	}
	total := 0
	for _, sub := range s.Subscriptions {
		total += len(sub.Channels)
	}
	all := make([]playlist.Channel, 0, total)
	active := make(map[string]activeRoute, total)
	hasDisabled := false
	for _, sub := range s.Subscriptions {
		hasDisabled = hasDisabled || sub.Disabled
		for i, ch := range sub.Channels {
			ch.SourceName = sub.Name
			all = append(all, ch)
			if sub.Disabled {
				continue
			}
			add := func(rawURL, label string, headers playlist.StreamHeaders) {
				if rawURL == "" {
					return
				}
				if _, exists := active[rawURL]; exists {
					return
				}
				if label == "" {
					label = sub.Name
				}
				active[rawURL] = activeRoute{&sub.Channels[i], len(active), label, sub.ID, headers}
			}
			if len(ch.Lines) == 0 {
				add(ch.URL, sub.Name, ch.Headers)
			} else {
				for _, line := range ch.Lines {
					add(line.URL, line.Label, line.Headers)
				}
			}
		}
	}
	// Resolve identities only once, including disabled caches. Filtering routes
	// in place replaces a second complete normalization/merge pass.
	merged := playlist.Merge(all)
	s.FavoriteURLs = make(map[string][]string, len(merged))
	keys := make(map[string]string, len(active))
	orders := map[string]int{}
	result := merged[:0]
	for _, group := range merged {
		key := playlist.ChannelKey(group)
		var first activeRoute
		haveFirst := false
		kept := group.Lines[:0]
		for _, line := range group.Lines {
			keys[line.URL] = key
			s.FavoriteURLs[key] = append(s.FavoriteURLs[key], line.URL)
			if s.Favorites[line.URL] {
				s.Favorites["channel:"+key] = true
			}
			if route, ok := active[line.URL]; ok {
				if !haveFirst || route.order < first.order {
					first = route
					haveFirst = true
				}
				line.Label = route.label
				line.SourceID = route.sourceID
				line.Headers = route.headers
				kept = append(kept, line)
			}
		}
		if !haveFirst {
			continue
		}
		if hasDisabled {
			sort.Slice(kept, func(i, j int) bool { return active[kept[i].URL].order < active[kept[j].URL].order })
		}
		ch := *first.channel
		ch.Key = key
		ch.SourceID = "" // Aggregate catalog may route through several subscriptions.
		ch.Lines = kept
		ch.URL = kept[0].URL
		if ch.TVGID == "" {
			ch.TVGID = group.TVGID
		}
		result = append(result, ch)
		if hasDisabled {
			orders[key] = first.order
		}
	}
	if hasDisabled {
		sort.Slice(result, func(i, j int) bool { return orders[result[i].Key] < orders[result[j].Key] })
	}
	s.Channels = result
	// Source-specific catalogs are prepared off the UI thread, with canonical
	// identities but only that source's routes (including URLs shared by sources).
	s.SourceChannels = make(map[string][]playlist.Channel, len(s.Subscriptions))
	for i, sub := range s.Subscriptions {
		byKey := make(map[string]int)
		seenURL := make(map[string]bool)
		var view []playlist.Channel
		if !sub.Disabled {
			capacity := sub.ChannelCount
			if capacity <= 0 || capacity > len(sub.Channels) {
				capacity = len(sub.Channels)
			}
			view = make([]playlist.Channel, 0, capacity)
		}
		for _, raw := range sub.Channels {
			add := func(line playlist.Line) {
				key, ok := keys[line.URL]
				if !ok {
					return
				}
				index, exists := byKey[key]
				if !exists {
					index = len(byKey)
					byKey[key] = index
				}
				if sub.Disabled || seenURL[line.URL] {
					return
				}
				seenURL[line.URL] = true
				if !exists {
					item := raw
					item.Key = key
					item.SourceID = sub.ID
					item.URL = line.URL
					item.Lines = nil
					view = append(view, item)
				}
				if line.Label == "" {
					line.Label = sub.Name
				}
				line.SourceID = sub.ID
				view[index].Lines = append(view[index].Lines, line)
			}
			if len(raw.Lines) == 0 {
				add(playlist.Line{URL: raw.URL, Headers: raw.Headers})
			} else {
				for _, line := range raw.Lines {
					add(line)
				}
			}
		}
		s.Subscriptions[i].ChannelCount = len(byKey)
		if !sub.Disabled {
			if len(view) > 0 && cap(view) > 4*len(view) {
				compact := make([]playlist.Channel, len(view))
				copy(compact, view)
				view = compact
			}
			s.SourceChannels[sub.ID] = view
		}
	}
}
