// SPDX-License-Identifier: MIT

package smartengine

import (
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-B13", b13TrueUp)
}

// b13TrueUp — Seasonal auto-budget true-up (BG6). Surfaces budgeting.SuggestTrueUps as
// opt-in, dismissable suggestions: a budget whose real spend has run persistently above its
// limit is proposed a new limit, learned seasonally (same-month-last-year) when a year of
// history exists, else from the trailing average.
//
// The dismissal Key encodes the SUGGESTED LEVEL, so acknowledging "raise Groceries to $480"
// silences only that level — if spend drifts further and the suggestion climbs to $520, the
// Key changes and the insight re-appears (the smart_adapter dismissal-key lesson).
//
// TODO(BG6-wire): the one-tap accept that writes the new Limit onto the budget needs an
// executable action wired in the budgets page / smart adapter (a "set budget limit N" action
// kind). Until then the action navigates to /budgets with the suggested amount in the copy so
// the raise is one edit away; the detection, level-encoded dismissal, and preview figure are
// all live.
func b13TrueUp(in Input) []smart.Insight {
	ups, err := budgeting.SuggestTrueUps(in.Budgets, in.Transactions, in.Categories, in.Now, in.Rates)
	if err != nil {
		return nil
	}
	var out []smart.Insight
	for _, u := range ups {
		name := budgetName(u.Budget)
		cur := u.Budget.Limit.Currency
		if cur == "" {
			cur = in.Base
		}
		basis := "your recent average"
		if u.Seasonal {
			basis = "this month in prior years"
		}
		detail := name + " has run about " + hmoneyc(u.LearnedMinor, cur) + "/mo over " +
			plural(int64(u.BasisMonths), "month") + " against a " + hmoneyc(u.CurrentLimitMinor, cur) +
			" budget. Based on " + basis + ", raising it to " + hmoneyc(u.SuggestedMinor, cur) +
			" would match reality. Accept, or leave it and keep the tighter target."
		out = append(out, smart.Insight{
			Feature: "SMART-B13",
			Page:    smart.PageBudgets,
			// Level-encoded key: a further drift (new SuggestedMinor) re-flags.
			Key:      "SMART-B13:" + u.Budget.ID + ":" + itoa64(u.SuggestedMinor),
			Title:    name + " is running above its budget — raise it to " + hmoneyc(u.SuggestedMinor, cur) + "?",
			Detail:   detail,
			Severity: smart.SeverityNudge,
		}.WithAmount(mny(u.SuggestedMinor-u.CurrentLimitMinor, cur)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: "Review the budget",
				Route: "/budgets", RelatedType: "budget", RelatedID: u.Budget.ID}))
	}
	return out
}
