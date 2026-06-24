// SPDX-License-Identifier: MIT

// Package backup decides when to remind someone to back up (export) their
// CashFlux data, so a local-first install doesn't quietly drift far from its
// last safe copy. It is pure Go with no platform dependencies and is unit-tested
// on native Go; the UI layer tracks the last-backup timestamp and surfaces the
// nudge.
package backup

import (
	"strings"
	"time"
)

// Cadence is how often a backup reminder should fire.
type Cadence string

const (
	// Off disables backup reminders entirely.
	Off Cadence = "off"
	// Weekly reminds once every seven days since the last backup.
	Weekly Cadence = "weekly"
	// Monthly reminds once every calendar month since the last backup.
	Monthly Cadence = "monthly"
)

// DefaultCadence is the gentle, non-naggy default: a monthly nudge.
const DefaultCadence = Monthly

// ParseCadence normalizes a stored or user-supplied value to a known Cadence,
// falling back to Off for anything unrecognized (so a bad value never nags).
func ParseCadence(s string) Cadence {
	switch Cadence(strings.ToLower(strings.TrimSpace(s))) {
	case Weekly:
		return Weekly
	case Monthly:
		return Monthly
	default:
		return Off
	}
}

// Schedules reports whether the cadence fires reminders at all.
func (c Cadence) Schedules() bool {
	return c == Weekly || c == Monthly
}

// NextDue returns when the next backup reminder is due after lastBackupAt, and
// whether the cadence schedules one. A zero lastBackupAt (never backed up)
// yields a due time far in the past, so an enabled cadence is due immediately.
func NextDue(c Cadence, lastBackupAt time.Time) (time.Time, bool) {
	switch c {
	case Weekly:
		return lastBackupAt.AddDate(0, 0, 7), true
	case Monthly:
		return lastBackupAt.AddDate(0, 1, 0), true
	default:
		return time.Time{}, false
	}
}

// Due reports whether a backup reminder should fire now: the cadence schedules
// reminders and at least one interval has elapsed since lastBackupAt. Never
// backed up (zero lastBackupAt) is due as soon as the cadence is enabled.
func Due(c Cadence, lastBackupAt, now time.Time) bool {
	next, ok := NextDue(c, lastBackupAt)
	if !ok {
		return false
	}
	return !now.Before(next)
}

// DaysSince returns whole days between lastBackupAt and now, clamped at zero. A
// zero lastBackupAt returns 0 (unknown), so callers can show "never backed up"
// rather than a nonsensical age.
func DaysSince(lastBackupAt, now time.Time) int {
	if lastBackupAt.IsZero() {
		return 0
	}
	d := now.Sub(lastBackupAt)
	if d < 0 {
		return 0
	}
	return int(d / (24 * time.Hour))
}
