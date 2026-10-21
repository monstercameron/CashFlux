// SPDX-License-Identifier: MIT

package attribution

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// computeLegacy freezes the pre-index Compute implementation from HEAD for
// parity tests and paired benchmarks. Keep it unchanged as the comparison path.
func computeLegacy(in Input) (Report, error) {
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
		if !a.CountsInNetWorth() {
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
		// Display label: payee when present, else the description — manual
		// entries often carry only Desc ("Car payment"), and a large-expense
		// finding with a blank subject reads broken.
		label := t.Payee
		if label == "" {
			label = t.Desc
		}
		ev := evTxn{id: t.ID, amountMinor: conv.Amount, payee: label, accountID: t.AccountID}
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
	// One issue = one finding: when two findings cite exactly the same evidence
	// set they are the same story wearing different labels (e.g. one account's
	// only activity IS the top category's activity) — keep the higher-ranked
	// one. Evidence-free findings (the headline) always stay.
	seen := map[string]bool{}
	deduped := items[:0]
	for _, it := range items {
		sig := ""
		if len(it.TxnIDs) > 0 {
			ids := append([]string(nil), it.TxnIDs...)
			sort.Strings(ids)
			sig = strings.Join(ids, "|")
		}
		if sig != "" && seen[sig] {
			continue
		}
		if sig != "" {
			seen[sig] = true
		}
		deduped = append(deduped, it)
	}
	items = deduped
	if len(items) > topN {
		items = items[:topN]
	}
	rep.Items = items
	return rep, nil
}
