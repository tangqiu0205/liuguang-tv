package store

import (
	"fmt"
	"testing"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
)

func benchmarkState(lines int) State {
	s := State{Favorites: map[string]bool{}, PreferredLines: map[string]string{}}
	for source := 0; source < 4; source++ {
		sub := Subscription{ID: fmt.Sprint(source), Name: fmt.Sprintf("Source %d", source)}
		for i := 0; i < lines/4; i++ {
			sub.Channels = append(sub.Channels, playlist.Channel{Name: fmt.Sprintf("Channel %d", i), TVGID: fmt.Sprintf("channel-%d", i), Group: fmt.Sprintf("Group %d", i%20), URL: fmt.Sprintf("https://example.org/%d/%d", source, i)})
		}
		s.Subscriptions = append(s.Subscriptions, sub)
	}
	return s
}

var snapshotSink State

func BenchmarkStateSnapshot(b *testing.B) {
	for _, lines := range []int{10000, 50000} {
		b.Run(fmt.Sprint(lines), func(b *testing.B) {
			s := benchmarkState(lines)
			for i := 0; i < 100; i++ {
				s.Favorites[fmt.Sprint(i)] = true
				s.PreferredLines[fmt.Sprint(i)] = "https://example.org/preferred"
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				snapshotSink = s.Snapshot()
			}
		})
	}
}

func BenchmarkRebuildCatalog(b *testing.B) {
	for _, lines := range []int{10000, 50000} {
		b.Run(fmt.Sprint(lines), func(b *testing.B) {
			s := benchmarkState(lines)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				s.RebuildChannels()
			}
		})
	}
}
