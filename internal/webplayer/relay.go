package webplayer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const maxManifestBytes = 4 << 20

var uriAttribute = regexp.MustCompile(`(^|[:,])URI="([^"]*)"`)

func (p *Player) relay(w http.ResponseWriter, r *http.Request, s *session, id string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	ref, ok := s.refs[id]
	if ok {
		ref.used = time.Now()
		s.refs[id] = ref
	}
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	// Both a navigation/stop and a disconnected media request cancel upstream IO.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	request, err := http.NewRequestWithContext(ctx, r.Method, ref.url, nil)
	if err != nil {
		http.Error(w, "invalid media", http.StatusBadGateway)
		return
	}
	if s.headers.UserAgent != "" {
		request.Header.Set("User-Agent", s.headers.UserAgent)
	}
	if s.headers.Referer != "" {
		request.Header.Set("Referer", s.headers.Referer)
	}
	request.Header.Set("Accept-Encoding", "identity")
	if value := r.Header.Get("Range"); value != "" {
		request.Header.Set("Range", value)
	}
	response, err := p.client.Do(request)
	if err != nil {
		http.Error(w, "media request failed", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		http.Error(w, "media unavailable", http.StatusBadGateway)
		return
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(response.StatusCode)
		return
	}
	body := bufio.NewReader(response.Body)
	head, _ := body.Peek(32)
	if bytes.HasPrefix(bytes.TrimSpace(bytes.TrimPrefix(head, []byte("\xef\xbb\xbf"))), []byte("#EXTM3U")) {
		data, err := io.ReadAll(io.LimitReader(body, maxManifestBytes+1))
		if err != nil || len(data) > maxManifestBytes {
			http.Error(w, "invalid manifest", http.StatusBadGateway)
			return
		}
		manifest, err := p.rewriteManifest(s, response.Request.URL, string(data))
		if err != nil {
			http.Error(w, "unsupported manifest", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = io.WriteString(w, manifest)
		return
	}
	// Only media metadata crosses the boundary: no upstream cookies, redirects,
	// authentication headers, errors or HTML can become executable app content.
	for _, key := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if value := response.Header.Get(key); value != "" {
			w.Header().Set(key, value)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, body)
}
func (p *Player) rewriteManifest(s *session, base *url.URL, manifest string) (string, error) {
	// Variable substitution and steering need a dedicated resolver; do not leak
	// unresolved/direct URLs out of this session's relay.
	if strings.Contains(manifest, "{$") || strings.Contains(manifest, "#EXT-X-CONTENT-STEERING:") {
		return "", errors.New("unsupported HLS extension")
	}
	lines := strings.Split(strings.TrimPrefix(manifest, "\ufeff"), "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			target, err := p.mediaURL(s, base, line)
			if err != nil {
				return "", err
			}
			lines[i] = target
			continue
		}
		var rewriteErr error
		lines[i] = uriAttribute.ReplaceAllStringFunc(line, func(attr string) string {
			match := uriAttribute.FindStringSubmatch(attr)
			target, err := p.mediaURL(s, base, match[2])
			if err != nil {
				rewriteErr = err
				return attr
			}
			return match[1] + `URI="` + target + `"`
		})
		if rewriteErr != nil {
			return "", rewriteErr
		}
	}
	return strings.Join(lines, "\n"), nil
}
func (p *Player) mediaURL(s *session, base *url.URL, raw string) (string, error) {
	relative, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	target := base.ResolveReference(relative)
	if !httpURL(target) {
		return "", errors.New("unsupported media protocol")
	}
	target.Fragment = ""
	key := target.String()
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.byURL[key]; ok {
		ref := s.refs[id]
		ref.used = now
		s.refs[id] = ref
		return p.prefix + "s/" + s.id + "/media/" + id, nil
	}
	if len(s.refs) >= 8192 {
		// Sliding retention bounds a long-running live session without exhausting
		// mappings after several hours. Keep recent playlist/key/segment identities.
		for id, ref := range s.refs {
			if id != "0" && now.Sub(ref.used) > 10*time.Minute {
				delete(s.refs, id)
				delete(s.byURL, ref.url)
			}
		}
		if len(s.refs) >= 8192 {
			return "", fmt.Errorf("too many media references")
		}
	}
	s.next++
	id := strconv.FormatUint(s.next, 10)
	s.refs[id] = mediaRef{url: key, used: now}
	s.byURL[key] = id
	return p.prefix + "s/" + s.id + "/media/" + id, nil
}
