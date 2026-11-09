package playlist

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRejectNonPlaylistResponses(t *testing.T) {
	base, _ := url.Parse("https://example.org/list/channels.m3u")
	for _, input := range []string{
		"<!DOCTYPE html>\n<html>\n<body>Please sign in</body>\n</html>",
		"\ufeff  <HTML><body>Login</body></HTML>",
		"<?xml version=\"1.0\"?><error>Forbidden</error>",
		"Please sign in",
		"{\"error\":\"Forbidden\"}",
		"#EXTM3U\n#EXTINF:-1,Good\nhttps://example.org/live\n<html>Login</html>",
	} {
		channels, err := Parse(strings.NewReader(input), base)
		if err == nil || len(channels) != 0 {
			t.Errorf("accepted non-playlist %q: %+v, %v", input, channels, err)
		}
	}
}

func TestStreamAddressCompatibility(t *testing.T) {
	base, _ := url.Parse("https://example.org/list/channels.m3u")
	for _, input := range []string{
		"#EXTM3U\n#EXTINF:-1,Test\n../live.m3u8",
		"#EXTM3U\n#EXTINF:-1,\n../live.m3u8",
		"https://example.org/live.m3u8",
	} {
		channels, err := Parse(strings.NewReader(input), base)
		if err != nil || len(channels) != 1 || channels[0].URL != "https://example.org/live.m3u8" {
			t.Errorf("valid playlist rejected: %+v, %v", channels, err)
		}
	}
}

func TestLoadRejectsHTML(t *testing.T) {
	for _, contentType := range []string{"text/html; charset=utf-8", "application/xhtml+xml", "text/plain", "application/octet-stream"} {
		t.Run(contentType, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", contentType)
				_, _ = w.Write([]byte("<!DOCTYPE html>\n<html>Login</html>"))
			}))
			defer server.Close()
			channels, err := Load(context.Background(), server.URL+"/list")
			if err == nil || len(channels) != 0 || !strings.Contains(err.Error(), "网页") {
				t.Fatalf("HTML was not rejected with a useful error: %+v, %v", channels, err)
			}
		})
	}
}
