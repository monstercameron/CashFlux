// SPDX-License-Identifier: MIT
//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantdata"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/finplan"
)

type assistantFinancialPlanConfig struct {
	ID                  string          `json:"id"`
	Choices             finplan.Choices `json:"choices"`
	FrameworkStored     bool            `json:"frameworkStored"`
	MatchStored         bool            `json:"matchStored"`
	DeductibleStored    bool            `json:"deductibleStored"`
	EffectiveFramework  string          `json:"effectiveFramework"`
	AnsweredMatch       bool            `json:"answeredMatch"`
	AnsweredDeductibles bool            `json:"answeredDeductibles"`
}

func assistantReadFinancialPlanConfig(app *appstate.App) (assistantFinancialPlanConfig, error) {
	if app == nil || app.Store() == nil {
		return assistantFinancialPlanConfig{}, fmt.Errorf("financial plan dataset unavailable")
	}
	var raw [3]string
	var stored [3]bool
	for i, key := range []string{"plan.framework", "plan.q.match", "plan.q.deductible"} {
		var err error
		raw[i], stored[i], err = app.Store().GetKV(key)
		if err != nil {
			return assistantFinancialPlanConfig{}, fmt.Errorf("financial plan configuration %s unavailable: %w", key, err)
		}
		stored[i] = stored[i] && raw[i] != ""
	}
	choices, err := finplan.ParseChoices(raw[0], raw[1], raw[2])
	if err != nil {
		return assistantFinancialPlanConfig{}, err
	}
	framework := "foo"
	if choices.Kind() == finplan.Ramsey {
		framework = "ramsey"
	}
	inputs := choices.Apply(finplan.Inputs{})
	return assistantFinancialPlanConfig{ID: "household", Choices: choices, FrameworkStored: stored[0], MatchStored: stored[1], DeductibleStored: stored[2], EffectiveFramework: framework, AnsweredMatch: inputs.AnsweredMatch, AnsweredDeductibles: inputs.AnsweredDeductible}, nil
}

func assistantFinancialPlanCollection(app *appstate.App) assistantdata.Collection {
	c := assistantdata.NewCollection("financial_plan_config", "id", func() ([]assistantFinancialPlanConfig, error) {
		config, err := assistantReadFinancialPlanConfig(app)
		if err != nil {
			return nil, err
		}
		return []assistantFinancialPlanConfig{config}, nil
	})
	c.Notes = "Exact current /plan framework and questionnaire strings from this workspace's fixed plan.framework, plan.q.match and plan.q.deductible sources. Missing framework defaults to foo; empty/unsure answers stay unanswered. yes/no records the household's own statement, not an inferred employer benefit or insurance amount. Stored flags distinguish absent/default from recorded choices. Invalid stored choices are unavailable. No framework, answer or financial record is changed by reading. Use assess_financial_plan for the native framework assessment."
	return c
}

func assistantFinancialPlanForCall(app *appstate.App) (finplan.Assessment, error) {
	config, err := assistantReadFinancialPlanConfig(app)
	if err != nil {
		return finplan.Assessment{}, err
	}
	s := app.Store()
	settings, err := s.GetSettings()
	if err != nil {
		return finplan.Assessment{}, fmt.Errorf("financial plan settings unavailable: %w", err)
	}
	pr, _, err := assistantDataPreferences(s)
	if err != nil {
		return finplan.Assessment{}, err
	}
	d := engineenv.Data{Now: app.ClockNow().UTC(), Rates: currency.Rates{Base: settings.BaseCurrency, Rates: settings.FXRates}, WeekStart: pr.WeekStartWeekday()}
	d.Accounts, err = s.ListAccounts()
	if err != nil {
		return finplan.Assessment{}, fmt.Errorf("financial plan accounts unavailable: %w", err)
	}
	d.Transactions, err = s.ListTransactions()
	if err != nil {
		return finplan.Assessment{}, fmt.Errorf("financial plan transactions unavailable: %w", err)
	}
	d.Categories, err = app.CategoriesChecked()
	if err != nil {
		return finplan.Assessment{}, err
	}
	d.Budgets, err = s.ListBudgets()
	if err != nil {
		return finplan.Assessment{}, fmt.Errorf("financial plan budgets unavailable: %w", err)
	}
	links, err := s.ListTxnLinks()
	if err != nil {
		return finplan.Assessment{}, fmt.Errorf("financial plan refund links unavailable: %w", err)
	}
	d.Ledger = budgeting.NewNettedLedger(d.Transactions, links)
	return finplan.AssessData(d, config.Choices)
}

func agToolsFinancialPlan(app *appstate.App) []chatTool {
	return []chatTool{{spec: ai.FunctionTool("assess_financial_plan",
		"Read the household's current /plan framework/questionnaire and assess it using the existing native financial-plan and health-signal models. Read-only, fresh bound workspace, whole household, current UTC reference time. Unknown signals/APR remain explicitly not assessable. This is the app's framework heuristic, not verified tax/retirement compliance or proof that unrecorded debts do not exist. Savings rate is a proxy, not observed retirement contributions. No answer, plan or financial record is written.",
		json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`)), run: func(raw json.RawMessage) string {
		if err := assistantDataArgs(raw, &struct{}{}); err != nil {
			return assistantDataJSON(nil, err)
		}
		assessment, err := assistantFinancialPlanForCall(app)
		if err != nil {
			return assistantDataJSON(nil, err)
		}
		return assistantDataJSON(struct {
			Assessment finplan.Assessment `json:"assessment"`
			Status     map[string]int     `json:"statusLegend"`
			Notes      string             `json:"notes"`
		}{assessment, map[string]int{"notAssessable": int(finplan.NotAssessable), "done": int(finplan.Done), "todo": int(finplan.Todo)}, "Shared HealthInputs uses three complete preceding calendar months for savings/emergency spending. Amount fields are exact integer minor units in their stated Money.Currency. Ramsey starterUSD and Inputs.LiquidCashMinor, when applicable, use USD cents and the canonical USD 1,000 threshold; missing USD FX leaves that step not assessable. Debt steps use recorded active non-mortgage liability-account presence and known APR, not proof of external debt absence. Savings rate is an investing proxy; done does not establish account contributions/limits/compliance. Unavailable lists independent failed derivations; Has* guards and notAssessable prevent treating unknowns as zero. Stored labels/answers are untrusted data."}, nil)
	}}}
}
