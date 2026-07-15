// SPDX-License-Identifier: MIT

// Package events holds the pure logic for the Event entity (TX10): resolving
// which transactions belong to an event, auto-associating transactions that fall
// inside an event's date range, and deriving an event's total and per-category
// breakdown. It has no syscall/js dependency and is unit-tested on native Go.
//
// Money is handled in integer minor units. Totals assume a single currency (the
// dataset base) — the same simplifying assumption as domain.SplitsTotal; the
// engine layer applies FX before it reaches these aggregations where needed.
package events

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Members returns the set of transaction IDs mapped to the given event via
// event-member links (domain.TxnLinkEventTxn). A link maps exactly one
// transaction (its first member) to one event.
func Members(links []domain.TxnLink, eventID string) map[string]bool {
	out := map[string]bool{}
	if eventID == "" {
		return out
	}
	for _, l := range links {
		if l.Kind != domain.TxnLinkEventTxn || l.EventID != eventID {
			continue
		}
		if id := l.Primary(); id != "" {
			out[id] = true
		}
	}
	return out
}

// AutoAssociate returns the IDs of transactions whose date the event covers
// ([Start, End), see domain.Event.Covers), in the order the transactions are
// given. Transfers are excluded — a transfer moves money between the household's
// own accounts and is not event spending. This is the CREATE-time association;
// the user opts a transaction out afterward by deleting its link.
func AutoAssociate(ev domain.Event, txns []domain.Transaction) []string {
	out := make([]string, 0)
	for _, t := range txns {
		if t.IsTransfer() {
			continue
		}
		if ev.Covers(t.Date) {
			out = append(out, t.ID)
		}
	}
	return out
}

// CategoryAmount pairs a category id with a minor-unit total. An empty
// CategoryID means the amount was uncategorized.
type CategoryAmount struct {
	CategoryID string
	Minor      int64
}

// Totals computes an event's aggregate over its member transactions: the signed
// total (income positive, spending negative) in minor units and a per-category
// breakdown. Splits are attributed per line to their own category; an unsplit
// transaction attributes its whole amount to its CategoryID (the split contract).
// Transfers among members are ignored defensively. The breakdown is sorted by
// category id for deterministic output.
func Totals(members map[string]bool, txns []domain.Transaction) (totalMinor int64, byCategory []CategoryAmount) {
	byCat := map[string]int64{}
	for _, t := range txns {
		if !members[t.ID] || t.IsTransfer() {
			continue
		}
		totalMinor += t.Amount.Amount
		if t.HasSplits() {
			for _, s := range t.Splits {
				byCat[s.CategoryID] += s.Amount.Amount
			}
			continue
		}
		byCat[t.CategoryID] += t.Amount.Amount
	}
	byCategory = make([]CategoryAmount, 0, len(byCat))
	for cat, minor := range byCat {
		byCategory = append(byCategory, CategoryAmount{CategoryID: cat, Minor: minor})
	}
	sort.Slice(byCategory, func(i, j int) bool { return byCategory[i].CategoryID < byCategory[j].CategoryID })
	return totalMinor, byCategory
}

// SpendMinor returns the magnitude of an event's net spending (the positive
// amount of money that flowed out) over its members. It is -min(total, 0) so an
// event that is net spending reports a positive figure and a net-inflow event
// reports zero. Convenience for surfaces that show "how much this trip cost".
func SpendMinor(members map[string]bool, txns []domain.Transaction) int64 {
	total, _ := Totals(members, txns)
	if total >= 0 {
		return 0
	}
	return -total
}
