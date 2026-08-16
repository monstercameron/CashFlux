// SPDX-License-Identifier: MIT

//go:build js && wasm

// COORDINATOR: register via append(tools, agToolsCoach(app, base, rates)...) in buildChatTools

package screens

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalcoach"
	"github.com/monstercameron/CashFlux/internal/negotiationprep"
)

// agToolsCoach adds the two coaching tools: goal check-ins (AG15) and
// subscription negotiation prep (AG16).
//
// Both are READ tools that produce a PROPOSAL. Neither changes anything, and that
// is the design rather than an omission — advice the user did not ask for should
// not also be advice they have to undo. The one thing either can file is a to-do,
// and that goes through the ordinary add_task tool with its own approval card.
func agToolsCoach(app *appstate.App, base string, rates currency.Rates) []chatTool {
	_ = rates
	money := func(minor int64) string { return insightsMoneyFmt(minor, base) }

	return []chatTool{
		{
			spec: ai.FunctionTool("check_goal_pace",
				"Review the user's savings goals and report which are behind, unfunded, or on track, "+
					"with the monthly contribution that would make each deadline. Use when the user asks "+
					"how their goals are doing, or whether they will hit a target. It deliberately says "+
					"nothing about paused goals, goals with no deadline, and goals reviewed in the last "+
					"two weeks — those are silences by design, not missing data.",
				json.RawMessage(`{"type":"object","properties":{"goal":{"type":"string","description":"optional: part of one goal's name, to review just that one"}}}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Goal string `json:"goal"`
				}
				_ = json.Unmarshal(raw, &a)
				want := strings.ToLower(strings.TrimSpace(a.Goal))

				coached := make([]goalcoach.Goal, 0, len(app.Goals()))
				for _, g := range app.Goals() {
					if want != "" && !strings.Contains(strings.ToLower(g.Name), want) {
						continue
					}
					coached = append(coached, coachGoal(g))
				}
				if len(coached) == 0 {
					return "No matching goals."
				}
				notes := goalcoach.CoachAll(coached, time.Now())
				if len(notes) == 0 {
					return "Nothing worth raising: every goal is on track, paused, without a deadline, or was reviewed recently."
				}
				var b strings.Builder
				for _, n := range notes {
					b.WriteString(goalNoteLine(n, money))
					b.WriteString("\n")
				}
				b.WriteString("\nThese are observations, not changes. Offer the adjustment; do not apply it.")
				return b.String()
			},
		},
		{
			spec: ai.FunctionTool("prepare_subscription_call",
				"Assemble what the user actually has to bargain with before they call a provider about a "+
					"recurring cost: how long they have paid, how much the price rose and from what, and "+
					"what it costs a year, plus a short script. Use when the user wants to cut or "+
					"renegotiate a subscription. It states plainly what the data does NOT support; never "+
					"add a competitor price of your own, because one the provider can disprove ends the call.",
				json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"part of the subscription or merchant name"},"competitor_note":{"type":"string","description":"optional: a comparison you looked up with web_search, quoted exactly"}},"required":["name"]}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Name           string `json:"name"`
					CompetitorNote string `json:"competitor_note"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Couldn't read the details."
				}
				want := strings.ToLower(strings.TrimSpace(a.Name))
				if want == "" {
					return "Which subscription?"
				}
				sub, ok := subscriptionFor(app, want, a.CompetitorNote)
				if !ok {
					return "Nothing matching that name in their recurring costs or transactions."
				}
				prep := negotiationprep.Build(sub, time.Now(), money)
				if !prep.Worthwhile() {
					return "There is not much to bargain with on " + prep.Name +
						": no meaningful tenure, no recorded price rise, and no comparison. Say so plainly " +
						"rather than sending them into a call they cannot win.\n\n" + prep.TaskNotes()
				}
				return prep.TaskNotes() +
					"\n\nOffer to file this as a to-do with add_task so the script is there when they call."
			},
		},
	}
}

// coachGoal adapts a stored goal to the coach's input.
func coachGoal(g domain.Goal) goalcoach.Goal {
	return goalcoach.Goal{
		ID:            g.ID,
		Name:          g.Name,
		TargetMinor:   g.TargetAmount.Amount,
		SavedMinor:    g.CurrentAmount.Amount,
		MonthlyMinor:  g.MonthlyContribution.Amount,
		TargetDate:    g.TargetDate,
		PausedUntil:   g.PausedUntil,
		Archived:      g.Archived,
		LastCheckedAt: g.LastReviewedAt,
	}
}

// goalNoteLine renders one coaching note as a line the model can use as it stands.
func goalNoteLine(n goalcoach.Note, money func(int64) string) string {
	switch n.Kind {
	case goalcoach.KindBehind:
		return fmt.Sprintf("- %s: behind by %s at the deadline. %s/month would make it, which is %s more than now, over %d months.",
			n.GoalName, money(n.ShortfallMinor), money(n.SuggestedMonthlyMinor), money(n.ExtraMonthlyMinor), n.MonthsRemaining)
	case goalcoach.KindUnfunded:
		return fmt.Sprintf("- %s: has a deadline but no monthly contribution. %s/month over %d months would do it.",
			n.GoalName, money(n.SuggestedMonthlyMinor), n.MonthsRemaining)
	case goalcoach.KindReached:
		return "- " + n.GoalName + ": fully funded."
	case goalcoach.KindAhead:
		return "- " + n.GoalName + ": ahead of the deadline at the current rate."
	default:
		return "- " + n.GoalName + ": on track."
	}
}

// subscriptionFor finds a recurring cost by name and gathers its charge history
// from the ledger, so a price rise is measured against what was actually PAID
// rather than against whatever the schedule currently claims.
func subscriptionFor(app *appstate.App, want, competitorNote string) (negotiationprep.Subscription, bool) {
	sub := negotiationprep.Subscription{CompetitorNote: competitorNote}
	found := false
	for _, r := range app.Recurring() {
		if !strings.Contains(strings.ToLower(r.Label), want) {
			continue
		}
		sub.Name = r.Label
		monthly := r.MonthlyEquivalent()
		if monthly < 0 {
			monthly = -monthly
		}
		sub.MonthlyMinor = monthly
		found = true
		break
	}
	for _, t := range app.Transactions() {
		if t.IsTransfer() || !t.IsExpense() {
			continue
		}
		if !strings.Contains(strings.ToLower(t.Payee+" "+t.Desc), want) {
			continue
		}
		if sub.Name == "" {
			sub.Name = strings.TrimSpace(t.Payee)
		}
		sub.Charges = append(sub.Charges, negotiationprep.Charge{At: t.Date, AmountMinor: t.Amount.Abs().Amount})
		found = true
	}
	// With charges but no schedule, the most recent charge IS the current cost.
	if sub.MonthlyMinor == 0 && len(sub.Charges) > 0 {
		latest := sub.Charges[0]
		for _, c := range sub.Charges {
			if c.At.After(latest.At) {
				latest = c
			}
		}
		sub.MonthlyMinor = latest.AmountMinor
	}
	return sub, found
}
