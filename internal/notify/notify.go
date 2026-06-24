// SPDX-License-Identifier: MIT

// Package notify is the pure, client-side notification core for CashFlux
// (B19 Phase A). It defines notification and rule types and the deterministic
// logic that decides when a rule may fire — quiet hours, channel selection, and
// idempotency keys for catch-up-on-wake — with no syscall/js and no I/O, so it
// unit-tests on native Go. The wasm/UI shell drives it (catch-up on open/return,
// the in-app center, and the browser Notifications API) on top of this core.
//
// Phase A is fully client-side: notifications fire only while the app is open
// (in-app center + toasts + browser pop-ups). External email/SMS is the deferred
// Phase B and intentionally has no presence here yet.
package notify

import (
	"fmt"
	"sort"
	"time"
)

// Channel is a delivery surface for a notification. Phase A ships InApp and
// Browser (both fire only while a tab is open); Email and SMS are Phase B.
type Channel string

const (
	ChannelInApp   Channel = "in-app"
	ChannelBrowser Channel = "browser"
)

// Event is the kind of situation a notification rule watches for. The set is
// the Phase-A recommended slice; their per-event evaluation lands with the
// catch-up engine. The constants are stable storage values.
type Event string

const (
	EventBillDue          Event = "bill-due"          // a bill's due date is near or passed
	EventBudgetThreshold  Event = "budget-threshold"  // a budget crossed near/over
	EventGoalMilestone    Event = "goal-milestone"    // a goal hit a pace/percent milestone
	EventStaleBalance     Event = "stale-balance"     // an account hasn't been updated in a while
	EventLargeTransaction Event = "large-transaction" // a transaction exceeded an amount
	EventDigest           Event = "digest"            // a periodic (weekly/monthly) summary
	EventBackupDue        Event = "backup-due"        // a periodic data-backup reminder is due
)

// Severity ranks a notification for ordering and styling in the center.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityCritical
)

// minutesPerDay bounds the quiet-hours window values.
const minutesPerDay = 24 * 60

// Rule is a user-configured notification preference for one Event. It is a
// plain value (serialized into the durable store); all gating logic lives on it
// as pure methods so the UI and the catch-up engine share one source of truth.
type Rule struct {
	ID       string    // stable id
	Event    Event     // what this rule watches
	Enabled  bool      // master on/off
	Channels []Channel // where matches are delivered

	// Threshold is the event-specific trigger level (e.g. budget percent, days
	// before a bill, stale-days, or a large-amount in minor units). Its meaning
	// is owned by the per-event evaluation; zero means "use the event default".
	Threshold int

	// QuietStartMin and QuietEndMin define a daily do-not-disturb window in
	// minutes since local midnight, in [0, 1440). End is exclusive. When the two
	// are equal, quiet hours are off. The window may wrap past midnight (start >
	// end), e.g. 22:00–07:00 is QuietStartMin 1320, QuietEndMin 420.
	QuietStartMin int
	QuietEndMin   int

	// FrequencyCap is the most notifications this rule may emit per its natural
	// period (0 = uncapped). The catch-up engine enforces it via a DeliveredLog.
	FrequencyCap int
}

// HasChannel reports whether the rule delivers on c.
func (r Rule) HasChannel(c Channel) bool {
	for _, ch := range r.Channels {
		if ch == c {
			return true
		}
	}
	return false
}

// InQuietHours reports whether the local clock time of t falls inside the rule's
// do-not-disturb window. A zero-width window (start == end) disables quiet hours.
// The window is half-open [start, end) and may wrap past midnight.
func (r Rule) InQuietHours(t time.Time) bool {
	if r.QuietStartMin == r.QuietEndMin {
		return false
	}
	m := t.Hour()*60 + t.Minute()
	s, e := r.QuietStartMin, r.QuietEndMin
	if s < e {
		return m >= s && m < e
	}
	// Window wraps past midnight, e.g. 22:00–07:00.
	return m >= s || m < e
}

// CanFireAt reports whether the rule is eligible to emit at time t: it must be
// enabled, deliver on at least one channel, and not be inside quiet hours.
// Per-event data conditions and the frequency cap are evaluated separately.
func (r Rule) CanFireAt(t time.Time) bool {
	return r.Enabled && len(r.Channels) > 0 && !r.InQuietHours(t)
}

// Notification is a single emitted item shown in the center / as a toast or
// browser pop-up. DedupeKey identifies the occurrence so catch-up on repeated
// opens doesn't re-fire the same thing.
type Notification struct {
	ID        string
	RuleID    string
	Event     Event
	Title     string
	Body      string
	At        time.Time
	Severity  Severity
	DedupeKey string
}

// DedupeKey builds the idempotency key for one occurrence of a rule — the rule
// id plus a string identifying the occurrence it covers (e.g. a due date, a
// budget period, or a digest period). The catch-up engine records delivered
// keys so reopening the app doesn't replay the same notifications.
func DedupeKey(ruleID, occurrence string) string {
	return ruleID + "@" + occurrence
}

// DayKey, WeekKey, and MonthKey render stable occurrence strings for the common
// schedules, so a daily/weekly/monthly notification fires at most once per
// period regardless of how often the app is reopened. They use t's own location.
func DayKey(t time.Time) string {
	return t.Format("2006-01-02")
}

// WeekKey renders an ISO-year-week key (e.g. "2026-W25").
func WeekKey(t time.Time) string {
	y, w := t.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", y, w)
}

// MonthKey renders a year-month key (e.g. "2026-06").
func MonthKey(t time.Time) string {
	return t.Format("2006-01")
}

// DeliveredLog records which notification occurrences have already been
// delivered, keyed by DedupeKey, so catch-up is idempotent across reopens. It is
// a map for direct (de)serialization into the durable store; use New or a
// non-nil literal before Mark.
type DeliveredLog map[string]bool

// NewDeliveredLog returns an empty, ready-to-use log.
func NewDeliveredLog() DeliveredLog { return DeliveredLog{} }

// Has reports whether key has already been delivered. Safe on a nil log.
func (l DeliveredLog) Has(key string) bool { return l[key] }

// Mark records key as delivered. The receiver must be non-nil.
func (l DeliveredLog) Mark(key string) { l[key] = true }

// Keys returns the delivered keys in sorted order (handy for stable persistence
// and tests).
func (l DeliveredLog) Keys() []string {
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
