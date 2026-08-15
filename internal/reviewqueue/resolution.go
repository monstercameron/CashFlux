// SPDX-License-Identifier: MIT

package reviewqueue

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/payeealias"
)

// --- C493: a durable vocabulary for "I looked at this" ----------------------

// State is what the user has decided about a queued charge. Skipping used to be
// in-memory only, so a skipped item re-blocked the head of the queue on the next
// visit and there was no way to say "this one is fine as it is".
type State string

const (
	// StateOpen: still needs a decision. The zero value, so an unknown id is open.
	StateOpen State = ""
	// StateSnoozed: hidden until Until, then it comes back.
	StateSnoozed State = "snoozed"
	// StateDismissed: the user has judged it; never queue it again.
	StateDismissed State = "dismissed"
)

// Resolution records one durable decision. Kept as a plain struct with exported
// fields so callers can JSON-marshal a map of them for persistence with no glue.
type Resolution struct {
	State State     `json:"state"`
	Until time.Time `json:"until,omitempty"`
}

// Resolutions maps a transaction id to its decision.
type Resolutions map[string]Resolution

// Active reports whether the resolution still suppresses the transaction at
// time now. A dismissal is permanent; a snooze expires.
func (r Resolution) Active(now time.Time) bool {
	switch r.State {
	case StateDismissed:
		return true
	case StateSnoozed:
		return r.Until.After(now)
	}
	return false
}

// Snooze hides a transaction until until.
func (rs Resolutions) Snooze(txnID string, until time.Time) {
	if txnID == "" {
		return
	}
	rs[txnID] = Resolution{State: StateSnoozed, Until: until}
}

// Dismiss hides a transaction permanently.
func (rs Resolutions) Dismiss(txnID string) {
	if txnID == "" {
		return
	}
	rs[txnID] = Resolution{State: StateDismissed}
}

// Reopen clears any decision, putting the transaction back in the queue.
func (rs Resolutions) Reopen(txnID string) { delete(rs, txnID) }

// Suppressed reports whether a transaction is currently hidden by a decision.
func (rs Resolutions) Suppressed(txnID string, now time.Time) bool {
	if rs == nil {
		return false
	}
	r, ok := rs[txnID]
	return ok && r.Active(now)
}

// Prune drops expired snoozes so the persisted map cannot grow without bound.
// Dismissals are kept: they are the user's standing decision.
func (rs Resolutions) Prune(now time.Time) {
	for id, r := range rs {
		if r.State == StateSnoozed && !r.Until.After(now) {
			delete(rs, id)
		}
	}
}

// --- C492: splits ------------------------------------------------------------

// ReasonSplitUnbalanced: the transaction is split across categories but the
// split amounts do not add up to the transaction's own amount, so some of the
// money is unaccounted for.
//
// A transaction whose splits DO reconcile is fully categorized — every analytics
// path reads Splits in preference to CategoryID — so it must not be queued at
// all. It previously was, because Needs only asked whether CategoryID was empty,
// and "fixing" it by assigning a flat category removed it from the queue while
// changing not one dollar of budget math.
const ReasonSplitUnbalanced Reason = 2

// --- C494: ordering + merchant grouping --------------------------------------

// MerchantKey returns the grouping key for a transaction's payee: the same
// normalization the batch action uses, so the group a user is shown is exactly
// the group an action will touch. Grouping on the raw descriptor instead means
// two charges displayed as "Amazon" never batch together.
func MerchantKey(t domain.Transaction) string {
	raw := strings.TrimSpace(t.Payee)
	if raw == "" {
		raw = strings.TrimSpace(t.Desc)
	}
	if raw == "" {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(payeealias.Normalize(raw)))
}

// Ranked is a queued transaction with the ranking inputs a caller supplied.
type Ranked struct {
	Txn domain.Transaction
	// Confidence is how sure the suggestion for this charge is: higher sorts
	// first. Callers pass 0 when they have no suggestion at all.
	Confidence int
}

// Group is one merchant's queued charges, ordered newest-first within the group.
type Group struct {
	// Key is the merchant grouping key.
	Key string
	// Merchant is the cleaned display name.
	Merchant string
	// Items are the queued transactions for this merchant.
	Items []domain.Transaction
	// Confidence is the group's ranking score — the MINIMUM across its items, so
	// a group is only as confirmable as its least certain member. Confirming a
	// whole merchant must never be safer-looking than its worst row.
	Confidence int
}

// Total returns the summed minor units of the group's charges.
func (g Group) Total() int64 {
	var n int64
	for _, t := range g.Items {
		n += t.Amount.Amount
	}
	return n
}

// GroupByMerchant buckets a ranked queue by merchant and orders the groups by
// confidence (highest first), then by size (biggest first — clearing a large
// merchant removes the most work), then by key for a stable result.
func GroupByMerchant(in []Ranked) []Group {
	byKey := map[string]*Group{}
	var order []string
	for _, r := range in {
		k := MerchantKey(r.Txn)
		if k == "" {
			k = "~unknown~"
		}
		g, ok := byKey[k]
		if !ok {
			raw := r.Txn.Payee
			if strings.TrimSpace(raw) == "" {
				raw = r.Txn.Desc
			}
			g = &Group{Key: k, Merchant: payeealias.Normalize(raw), Confidence: r.Confidence}
			byKey[k] = g
			order = append(order, k)
		}
		g.Items = append(g.Items, r.Txn)
		if r.Confidence < g.Confidence {
			g.Confidence = r.Confidence
		}
	}
	out := make([]Group, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		if len(out[i].Items) != len(out[j].Items) {
			return len(out[i].Items) > len(out[j].Items)
		}
		return out[i].Key < out[j].Key
	})
	return out
}
