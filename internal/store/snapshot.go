package store

import "time"

// Snapshot copies mutable metadata and maps. Channel/line slices are immutable:
// callers replace a cache via UpdateSubscriptionCache instead of editing it.
// This keeps UI submission proportional to subscriptions/preferences, not lines.
func (s State) Snapshot() State {
	clone := s
	clone.EPGSources = append([]EPGSourceEntry(nil), s.EPGSources...)
	clone.Subscriptions = append([]Subscription(nil), s.Subscriptions...)
	if s.EPGChecks != nil {
		clone.EPGChecks = make(map[string]time.Time, len(s.EPGChecks))
		for source, at := range s.EPGChecks {
			clone.EPGChecks[source] = at
		}
	}
	if s.Favorites != nil {
		clone.Favorites = make(map[string]bool, len(s.Favorites))
		for key, value := range s.Favorites {
			clone.Favorites[key] = value
		}
	}
	if s.PreferredLines != nil {
		clone.PreferredLines = make(map[string]string, len(s.PreferredLines))
		for key, value := range s.PreferredLines {
			clone.PreferredLines[key] = value
		}
	}
	return clone
}
