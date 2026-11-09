// Package playlist reads bounded UTF-8 M3U playlists. Stream addresses are never executed as commands.
package playlist

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/tangqiu0205/LiuguangTV/internal/diagnostic"
)

const MaxBytes = 16 << 20

type Line struct {
	SourceID string        `json:"-"` // Derived subscription ownership, not persisted.
	URL      string        `json:"url"`
	Label    string        `json:"label,omitempty"`
	Headers  StreamHeaders `json:"headers,omitzero"`
}

type Channel struct {
	SourceID   string        `json:"-"` // Non-empty only for a source-scoped catalog.
	SourceName string        `json:"-"` // Derived label, never persisted in source caches.
	Key        string        `json:"key,omitempty"`
	Lines      []Line        `json:"lines,omitempty"`
	Name       string        `json:"name"`
	Group      string        `json:"group"`
	URL        string        `json:"url"`
	TVGID      string        `json:"tvgId,omitempty"`
	Headers    StreamHeaders `json:"headers,omitzero"`
}

var attributes = regexp.MustCompile(`([\w-]+)="([^"]*)"`)

// ValidStream deliberately excludes local files and executable/custom protocols.
func ValidStream(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "udp", "rtp", "rtsp":
		return true
	}
	return false
}

// Document preserves playlist-level EPG metadata separately from channels.
type Document struct {
	Channels        []Channel
	EPGURLs         []string
	SkippedReferers int
}

func Parse(r io.Reader, base *url.URL) ([]Channel, error) {
	return parse(r, base, nil, nil)
}

func ParseDocument(r io.Reader, base *url.URL) (Document, error) {
	var doc Document
	var err error
	doc.Channels, err = parse(r, base, &doc.EPGURLs, &doc.SkippedReferers)
	return doc, err
}

func parse(r io.Reader, base *url.URL, epgURLs *[]string, skippedReferers *int) ([]Channel, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("节目单超过 16 MiB")
	}
	scanner := bufio.NewScanner(strings.NewReader(strings.TrimPrefix(string(data), "\ufeff")))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var result []Channel
	seen := map[string]bool{}
	pending := Channel{}
	hasEXTINF := false
	txtGroup := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Markup is not a stream address, even when resolving it would produce
		// a syntactically valid HTTP URL. Reject the whole response, not a partial list.
		if strings.HasPrefix(line, "<") {
			return nil, fmt.Errorf("内容是网页或 XML，不是 M3U 频道列表；请检查订阅地址是否需要登录")
		}
		if strings.HasPrefix(line, "#EXTM3U") && epgURLs != nil {
			for _, m := range attributes.FindAllStringSubmatch(line, -1) {
				if m[1] != "x-tvg-url" && m[1] != "url-tvg" && m[1] != "tvg-url" {
					continue
				}
				for _, raw := range strings.FieldsFunc(m[2], func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
					u, err := url.Parse(raw)
					if err != nil {
						continue
					}
					if base != nil {
						u = base.ResolveReference(u)
					}
					if (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
						found := false
						for _, existing := range *epgURLs {
							if existing == u.String() {
								found = true
								break
							}
						}
						if !found {
							*epgURLs = append(*epgURLs, u.String())
						}
					}
				}
			}
		}
		// Some channel-list exporters include this tag without any HLS segments.
		// It alone is not evidence that the document is an HLS playlist.
		if line == "#EXT-X-INDEPENDENT-SEGMENTS" {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-") {
			return nil, fmt.Errorf("这是 HLS 播放地址，不是频道节目单；请将它写入 M3U 频道列表后再导入")
		}
		if strings.HasPrefix(line, "#EXTINF:") {
			hasEXTINF = true
			pending = Channel{}
			quoted := false
			for i, ch := range line {
				if ch == '"' {
					quoted = !quoted
				}
				if ch == ',' && !quoted {
					pending.Name = strings.TrimSpace(line[i+1:])
					break
				}
			}
			for _, m := range attributes.FindAllStringSubmatch(line, -1) {
				switch m[1] {
				case "group-title":
					pending.Group = m[2]
				case "http-user-agent", "http-referrer":
					if err := pending.Headers.set(m[1], m[2], skippedReferers); err != nil {
						return nil, err
					}
				case "tvg-id":
					pending.TVGID = m[2]
				case "tvg-name":
					if pending.Name == "" {
						pending.Name = m[2]
					}
				}
			}
			continue
		}
		if strings.HasPrefix(line, "#EXTVLCOPT:") {
			key, value, ok := strings.Cut(strings.TrimPrefix(line, "#EXTVLCOPT:"), "=")
			if ok && hasEXTINF {
				if err := pending.Headers.set(strings.TrimSpace(key), strings.TrimSpace(value), skippedReferers); err != nil {
					return nil, err
				}
			}
			continue
		}
		if strings.HasPrefix(line, "#EXTGRP:") {
			pending.Group = strings.TrimSpace(strings.TrimPrefix(line, "#EXTGRP:"))
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		raw := line
		// TXT entries are recognized by content, not the filename or MIME type.
		// Do not split bare URLs: commas are legal in paths and queries.
		if !hasEXTINF && !ValidStream(raw) {
			name, address, ok := strings.Cut(raw, ",")
			name, address = strings.TrimSpace(name), strings.TrimSpace(address)
			if ok && name != "" {
				if address == "#genre#" {
					txtGroup = name
					pending = Channel{}
					continue
				}
				// TXT addresses must be absolute; never resolve arbitrary text.
				stream, _, _ := strings.Cut(address, "|")
				if ValidStream(stream) {
					pending = Channel{Name: name, Group: txtGroup}
					raw = address
				} else {
					return nil, fmt.Errorf("TXT 频道播放地址无效；仅支持 HTTP(S)、UDP、RTP、RTSP")
				}
			}
		}
		var headerErr error
		raw, pending.Headers, headerErr = parseStreamHeaders(raw, pending.Headers, skippedReferers)
		if headerErr != nil {
			return nil, headerErr
		}
		if base != nil {
			if u, e := url.Parse(raw); e == nil {
				// Bare absolute URLs are supported, but arbitrary response text
				// must not become a relative channel URL without EXTINF metadata.
				if !u.IsAbs() && !hasEXTINF {
					return nil, fmt.Errorf("相对播放地址缺少 EXTINF 频道信息；请检查是否为 M3U 频道列表")
				}
				raw = base.ResolveReference(u).String()
			}
		}
		if ValidStream(raw) && !seen[raw] {
			pending.URL = raw
			if pending.Name == "" {
				pending.Name = fmt.Sprintf("频道 %d", len(result)+1)
			}
			if pending.Group == "" {
				pending.Group = "未分组"
			}
			result = append(result, pending)
			seen[raw] = true
		}
		pending = Channel{}
		hasEXTINF = false
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("没有找到有效频道；请导入频道列表，而不是 HLS 视频分片清单")
	}
	return result, nil
}

func Load(ctx context.Context, source string) ([]Channel, error) {
	doc, err := LoadDocument(ctx, source)
	return doc.Channels, err
}

func LoadDocument(ctx context.Context, source string, logs ...*diagnostic.Logger) (Document, error) {
	var log *diagnostic.Logger
	if len(logs) > 0 {
		log = logs[0]
	}
	parseResponse := func(r io.Reader, base *url.URL) (Document, error) {
		doc, err := ParseDocument(r, base)
		log.Record(diagnostic.PlaylistParse, source, err)
		if doc.SkippedReferers > 0 {
			log.Record(diagnostic.PlaylistSkippedReferers, source, nil, doc.SkippedReferers)
		}
		return doc, err
	}
	u, err := url.Parse(source)
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return Document{}, err
		}
		client := &http.Client{Timeout: 25 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			log.Record(diagnostic.PlaylistDownload, source, err)
			return Document{}, fmt.Errorf("下载失败: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			err := fmt.Errorf("HTTP %d", resp.StatusCode)
			log.Record(diagnostic.PlaylistDownload, source, err, resp.StatusCode)
			return Document{}, err
		}
		log.Record(diagnostic.PlaylistDownload, source, nil, resp.StatusCode)
		contentType := strings.ToLower(strings.TrimSpace(strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0]))
		if contentType == "text/html" || contentType == "application/xhtml+xml" {
			err := fmt.Errorf("订阅返回了网页，不是 M3U 频道列表；请检查订阅地址是否需要登录")
			log.Record(diagnostic.PlaylistParse, source, err)
			return Document{}, err
		}
		return parseResponse(resp.Body, resp.Request.URL)
	}
	f, err := os.Open(source)
	log.Record(diagnostic.PlaylistDownload, source, err)
	if err != nil {
		return Document{}, err
	}
	defer f.Close()
	return parseResponse(f, nil)
}
