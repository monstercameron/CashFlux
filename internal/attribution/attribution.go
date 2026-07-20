// SPDX-License-Identifier: MIT

// Package attribution is the E1 "what changed, and why" engine: it decomposes
// the movement of a window of activity — typically "since your last visit" —
// into a small set of ranked, evidence-carrying findings, so a surface (the
// dashboard card first) can answer "what changed / why / how much" in seconds
// instead of making the user diff accounts and transactions by hand.
//
// The decomposition reuses the app's canonical conventions so its numbers agree
// with every other surface: windows are half-open [Since, Until) and a balance
// "as of" a cutoff counts transactions strictly before it (ledger.NetWorthSeries);
// liabilities contribute the magnitude of their balance (ledger.NetWorth);
// income/spending follow PeriodTotals semantics (non-transfer, CountsInReports).
// Balance-update adjustment rows are identified by the caller-supplied
// IsAdjustment predicate (the same seam ledger.BalanceProvenance uses), so the
// "cash flow vs balance adjustments" split matches what the app itself posted.
//
// The headline decomposition always sums exactly: the residual between the true
// net-worth delta and the flow + adjustments parts is reported as PartOther
// (report-excluded transactions, cross-currency transfer asymmetry, and the
// liability sign edge cases live there), never silently dropped — determinism
// and explainability over false precision.
//
// Pure Go, no syscall/js; unit-tested on native Go. All output amounts are
// signed base-currency minor units (negative = money out / net worth down).
package attribution

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Kind identifies what a finding is about, so the view can pick an icon and
// phrase the sentence. Values are stable (persisted in dismiss keys).
type Kind string

const (
	// KindNetWorth is the pinned headline: the window's net-worth movement,
	// decomposed into flow / adjustments / other parts.
	KindNetWorth Kind = "networth"
	// KindAccount is the single account whose balance moved the most, with its
	// own flow-vs-adjustment split.
	KindAccount Kind = "account"
	// KindCategory is the category that concentrated the most spending.
	KindCategory Kind = "category"
	// KindIncome is income that landed in the window.
	KindIncome Kind = "income"
	// KindLargeTxn is one unusually large single expense (≥ LargeTxnShare of
	// the window's spending).
	KindLargeTxn Kind = "largetxn"
	// KindNewPayee is a merchant seen for the first time ever in this window.
	KindNewPayee Kind = "newpayee"
)

// PartKind labels one component of a finding's "why" decomposition.
type PartKind string

const (
	// PartFlow is ordinary cash flow: income minus spending, excluding
	// balance-adjustment rows.
	PartFlow PartKind = "flow"
	// PartAdjustments is the net effect of update-balance adjustment rows
	// (manual reconciles, investment revaluations entered as balance sets).
	PartAdjustments PartKind = "adjustments"
	// PartOther is the exact residual to the true delta: report-excluded
	// transactions, FX transfer asymmetry, liability sign edge cases.
	PartOther PartKind = "other"
	// PartIncome / PartSpending split PartFlow when a view wants both sides.
	PartIncome   PartKind = "income"
	PartSpending PartKind = "spending"
)

// Part is one signed component of a finding's decomposition, in base minor units.
type Part struct {
	Kind        PartKind
	AmountMinor int64
}

// Item is one ranked finding. Identifier fields are structured (IDs + names),
// not display strings — the UI phrases and links them. TxnIDs is the evidence:
// the specific transactions behind the number, largest-impact first.
type Item struct {
	Kind        Kind
	AccountID   string
	AccountName string
	CategoryID  string // KindCategory: may be "" (uncategorized spend)
	Payee       string // KindNewPayee / KindLargeTxn
	AmountMinor int64  // signed headline impact in base minor units
	Parts       []Part // KindNetWorth / KindAccount decomposition
	Count       int    // contributing transactions (KindNewPayee: new payees seen)
	TxnIDs      []string
}

// Report is the full attribution of one window.
type Report struct {
	Since, Until time.Time
	Base         string

	NetDeltaMinor    int64 // true net-worth movement over the window
	IncomeMinor      int64 // ≥0, PeriodTotals semantics, excluding adjustments
	SpendingMinor    int64 // ≥0, PeriodTotals semantics, excluding adjustments
	AdjustmentsMinor int64 // signed net-worth effect of adjustment rows
	OtherMinor       int64 // signed exact residual (see package doc)
	TxnCount         int   // transactions dated inside the window

	Items []Item // ranked, KindNetWorth pinned first, capped at TopN
}

// Input configures one attribution pass.
type Input struct {
	Accounts []domain.Account
	Txns     []domain.Transaction
	Rates    currency.Rates
	// Since / Until bound the half-open window [Since, Until).
	Since, Until time.Time
	// IsAdjustment identifies update-balance adjustment rows (nil = none).
	IsAdjustment func(domain.Transaction) bool
	// TopN caps Items (0 = DefaultTopN).
	TopN int
}

// DefaultTopN is the finding cap when Input.TopN is zero — the "top 3 things
// that changed" the 20-second dashboard read is built around.
const DefaultTopN = 3

// LargeTxnShare is the minimum share of the window's spending a single expense
// must reach to earn its own KindLargeTxn finding (avoids echoing every routine
// purchase back as "news").
const LargeTxnShare = 0.30

// evidenceCap bounds TxnIDs per finding.
const evidenceCap = 3

// Compute runs the attribution. A window with no activity and no movement
// yields a Report with zero Items.
func Compute(in Input) (Report, error) {
	base := in.Rates.Base
	rep := Report{Since: in.Since, Until: in.Until, Base: base}
	topN := in.TopN
	if topN <= 0 {
		topN = DefaultTopN
	}
	isAdj := in.IsAdjustment
	if isAdj == nil {
		isAdj = func(domain.Transaction) bool { return false }
	}

	// True net-worth delta: per-account balances strictly before each cutoff,
	// mapped through the canonical liability-magnitude convention. Account
	// contributions are collected on the way so KindAccount is exact.
	type acctMove struct {
		acct         domain.Account
		contribMinor int64 // net-worth terms, base minor
		flowMinor    int64 // non-adjustment txn sum, net-worth terms
		adjMinor     int64 // adjustment txn sum, net-worth terms
		txnIDs       []string
		txnAbs       []int64
		count        int
	}
	moves := make([]acctMove, 0, len(in.Accounts))
	for _, a := range in.Accounts {
		if a.Archived {
			continue
		}
		balSince, balUntil := a.OpeningBalance.Amount, a.OpeningBalance.Amount
		var flowAcct, adjAcct int64 // account-currency window sums
		m := acctMove{acct: a}
		for _, t := range in.Txns {
			if t.AccountID != a.ID {
				continue
			}
			if t.Date.Before(in.Since) {
				balSince += t.Amount.Amount
			}
			if t.Date.Before(in.Until) {
				balUntil += t.Amount.Amount
			}
			if dateutil.InRange(t.Date, in.Since, in.Until) {
				if isAdj(t) {
					adjAcct += t.Amount.Amount
				} else {
					flowAcct += t.Amount.Amount
				}
				m.count++
				m.txnIDs = append(m.txnIDs, t.ID)
				m.txnAbs = append(m.txnAbs, abs64(t.Amount.Amount))
			}
		}
		// Net-worth contribution factor for this account over the window: assets
		// contribute their delta; liabilities contribute minus the change in the
		// magnitude of their balance (ledger.NetWorth's convention). For a
		// liability whose balance keeps one sign across the window that is a
		// simple ±1 factor; a sign-crossing liability is handled exactly for the
		// total (magnitudes at each cutoff) with the flow/adj split left linear —
		// the headline residual (PartOther) absorbs any difference.
		factor := int64(1)
		contribAcct := balUntil - balSince
		if a.Class == domain.ClassLiability {
			contribAcct = -(abs64(balUntil) - abs64(balSince))
			if balSince < 0 || (balSince == 0 && balUntil < 0) {
				factor = 1 // negative-stored debt: |b| = -b, so -Δ|b| = Δb
			} else {
				factor = -1 // positive-stored "amount you owe"
			}
		}
		conv := func(minor int64) (int64, error) {
			c, err := in.Rates.Convert(money.New(minor, a.Currency), base)
			if err != nil {
				return 0, fmt.Errorf("attribution: account %s: %w", a.ID, err)
			}
			return c.Amount, nil
		}
		var err error
		if m.contribMinor, err = conv(contribAcct); err != nil {
			return Report{}, err
		}
		if m.flowMinor, err = conv(factor * flowAcct); err != nil {
			return Report{}, err
		}
		if m.adjMinor, err = conv(factor * adjAcct); err != nil {
			return Report{}, err
		}
		rep.NetDeltaMinor += m.contribMinor
		rep.AdjustmentsMinor += m.adjMinor
		if m.count > 0 || m.contribMinor != 0 {
			moves = append(moves, m)
		}
	}

	// Window scan: income/spending (PeriodTotals semantics, adjustments carved
	// out), per-category spend, largest expense, income evidence, new payees.
	catSpend := map[string]int64{}
	catTxns := map[string][]evTxn{}
	var incomeTxns, spendTxns []evTxn
	var largest evTxn
	firstSeen := map[string]time.Time{} // lowercase payee → first date ever
	type newPayee struct {
		payee string
		ev    evTxn
	}
	newPayees := []newPayee{}
	for _, t := range in.Txns {
		if p := strings.ToLower(strings.TrimSpace(t.Payee)); p != "" && !t.IsTransfer() && !isAdj(t) {
			if cur, ok := firstSeen[p]; !ok || t.Date.Before(cur) {
				firstSeen[p] = t.Date
			}
		}
		if !dateutil.InRange(t.Date, in.Since, in.Until) {
			continue
		}
		rep.TxnCount++
		if t.IsTransfer() || !t.CountsInReports() || isAdj(t) {
			continue
		}
		conv, err := in.Rates.Convert(t.Amount, base)
		if err != nil {
			return Report{}, fmt.Errorf("attribution: txn %s: %w", t.ID, err)
		}
		ev := evTxn{id: t.ID, amountMinor: conv.Amount, payee: t.Payee, accountID: t.AccountID}
		switch {
		case t.IsIncome():
			rep.IncomeMinor += conv.Amount
			incomeTxns = append(incomeTxns, ev)
		case t.IsExpense():
			mag := abs64(conv.Amount)
			rep.SpendingMinor += mag
			spendTxns = append(spendTxns, ev)
			catSpend[t.CategoryID] += mag
			catTxns[t.CategoryID] = append(catTxns[t.CategoryID], ev)
			if mag > abs64(largest.amountMinor) {
				largest = ev
			}
		}
	}
	// New payees are an expense-side awareness signal (first time you've ever
	// PAID this merchant — SMART-T19's fraud/novelty rationale); a new income
	// source is already the income finding's story.
	for _, t := range in.Txns {
		if !dateutil.InRange(t.Date, in.Since, in.Until) || t.IsTransfer() || isAdj(t) || !t.IsExpense() {
			continue
		}
		p := strings.ToLower(strings.TrimSpace(t.Payee))
		if p == "" || firstSeen[p].Before(in.Since) {
			continue
		}
		conv, err := in.Rates.Convert(t.Amount, base)
		if err != nil {
			return Report{}, fmt.Errorf("attribution: txn %s: %w", t.ID, err)
		}
		newPayees = append(newPayees, newPayee{payee: t.Payee, ev: evTxn{id: t.ID, amountMinor: conv.Amount}})
	}

	flow := rep.IncomeMinor - rep.SpendingMinor
	rep.OtherMinor = rep.NetDeltaMinor - flow - rep.AdjustmentsMinor

	// ── Findings ──────────────────────────────────────────────────────────────
	var items []Item

	if rep.NetDeltaMinor != 0 || rep.TxnCount > 0 {
		parts := []Part{{Kind: PartFlow, AmountMinor: flow}}
		if rep.AdjustmentsMinor != 0 {
			parts = append(parts, Part{Kind: PartAdjustments, AmountMinor: rep.AdjustmentsMinor})
		}
		if rep.OtherMinor != 0 {
			parts = append(parts, Part{Kind: PartOther, AmountMinor: rep.OtherMinor})
		}
		items = append(items, Item{
			Kind: KindNetWorth, AmountMinor: rep.NetDeltaMinor, Parts: parts, Count: rep.TxnCount,
		})
	}

	if len(moves) > 0 {
		sort.SliceStable(moves, func(i, j int) bool {
			if d := abs64(moves[i].contribMinor) - abs64(moves[j].contribMinor); d != 0 {
				return d > 0
			}
			return moves[i].acct.ID < moves[j].acct.ID
		})
		top := moves[0]
		if top.contribMinor != 0 {
			parts := []Part{}
			if top.flowMinor != 0 {
				parts = append(parts, Part{Kind: PartFlow, AmountMinor: top.flowMinor})
			}
			if top.adjMinor != 0 {
				parts = append(parts, Part{Kind: PartAdjustments, AmountMinor: top.adjMinor})
			}
			items = append(items, Item{
				Kind: KindAccount, AccountID: top.acct.ID, AccountName: top.acct.Name,
				AmountMinor: top.contribMinor, Parts: parts, Count: top.count,
				TxnIDs: topEvidenceIDs(top.txnIDs, top.txnAbs),
			})
		}
	}

	if topCat, spent := maxCategory(catSpend); spent > 0 {
		items = append(items, Item{
			Kind: KindCategory, CategoryID: topCat, AmountMinor: -spent,
			Count: len(catTxns[topCat]), TxnIDs: topEvidence(catTxns[topCat]),
		})
	}

	if rep.IncomeMinor > 0 {
		items = append(items, Item{
			Kind: KindIncome, AmountMinor: rep.IncomeMinor,
			Count: len(incomeTxns), TxnIDs: topEvidence(incomeTxns),
		})
	}

	if largest.id != "" && rep.SpendingMinor > 0 &&
		float64(abs64(largest.amountMinor)) >= LargeTxnShare*float64(rep.SpendingMinor) {
		items = append(items, Item{
			Kind: KindLargeTxn, Payee: largest.payee, AccountID: largest.accountID,
			AmountMinor: largest.amountMinor, Count: 1, TxnIDs: []string{largest.id},
		})
	}

	if len(newPayees) > 0 {
		sort.SliceStable(newPayees, func(i, j int) bool {
			if d := abs64(newPayees[i].ev.amountMinor) - abs64(newPayees[j].ev.amountMinor); d != 0 {
				return d > 0
			}
			return newPayees[i].ev.id < newPayees[j].ev.id
		})
		items = append(items, Item{
			Kind: KindNewPayee, Payee: newPayees[0].payee,
			AmountMinor: newPayees[0].ev.amountMinor,
			Count:       len(newPayees), TxnIDs: []string{newPayees[0].ev.id},
		})
	}

	// Rank: the net-worth headline stays pinned first; everything else by
	// absolute impact, tie-broken by Kind then first evidence id (total order,
	// deterministic render).
	if len(items) > 1 {
		rest := items[1:]
		if items[0].Kind != KindNetWorth {
			rest = items
		}
		sort.SliceStable(rest, func(i, j int) bool {
			if d := abs64(rest[i].AmountMinor) - abs64(rest[j].AmountMinor); d != 0 {
				return d > 0
			}
			if rest[i].Kind != rest[j].Kind {
				return rest[i].Kind < rest[j].Kind
			}
			return firstID(rest[i]) < firstID(rest[j])
		})
	}
	if len(items) > topN {
		items = items[:topN]
	}
	rep.Items = items
	return rep, nil
}

// evTxn is one evidence transaction candidate.
type evTxn struct {
	id          string
	amountMinor int64
	payee       string
	accountID   string
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func firstID(it Item) string {
	if len(it.TxnIDs) > 0 {
		return it.TxnIDs[0]
	}
	return it.AccountID + it.CategoryID + it.Payee
}

// topEvidence returns up to evidenceCap txn ids, largest magnitude first.
func topEvidence(evs []evTxn) []string {
	sorted := append([]evTxn(nil), evs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if d := abs64(sorted[i].amountMinor) - abs64(sorted[j].amountMinor); d != 0 {
			return d > 0
		}
		return sorted[i].id < sorted[j].id
	})
	if len(sorted) > evidenceCap {
		sorted = sorted[:evidenceCap]
	}
	out := make([]string, len(sorted))
	for i, e := range sorted {
		out[i] = e.id
	}
	return out
}

// topEvidenceIDs is topEvidence over parallel id/magnitude slices.
func topEvidenceIDs(ids []string, mags []int64) []string {
	evs := make([]evTxn, len(ids))
	for i := range ids {
		evs[i] = evTxn{id: ids[i], amountMinor: mags[i]}
	}
	return topEvidence(evs)
}

// maxCategory returns the category with the largest spend (ties broken by the
// lexically-smallest id so the result is deterministic).
func maxCategory(spend map[string]int64) (string, int64) {
	bestID, best := "", int64(0)
	ids := make([]string, 0, len(spend))
	for id := range spend {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if spend[id] > best {
			bestID, best = id, spend[id]
		}
	}
	return bestID, best
}
