package app

import (
	"strings"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

type channelIndex struct {
	channels []playlist.Channel
	groups   []string
	groupSet map[string]bool
	search   []string
	sources  map[string]channelIndex
}

func indexChannels(channels []playlist.Channel) channelIndex {
	index := channelIndex{channels: channels, groups: []string{"全部分组"}, groupSet: map[string]bool{"全部分组": true}, search: make([]string, len(channels))}
	for i, ch := range channels {
		if !index.groupSet[ch.Group] {
			index.groups = append(index.groups, ch.Group)
			index.groupSet[ch.Group] = true
		}
		index.search[i] = strings.ToLower(ch.Name)
	}
	return index
}

func indexCatalog(state store.State) channelIndex {
	index := indexChannels(state.Channels)
	index.sources = make(map[string]channelIndex, len(state.SourceChannels))
	for id, channels := range state.SourceChannels {
		index.sources[id] = indexChannels(channels)
	}
	return index
}

var emptyChannelIndex = indexChannels(nil)

func (a *application) libraryIndex() channelIndex {
	if a.state.SelectedSource == "" {
		return a.channelCache.index
	}
	if index, ok := a.channelCache.index.sources[a.state.SelectedSource]; ok {
		return index
	}
	return emptyChannelIndex
}

// A filtered view retains only row indexes, never copies all matching channels.
// Older frame callbacks can safely retain their immutable selection.
type channelSelection struct {
	channels []playlist.Channel
	rows     []int
	all      bool
}

func (s channelSelection) Len() int {
	if s.all {
		return len(s.channels)
	}
	return len(s.rows)
}
func (s channelSelection) At(i int) playlist.Channel {
	if s.all {
		return s.channels[i]
	}
	return s.channels[s.rows[i]]
}

type channelViewCache struct {
	index             channelIndex
	initialized       bool
	valid             bool
	query, group      string
	source            string
	favoritesOnly     bool
	favoritesRevision uint64
	filtered          channelSelection
}

func sameChannels(a, b []playlist.Channel) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}
func (a *application) installChannelIndex(index channelIndex) {
	a.channelCache = channelViewCache{index: index, initialized: true}
}
func (a *application) ensureChannelIndex() {
	if !a.channelCache.initialized || !sameChannels(a.state.Channels, a.channelCache.index.channels) {
		a.installChannelIndex(indexCatalog(a.state))
	}
}
func (a *application) filteredChannels() channelSelection {
	a.ensureChannelIndex()
	cache := &a.channelCache
	index := a.libraryIndex()
	query := strings.ToLower(strings.TrimSpace(a.query))
	if cache.valid && cache.source == a.state.SelectedSource && cache.query == query && cache.group == a.group && cache.favoritesOnly == a.favoritesOnly && cache.favoritesRevision == a.favoritesRevision {
		return cache.filtered
	}
	cache.source = a.state.SelectedSource
	cache.query = query
	cache.group = a.group
	cache.favoritesOnly = a.favoritesOnly
	cache.favoritesRevision = a.favoritesRevision
	cache.valid = true
	if query == "" && a.group == "全部分组" && !a.favoritesOnly {
		cache.filtered = channelSelection{channels: index.channels, all: true}
		return cache.filtered
	}
	rows := make([]int, 0, min(256, len(index.channels)))
	for i, ch := range index.channels {
		if a.group != "全部分组" && a.group != ch.Group {
			continue
		}
		if a.favoritesOnly && !a.isFavorite(ch) {
			continue
		}
		if query != "" && !strings.Contains(index.search[i], query) {
			continue
		}
		rows = append(rows, i)
	}
	cache.filtered = channelSelection{channels: index.channels, rows: rows}
	return cache.filtered
}
