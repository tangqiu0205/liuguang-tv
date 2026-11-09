package app

import (
	"fmt"
	"testing"

	"github.com/tangqiu0205/LiuguangTV/internal/playlist"
	"github.com/tangqiu0205/LiuguangTV/internal/store"
)

var filteredSink channelSelection

func BenchmarkChangedChannelFilter(b *testing.B) {
	a := &application{group: "全部分组"}
	for i := 0; i < 50000; i++ {
		a.state.Channels = append(a.state.Channels, playlist.Channel{Name: fmt.Sprintf("Channel %d", i), Group: fmt.Sprintf("Group %d", i%20), URL: fmt.Sprintf("https://example.org/%d", i)})
	}
	a.ensureChannelIndex()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if a.query == "channel 2" {
			a.query = "channel 3"
		} else {
			a.query = "channel 2"
		}
		filteredSink = a.filteredChannels()
	}
}

func BenchmarkCachedChannelFilter(b *testing.B) {
	for _, count := range []int{10000, 50000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			a := &application{group: "全部分组", query: "channel 2", state: store.State{}}
			for i := 0; i < count; i++ {
				a.state.Channels = append(a.state.Channels, playlist.Channel{Name: fmt.Sprintf("Channel %d", i), Group: fmt.Sprintf("Group %d", i%20), URL: fmt.Sprintf("https://example.org/%d", i)})
			}
			a.filteredChannels()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				filteredSink = a.filteredChannels()
			}
		})
	}
}
