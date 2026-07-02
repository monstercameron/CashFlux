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
	// Route is the in-app path the notification links to (its alerting resource — e.g. a
	// bill-due item → /bills, a budget alert → /budgets). Empty = not clickable. Set by
	// runNotifyCatchUp from the source event; JSON round-trips, no store migration needed.
	Route string `json:"route,omitempty"`
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
