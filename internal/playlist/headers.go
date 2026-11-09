package playlist

import (
	"fmt"
	"net/url"
	"strings"
)

// StreamHeaders is deliberately not a map: subscriptions cannot inject arbitrary
// HTTP headers or mpv options. Values belong to a single stream, never a source.
type StreamHeaders struct {
	UserAgent string `json:"userAgent,omitempty"`
	Referer   string `json:"referer,omitempty"`
}

func (h StreamHeaders) Validate() error {
	for _, value := range []string{h.UserAgent, h.Referer} {
		if len(value) > 4096 {
			return fmt.Errorf("播放请求头超过长度限制")
		}
		for _, r := range value {
			if r < 32 || r == 127 {
				return fmt.Errorf("播放请求头包含非法控制字符")
			}
		}
	}
	if h.Referer != "" {
		u, err := url.Parse(h.Referer)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
			return fmt.Errorf("Referer 必须是无用户凭据的 HTTP(S) 地址")
		}
	}
	return nil
}
func (h *StreamHeaders) set(key, value string, skippedReferers *int) error {
	switch strings.ToLower(key) {
	case "http-user-agent", "user-agent":
		h.UserAgent = value
	case "http-referrer", "referer", "referrer":
		// An invalid higher-priority value must not expose a lower-priority one.
		h.Referer = value
		if err := (StreamHeaders{Referer: value}).Validate(); err != nil {
			h.Referer = ""
			if skippedReferers != nil {
				*skippedReferers++
			}
			return nil
		}
	default:
		return nil // Unknown playlist options are never passed to mpv.
	}
	return h.Validate()
}

// Priority: URL suffix > EXTVLCOPT > EXTINF attributes. URL suffix values use
// percent encoding (QueryUnescape), including %2B for a literal plus sign.
func parseStreamHeaders(raw string, h StreamHeaders, skippedReferers *int) (string, StreamHeaders, error) {
	stream, suffix, ok := strings.Cut(raw, "|")
	if !ok {
		return raw, h, nil
	}
	for _, field := range strings.Split(suffix, "&") {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			return "", h, fmt.Errorf("播放请求头后缀格式无效")
		}
		value, err := url.QueryUnescape(value)
		if err != nil {
			return "", h, fmt.Errorf("播放请求头编码无效")
		}
		if err = h.set(key, value, skippedReferers); err != nil {
			return "", h, err
		}
	}
	return stream, h, nil
}
