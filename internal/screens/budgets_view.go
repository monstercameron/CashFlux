// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// budgetViewBuilder formats the pure budgeting pipeline's results for the screen.
// budgetViewBuilder carries the explicit results for It.
type budgetViewBuilder struct {
	input    budgeting.ViewInput
	view     budgetView
	leakSeen map[string]bool
	paceRisk map[string]bool
	budgets  map[string]domain.Budget
}

func newBudgetViewBuilder(input budgeting.ViewInput, showLastMonth bool) *budgetViewBuilder {
	v := budgetView{
		Base: input.Base, Anchor: input.Period.Anchor, LastMonthMode: showLastMonth,
		CatName: map[string]string{}, PaceOver: map[string]string{}, PaceMark: map[string]budgetPaceMark{},
		RollCarry: map[string]string{}, RollNeg: map[string]bool{}, RollEffCap: map[string]string{}, RollEffCapMath: map[string]string{},
		ProratedRest: map[string]string{}, Covered: map[string]bool{}, EarlyPeriod: map[string]bool{},
		Committed: map[string]budgetCommitted{}, PeriodFrom: map[string]string{}, PeriodTo: map[string]string{},
		LastMonth: map[string]budgetLastMonth{}, EffMethod: map[string]budgeting.Methodology{},
		EnvAvail: map[string]string{}, EnvNeg: map[string]bool{}, EnvDebtStart: map[string]string{},
		Statuses: make([]budgeting.Status, 0),
	}
	for _, c := range input.Categories {
		v.CatName[c.ID] = c.Name
	}
	return &budgetViewBuilder{input: input, view: v, leakSeen: map[string]bool{}, paceRisk: map[string]bool{}, budgets: map[string]domain.Budget{}}
}

func (builder *budgetViewBuilder) addIssues(issues []budgeting.ViewIssue) {
	v := &builder.view
	for _, issue := range issues {
		args := []any{"budget_id", issue.BudgetID, "error", issue.Err}
		if issue.Field != "" {
			args = append(args, "field", issue.Field)
		}
		slog.Error(issue.Operation, args...)
		b := builder.budgets[issue.BudgetID]
		v.Incomplete = append(v.Incomplete, fmt.Sprintf("%s: %s", budgetTitle(b.Name, v.CatName[b.CategoryID]), uistate.T("budgets.incomplete")))
	}
}

func (builder *budgetViewBuilder) addBudget(result budgeting.ViewBudget) {
	v, in := &builder.view, builder.input
	b, st := result.Budget, result.Status
	builder.budgets[b.ID] = b
	v.PeriodFrom[b.ID], v.PeriodTo[b.ID] = dateutil.FormatDate(result.Start), dateutil.FormatDate(result.End.AddDate(0, 0, -1))
	if result.Covered {
		v.Covered[b.ID] = true
	}
	if result.HasCarry {
		v.RollCarry[b.ID] = budgetRemainPhrase(result.Carry)
		v.RollNeg[b.ID] = result.Carry.IsNegative()
	}
	v.RolledOver += result.PooledRollover
	builder.addCap(result)
	builder.addIssues(result.Issues)
	if !result.Valid {
		return
	}
	v.Statuses = append(v.Statuses, st)
	for _, sus := range txnclassify.SuspectsIn(in.Transactions, func(id string) string { return v.CatName[id] }, result.Start, result.End) {
		if (!b.TracksCategory(sus.Txn.CategoryID) && !result.Covers[sus.Txn.CategoryID]) || builder.leakSeen[sus.Txn.ID] {
			continue
		}
		builder.leakSeen[sus.Txn.ID] = true
		v.LeakRows = append(v.LeakRows, sus.Txn)
	}
	builder.addCommitted(result)
	if result.Early {
		v.EarlyPeriod[b.ID] = true
	}
	builder.addPrior(result)
	if result.PaceOver.IsPositive() {
		v.PaceOver[b.ID] = fmtMoney(result.PaceOver)
		builder.paceRisk[b.ID] = true
	}
	if mark, ok := buildBudgetPaceMark(st, result.Committed, result.Start, result.End, in.Period.Now); ok {
		v.PaceMark[b.ID] = mark
	}
	if result.ProratedRest > 0 {
		v.ProratedRest[b.ID] = fmtMoney(money.New(result.ProratedRest, in.Base))
	}
}

func (builder *budgetViewBuilder) addCap(result budgeting.ViewBudget) {
	b, limit := result.Budget, result.Status.Limit
	if limit.Amount == b.Limit.Amount {
		return
	}
	v := &builder.view
	v.RollEffCap[b.ID] = fmtMoney(limit)
	parts := []string{uistate.T("budgets.capMathLimit", fmtMoney(b.Limit))}
	if delta := result.CarryDelta; delta > 0 {
		parts = append(parts, uistate.T("budgets.capMathCarryPlus", fmtMoney(money.New(delta, limit.Currency))))
	} else if delta < 0 {
		parts = append(parts, uistate.T("budgets.capMathCarryMinus", fmtMoney(money.New(-delta, limit.Currency))))
	}
	if boost := result.Boost; boost > 0 {
		parts = append(parts, uistate.T("budgets.capMathBoost", fmtMoney(money.New(boost, limit.Currency))))
	} else if boost < 0 {
		parts = append(parts, uistate.T("budgets.capMathBoostMinus", fmtMoney(money.New(-boost, limit.Currency))))
	}
	v.RollEffCapMath[b.ID] = strings.Join(parts, " ")
}

func (builder *budgetViewBuilder) addCommitted(result budgeting.ViewBudget) {
	bc, has := buildBudgetCommitted(result.Status, result.Budget, builder.input.Recurring, result.Committed)
	if len(result.LandingItems) > 0 {
		var notes []string
		for _, item := range result.LandingItems {
			notes = append(notes, uistate.T("budgets.landedNote", fmtMoney(item.Amount), item.Label))
		}
		note := strings.Join(notes, "; ")
		if has {
			if bc.SetAsideNote != "" {
				bc.SetAsideNote += "; " + note
			} else {
				bc.SetAsideNote = note
			}
		} else {
			bc, has = budgetCommitted{SetAsideNote: note}, true
		}
	}
	if has {
		builder.view.Committed[result.Budget.ID] = bc
	}
}

func (builder *budgetViewBuilder) addPrior(result budgeting.ViewBudget) {
	if !result.HasPrior {
		return
	}
	v := &builder.view
	v.LastTotalSpent += result.PriorSpentBase
	v.LastTotalLimit += result.PriorLimitBase
	comparison := budgeting.ComparePriorSpend(result.Prior.Spent.Amount, result.Status.Limit.Amount)
	lm := budgetLastMonth{Spent: fmtMoney(result.Prior.Spent), Pct: comparison.Percent, Fill: comparison.Fill}
	if comparison.Delta >= 0 {
		lm.Delta = uistate.T("budgets.lastMonthUnder", fmtMoney(money.New(comparison.Delta, result.Status.Spent.Currency)))
	} else {
		lm.Over = true
		lm.Delta = uistate.T("budgets.lastMonthOver", fmtMoney(money.New(-comparison.Delta, result.Status.Spent.Currency)))
	}
	v.LastMonth[result.Budget.ID] = lm
}

func (builder *budgetViewBuilder) summarize() {
	v := &builder.view
	budgeting.SortViewStatuses(v.Statuses, builder.paceRisk)
	roll, issues, err := budgeting.SummarizeViewStatuses(v.Statuses, v.Base, builder.input.Rates)
	builder.addIssues(issues)
	if err != nil {
		slog.Error("summarize budget rollup", "error", err)
		v.Incomplete = append(v.Incomplete, uistate.T("budgets.incomplete"))
	}
	v.OverCount, v.NearCount = roll.OverCount, roll.NearCount
	v.TotalSpent, v.TotalLimit, v.TotalOver = roll.SpentMinor, roll.LimitMinor, roll.OverMinor
	v.SavingsBudgeted, v.SavingsMoved = roll.SavingsBudgetedMinor, roll.SavingsMovedMinor
}

func (builder *budgetViewBuilder) addEnvelopes(budgets []domain.Budget, householdMethod string) {
	v, in := &builder.view, builder.input
	p := in.Period
	v.Method = budgeting.ParseMethodology(householdMethod)
	for _, b := range budgets {
		method := v.Method
		if override := budgeting.ParseMethodology(b.Methodology); b.Methodology != "" && override.Valid() {
			method = override
		}
		v.EffMethod[b.ID] = method
		if method != budgeting.MethodEnvelope {
			continue
		}
		available, err := budgeting.EnvelopeAvailableAnchored(b, in.Transactions, p.Anchor, p.WeekStart, p.PayCycleAnchor, in.Rates, categorytree.DescendantsOfAll(in.Categories, b.TrackedCategoryIDs()))
		if err != nil {
			builder.addIssues([]budgeting.ViewIssue{{Operation: "calculate envelope availability", BudgetID: b.ID, Err: err}})
			continue
		}
		if available.IsNegative() {
			v.EnvAvail[b.ID] = fmtMoney(available.Abs()) + " " + uistate.T("budgets.overdrawnWord")
			_, end := budgeting.PeriodRangeAnchored(b.Period, p.Anchor, p.WeekStart, p.PayCycleAnchor)
			v.EnvDebtStart[b.ID] = uistate.T("budgets.envelopeDebtStart", budgetMonthName(end, false), fmtMoney(available.Abs()))
		} else {
			v.EnvAvail[b.ID] = fmtMoney(available)
		}
		v.EnvNeg[b.ID] = available.IsNegative()
	}
}

func (builder *budgetViewBuilder) addIncome(pr prefs.Prefs) {
	v, in := &builder.view, builder.input
	basis := budgeting.ViewIncomeBasis{Mode: pr.BudgetIncomeMode, PaycheckMin: pr.BudgetPaycheckMinMinor, Fixed: pr.MonthlyIncomeMinor, CategoryIDs: pr.BudgetIncomeCategoryIDs, AverageMonths: pr.BudgetIncomeAvgMonths}
	income, err := budgeting.ResolveViewIncome(basis, in.Transactions, in.Categories, in.Period, in.Base, in.Rates)
	explicit := basis.Mode == budgeting.IncomeModeAll || basis.Mode == budgeting.IncomeModePaychecks || basis.Mode == budgeting.IncomeModeFixed || basis.Mode == budgeting.IncomeModeCategories
	if err != nil {
		operation := "calculate budget income"
		if explicit {
			operation = "calculate selected budget income"
		}
		slog.Error(operation, "error", err)
		v.Incomplete = append(v.Incomplete, uistate.T("budgets.incomplete"))
	}
	if !explicit && income == 0 {
		monthStart, _ := budgeting.PeriodRange(domain.PeriodMonthly, in.Period.Anchor, in.Period.WeekStart)
		if raw, _, err := ledger.PeriodTotals(in.Transactions, dateutil.AddMonths(monthStart, -1), monthStart, in.Rates); err == nil {
			income = raw.Amount
		}
	}
	v.BannerIncome = income
}

func (builder *budgetViewBuilder) addSavings(app *appstate.App, memberID string) {
	v, in := &builder.view, builder.input
	v.SavingsAccts = computeSavingsAccounts(app, memberID, in.Period.Anchor, in.Base, in.Rates)
	for _, account := range v.SavingsAccts {
		v.SavingsAssigned += account.AssignedBase
	}
	// Preserve the existing account/budget overlap until the savings reconciliation ticket.
	v.SavingsAssigned += v.SavingsBudgeted
	for _, goal := range app.Goals() {
		if goal.IsSinkingFund && !goal.Archived {
			v.TotalFundSetAside += goalsvc.FundSetAsideMinor(goal, in.Period.Now)
		}
	}
	v.AgeMoney = computeAgeOfMoney(in.Transactions, in.Base, in.Rates)
}
