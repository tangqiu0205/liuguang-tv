package playlist

import (
	"net/url"
	"strings"
	"testing"
)

func TestLogoParsingAndMerge(t *testing.T) {
	base, _ := url.Parse("https://example.org/lists/channels.m3u")
	channels, err := Parse(strings.NewReader("#EXTM3U\n#EXTINF:-1,Demo\nhttps://example.org/one\n#EXTINF:-1 tvg-logo=\"../logo.png\",Demo\nhttps://example.org/two\n#EXTINF:-1 tvg-logo=\"file:///secret\",Other\nhttps://example.org/three\n"), base)
	if err != nil {
		t.Fatal(err)
	}
	if channels[1].Logo != "https://example.org/logo.png" || channels[2].Logo != "" {
		t.Fatalf("unexpected logos: %+v", channels)
	}
	merged := Merge(channels)
	if len(merged) != 2 || merged[0].Logo != channels[1].Logo {
		t.Fatalf("unexpected merge: %+v", merged)
	}
}
