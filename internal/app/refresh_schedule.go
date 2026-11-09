package app

import "time"

const dailyRefreshInterval = 24 * time.Hour

func dailyRefreshDue(last, now time.Time) bool {
	return last.IsZero() || last.After(now) || !now.Before(last.Add(dailyRefreshInterval))
}

// Explicit refresh buttons bypass this schedule. Both successful and failed
// automatic attempts wait a day; cancellation does not consume the interval.
func (a *application) pollSubscriptions() {
	now := time.Now()
	a.nextSubscriptionCheck = now.Add(time.Minute)
	for _, sub := range a.state.Subscriptions {
		last := sub.CheckedAt
		if last.IsZero() {
			last = sub.UpdatedAt
		} // migrate existing caches without a redownload
		if !sub.Disabled && sub.Source != "" && dailyRefreshDue(last, now) {
			a.refreshSubscription(sub.ID)
		}
	}
}
