// SPDX-License-Identifier: MIT

// Package notificationview prepares notification triage and collapsed groups
// using pure Go so the screen's filtering and grouping can be tested natively.
package notificationview

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

// Filter selects a triage bucket, household member and optional severity.
// "watching" selects informational items; every other View selects attention.
type Filter struct{ View, MemberID, Severity string }

// Select hides snoozed items, keeps the first copy of each title/body event,
// then applies the triage, member and severity lenses. Household-wide items
// pass every member lens. Empty severity is treated as info. Input is immutable.
func Select(feed []uistate.FeedItem, now int64, f Filter) []uistate.FeedItem {
	type signature struct{ title, body string }
	seen := make(map[signature]struct{}, len(feed))
	out := make([]uistate.FeedItem, 0, len(feed))
	for _, it := range feed {
		if it.SnoozedUntil > now {
			continue
		}
		key := signature{it.Title, it.Body}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if uistate.NeedsAttention(it) == (f.View == "watching") || !it.MatchesMemberFilter(f.MemberID) {
			continue
		}
		sev := it.Severity
		if sev == "" {
			sev = "info"
		}
		if f.Severity != "" && f.Severity != sev {
			continue
		}
		out = append(out, it)
	}
	return out
}

// Counts describes the currently selected bucket before severity filtering.
type Counts struct{ Critical, Warning, Info, Unread, New int }

// Count tallies visible items without allocating intermediate unread/new lists.
func Count(items []uistate.FeedItem, lastSeen int64) Counts {
	var c Counts
	for _, it := range items {
		switch it.Severity {
		case "critical":
			c.Critical++
		case "warning":
			c.Warning++
		default:
			c.Info++
		}
		if !it.Read {
			c.Unread++
		}
		if it.At > lastSeen {
			c.New++
		}
	}
	return c
}

// Kind returns the rule prefix of a notification ID, or empty for legacy IDs.
func Kind(id string) string {
	if i := strings.IndexByte(id, '@'); i >= 0 {
		return id[:i]
	}
	return ""
}

// Group is one render entry. Kind is empty for an individual item. Grouped
// entries carry all items in their existing attention order and appear where
// their first item appeared; critical items disqualify the entire rule kind.
type Group struct {
	Kind  string
	Items []uistate.FeedItem
}

// Key identifies an entry across removals and filter changes, preserving a
// group's expansion state without moving it to the next rendered group.
func (g Group) Key() string {
	if g.Kind != "" {
		return "group:" + g.Kind
	}
	if len(g.Items) > 0 {
		return "item:" + g.Items[0].ID
	}
	return ""
}

// GroupItems collapses kinds at or above minimum unless any item is critical.
// minimum <= 0 disables grouping. The input order and storage are untouched.
func GroupItems(items []uistate.FeedItem, minimum int) []Group {
	type tally struct {
		count    int
		critical bool
	}
	counts := make(map[string]tally)
	for _, it := range items {
		k := Kind(it.ID)
		if k == "" {
			continue
		}
		c := counts[k]
		c.count++
		c.critical = c.critical || it.Severity == "critical"
		counts[k] = c
	}
	out := make([]Group, 0, len(items))
	positions := make(map[string]int)
	for _, it := range items {
		k := Kind(it.ID)
		c := counts[k]
		if minimum <= 0 || k == "" || c.count < minimum || c.critical {
			out = append(out, Group{Items: []uistate.FeedItem{it}})
			continue
		}
		if pos, ok := positions[k]; ok {
			out[pos].Items = append(out[pos].Items, it)
			continue
		}
		positions[k] = len(out)
		out = append(out, Group{Kind: k, Items: make([]uistate.FeedItem, 0, c.count)})
		out[len(out)-1].Items = append(out[len(out)-1].Items, it)
	}
	return out
}
