// SPDX-License-Identifier: MIT

package appstate

import (
	"github.com/monstercameron/CashFlux/internal/rules"
)

// CreditRuleHits records that a set of rules filed transactions, persisting each
// one's running total and the time it last fired (C372).
//
// The workbench already showed a LIVE match count, recomputed on every render.
// That answers "what would this rule catch in today's ledger" — a genuinely
// different question from "has this rule ever done anything". A rule whose
// merchant stopped appearing shows a live count of zero and reads as broken;
// with a durable tally it reads as a rule that worked forty times and has gone
// quiet, which is what the user needs in order to decide whether to keep it.
//
// It is credited only where a transaction is actually WRITTEN through a rule.
// AutoCategorizeTransaction is deliberately not a crediting seam even though it
// is where a rule fires: it is also used to PREVIEW a suggestion (the review
// surface calls it to see what a rule would do), and a preview that inflates the
// count would make the number mean nothing.
//
// A rule that has since been deleted is skipped rather than resurrected.
func (a *App) CreditRuleHits(perRule map[string]int) error {
	if len(perRule) == 0 {
		return nil
	}
	now := a.now()
	for _, r := range a.Rules() {
		n := perRule[r.ID]
		if n <= 0 {
			continue
		}
		if err := a.store.PutRule(r.RecordHits(n, now)); err != nil {
			return err
		}
	}
	return nil
}

// creditOneRule is CreditRuleHits for the single-rule backfill path.
func (a *App) creditOneRule(ruleID string, n int) error {
	if n <= 0 {
		return nil
	}
	return a.CreditRuleHits(map[string]int{ruleID: n})
}

// autoRuleFor reports which rule would file this transaction on entry, so an
// import batch can tally hits without re-deriving the match. It mirrors
// AutoCategorizeTransaction's primary (category) match exactly — the bill-link
// action is applied independently there and is not what "this rule filed it"
// means.
func (a *App) autoRuleFor(payee, desc string, amountMinor int64, accountID string, td rules.TxnDate) string {
	r := rules.FirstMatchFull(a.transactionAutoRules(), payee, desc, amountMinor, accountID, td)
	if r == nil {
		return ""
	}
	return r.ID
}
