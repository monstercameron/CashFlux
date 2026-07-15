// SPDX-License-Identifier: MIT

package smartengine

import (
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-A9", a9FeeBleed)
}

// Fee-bleed tunables. A "fee-like" debit is small and recurring; an account is
// "otherwise dormant" when its only activity over the lookback is such fees — no
// deposits, no meaningful spending, no transfers.
const (
	feeBleedMonths   = 6     // lookback window for judging dormancy
	feeBleedMaxMinor = 30_00 // a debit at or below this magnitude looks like a fee ($30)
	feeBleedMinHits  = 2     // at least this many fee-like debits to call it recurring
)

// SMART-A9 — Fee-bleed on a dormant account (AC14). The sharp companion to the
// dormant-account nudge (SMART-A2): an account with NO real activity for months
// that is still being drained by a small recurring fee — a maintenance charge you
// forgot about. It offers a one-tap close-it task so the account can be shut and
// the fee stopped. The dismissal key encodes the account, so closing one dormant
// account never suppresses the flag on another.
//
// One-tap paths: the create-task action below opens a close-it task via the
// existing ActionCreateTask wiring. TODO(AC14-wire): a direct "archive account"
// one-tap and attaching a domain.TaskResolve rule so XC8 auto-closes the task
// when the balance zeroes both require changes in internal/screens/smart_adapter.go
// (a forbidden file this agent must not edit) — a new smart.ActionKind and the
// resolve-rule payload need adding there. Reported as deferred.
func a9FeeBleed(in Input) []smart.Insight {
	cutoff := dateutil.AddMonths(in.Now, -feeBleedMonths)
	var out []smart.Insight
	for _, a := range activeAssetAccounts(in.Accounts) {
		txns := txnsForAccount(in.Transactions, a.ID)
		if len(txns) == 0 {
			continue
		}
		var feeCount int
		var feeTotalMinor int64
		otherwiseDormant := true
		for _, t := range txns {
			if t.Date.Before(cutoff) {
				continue // outside the window — ignore old real activity
			}
			// Any credit, transfer, or non-trivial debit in the window means the
			// account is NOT dormant: this is the plain-fee case only.
			if t.IsTransfer() || !t.Amount.IsNegative() {
				otherwiseDormant = false
				break
			}
			mag := -t.Amount.Amount
			if mag > feeBleedMaxMinor {
				otherwiseDormant = false
				break
			}
			feeCount++
			feeTotalMinor += mag
		}
		if !otherwiseDormant || feeCount < feeBleedMinHits {
			continue
		}
		// Annualize the observed fee drain over the window months.
		annualMinor := feeTotalMinor * 12 / feeBleedMonths
		ins := smart.Insight{
			Feature: "SMART-A9",
			Page:    smart.PageAccounts,
			Key:     "SMART-A9:" + a.ID,
			Title:   a.Name + " is paying a fee for nothing",
			Detail: "The only activity on " + a.Name + " in the last " + plural(feeBleedMonths, "month") +
				" is " + plural(int64(feeCount), "fee-like charge") + " totaling " + in.hmoney(feeTotalMinor) +
				" — about " + in.hmoney(annualMinor) + "/yr on a dormant account. Consider closing it to stop the bleed.",
			Severity: smart.SeverityWarn,
		}.WithAmount(in.baseMoney(annualMinor)).
			WithAction(smart.Action{
				Kind:        smart.ActionCreateTask,
				Label:       "Add a task to close it",
				TaskTitle:   "Close " + a.Name + " and stop the monthly fee",
				TaskNotes:   "This account has had no real activity in " + plural(feeBleedMonths, "month") + " but is still charged a recurring fee. Close it once the balance is zeroed.",
				RelatedType: "account",
				RelatedID:   a.ID,
			})
		out = append(out, ins)
	}
	return out
}
