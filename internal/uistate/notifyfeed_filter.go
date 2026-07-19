// SPDX-License-Identifier: MIT

// Pure, platform-independent types and feed-filter helpers for the Notification
// Center (C268). This file has NO build constraints so it compiles on native Go
// and can be covered by regular `go test` without a WASM environment.

package uistate

// FeedItem is one entry in the Notification Center (C75): the title/body of an
// emitted notification, when it fired, whether the user has read it, the
// severity level for visual differentiation (C267), and an optional snooze
// deadline (C268). Severity is one of "info", "warning", or "critical". Legacy
// items with no Severity value render at the info level — no migration needed.
// SnoozedUntil is a unix-second timestamp; zero or past means not snoozed.
type FeedItem struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Body         string `json:"body,omitempty"`
	At           int64  `json:"at"` // unix seconds
	Read         bool   `json:"read,omitempty"`
	Severity     string `json:"severity,omitempty"`     // "info" | "warning" | "critical"; empty = info
	SnoozedUntil int64  `json:"snoozedUntil,omitempty"` // unix seconds; zero = not snoozed (C268)
	// DueAt is the unix-second due date carried by due-date alerts (bill-due),
	// so the center can re-render a stale "due in N days" body as "overdue by
	// N days" once the date passes — a notification body is written once, but
	// the obligation keeps aging. Zero for notifications with no due date.
	DueAt int64 `json:"dueAt,omitempty"`
}

// OverdueDays returns how many whole calendar days past its due date a
// due-date alert is at now (both unix seconds): 0 when dueAt is zero, unset,
// today, or still ahead. Day boundaries are evaluated in UTC to stay
// deterministic; a bill due yesterday reports 1.
func OverdueDays(dueAt, now int64) int {
	if dueAt <= 0 {
		return 0
	}
	day := func(ts int64) int64 { return ts / 86400 }
	d := int(day(now) - day(dueAt))
	if d < 0 {
		return 0
	}
	return d
}

// DueToday reports whether a due-date alert falls on the current calendar day
// (both unix seconds): true only when dueAt is set and shares now's day and is
// not already past. It is the "due in 0 days, not yet overdue" case a bill-due
// row shows as "Due today" instead of the awkward "Due in 0 days". Day
// boundaries are evaluated in UTC to match OverdueDays, so exactly one of
// DueToday / OverdueDays>0 is ever true for a given alert. Zero/unset dueAt is
// never due today.
func DueToday(dueAt, now int64) bool {
	if dueAt <= 0 {
		return false
	}
	day := func(ts int64) int64 { return ts / 86400 }
	return day(dueAt) == day(now)
}

// NeedsAttention reports whether a feed item belongs in the "Needs you" triage
// bucket: critical or warning severity — an action to take or a decision to
// make. Everything else (info notes, reminders, changed-item digests) is calm
// "Watching" material. This is the split that turns a punishing wall of counts
// into a short list of what actually wants the user right now.
func NeedsAttention(it FeedItem) bool {
	switch it.Severity {
	case "critical", "warning":
		return true
	default:
		return false
	}
}

// PartitionTriage splits items into the "Needs you" bucket (NeedsAttention true)
// and the "Watching" bucket (everything else), preserving the input order within
// each bucket. Neither slice shares element storage that callers may mutate
// through; both are freshly allocated. A nil/empty input yields two empty slices.
func PartitionTriage(items []FeedItem) (needs, watching []FeedItem) {
	needs = make([]FeedItem, 0, len(items))
	watching = make([]FeedItem, 0, len(items))
	for _, it := range items {
		if NeedsAttention(it) {
			needs = append(needs, it)
		} else {
			watching = append(watching, it)
		}
	}
	return needs, watching
}

// DedupeFeed removes entries that repeat the same underlying event — an
// identical Title and Body — keeping the FIRST occurrence and preserving order.
// Digest emitters can surface one event more than once (e.g. a rolled-up bill
// that also fires its own bill-due card); showing it twice reads as noise, so
// the feed collapses exact repeats. Items are matched on their stored Body, not
// any render-time overdue rewrite, so the signature is stable. The returned
// slice is freshly allocated; the input is not mutated.
func DedupeFeed(items []FeedItem) []FeedItem {
	seen := make(map[string]struct{}, len(items))
	out := make([]FeedItem, 0, len(items))
	for _, it := range items {
		sig := it.Title + "\x00" + it.Body
		if _, dup := seen[sig]; dup {
			continue
		}
		seen[sig] = struct{}{}
		out = append(out, it)
	}
	return out
}

// NewSinceLastSeen returns the subset of items whose At timestamp is strictly
// greater than lastSeen (C271). Items with At == lastSeen are excluded: the
// boundary semantics treat lastSeen as the instant the center was last viewed,
// so an item present at that exact moment is not "new". An empty or nil items
// slice returns nil. The returned slice shares backing storage with items;
// callers must not mutate its elements.
func NewSinceLastSeen(items []FeedItem, lastSeen int64) []FeedItem {
	out := make([]FeedItem, 0, len(items))
	for _, it := range items {
		if it.At > lastSeen {
			out = append(out, it)
		}
	}
	return out
}

// VisibleFeed returns only the items that should be shown in the Notification
// Center at the given unix-second timestamp now. An item is hidden when it has
// a SnoozedUntil value that is strictly greater than now (i.e. still snoozed).
// Items with SnoozedUntil == 0 or SnoozedUntil <= now are always included.
// The returned slice shares backing storage with items; callers must not mutate
// its elements.
func VisibleFeed(items []FeedItem, now int64) []FeedItem {
	out := make([]FeedItem, 0, len(items))
	for _, it := range items {
		if it.SnoozedUntil > now {
			continue // still snoozed — hide
		}
		out = append(out, it)
	}
	return out
}
