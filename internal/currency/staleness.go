package currency

import "time"

// DefaultRateMaxAge is how long an FX rate stays "fresh" before it's flagged as
// stale in the UI — manual rates drift, and a 30-day-old rate quietly skews every
// multi-currency total until the user refreshes it (L4).
const DefaultRateMaxAge = 30 * 24 * time.Hour

// RateStale reports whether an FX rate set at updatedAt should be flagged stale as
// of now, given maxAge. A zero updatedAt (the rate's age is unknown — e.g. seeded
// sample data that was never hand-edited) is treated as NOT stale, so the warning
// only fires for rates the user actually set and then left to age.
func RateStale(updatedAt, now time.Time, maxAge time.Duration) bool {
	if updatedAt.IsZero() {
		return false
	}
	return now.Sub(updatedAt) > maxAge
}
