// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
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
	issues   []budgeting.ViewIssue
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
	return &budgetViewBuilder{input: input, view: v, leakSeen: map[string]bool{}, paceRisk: map[string]bool{}}
}

func (builder *budgetViewBuilder) addIssues(issues []budgeting.ViewIssue) {
	v := &builder.view
	previous := len(builder.issues)
	builder.issues = budgeting.DeduplicateViewIssues(append(builder.issues, issues...))
	for _, issue := range builder.issues[previous:] {
		args := []any{"budget_id", issue.BudgetID, "error", issue.Err}
		if issue.Field != "" {
			args = append(args, "field", issue.Field)
		}
		if issue.Informational {
			slog.Info(issue.Operation, args...)
			continue
		}
		slog.Error(issue.Operation, args...)
		// A budget-local failure travels on Status.Error/Issues. Incomplete is
		// reserved for unavailable page-wide figures because existing widgets
		// treat any entry there as a reason to blank the whole headline.
		if issue.BudgetID == "" && len(v.Incomplete) == 0 {
			v.Incomplete = []string{uistate.T("budgets.incomplete")}
		}
	}
}

func (builder *budgetViewBuilder) addBudget(result budgeting.ViewBudget) {
	v, in := &builder.view, builder.input
	b, st := result.Budget, result.Status
	v.PeriodFrom[b.ID], v.PeriodTo[b.ID] = dateutil.FormatDate(result.Start), dateutil.FormatDate(result.End.AddDate(0, 0, -1))
	if result.Covered {
		v.Covered[b.ID] = true
	}
	if result.HasCarry {
		v.RollCarry[b.ID] = budgetRemainPhrase(result.Carry)
		v.RollNeg[b.ID] = result.Carry.IsNegative()
	}
	if amount, err := money.Add(v.RolledOver, result.PooledRollover); err != nil {
		builder.addIssues([]budgeting.ViewIssue{{Operation: "sum pooled budget rollover", BudgetID: b.ID, Err: err}})
	} else {
		v.RolledOver = amount
	}
	builder.addCap(result)
	builder.addIssues(result.Issues)
	v.Statuses = append(v.Statuses, st)
	if !result.Valid {
		return
	}
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
	if result.RestPerDay > 0 {
		currency := result.Status.Remaining.Currency
		if currency == "" {
			currency = in.Base
		}
		v.ProratedRest[b.ID] = fmtMoney(money.New(result.RestPerDay, currency))
	}
}

func (builder *budgetViewBuilder) addCap(result budgeting.ViewBudget) {
	b, limit := result.Budget, result.Status.Limit
	base, err := budgeting.BudgetBaseAt(b, result.Start)
	if err != nil {
		return
	}
	if !result.Valid || limit.Amount == base.Amount {
		return
	}
	v := &builder.view
	v.RollEffCap[b.ID] = fmtMoney(limit)
	parts := []string{uistate.T("budgets.capMathLimit", fmtMoney(base))}
	if delta := result.CarryDelta; delta > 0 {
		parts = append(parts, uistate.T("budgets.capMathCarryPlus", fmtMoney(money.New(delta, limit.Currency))))
	} else if delta < 0 {
		magnitude, err := money.Neg(delta)
		if err != nil {
			builder.addIssues([]budgeting.ViewIssue{{Operation: "format carried budget debt", BudgetID: b.ID, Err: err}})
			return
		}
		parts = append(parts, uistate.T("budgets.capMathCarryMinus", fmtMoney(money.New(magnitude, limit.Currency))))
	}
	if boost := result.Boost; boost > 0 {
		parts = append(parts, uistate.T("budgets.capMathBoost", fmtMoney(money.New(boost, limit.Currency))))
	} else if boost < 0 {
		magnitude, err := money.Neg(boost)
		if err != nil {
			builder.addIssues([]budgeting.ViewIssue{{Operation: "format budget reduction", BudgetID: b.ID, Err: err}})
			return
		}
		parts = append(parts, uistate.T("budgets.capMathBoostMinus", fmtMoney(money.New(magnitude, limit.Currency))))
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
	spent, err := money.Add(v.LastTotalSpent, result.PriorSpentBase)
	if err != nil {
		builder.addIssues([]budgeting.ViewIssue{{Operation: "sum prior budget spending", BudgetID: result.Budget.ID, Err: err}})
		return
	}
	limit, err := money.Add(v.LastTotalLimit, result.PriorLimitBase)
	if err != nil {
		builder.addIssues([]budgeting.ViewIssue{{Operation: "sum prior budget limits", BudgetID: result.Budget.ID, Err: err}})
		return
	}
	v.LastTotalSpent, v.LastTotalLimit = spent, limit
	comparison := budgeting.ComparePriorSpend(result.Prior.Spent.Amount, result.Prior.Limit.Amount)
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
	if roll.Excluded > 0 && len(v.Incomplete) == 0 {
		v.Incomplete = []string{uistate.T("budgets.incomplete")}
	}
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
		available, err := budgeting.EnvelopeAvailableForView(b, in, categorytree.DescendantsOfAll(in.Categories, b.TrackedCategoryIDs()))
		if err != nil {
			builder.addIssues([]budgeting.ViewIssue{{Operation: "calculate envelope availability", BudgetID: b.ID, Err: err}})
			continue
		}
		if available.IsNegative() {
			v.EnvAvail[b.ID] = fmtMoney(available.Abs()) + " " + uistate.T("budgets.overdrawnWord")
			_, end := budgeting.PeriodRangeAnchored(b.Period, p.Anchor, p.WeekStart, p.PayCycleAnchor)
			v.EnvDebtStart[b.ID] = uistate.T("budgets.envelopeDebtStartPeriod", uistate.LoadPrefs().FormatDate(end), fmtMoney(available.Abs()))
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
		var err error
		v.SavingsAssigned, err = money.Add(v.SavingsAssigned, account.AssignedBase)
		if err != nil {
			builder.addIssues([]budgeting.ViewIssue{{Operation: "sum savings assignments", Err: err}})
			v.SavingsIncomplete = true
		}
		if account.ConversionError != nil {
			builder.addIssues([]budgeting.ViewIssue{{Operation: "convert savings assignment", Err: account.ConversionError}})
			v.SavingsIncomplete = true
		}
	}
	// Preserve the existing account/budget overlap until the savings reconciliation ticket.
	var savingsErr error
	v.SavingsAssigned, savingsErr = money.Add(v.SavingsAssigned, v.SavingsBudgeted)
	if savingsErr != nil {
		builder.addIssues([]budgeting.ViewIssue{{Operation: "sum budget savings assignments", Err: savingsErr}})
		v.SavingsIncomplete = true
	}
	for _, goal := range app.Goals() {
		if goal.IsSinkingFund && !goal.Archived {
			amount, err := sinkingFundSetAsideBase(goal, in.Period.Now, in.Base, in.Rates)
			if err == nil && amount == 0 {
				continue
			}
			if err != nil {
				builder.addIssues([]budgeting.ViewIssue{{Operation: "convert sinking-fund contribution", Err: err}})
				continue
			}
			v.TotalFundSetAside, err = money.Add(v.TotalFundSetAside, amount)
			if err != nil {
				builder.addIssues([]budgeting.ViewIssue{{Operation: "sum sinking-fund contributions", Err: err}})
			}
		}
	}
	v.AgeMoney = budgetAgeOfMoney(app, in.Transactions, in.Base, in.Rates)
}

func sinkingFundSetAsideBase(goal domain.Goal, now time.Time, base string, rates currency.Rates) (int64, error) {
	amount := goalsvc.FundSetAsideMinor(goal, now)
	if amount == 0 {
		return 0, nil
	}
	return currency.ConvertBetween(amount, goal.TargetAmount.Currency, base, rates)
}
