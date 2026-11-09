package playlist

import (
	"strings"
	"unicode"
)

// ChannelKey retains all region/name text and + signs. Only cosmetic separators
// are removed. An explicit quality marker also distinguishes a shared tvg-id.
func ChannelKey(ch Channel) string {
	if ch.Key != "" {
		return ch.Key
	}
	return keyForName(ch, normalizeName(ch.Name))
}
func keyForName(ch Channel, name string) string {
	if ch.Key != "" {
		return ch.Key
	}
	if id := strings.TrimSpace(ch.TVGID); id != "" {
		return "id:" + strings.ToLower(id) + "|quality:" + quality(name)
	}
	if name == "" {
		return "url:" + ch.URL
	}
	return "name:" + name
}
func normalizeName(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("-_.·", r) {
			return -1
		}
		return unicode.ToUpper(r)
	}, name)
}
func quality(name string) string {
	for _, q := range []string{"8K", "4K", "2160P", "UHD", "FHD", "1080P", "720P", "超高清", "超清", "高清", "标清", "HD", "SD"} {
		if strings.Contains(name, q) {
			return q
		}
	}
	return ""
}
func Lines(ch Channel) []Line {
	if len(ch.Lines) > 0 {
		return ch.Lines
	}
	if ch.URL != "" {
		return []Line{{URL: ch.URL, Headers: ch.Headers}}
	}
	return nil
}

// Merge preserves subscription/playlist order; a URL appears only once globally.
func Merge(raw []Channel) []Channel {
	// A source without tvg-id may join an unambiguous named ID group, even
	// when that source precedes the ID-bearing source in subscription order.
	capacity := min(len(raw), 4096)
	aliases := make(map[string]string, capacity)
	ambiguous := map[string]bool{}
	keys, names := make([]string, len(raw)), make([]string, len(raw))
	for i, ch := range raw {
		name := normalizeName(ch.Name)
		key := keyForName(ch, name)
		keys[i], names[i] = key, name
		if strings.TrimSpace(ch.TVGID) == "" {
			continue
		}
		if previous, ok := aliases[name]; ok && previous != key {
			ambiguous[name] = true
		}
		aliases[name] = key
	}
	result := make([]Channel, 0, capacity)
	byKey := make(map[string]int, capacity)
	seenURL := make(map[string]bool, len(raw))
	for i, ch := range raw {
		key := keys[i]
		if strings.TrimSpace(ch.TVGID) == "" && (ch.Key == "" || strings.HasPrefix(ch.Key, "name:")) {
			name := names[i]
			if alias, ok := aliases[name]; ok && !ambiguous[name] {
				key = alias
			}
		}
		fallback := [1]Line{{URL: ch.URL, Label: ch.SourceName, Headers: ch.Headers}}
		lines := ch.Lines
		if len(lines) == 0 {
			lines = fallback[:]
		}
		for _, line := range lines {
			if line.Label == "" {
				line.Label = ch.SourceName
			}
			if line.URL == "" || seenURL[line.URL] {
				continue
			}
			seenURL[line.URL] = true
			index, ok := byKey[key]
			if !ok {
				index = len(result)
				byKey[key] = index
				item := ch
				item.Key = key
				item.URL = line.URL
				item.Lines = make([]Line, 0, 2)
				result = append(result, item)
			}
			if result[index].TVGID == "" && ch.TVGID != "" {
				result[index].TVGID = ch.TVGID
			}
			result[index].Lines = append(result[index].Lines, line)
		}
	}
	return result
}

func PreferredLine(ch Channel, preferred string) int {
	for i, line := range Lines(ch) {
		if line.URL == preferred {
			return i
		}
	}
	return 0
}
func NextLine(ch Channel, tried map[string]bool) int {
	for i, line := range Lines(ch) {
		if !tried[line.URL] {
			return i
		}
	}
	return -1
}
