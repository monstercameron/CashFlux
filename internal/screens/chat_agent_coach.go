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
	"github.com/monstercameron/CashFlux/internal/money"
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

	return []chatTool{
		{

			spec: ai.FunctionTool("check_goal_pace",
				"Review savings-goal coverage and compare the configured monthly plan with the canonical contribution needed for its deadline. Money is stated in each goal's target currency; saved money includes recorded contribution history, and coverage also includes virtual set-aside amounts. A configured plan is not evidence of an actual funding pace. Archived, paused, recently reviewed (14 days), nonfinancial, and incomplete goals without a future deadline receive no savings-pace recommendation. Completed targets may be acknowledged. Use list_goals for nonfinancial or deliberately quiet goals' canonical progress.",
				json.RawMessage(`{"type":"object","properties":{"goal":{"type":"string","description":"optional: part of one goal's name, to review just that one"}},"additionalProperties":false}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Goal string `json:"goal"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Goal coaching unavailable: invalid arguments: " + err.Error()
				}
				now := assistantGoalCoachNow()
				reviews, err := assistantGoalCoachForCall(app, strings.TrimSpace(a.Goal), now)
				if err != nil {
					return "Goal coaching unavailable: " + err.Error() + ". No funding or completion verdict was inferred."
				}
				if len(reviews) == 0 {
					return "No matching goals."
				}
				var b strings.Builder
				fmt.Fprintf(&b, "Savings-goal review at %s. Amounts use each goal's target currency. Saved money already includes recorded contributions; coverage also includes virtual set-aside amounts. Configured-plan comparisons are not an observed funding pace.\n", now.Format(time.RFC3339))
				silences := map[goalcoach.Silence]int{}
				notes := 0
				for _, review := range reviews {
					if review.Kind == goalcoach.KindSilent {
						silences[review.Silence]++
						continue
					}
					b.WriteString(goalReviewLine(review) + "\n")
					notes++
				}
				if notes == 0 {
					b.WriteString("No savings-pace coaching note was produced; this does not establish that the goals are on track.\n")
				}
				var omitted []string
				for _, reason := range []goalcoach.Silence{goalcoach.SilenceArchived, goalcoach.SilencePaused, goalcoach.SilenceRecent, goalcoach.SilenceNonfinancial, goalcoach.SilenceNoDeadline, goalcoach.SilenceDeadline} {
					if count := silences[reason]; count > 0 {
						omitted = append(omitted, fmt.Sprintf("%d %s", count, reason))
					}
				}
				if len(omitted) > 0 {
					b.WriteString("No savings-pace recommendation for: " + strings.Join(omitted, "; ") + ". Use list_goals for their current canonical progress.\n")
				}
				b.WriteString("These are observations and proposals, not changes. Do not apply an adjustment without approval.")
				return b.String()
			},
		},
		{
			spec: ai.FunctionTool("prepare_subscription_call",
				"Prepare a subscription negotiation from fresh household data in the selected active schedule's currency. Supply name or recurring_id to identify one outgoing cost. Current annual cost comes directly from its known billing cadence; historical charges alone never establish a current monthly cost. Future, transfer, income and report-excluded rows are omitted. Mixed-currency or nonmonthly history cannot establish a nominal monthly price rise. Competitor notes are supplied evidence, never invented.",
				json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"provider name, required unless recurring_id is supplied"},"recurring_id":{"type":"string","description":"optional exact active outgoing schedule ID"},"competitor_note":{"type":"string","description":"optional externally sourced comparison, quoted exactly"}},"additionalProperties":false}`)),
			run: func(raw json.RawMessage) string {
				var a struct {
					Name           string `json:"name"`
					RecurringID    string `json:"recurring_id"`
					CompetitorNote string `json:"competitor_note"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return "Subscription preparation unavailable: invalid arguments: " + err.Error()
				}
				now := assistantSubscriptionNow()
				prepared, err := assistantSubscriptionForCall(app, negotiationprep.Selection{Name: a.Name, RecurringID: a.RecurringID, CompetitorNote: a.CompetitorNote}, now)
				if err != nil {
					return "Subscription preparation unavailable: " + err.Error() + ". No current cost or price-rise claim was inferred."
				}
				prep, err := negotiationprep.BuildChecked(prepared.Subscription, now, func(minor int64) string { return goalReviewMoney(money.New(minor, prepared.Currency)) })
				if err != nil {
					return "Subscription preparation unavailable: " + err.Error() + ". No current cost or price-rise claim was inferred."
				}
				out := prepared.Basis(now) + "\n\n" + prep.TaskNotes()
				if !prep.Worthwhile() {
					return out + "\n\nNo strong negotiation point was established from tenure, price history or the supplied comparison."
				}
				return out + "\n\nOffer to file this as a to-do with add_task so the script is there when they call."
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

var assistantGoalCoachNow = time.Now

// assistantGoalCoachForCall loads each stored Goal with its embedded contribution
// log and rolled-up history. CurrentAmount is the canonical saved-money record;
// adding those history entries again would double-count already recorded money.
func assistantGoalCoachForCall(app *appstate.App, want string, now time.Time) ([]goalcoach.Review, error) {
	if app == nil || app.Store() == nil {
		return nil, fmt.Errorf("the local workspace is unavailable")
	}
	s := app.Store()
	settings, err := s.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("read goal-coaching settings: %w", err)
	}
	all, err := s.ListGoals()
	if err != nil {
		return nil, fmt.Errorf("read goal-coaching goals and contribution history: %w", err)
	}
	tasks, err := s.ListTasks()
	if err != nil {
		return nil, fmt.Errorf("read goal-coaching to-dos: %w", err)
	}
	selected := make([]domain.Goal, 0, len(all))
	want = strings.ToLower(want)
	for _, g := range all {
		if want == "" || strings.Contains(strings.ToLower(g.Name), want) {
			selected = append(selected, g)
		}
	}
	currentBase := settings.BaseCurrency
	if currentBase == "" {
		currentBase = "USD"
	}
	return goalcoach.ReviewAll(selected, tasks, currency.Rates{Base: currentBase, Rates: settings.FXRates}, now), nil
}

func goalReviewMoney(m money.Money) string { return fmtMoney(m) + " " + m.Currency }

// goalReviewLine formats the checked native facts; every monetary value retains
// its own currency, and unavailable facts never become a zero or on-track claim.
func goalReviewLine(r goalcoach.Review) string {
	prefix := "- " + r.GoalName + ": "
	if r.Kind == goalcoach.KindUnavailable {
		return prefix + "coaching unavailable: " + r.Err.Error() + ". No funding or completion verdict was inferred."
	}
	coverage := r.Evaluation.Coverage
	if r.Kind == goalcoach.KindReached {
		return prefix + fmt.Sprintf("target covered: saved %s plus set aside %s covers %s of target %s.", goalReviewMoney(coverage.Saved), goalReviewMoney(coverage.SetAside), goalReviewMoney(coverage.Covered), goalReviewMoney(r.Target))
	}
	required := goalReviewMoney(r.Evaluation.MonthlyNeeded)
	remaining := goalReviewMoney(coverage.Remaining)
	if r.Kind == goalcoach.KindUnfunded {
		return prefix + fmt.Sprintf("no positive monthly contribution is configured. Remaining %s; canonical requirement %s/month is a proposal, not evidence of actual funding.", remaining, required)
	}
	var line string
	if r.Kind == goalcoach.KindBehind {
		line = prefix + fmt.Sprintf("configured monthly plan %s is below the canonical requirement %s/month; proposal: increase by %s/month. Remaining %s.", goalReviewMoney(r.Monthly), required, goalReviewMoney(r.Extra), remaining)
	} else {
		line = prefix + fmt.Sprintf("configured monthly plan %s meets the canonical requirement %s/month. Remaining %s.", goalReviewMoney(r.Monthly), required, remaining)
	}
	if r.HasProjection {
		line += " Configured-plan finish: " + r.ProjectedFinish.Format("2006-01-02") + "."
	} else {
		line += " Configured-plan finish date is unavailable."
	}
	return line
}

var assistantSubscriptionNow = time.Now

// assistantSubscriptionForCall reads fresh, fallible household evidence on every
// invocation. A failed store read cannot become an empty or zero-cost verdict.
func assistantSubscriptionForCall(app *appstate.App, selection negotiationprep.Selection, now time.Time) (negotiationprep.Prepared, error) {
	if app == nil || app.Store() == nil {
		return negotiationprep.Prepared{}, fmt.Errorf("the local workspace is unavailable")
	}
	store := app.Store()
	settings, err := store.GetSettings()
	if err != nil {
		return negotiationprep.Prepared{}, fmt.Errorf("read subscription settings: %w", err)
	}
	recurring, err := store.ListRecurring()
	if err != nil {
		return negotiationprep.Prepared{}, fmt.Errorf("read subscription schedules: %w", err)
	}
	transactions, err := store.ListTransactions()
	if err != nil {
		return negotiationprep.Prepared{}, fmt.Errorf("read subscription history: %w", err)
	}
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	return negotiationprep.Prepare(negotiationprep.Input{Recurring: recurring, Transactions: transactions, Rates: currency.Rates{Base: base, Rates: settings.FXRates}, Now: now}, selection)
}
