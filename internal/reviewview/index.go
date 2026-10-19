// SPDX-License-Identifier: MIT

// Package reviewview derives the Review surface from explicit snapshots. It has
// no browser, framework, persistence, or clock dependencies.
package reviewview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/reviewscope"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// Tier describes the strength of local evidence for a merchant-wide choice.
type Tier int

const (
	// TierNone means no local suggestion is available.
	TierNone Tier = iota
	// TierLook means a suggestion needs a human glance.
	TierLook
	// TierReady means all charges have high-confidence, matching suggestions.
	TierReady
)

// Row is one merchant group with everything the UI needs, computed once
// per render. C510: building this ONCE and passing it down is what keeps a
// 250-row list from being O(n²) — sibling counts, suggestions and merchant
// lookups all used to be recomputed per row.
type Row struct {
	Group              reviewqueue.Group
	Tier               Tier
	Suggestion         catsuggest.Suggestion
	HasSugg            bool
	SuggestionConflict bool
}

// Index is the scoped queue and the context lookups shared by its child views.
type Index struct {
	Rows  []Row
	Total int // charges still queued IN THE ACTIVE SCOPE
	Scope reviewscope.Scope
	// QueueTotal and ViewTotal are the two denominators the scope selector offers
	// (C554): everything awaiting review, and the part of it the ledger's own
	// search/filters/period currently show. Both are computed every build so the
	// selector can state the count of the scope you are NOT in — a choice between
	// two options where only one has a number attached is not really a choice.
	QueueTotal int
	ViewTotal  int
	Byid       map[string]Row
	// TxnByID lets the context band resolve link members without rescanning
	// every transaction per row (it was an O(links x transactions) nested loop).
	TxnByID map[string]domain.Transaction
	// Dupes maps a transaction id to the ids it looks like a duplicate of.
	// Computed once here rather than re-running duplicate detection over the
	// whole ledger on every render of the single-charge view.
	Dupes map[string][]string
}

// VisibleLoanPayments excludes dismissed offers while retaining suggestion order.
func VisibleLoanPayments(all []txnclassify.PaymentSuggestion, dismissed map[string]bool) []txnclassify.PaymentSuggestion {
	visible := make([]txnclassify.PaymentSuggestion, 0, len(all))
	for _, suggestion := range all {
		if !dismissed[suggestion.TransactionID] {
			visible = append(visible, suggestion)
		}
	}
	return visible
}

// PruneDismissals retains decisions while their transactions exist and are not
// transfers. A temporarily missing suggestion must not erase a user's decision.
// The second result reports whether any stored entry was removed.
func PruneDismissals(dismissed map[string]bool, transactions []domain.Transaction) (map[string]bool, bool) {
	active := make(map[string]struct{}, len(transactions))
	for _, txn := range transactions {
		if !txn.IsTransfer() {
			active[txn.ID] = struct{}{}
		}
	}
	pruned := make(map[string]bool, len(dismissed))
	changed := false
	for id, value := range dismissed {
		if _, ok := active[id]; ok && value {
			pruned[id] = true
		} else {
			changed = true
		}
	}
	return pruned, changed
}

// SharedSuggestion promotes evidence only if every charge agrees on a category.
// Confidence is the weakest member's; partial or differing evidence conflicts.
func SharedSuggestion(g reviewqueue.Group, byID map[string]catsuggest.Suggestion) (catsuggest.Suggestion, bool, bool) {
	var shared catsuggest.Suggestion
	foundAny, missing := false, false
	for _, txn := range g.Items {
		s, ok := byID[txn.ID]
		if !ok || s.CategoryID == "" {
			missing = true
			continue
		}
		if !foundAny {
			shared = s
			foundAny = true
			continue
		}
		if s.CategoryID != shared.CategoryID {
			return catsuggest.Suggestion{}, false, true
		}
		if s.Confidence < shared.Confidence {
			shared.Confidence = s.Confidence
		}
	}
	if foundAny && missing {
		return catsuggest.Suggestion{}, false, true
	}
	return shared, foundAny && len(g.Items) > 0, false
}

// TierOf classifies available, nonempty local evidence by confidence.
func TierOf(s catsuggest.Suggestion, ok bool) Tier {
	if !ok || s.CategoryID == "" {
		return TierNone
	}
	if s.Confidence >= catsuggest.ConfHigh {
		return TierReady
	}
	return TierLook
}

// LedgerVisibleIDs applies the ledger's filters and ordering without pagination.
func LedgerVisibleIDs(txns []domain.Transaction, c txnfilter.Criteria) []string {
	c.Page, c.PageSize = 1, txnfilter.PageSizeAll
	shown := txnfilter.Apply(txns, c)
	out := make([]string, 0, len(shown))
	for _, t := range shown {
		out = append(out, t.ID)
	}
	return out
}

// RestrictQueue intersects queue and visible IDs while preserving queue order.
func RestrictQueue(queue []domain.Transaction, visible []string) []domain.Transaction {
	ids := make([]string, 0, len(queue))
	for _, t := range queue {
		ids = append(ids, t.ID)
	}
	keep := make(map[string]bool, len(queue))
	for _, id := range reviewscope.Restrict(ids, visible) {
		keep[id] = true
	}
	out := make([]domain.Transaction, 0, len(keep))
	for _, t := range queue {
		if keep[t.ID] {
			out = append(out, t)
		}
	}
	return out
}

// ChargeIDs returns a new slice of the row's scoped transaction identities.
func ChargeIDs(r Row) []string {
	out := make([]string, 0, len(r.Group.Items))
	for _, t := range r.Group.Items {
		out = append(out, t.ID)
	}
	return out
}

// TierCount counts merchants and charges in one confidence tier.
func TierCount(rows []Row, t Tier) (merchants, charges int) {
	for _, r := range rows {
		if r.Tier != t {
			continue
		}
		merchants++
		charges += len(r.Group.Items)
	}
	return
}

// TierClass selects the existing CSS modifier for a confidence tier.
func TierClass(t Tier) string {
	switch t {
	case TierReady:
		return "is-ready"
	case TierLook:
		return "is-look"
	}
	return "is-none"
}

// Input is a frozen snapshot for building an index. Suggestions are resolved
// once by the caller; Now is used only for snooze expiry, never conflict ordering.
type Input struct {
	Transactions      []domain.Transaction
	Resolutions       reviewqueue.Resolutions
	Suggestions       map[string]catsuggest.Suggestion
	Scope             reviewscope.Scope
	Ledger            txnfilter.Criteria
	FallbackEmptyView bool
	Now               time.Time
}

// BuildIndex selects, groups, ranks, and indexes the active review population.
func BuildIndex(in Input) Index {
	scope := reviewscope.Normalize(in.Scope)
	idx := Index{Byid: map[string]Row{}, Scope: scope}
	if in.Transactions == nil {
		return idx
	}
	res := in.Resolutions
	txns := in.Transactions
	queue := reviewqueue.QueueOpen(txns, res, in.Now)
	idx.QueueTotal = len(queue)
	inView := RestrictQueue(queue, LedgerVisibleIDs(txns, in.Ledger))
	idx.ViewTotal = len(inView)
	if in.FallbackEmptyView && scope == reviewscope.ScopeCurrentView && idx.ViewTotal == 0 && idx.QueueTotal > 0 {
		scope = reviewscope.ScopeEntireQueue
		idx.Scope = scope
	}
	if reviewscope.Normalize(scope) == reviewscope.ScopeCurrentView {
		queue = inView
	}
	idx.Total = len(queue)

	idx.TxnByID = make(map[string]domain.Transaction, len(txns))
	for _, t := range txns {
		idx.TxnByID[t.ID] = t
	}
	idx.Dupes = map[string][]string{}
	for _, g := range dedupe.FindDuplicates(txns) {
		if len(g.IDs) < 2 {
			continue
		}
		for _, id := range g.IDs {
			idx.Dupes[id] = g.IDs
		}
	}

	// Resolve every charge in its own rule context. Account, amount and date can
	// change a rule result even when two charges share a merchant.
	suggByID := make(map[string]catsuggest.Suggestion, len(queue))
	ranked := make([]reviewqueue.Ranked, 0, len(queue))
	for _, t := range queue {
		s, ok := in.Suggestions[t.ID]
		if ok {
			suggByID[t.ID] = s
		}
		conf := 0
		if ok {
			conf = int(s.Confidence)
		}
		ranked = append(ranked, reviewqueue.Ranked{Txn: t, Confidence: conf})
	}

	for _, g := range reviewqueue.GroupByMerchant(ranked) {
		s, ok, conflict := SharedSuggestion(g, suggByID)
		row := Row{Group: g, Tier: TierOf(s, ok), Suggestion: s, HasSugg: ok,
			SuggestionConflict: conflict}
		idx.Rows = append(idx.Rows, row)
		idx.Byid[g.Key] = row
	}
	return idx
}

// Candidates returns the active scoped queue before suggestions are resolved.
// The adapter uses this to avoid resolving already-reviewed or out-of-scope
// transactions. The same explicit timestamp is supplied again to BuildIndex.
func Candidates(in Input) []domain.Transaction {
	queue := reviewqueue.QueueOpen(in.Transactions, in.Resolutions, in.Now)
	if reviewscope.Normalize(in.Scope) != reviewscope.ScopeCurrentView {
		return queue
	}
	visible := RestrictQueue(queue, LedgerVisibleIDs(in.Transactions, in.Ledger))
	if in.FallbackEmptyView && len(visible) == 0 && len(queue) > 0 {
		return queue
	}
	return visible
}
