// Package savings provides pure business-logic helpers for the automated
// savings-sweep feature (R19). All functions are free of syscall/js and
// depend only on the standard library, so they can be unit-tested with a
// plain `go test` invocation against any native Go toolchain.
package savings

import (
	"fmt"
	"time"
)

// RoundUpDelta returns the number of minor-currency units needed to round
// amountMinor UP to the nearest multiple of granularity.
//
//	RoundUpDelta(347, 100) → 53  (347 → 400, delta = 53)
//	RoundUpDelta(500, 100) → 0   (already on a boundary)
//
// amountMinor must be passed as a positive magnitude (absolute value of the
// spend). If granularity is ≤ 0 the function returns 0 (undefined).
func RoundUpDelta(amountMinor, granularity int64) int64 {
	if granularity <= 0 {
		return 0
	}
	rem := amountMinor % granularity
	if rem == 0 {
		return 0
	}
	return granularity - rem
}

// SurplusMinor returns the minor-currency amount available to sweep into
// savings. The surplus is clamped to [0, cap]:
//
//	surplus = liquid - billsDue - goalContribs
//	result  = max(0, surplus)            if cap ≤ 0 (no cap)
//	result  = clamp(surplus, 0, cap)     otherwise
func SurplusMinor(liquid, billsDue, goalContribs, cap int64) int64 {
	surplus := liquid - billsDue - goalContribs
	if surplus <= 0 {
		return 0
	}
	if cap <= 0 {
		return surplus
	}
	if surplus > cap {
		return cap
	}
	return surplus
}

// IsScheduleDue reports whether enough time has elapsed since lastRun for the
// given cadence to fire again.
//
// Recognised cadences: "daily", "weekly", "biweekly", "monthly".
// If lastRun is the zero value the schedule is considered immediately due.
// An unrecognised cadence returns false.
func IsScheduleDue(lastRun time.Time, cadence string, now time.Time) bool {
	if lastRun.IsZero() {
		return true
	}
	switch cadence {
	case "daily":
		return !now.Before(lastRun.AddDate(0, 0, 1))
	case "weekly":
		return !now.Before(lastRun.AddDate(0, 0, 7))
	case "biweekly":
		return !now.Before(lastRun.AddDate(0, 0, 14))
	case "monthly":
		return !now.Before(lastRun.AddDate(0, 1, 0))
	default:
		return false
	}
}

// PeriodKey returns a stable, human-readable string key identifying the
// period that contains t. Keys are suitable for deduplication (e.g. ensuring
// a scheduled sweep only runs once per period).
//
// Supported periods:
//
//	"monthly"   → "2006-01"   (year-month)
//	"weekly"    → ISO 8601 year+week, e.g. "2026-W26"
//	"biweekly"  → a stable 14-day bucket anchored to the Unix epoch,
//	              e.g. "2026-B13" where the number is the 0-based bucket index
//	"daily"     → "2006-01-02"
//	default     → "2006-01"   (same as monthly)
func PeriodKey(t time.Time, period string) string {
	switch period {
	case "monthly":
		return t.Format("2006-01")
	case "weekly":
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case "biweekly":
		// Anchor: 2-week buckets counted from the Unix epoch (1970-01-01).
		// Days since epoch / 14 gives a stable, monotonically increasing index.
		epoch := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		days := int(t.UTC().Truncate(24*time.Hour).Sub(epoch).Hours() / 24)
		bucket := days / 14
		// Use the year of the bucket's first day for the label.
		bucketStart := epoch.AddDate(0, 0, bucket*14)
		return fmt.Sprintf("%d-B%d", bucketStart.Year(), bucket)
	case "daily":
		return t.Format("2006-01-02")
	default:
		return t.Format("2006-01")
	}
}
