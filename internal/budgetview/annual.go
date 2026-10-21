// SPDX-License-Identifier: MIT
package budgetview

import (
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/budgetplan"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"strconv"
	"time"
)

// ScenarioStepMinor is one income step in base currency minor units.
const ScenarioStepMinor int64 = 10000

// StepScenario performs checked addition, preserving the previous value on overflow.
func StepScenario(current, step int64) int64 {
	next, err := money.Add(current, step)
	if err != nil {
		return current
	}
	return next
}

// AnnualBuild includes the checked grid and its budget category cover sets.
type AnnualBuild struct {
	Grid   budgeting.AnnualGrid
	Covers map[string]map[string]bool
	Err    error
}

// BuildAnnual delegates domain arithmetic and computes category coverage once.
func BuildAnnual(budgets []domain.Budget, ledger budgeting.NettedLedger, cats []domain.Category, year int, rates currency.Rates, week time.Weekday, now time.Time) AnnualBuild {
	covers := make(map[string]map[string]bool, len(budgets))
	for _, b := range budgets {
		covers[b.ID] = categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs())
	}
	grid, err := budgeting.BuildAnnualGrid(budgets, ledger, year, rates, week, now, covers)
	return AnnualBuild{Grid: grid, Covers: covers, Err: err}
}

// CurrentMonth returns the zero-based current column, or -1 for another year.
func CurrentMonth(now time.Time, year int) int {
	if now.Year() == year {
		return int(now.Month()) - 1
	}
	return -1
}

// FutureMonth is the first projected column (12 means there are none).
func FutureMonth(grid budgeting.AnnualGrid, now time.Time) int {
	if grid.CurrentMonth >= 0 {
		return grid.CurrentMonth + 1
	}
	if grid.Year > now.Year() {
		return 0
	}
	return 12
}

// AnnualProjection holds the future-month amounts after coverage deduplication.
type AnnualProjection struct {
	FromMonth int
	PerBudget map[string]budgetplan.MonthAmounts
	Totals    budgetplan.MonthAmounts
	Err       error
}

// ProjectAnnual maps recurring bills and goals onto rows and unique column totals.
func ProjectAnnual(grid budgeting.AnnualGrid, covers map[string]map[string]bool, recurrings []domain.Recurring, goals []domain.Goal, now time.Time, rates currency.Rates) AnnualProjection {
	out := AnnualProjection{FromMonth: FutureMonth(grid, now)}
	var all, spending []string
	for _, r := range grid.Rows {
		all = append(all, r.BudgetID)
		if !r.Saving && r.Error == nil {
			spending = append(spending, r.BudgetID)
		}
	}
	projection := budgetplan.ProjectWithGoalRates(recurrings, goals, grid.Year, out.FromMonth, grid.Currency, rates)
	out.PerBudget, out.Err = projection.PerBudget(all, covers)
	if out.Err != nil {
		return out
	}
	out.Totals, out.Err = projection.TotalUnique(spending, covers)
	return out
}

// AnnualScenario runs the funding waterfall in display order for each month.
func AnnualScenario(grid budgeting.AnnualGrid, deltaMinor int64) (map[string][12]int64, int) {
	out := map[string][12]int64{}
	count := 0
	for m := 0; m < 12; m++ {
		plans := make([]budgetplan.BudgetPlan, 0, len(grid.Rows))
		for _, r := range grid.Rows {
			if r.Saving || r.Error != nil {
				continue
			}
			plans = append(plans, budgetplan.BudgetPlan{BudgetID: r.BudgetID, Name: r.Name, PlanMinor: r.Cells[m].Plan.Amount})
		}
		res := budgetplan.Evaluate(budgetplan.ScenarioInput{
			IncomeMinor:      grid.MonthPlanTotals[m].Amount,
			IncomeDeltaMinor: deltaMinor,
			Plans:            plans,
		})
		for _, f := range res.Funded {
			if f.ShortfallMinor > 0 {
				row := out[f.BudgetID]
				row[m] = f.ShortfallMinor
				out[f.BudgetID] = row
				count++
			}
		}
	}
	return out, count
}

// GridColClass returns the historical column tone modifiers.
func GridColClass(i, current, fromMonth int) string {
	switch {
	case i == 12:
		return " is-total"
	case i == current:
		return " is-current"
	case i >= fromMonth:
		return " is-future"
	default:
		return ""
	}
}

// WindowRange returns the inclusive bounds of a half-year or the full year.
func WindowRange(win string) (first, last int) {
	switch win {
	case "h1":
		return 0, 5
	case "h2":
		return 6, 11
	}
	return 0, 11
}

// MonthDates returns the half-open month window in canonical UTC dates.
func MonthDates(year, month int) (string, string) {
	start := dateutil.MonthStart(time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC))
	return dateutil.FormatDate(start), dateutil.FormatDate(dateutil.AddMonths(start, 1))
}

// MonthLabel joins a translated full month name and year, or just an invalid year.
func MonthLabel(month, year int, t Translate) string {
	if month < 0 || month > 11 {
		return strconv.Itoa(year)
	}
	return t(MonthFullKeys[month]) + " " + strconv.Itoa(year)
}

// MonthFullKeys are the existing full-month catalog keys, in calendar order.
var MonthFullKeys = [12]string{
	"budgets.month.january", "budgets.month.february", "budgets.month.march", "budgets.month.april",
	"budgets.month.mayFull", "budgets.month.june", "budgets.month.july", "budgets.month.august",
	"budgets.month.september", "budgets.month.october", "budgets.month.november", "budgets.month.december",
}

// AnnualPlanTotal returns the normalized yearly total; excluded budgets are omitted.
func AnnualPlanTotal(budgets []domain.Budget, base string, rateTables ...currency.Rates) int64 {
	rates := currency.Rates{Base: base}
	if len(rateTables) > 0 {
		rates = rateTables[0]
	}
	normalized, _ := budgeting.NormalizeBudgets(budgets, domain.PeriodYearly, rates)
	return normalized.Total.Amount
}

// AnnualPlanLabel formats the plan and its existing partial-total qualifier.
func AnnualPlanLabel(budgets []domain.Budget, rates currency.Rates, t Translate) string {
	normalized, _ := budgeting.NormalizeBudgets(budgets, domain.PeriodYearly, rates)
	label := FormatMoney(normalized.Total)
	if normalized.Excluded > 0 {
		if normalized.Excluded == 1 {
			label += " " + t("bud2.excludesOneBudget")
		} else {
			label += " " + t("bud2.excludesManyBudgets", normalized.Excluded)
		}
	}
	return label
}

// BudgetCategories indexes drill-down category IDs by budget identity.
func BudgetCategories(budgets []domain.Budget) map[string][]string {
	out := map[string][]string{}
	for _, b := range budgets {
		out[b.ID] = b.TrackedCategoryIDs()
	}
	return out
}

// AnnualCellPresentation is the complete non-rendering decision for a grid cell.
type AnnualCellPresentation struct{ Class, PrimaryClass, Primary, Title, Aria string }

// PresentAnnualCell preserves tone precedence, projected figures and accessible copy.
func PresentAnnualCell(cell budgeting.AnnualGridCell, current, future bool, projected, shortfall int64, base, budgetName, monthLabel string, t Translate) AnnualCellPresentation {
	out := AnnualCellPresentation{Class: "budget-annualgrid-td", PrimaryClass: "budget-annualgrid-actual", Primary: FormatMoney(cell.Actual)}
	if current {
		out.Class += " is-current"
	}
	if future {
		out.Class += " is-future"
	}
	if cell.Over && !future {
		out.Class += " is-over"
	}
	if shortfall > 0 {
		out.Class += " is-underfunded"
	}
	out.Aria = t("budgets.gridCellAria", budgetName, monthLabel, FormatMoney(cell.Actual), FormatMoney(cell.Plan))
	if future && projected > 0 {
		amount := FormatMoney(money.New(projected, base))
		out.PrimaryClass, out.Primary = "budget-annualgrid-projected", amount
		out.Title = t("budgets.gridProjectedTitle", amount)
		out.Aria = t("budgets.gridCellAriaProjected", budgetName, monthLabel, amount, FormatMoney(cell.Plan))
	}
	if shortfall > 0 {
		amount := FormatMoney(money.New(shortfall, base))
		out.Title = t("budgets.scenarioUnderTitle", amount)
		out.Aria += " " + t("budgets.gridCellAriaShort", amount)
	}
	if out.Title == "" {
		out.Title = out.Aria
	}
	return out
}

// StepYear shifts the viewed year with the existing integer navigation semantics.
func StepYear(year, step int) int { return year + step }
