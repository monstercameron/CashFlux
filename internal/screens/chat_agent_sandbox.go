// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsSandbox(app, base, rates)...) in buildChatTools

package screens

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
)

// agToolsSandbox adds the what-if scenario tool (AG2).
//
// "What if I moved to an £1,800 flat?" has no good answer from arithmetic alone:
// the interesting effects are second-order — the budget that stops fitting, the
// goal whose date slips, the runway that shortens — and they come out of the same
// engines that compute the real figures. So the tool copies the dataset, applies
// the change to the copy, and reports the DIFFERENCE.
//
// It is a read tool despite writing, because everything it writes is to a
// disposable copy that is discarded when the call returns. That is what makes it
// safe to run without an approval card: there is nothing to approve, because
// nothing happened. Acting on a scenario for real goes back through the ordinary
// changeset review, one approved step at a time.
func agToolsSandbox(app *appstate.App, base string, rates currency.Rates) []chatTool {
	_ = rates
	money0 := func(minor int64) string { return insightsMoneyFmt(minor, base) }

	return []chatTool{
		{
			spec: ai.FunctionTool("run_what_if",
				"Try a hypothetical change against a COPY of the user's data and report what it moves — "+
					"net worth, runway, budgets, goal dates. Use for questions like \"what if rent went up "+
					"to 1800?\" or \"what if I saved 200 more a month?\". Nothing is saved: the copy is "+
					"thrown away when this returns. Report the movements, then ask whether they want to "+
					"make any of it real — do not apply anything yourself.",
				json.RawMessage(`{"type":"object","properties":{`+
					`"label":{"type":"string","description":"what the scenario is, in the user's words"},`+
					`"monthly_change":{"type":"number","description":"a recurring monthly amount to add; NEGATIVE for a new or increased cost, positive for extra income or saving"},`+
					`"months":{"type":"integer","description":"how many months to run it for; defaults to 12"}`+
					`},"required":["label","monthly_change"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Label         string  `json:"label"`
					MonthlyChange float64 `json:"monthly_change"`
					Months        int     `json:"months"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Couldn't read the scenario."
				}
				label := strings.TrimSpace(a.Label)
				if label == "" {
					label = "this scenario"
				}
				if a.MonthlyChange == 0 {
					return "That scenario doesn't change anything month to month, so there's nothing to compare."
				}
				months := a.Months
				if months <= 0 {
					months = 12
				}
				if months > 60 {
					// Beyond five years the projection is dominated by assumptions
					// nobody stated, and a long horizon makes a guess look precise.
					months = 60
				}

				sandbox, err := app.NewSandbox()
				if err != nil {
					return "Couldn't set up a scenario to try that in."
				}
				acct, ok := scenarioAccount(app)
				if !ok {
					return "There's no account to run the scenario against."
				}
				// math.Round, not int64(x*100+0.5): the conversion truncates TOWARD
				// ZERO, so the +0.5 trick rounds the wrong way for every negative
				// value — and negative is the documented common case here (a new or
				// increased cost). -58.333 became -58.32, under-stating the scenario
				// by a cent per month for up to sixty months.
				minor := int64(math.Round(a.MonthlyChange * 100))
				start := time.Now()
				for i := 0; i < months; i++ {
					if err := sandbox.App.PutTransaction(domain.Transaction{
						ID:        id.New(),
						AccountID: acct,
						Payee:     label,
						Desc:      label,
						Amount:    money.New(minor, base),
						Date:      start.AddDate(0, i, 0),
					}); err != nil {
						return "Couldn't apply the scenario: " + err.Error()
					}
				}

				changes := sandbox.Diff(1)
				if len(changes) == 0 {
					return fmt.Sprintf("Over %d months, %s doesn't move any of the figures we track by a meaningful amount.", months, label)
				}
				var b strings.Builder
				fmt.Fprintf(&b, "Scenario: %s, %s a month for %d months.\nNothing was saved — this ran against a copy.\n\nWhat it moves:\n",
					label, money0(minor), months)
				for i, c := range changes {
					if i >= 8 {
						fmt.Fprintf(&b, "(and %d smaller movements)\n", len(changes)-i)
						break
					}
					fmt.Fprintf(&b, "- %s: %s → %s\n", c.Name, formatVar(c.Before), formatVar(c.After))
				}
				b.WriteString("\nReport the movements that matter, in plain English. " +
					"Then ask whether they want to make any of it real — do not apply it yourself.")
				return b.String()
			},
		},
	}
}

// scenarioAccount picks the account a hypothetical cost lands in: the first
// unarchived asset account, which is where a household's day-to-day money is. The
// choice barely matters for the diff — the engine aggregates — but it has to be a
// real account or the write is rejected.
func scenarioAccount(app *appstate.App) (string, bool) {
	for _, ac := range app.Accounts() {
		if !ac.Archived && ac.Class == domain.ClassAsset {
			return ac.ID, true
		}
	}
	for _, ac := range app.Accounts() {
		if !ac.Archived {
			return ac.ID, true
		}
	}
	return "", false
}

// formatVar renders an engine variable for the diff listing. Engine variables are
// a mix of money in minor units, counts and ratios, and the diff has no type
// information — so this prints the number plainly rather than dressing a count up
// as a currency, which would be a confident lie about a third of the rows.
func formatVar(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.2f", v)
}
