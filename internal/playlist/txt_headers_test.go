package playlist

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTXTLocalAndHTTP(t *testing.T) {
	input := "\ufeff央视高清,#genre#\r\nCCTV1,http://[2409:8087::22]:6610/live.m3u8?a=1,b=2\r\nCCTV1,https://example.org/second\r\n卫视,#genre#\r\n新闻,https://example.org/news\r\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/x-mpegurl")
		w.Write([]byte(input))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "channels.txt")
	if err := os.WriteFile(path, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{path, server.URL + "/tv.m3u"} {
		doc, err := LoadDocument(context.Background(), source)
		if err != nil || len(doc.Channels) != 3 {
			t.Fatalf("%s: %+v %v", source, doc, err)
		}
		if doc.Channels[0].Name != "CCTV1" || doc.Channels[0].Group != "央视高清" || !strings.HasSuffix(doc.Channels[0].URL, "?a=1,b=2") || doc.Channels[2].Group != "卫视" {
			t.Fatalf("TXT metadata lost: %+v", doc.Channels)
		}
		merged := Merge(doc.Channels)
		if len(merged) != 2 || len(merged[0].Lines) != 2 {
			t.Fatalf("TXT routes not merged: %+v", merged)
		}
	}
}
func TestTXTValidation(t *testing.T) {
	base, _ := url.Parse("https://example.org/tv.m3u")
	for _, input := range []string{"频道,file:///secret", "频道,javascript:evil", "频道,../live.m3u8", "频道,#genre#", "频道,https://example.org/live\n<html>bad</html>", "频道,https://example.org/live\n#EXT-X-TARGETDURATION:6"} {
		if _, err := Parse(strings.NewReader(input), base); err == nil {
			t.Fatalf("accepted unsafe TXT %q", input)
		}
	}
	ch, err := Parse(strings.NewReader("频道,https://example.org/live"), nil)
	if err != nil || ch[0].Group != "未分组" {
		t.Fatalf("%+v %v", ch, err)
	}
}
func TestHeaderPriorityIsolationAndMerge(t *testing.T) {
	input := "#EXTM3U\n" +
		`#EXTINF:-1 http-user-agent="attribute" http-referrer="https://example.org/a",Demo` + "\n" +
		"#EXTVLCOPT:http-user-agent=vlc\n#EXTVLCOPT:script=/evil\n" +
		"https://example.org/one|User-Agent=suffix%2Bagent&Referer=https%3A%2F%2Fexample.org%2Fb&script=evil\n" +
		"#EXTINF:-1,Demo\nhttps://example.org/two\n"
	ch, err := Parse(strings.NewReader(input), nil)
	if err != nil || len(ch) != 2 {
		t.Fatalf("%+v %v", ch, err)
	}
	want := StreamHeaders{UserAgent: "suffix+agent", Referer: "https://example.org/b"}
	if ch[0].Headers != want || ch[1].Headers != (StreamHeaders{}) || ch[0].URL != "https://example.org/one" {
		t.Fatalf("headers leaked or priority wrong: %+v", ch)
	}
	merged := Merge(ch)
	if len(merged) != 1 || merged[0].Lines[0].Headers != want || merged[0].Lines[1].Headers != (StreamHeaders{}) {
		t.Fatalf("line headers lost: %+v", merged)
	}
	vlc, err := Parse(strings.NewReader("#EXTM3U\n#EXTINF:-1,Demo\n#EXTVLCOPT:http-user-agent=vlc\nhttps://example.org/live"), nil)
	if err != nil || vlc[0].Headers.UserAgent != "vlc" {
		t.Fatalf("VLC header rejected: %+v %v", vlc, err)
	}
	txt, err := Parse(strings.NewReader("Demo,https://example.org/live|User-Agent=txt"), nil)
	if err != nil || txt[0].Headers.UserAgent != "txt" {
		t.Fatalf("TXT suffix rejected: %+v %v", txt, err)
	}
}
func TestInvalidReferersAreSkipped(t *testing.T) {
	input := "#EXTM3U\n" +
		`#EXTINF:-1 http-referrer="salamtelevisi.com",Attribute` + "\nhttps://example.org/one\n" +
		`#EXTINF:-1 http-referrer="https://example.org/valid",VLC` + "\n#EXTVLCOPT:http-referrer=file://secret\nhttps://example.org/two\n" +
		`#EXTINF:-1 http-referrer="https://example.org/valid",Suffix` + "\nhttps://example.org/three|Referer=https%3A%2F%2Fuser%3Apass%40example.org%2F\n" +
		"#EXTINF:-1,Control\nhttps://example.org/four|Referer=bad%0D%0AInjected\n" +
		`#EXTINF:-1 http-referrer="https://example.org/good",Valid` + "\nhttps://example.org/five\n"
	doc, err := ParseDocument(strings.NewReader(input), nil)
	if err != nil || len(doc.Channels) != 5 || doc.SkippedReferers != 4 {
		t.Fatalf("unexpected document: %+v, %v", doc, err)
	}
	for _, ch := range doc.Channels[:4] {
		if ch.Headers.Referer != "" {
			t.Fatalf("invalid referer retained: %+v", ch)
		}
	}
	if doc.Channels[4].Headers.Referer != "https://example.org/good" {
		t.Fatalf("valid referer lost: %+v", doc.Channels[4])
	}
}

func TestRejectUnsafeHeaders(t *testing.T) {
	for _, suffix := range []string{"User-Agent=bad%0D%0AInjected%3Ayes", "User-Agent=%00", "User-Agent=%zz", "script"} {
		_, err := Parse(strings.NewReader("#EXTM3U\n#EXTINF:-1,Demo\nhttps://example.org/live|"+suffix), nil)
		if err == nil {
			t.Fatalf("accepted invalid headers %s", suffix)
		}
		if strings.Contains(err.Error(), "Injected") {
			t.Fatal("error leaked header value")
		}
	}
}
