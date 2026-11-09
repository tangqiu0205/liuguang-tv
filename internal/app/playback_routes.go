package app

import "github.com/tangqiu0205/LiuguangTV/internal/playlist"

// playbackRoutes preserves the current stream but refreshes candidates for the
// next manual/automatic switch. Permissions come from authoritative subscription
// metadata, not a possibly still-pending background catalog rebuild.
func (a *application) playbackRoutes() playlist.Channel {
	return a.routesFor(a.activeChannel, a.playbackCatalogBound)
}
func (a *application) routesFor(ch playlist.Channel, bound bool) playlist.Channel {
	if !bound {
		return ch
	}
	catalog := a.state.Channels
	if ch.SourceID != "" {
		catalog = a.state.SourceChannels[ch.SourceID]
	}
	for _, latest := range catalog {
		if playlist.ChannelKey(latest) == playlist.ChannelKey(ch) {
			ch = latest
			break
		}
	}
	allowed := map[string]playlist.Line{}
	var fresh []playlist.Line
	for _, sub := range a.state.Subscriptions {
		if sub.Disabled || (ch.SourceID != "" && sub.ID != ch.SourceID) {
			continue
		}
		for _, raw := range sub.Channels {
			sameKey := playlist.ChannelKey(raw) == playlist.ChannelKey(ch)
			for _, line := range playlist.Lines(raw) {
				if _, exists := allowed[line.URL]; exists {
					continue
				}
				line.SourceID = sub.ID
				if line.Label == "" {
					line.Label = sub.Name
				}
				allowed[line.URL] = line
				if sameKey {
					fresh = append(fresh, line)
				}
			}
		}
	}
	lines := make([]playlist.Line, 0, len(playlist.Lines(ch)))
	seen := map[string]bool{}
	candidates := append(append([]playlist.Line(nil), playlist.Lines(ch)...), fresh...)
	for _, line := range candidates {
		// A source-owned route must not silently change providers when two
		// subscriptions happen to share the same stream URL.
		if current, ok := allowed[line.URL]; ok && !seen[line.URL] {
			lines = append(lines, current)
			seen[line.URL] = true
		}
	}
	ch.Lines = lines
	ch.URL = ""
	if len(lines) > 0 {
		ch.URL = lines[0].URL
	}
	return ch
}

func (a *application) playingEPGChannel() playlist.Channel {
	ch := a.activeChannel
	if a.activeSourceID != "" {
		ch.SourceID = a.activeSourceID
	}
	return ch
}
