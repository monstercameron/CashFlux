// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/validate"
)

type assistantGoalContributionArgs struct {
	GoalID   string          `json:"goal_id"`
	Goal     string          `json:"goal"`
	Amount   json.RawMessage `json:"amount"`
	Currency string          `json:"currency"`
}

type assistantGoalContributionPlan struct {
	goal      domain.Goal
	amount    money.Money
	rates     currency.Rates
	err       error
	completed bool
	result    string
}

func assistantContributionArgs(raw json.RawMessage) (assistantGoalContributionArgs, error) {
	var a assistantGoalContributionArgs
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return a, fmt.Errorf("couldn't read the contribution details: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return a, fmt.Errorf("provide one contribution object")
	}
	if strings.TrimSpace(a.GoalID) == "" && strings.TrimSpace(a.Goal) == "" {
		return a, fmt.Errorf("provide a nonblank goal_id or a uniquely matching goal name")
	}
	return a, nil
}

func resolveAssistantContributionGoal(all []domain.Goal, a assistantGoalContributionArgs) (domain.Goal, error) {
	if id := strings.TrimSpace(a.GoalID); id != "" {
		for _, g := range all {
			if g.ID == id {
				return g, nil
			}
		}
		return domain.Goal{}, fmt.Errorf("no goal with ID %q", id)
	}
	q := strings.TrimSpace(a.Goal)
	if q == "" {
		return domain.Goal{}, fmt.Errorf("provide a nonblank goal_id or goal name")
	}
	var exact, partial []domain.Goal
	for _, g := range all {
		if strings.EqualFold(g.Name, q) {
			exact = append(exact, g)
		} else if strings.Contains(strings.ToLower(g.Name), strings.ToLower(q)) {
			partial = append(partial, g)
		}
	}
	matches := exact
	if len(matches) == 0 {
		matches = partial
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return domain.Goal{}, fmt.Errorf("no goal matching %q", q)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	choices := make([]string, 0, len(matches))
	for _, g := range matches {
		choices = append(choices, fmt.Sprintf("%q (ID %s)", g.Name, g.ID))
	}
	return domain.Goal{}, fmt.Errorf("several goals match %q; provide goal_id: %s", q, strings.Join(choices, "; "))
}

func assistantContributionRates(app *appstate.App) (currency.Rates, error) {
	s, err := app.Store().GetSettings()
	if err != nil {
		return currency.Rates{}, fmt.Errorf("read contribution exchange rates: %w", err)
	}
	base := strings.ToUpper(strings.TrimSpace(s.BaseCurrency))
	if base == "" {
		base = "USD"
	}
	return currency.Rates{Base: base, Rates: s.FXRates}, nil
}

func prepareAssistantGoalContribution(app *appstate.App, raw json.RawMessage) assistantGoalContributionPlan {
	var p assistantGoalContributionPlan
	a, err := assistantContributionArgs(raw)
	if err != nil {
		p.err = err
		return p
	}
	if app == nil {
		p.err = fmt.Errorf("the local workspace is unavailable")
		return p
	}
	all, err := app.Store().ListGoals()
	if err != nil {
		p.err = fmt.Errorf("read contribution goals: %w", err)
		return p
	}
	p.goal, err = resolveAssistantContributionGoal(all, a)
	if err != nil {
		p.err = err
		return p
	}
	if p.goal.Archived {
		p.err = fmt.Errorf("goal ID %s is archived; review it before adding money", p.goal.ID)
		return p
	}
	if !p.goal.IsFinancial() {
		p.err = fmt.Errorf("goal ID %s is not a financial goal", p.goal.ID)
		return p
	}
	code := p.goal.CurrentAmount.Currency
	if code == "" && p.goal.CurrentAmount.Amount == 0 {
		code = p.goal.TargetAmount.Currency
	}
	if !currency.Valid(code) {
		p.err = fmt.Errorf("goal ID %s has an unsupported saved-money currency %q", p.goal.ID, code)
		return p
	}
	if a.Currency != "" && !strings.EqualFold(strings.TrimSpace(a.Currency), code) {
		p.err = fmt.Errorf("the contribution must use the goal's saved-money currency %s", code)
		return p
	}
	decimal := string(a.Amount)
	if len(a.Amount) > 0 && a.Amount[0] == '"' {
		if err := json.Unmarshal(a.Amount, &decimal); err != nil {
			p.err = fmt.Errorf("invalid contribution amount: %w", err)
			return p
		}
	}
	minor, err := money.ParseMinor(decimal, currency.Decimals(code))
	if err != nil {
		p.err = fmt.Errorf("invalid contribution amount in %s: %w", code, err)
		return p
	}
	if minor <= 0 {
		p.err = fmt.Errorf("contribution amount must be greater than zero")
		return p
	}
	p.amount = money.New(minor, code)
	p.rates, err = assistantContributionRates(app)
	if err != nil {
		p.err = err
		return p
	}
	// These are checked canonical money/coverage calls, not a second persistence
	// path. The intended copy is used only to validate the approval preview.
	if _, err = goals.EvaluateCoverage(p.goal, p.rates); err != nil {
		p.err = fmt.Errorf("goal coverage is unavailable: %w", err)
		return p
	}
	updated, err := money.Add(p.goal.CurrentAmount.Amount, p.amount.Amount)
	if err != nil {
		p.err = fmt.Errorf("contribution saved total: %w", err)
		return p
	}
	intended := p.goal
	intended.CurrentAmount = money.New(updated, code)
	if issues := validate.ValidateGoal(intended); !issues.OK() {
		p.err = fmt.Errorf("the contribution is not supported for this goal: %w", issues)
		return p
	}
	if _, err = goals.EvaluateCoverage(intended, p.rates); err != nil {
		p.err = fmt.Errorf("contribution coverage is unavailable: %w", err)
	}
	return p
}

func assistantContributionMoney(m money.Money) string {
	text, err := money.FormatMinorChecked(m.Amount, currency.Decimals(m.Currency))
	if err != nil {
		return "unavailable " + m.Currency
	}
	return text + " " + m.Currency
}

// agToolsGoalWrites binds a memo-only contribution to its reviewed goal and FX
// snapshot. The canonical service owns validation, history, atomic save and undo.
func agToolsGoalWrites(app *appstate.App, _ string, _ currency.Rates) []chatTool {
	prepared := map[string]*assistantGoalContributionPlan{}
	return []chatTool{{
		spec: ai.FunctionTool("add_goal_contribution", "Record a positive memo-only contribution toward a financial savings goal; this updates saved progress and contribution history without posting a ledger transaction. Prefer exact goal_id, otherwise an exact or uniquely matching goal name. Amount is an exact decimal number or string in the goal's SAVED-money currency (shown by preview); currency may explicitly confirm it. Archived/nonfinancial goals, ambiguous names, unavailable FX, nonpositive/overflowing amounts and stale approvals are rejected. Contribution time is now; a historical date is unsupported. Requires approval.",
			json.RawMessage(`{"type":"object","properties":{"goal_id":{"type":"string"},"goal":{"type":"string"},"amount":{"type":["number","string"]},"currency":{"type":"string"}},"required":["amount"],"anyOf":[{"required":["goal_id"]},{"required":["goal"]}],"additionalProperties":false}`)),
		mutates: true,
		preview: func(raw json.RawMessage) string {
			p := prepareAssistantGoalContribution(app, raw)
			prepared[string(raw)] = &p
			if p.err != nil {
				return "Can't record that contribution: " + p.err.Error()
			}
			return fmt.Sprintf("Record a memo-only contribution of %s toward %q (ID %s). Saved money is recorded in %s; the target uses %s. Record its contribution history at the current time.", assistantContributionMoney(p.amount), p.goal.Name, p.goal.ID, p.amount.Currency, p.goal.TargetAmount.Currency)
		},
		run: func(raw json.RawMessage) string {
			p, ok := prepared[string(raw)]
			if !ok {
				return "Review and approve the resolved goal contribution preview before recording it."
			}
			if p.err != nil {
				return "Can't record that contribution: " + p.err.Error()
			}
			if p.completed {
				return "This approved contribution was already recorded. " + p.result
			}
			all, err := app.Store().ListGoals()
			if err != nil {
				return "Couldn't read the current goal: " + err.Error()
			}
			var current domain.Goal
			found := false
			for _, g := range all {
				if g.ID == p.goal.ID {
					current = g
					found = true
					break
				}
			}
			if !found {
				return fmt.Sprintf("Goal ID %s no longer exists. Review the goal before recording a contribution.", p.goal.ID)
			}
			if !reflect.DeepEqual(current, p.goal) {
				return fmt.Sprintf("Goal ID %s changed after the preview. Review and approve its current details before recording a contribution.", p.goal.ID)
			}
			rates, err := assistantContributionRates(app)
			if err != nil {
				return "Couldn't validate the contribution: " + err.Error()
			}
			if !reflect.DeepEqual(rates, p.rates) {
				return "Exchange rates changed after the preview. Review and approve the goal contribution again."
			}
			if _, err := goals.EvaluateCoverage(current, rates); err != nil {
				return "Goal coverage is unavailable: " + err.Error()
			}
			if _, err := app.ContributeToGoal(current, p.amount, false); err != nil {
				return "Couldn't record the contribution: " + err.Error()
			}
			p.completed = true
			p.result = fmt.Sprintf("Recorded a memo-only contribution of %s toward %q (ID %s), with contribution history for undo.%s", assistantContributionMoney(p.amount), current.Name, current.ID, openLink("/goals", current.ID))
			return p.result
		},
	}}
}
