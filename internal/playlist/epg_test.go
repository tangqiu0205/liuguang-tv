package playlist

import (
	"net/url"
	"strings"
	"testing"
)

func TestDocumentEPGURLs(t *testing.T) {
	base, _ := url.Parse("https://example.com/list/channels.m3u")
	raw := `#EXTM3U x-tvg-url="../epg.xml.gz,https://other.example/epg.xml" url-tvg="../epg.xml.gz" tvg-url="file:///secret"
#EXTINF:-1 tvg-id="cctv1",CCTV-1
https://example.com/live
`
	doc, err := ParseDocument(strings.NewReader(raw), base)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Channels) != 1 || doc.Channels[0].TVGID != "cctv1" {
		t.Fatal("lost channel")
	}
	if len(doc.EPGURLs) != 2 || doc.EPGURLs[0] != "https://example.com/epg.xml.gz" || doc.EPGURLs[1] != "https://other.example/epg.xml" {
		t.Fatalf("bad URLs: %v", doc.EPGURLs)
	}
}
func TestPlainPlaylistStillWorks(t *testing.T) {
	doc, err := ParseDocument(strings.NewReader("#EXTM3U\n#EXTINF:-1,频道\nhttps://example.com/live\n"), nil)
	if err != nil || len(doc.Channels) != 1 || len(doc.EPGURLs) != 0 {
		t.Fatalf("%+v %v", doc, err)
	}
}
