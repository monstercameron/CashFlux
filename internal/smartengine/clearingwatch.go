// SPDX-License-Identifier: MIT

package smartengine

import (
	"github.com/monstercameron/CashFlux/internal/clearwatch"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-T23", t23StaleUncleared)
}

// clearingLookbackDays bounds the history a window is learned from.
//
// A year. An account's clearing behaviour changes when the bank changes its
// systems or the household changes card, and a median stretching back five years
// would defend the old behaviour against the new one long after it stopped being
// true.
const clearingLookbackDays = 365

// t23StaleUncleared — EC-4. Flags charges still uncleared well past the point
// this account's own charges normally clear.
//
// It feeds reconciliation: a charge that never posted is either a hold that fell
// off, a duplicate entry, or a payment that silently failed, and all three are
// worth knowing before the statement arrives.
func t23StaleUncleared(in Input) []smart.Insight {
	cut := in.Now.AddDate(0, 0, -clearingLookbackDays)
	observed := map[string][]int{}
	pending := map[string][]domain.Transaction{}
	for _, t := range in.Transactions {
		if t.Date.Before(cut) || t.Date.After(in.Now) {
			continue
		}
		if d, ok := t.DaysToClear(); ok {
			observed[t.AccountID] = append(observed[t.AccountID], d)
			continue
		}
		if !t.Cleared {
			pending[t.AccountID] = append(pending[t.AccountID], t)
		}
	}

	// ONE finding, not one per account. The transactions strip shows three items,
	// and eight clearing warnings would push everything else off the page — at
	// which point the reader stops reading the strip rather than clearing the
	// charges. The worst is named and the rest are counted, the same way a diffuse
	// overspend is reported.
	var worst domain.Transaction
	var worstAccount domain.Account
	worstOver := 0
	worstWindow := clearwatch.Window{}
	others := 0
	for _, a := range in.Accounts {
		if a.Archived {
			continue
		}
		w := clearwatch.Learn(observed[a.ID])
		if !w.Known {
			continue // no normal for this account, so nothing can be abnormal
		}
		// The oldest stuck charge on this account, measured against ITS window.
		var acctWorst domain.Transaction
		acctOver := 0
		for _, t := range pending[a.ID] {
			age := dateutil.DaysBetween(t.Date, in.Now)
			over, late := w.OverdueBy(age)
			if late && over > acctOver {
				acctWorst, acctOver = t, over
			}
		}
		if acctOver == 0 {
			continue
		}
		// "Worst" is how far past ITS OWN account's normal a charge has gone, not
		// how old it is: five days pending on a same-day account is a stronger
		// signal than ten on one that takes a week, and ranking by age alone would
		// always name the slowest account.
		if acctOver > worstOver {
			if worstOver > 0 {
				others++
			}
			worst, worstAccount, worstOver, worstWindow = acctWorst, a, acctOver, w
			continue
		}
		others++
	}
	if worstOver == 0 {
		return nil
	}

	age := dateutil.DaysBetween(worst.Date, in.Now)
	ins := smart.Insight{
		Feature:  "SMART-T23",
		Page:     smart.PageTransactions,
		Key:      "SMART-T23:" + worst.ID,
		Severity: smart.SeverityNudge,
	}.
		WithTitle("smart.t23.title", "A charge on %s still hasn't cleared", worstAccount.Name).
		WithDetail("smart.t23.detail",
			"%s from %s is %s old and still pending. Charges on %s usually clear in %s, going on the last %s.",
			firstNonEmptySample(worst.Payee, worst.Desc), worst.Date.Format("Jan 2"),
			plural(int64(age), "day"), worstAccount.Name,
			plural(int64(worstWindow.TypicalDays), "day"), plural(int64(worstWindow.Samples), "charge")).
		WithAmount(worst.Amount.Abs()).
		WithAction(smart.Action{Kind: smart.ActionNavigate, Route: "/transactions",
			RelatedType: "transaction", RelatedID: worst.ID}.
			WithLabel("smart.t23.action", "Open transactions"))
	if others > 0 {
		// Counted, not listed: the reader needs to know this is not the only one,
		// and does not need eight rows to learn it.
		ins = ins.WithDetail("smart.t23.detailMore",
			"%s from %s is %s old and still pending. Charges on %s usually clear in %s, going on the last %s. %s also have one.",
			firstNonEmptySample(worst.Payee, worst.Desc), worst.Date.Format("Jan 2"),
			plural(int64(age), "day"), worstAccount.Name,
			plural(int64(worstWindow.TypicalDays), "day"), plural(int64(worstWindow.Samples), "charge"),
			plural(int64(others), "other account"))
	}
	return []smart.Insight{ins}
}

// firstNonEmptySample is the merchant to name in the sentence: the payee when
// there is one, the description otherwise. A row with neither gets a stand-in
// rather than an empty gap in the middle of a sentence.
func firstNonEmptySample(payee, desc string) string {
	if payee != "" {
		return payee
	}
	if desc != "" {
		return desc
	}
	return "That charge"
}
