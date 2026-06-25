// SPDX-License-Identifier: MIT

// Package smartdigest builds the SMART proactive digest: a single summary
// item produced from the top active insights across all pages, posted to the
// notification feed on a user-chosen cadence. It is pure Go with no
// syscall/js, so it unit-tests on native Go.
//
// Design contract:
//   - now must be passed explicitly — this package never calls time.Now().
//   - Deduplication is period-keyed: each cadence window (e.g. "2026-W24")
//     produces at most one digest. A delivered period key is recorded in the
//     caller-owned notify.DeliveredLog so subsequent calls within the same
//     window return ok=false without re-spending.
//   - The digest is suppressed when there are no active insights worth posting.
package smartdigest

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/notify"
	"github.com/monstercameron/CashFlux/internal/smart"
)

// Item is the output of Build: the title, body, unix-second timestamp, and
// stable ID for one digest posting. The wasm driver layer maps it to a
// uistate.FeedItem before writing to the notification feed.
type Item struct {
	// ID is stable and period-scoped (equal to the PeriodKey) so the feed's
	// ID-based deduplication in PrependNotifyFeed never double-posts.
	ID    string
	Title string
	Body  string
	At    int64 // unix seconds of now at build time
}

// digestTopN is the maximum number of insights to include in one digest.
const digestTopN = 5

// digestMinN is the minimum number of insights required to post a digest.
// A digest with zero findings would read as "nothing to report" — suppress
// rather than post an empty summary.
const digestMinN = 1

// PeriodKey returns the stable deduplication key for the digest window that
// contains now, based on the feature's run cadence. It is the same token
// recorded in the DeliveredLog so a re-run within the same window is a no-op.
func PeriodKey(c smart.Cadence, now time.Time) string {
	switch c {
	case smart.CadenceDaily:
		return "digest:" + notify.DayKey(now)
	case smart.CadenceWeekly:
		return "digest:" + notify.WeekKey(now)
	case smart.CadenceMonthly:
		return "digest:" + notify.MonthKey(now)
	default:
		// For non-interval cadences (on_open, on_change) fall back to a day key
		// as the smallest sensible dedup window — same-day re-opens don't spam.
		return "digest:" + notify.DayKey(now)
	}
}

// periodLabel returns a short, friendly period name for the digest title.
func periodLabel(c smart.Cadence, now time.Time) string {
	switch c {
	case smart.CadenceDaily:
		return "daily"
	case smart.CadenceWeekly:
		_, w := now.ISOWeek()
		return fmt.Sprintf("weekly (week %d)", w)
	case smart.CadenceMonthly:
		return "monthly (" + now.Format("January") + ")"
	default:
		return "money"
	}
}

// Build selects the top insights from active (already filtered) insights,
// builds a single digest Item, records the period key as delivered, and
// returns (item, true). It returns (zero, false) when:
//   - active is empty, or
//   - the period key is already in delivered (dedup — same window, no re-post).
//
// The caller must pass now explicitly; this function never calls time.Now().
// The Item.ID is stable and derived from the period key so the notification
// feed's ID-based deduplication can detect and skip a re-delivery.
func Build(
	active []smart.Insight,
	now time.Time,
	cadence smart.Cadence,
	delivered notify.DeliveredLog,
) (Item, bool) {
	if len(active) == 0 {
		return Item{}, false
	}

	key := PeriodKey(cadence, now)

	// Dedup: if this period was already delivered, do not re-post.
	if delivered.Has(key) {
		return Item{}, false
	}

	// Select top N by severity (descending), stable tie-break by Key.
	picked := selectTop(active, digestTopN)
	if len(picked) < digestMinN {
		return Item{}, false
	}

	// Build the human-readable body: a short bullet list of the picked titles.
	var body strings.Builder
	for i, ins := range picked {
		if i > 0 {
			body.WriteString("\n")
		}
		body.WriteString("• ")
		body.WriteString(ins.Title)
	}

	noun := "things"
	if len(picked) == 1 {
		noun = "thing"
	}
	label := periodLabel(cadence, now)
	title := fmt.Sprintf("Your %s digest — %d %s to look at", label, len(picked), noun)

	item := Item{
		ID:    key, // stable, period-scoped; PrependNotifyFeed deduplicates by ID
		Title: title,
		Body:  body.String(),
		At:    now.Unix(),
	}

	// Record delivery so a re-run within the same window is suppressed.
	delivered.Mark(key)

	return item, true
}

// selectTop returns up to n insights sorted by severity descending, with a
// stable tie-break on Key ascending. It does not mutate the input slice.
func selectTop(in []smart.Insight, n int) []smart.Insight {
	cp := make([]smart.Insight, len(in))
	copy(cp, in)
	sort.SliceStable(cp, func(i, j int) bool {
		if cp[i].Severity != cp[j].Severity {
			return cp[i].Severity > cp[j].Severity
		}
		return cp[i].Key < cp[j].Key
	})
	if len(cp) > n {
		cp = cp[:n]
	}
	return cp
}
