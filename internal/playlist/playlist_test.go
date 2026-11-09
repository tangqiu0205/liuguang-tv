package playlist

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	base, _ := url.Parse("https://example.org/list/channels.m3u")
	input := "\ufeff#EXTM3U\r\n#EXTINF:-1 tvg-id=\"c1\" group-title=\"新闻,综合\",新闻 HD\r\n../live/one.m3u8\r\n#EXTINF:-1,Duplicate\n../live/one.m3u8\n#EXTINF:-1,Unsafe\nfile:///etc/passwd\n#EXTINF:-1,体育\n#EXTGRP:运动\nudp://@239.1.1.1:1234\n"
	channels, err := Parse(strings.NewReader(input), base)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 2 {
		t.Fatalf("got %+v", channels)
	}
	if channels[0].Name != "新闻 HD" || channels[0].Group != "新闻,综合" || channels[0].TVGID != "c1" || channels[0].URL != "https://example.org/live/one.m3u8" {
		t.Fatalf("got %+v", channels[0])
	}
	if channels[1].Group != "运动" {
		t.Fatal(channels[1])
	}
}
func TestParseChannelListWithIndependentSegments(t *testing.T) {
	input := "#EXTM3U\n#EXT-X-INDEPENDENT-SEGMENTS\n#EXTINF:-1 tvg-id=\"CCTV1\" group-title=\"央视卫视\",CCTV1\n#EXTGRP:央视卫视\nhttps://example.org/watch.php?channel=CCTV1&token=test\n"
	channels, err := Parse(strings.NewReader(input), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].Name != "CCTV1" || channels[0].TVGID != "CCTV1" || channels[0].Group != "央视卫视" || channels[0].URL != "https://example.org/watch.php?channel=CCTV1&token=test" {
		t.Fatalf("unexpected channels: %+v", channels)
	}
	for _, hls := range []string{
		"#EXT-X-TARGETDURATION:6\n#EXTINF:6,\nhttps://example.org/segment.ts",
		"#EXT-X-STREAM-INF:BANDWIDTH=1000000\nhttps://example.org/variant.m3u8",
	} {
		if _, err := Parse(strings.NewReader("#EXTM3U\n#EXT-X-INDEPENDENT-SEGMENTS\n"+hls), nil); err == nil {
			t.Fatal("HLS playlist must still be rejected")
		}
	}
}

func TestInvalidAndHLS(t *testing.T) {
	for _, input := range []string{"", "#EXTM3U\nfile:///a", "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\nhttps://example.org/segment.ts", strings.Repeat("x", MaxBytes+1)} {
		if _, err := Parse(strings.NewReader(input), nil); err == nil {
			t.Fatal("expected error")
		}
	}
	for _, raw := range []string{"--script=evil", "file:///a", "javascript:alert(1)", "https:///missing-host"} {
		if ValidStream(raw) {
			t.Fatal(raw)
		}
	}
}
func TestLoadHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			http.Error(w, "bad", 403)
			return
		}
		fmt.Fprint(w, "#EXTM3U\n#EXTINF:-1,Test\n/live.m3u8\n")
	}))
	defer server.Close()
	got, err := Load(context.Background(), server.URL+"/channels")
	if err != nil || len(got) != 1 || got[0].URL != server.URL+"/live.m3u8" {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := Load(context.Background(), server.URL+"/bad"); err == nil {
		t.Fatal("expected HTTP error")
	}
}
