package notify

import (
	"sort"
	"time"
)

// Candidate is a potential notification occurrence discovered for the gap since
// the app was last open. The per-event evaluators (built later) produce these;
// CatchUp turns them into the notifications to actually surface. Keeping the
// engine candidate-driven lets it be tested without any event-specific data and
// keeps the "which events ship first" decision out of the core.
type Candidate struct {
	RuleID        string    // the rule this occurrence belongs to
	Event         Event     // event kind (for display/grouping)
	OccurrenceKey string    // stable per-occurrence id for dedupe (a due date, a period, …)
	At            time.Time // when the occurrence is dated
	Title         string
	Body          string
	Severity      Severity
}

// CatchUp turns the candidate occurrences discovered for the gap since the app
// was last open into the notifications to surface now (the "while you were away"
// summary). It applies rule gating (the rule must exist, be enabled, and have at
// least one channel), idempotency (occurrences already in log are skipped), and
// each rule's frequency cap (at most FrequencyCap emitted per rule, keeping the
// most recent). It mutates log, marking every fresh occurrence it considered —
// including ones dropped by the cap — as delivered, so reopening the app neither
// replays nor re-floods. Results are returned newest-first (ties broken by key).
//
// Quiet hours are intentionally NOT applied here: a catch-up summary is shown
// when the user actively opens the app, which is not an interruption. Quiet
// hours gate live, in-session firing (Rule.CanFireAt), not the catch-up digest.
func CatchUp(rules []Rule, candidates []Candidate, now time.Time, log DeliveredLog) []Notification {
	if log == nil {
		log = NewDeliveredLog()
	}
	byID := make(map[string]Rule, len(rules))
	for _, r := range rules {
		byID[r.ID] = r
	}

	// Collect fresh (not-yet-delivered) candidates per eligible rule.
	type fresh struct {
		c   Candidate
		key string
	}
	perRule := map[string][]fresh{}
	for _, c := range candidates {
		r, ok := byID[c.RuleID]
		if !ok || !r.Enabled || len(r.Channels) == 0 {
			continue
		}
		key := DedupeKey(c.RuleID, c.OccurrenceKey)
		if log.Has(key) {
			continue
		}
		perRule[c.RuleID] = append(perRule[c.RuleID], fresh{c: c, key: key})
	}

	var out []Notification
	for ruleID, list := range perRule {
		r := byID[ruleID]
		// Chronological so "most recent" is well-defined for the cap.
		sort.SliceStable(list, func(i, j int) bool { return list[i].c.At.Before(list[j].c.At) })

		// Mark every fresh occurrence delivered (so a long gap collapses and never
		// replays), but only emit the kept subset under the frequency cap.
		keep := list
		if r.FrequencyCap > 0 && len(list) > r.FrequencyCap {
			keep = list[len(list)-r.FrequencyCap:]
		}
		kept := make(map[string]bool, len(keep))
		for _, f := range keep {
			kept[f.key] = true
		}
		for _, f := range list {
			log.Mark(f.key)
			if !kept[f.key] {
				continue
			}
			out = append(out, Notification{
				ID:        f.key,
				RuleID:    ruleID,
				Event:     f.c.Event,
				Title:     f.c.Title,
				Body:      f.c.Body,
				At:        f.c.At,
				Severity:  f.c.Severity,
				DedupeKey: f.key,
			})
		}
	}

	// Newest first, with a stable tiebreaker so the result is deterministic
	// regardless of map iteration order.
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].At.Equal(out[j].At) {
			return out[i].At.After(out[j].At)
		}
		return out[i].DedupeKey < out[j].DedupeKey
	})
	return out
}
