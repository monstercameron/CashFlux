// SPDX-License-Identifier: MIT
package budgetview

import (
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/budgetplan"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/untracked"
	"strings"
)

// TrackingFooterPresentation is the checklist's count copy and save decision.
type TrackingFooterPresentation struct {
	Count    string
	Disabled bool
}

// PresentTrackingFooter uses the existing singular/plural catalog pairs.
func PresentTrackingFooter(categories, tags int, t Translate) TrackingFooterPresentation {
	key := func(prefix string, n int) string {
		if n == 1 {
			return prefix + ".one"
		}
		return prefix + ".many"
	}
	return TrackingFooterPresentation{Count: t("budgets.tracksCount", t(key("bc4b4.categoryCount", categories), categories), t(key("bc4b4.tagCount", tags), tags)), Disabled: categories == 0 && tags == 0}
}

// TrackRowPresentation supplies formatted inputs and the existing optional hint.
type TrackRowPresentation struct {
	Class, Subtitle, Amount string
	ShowRaise               bool
}

// PresentTrackRow formats a resolved candidate without changing its monetary value.
func PresentTrackRow(r TrackRow, base string, t Translate) TrackRowPresentation {
	out := TrackRowPresentation{Class: "track-row", Amount: money.FormatMinor(r.Amount, currency.Decimals(base)), ShowRaise: r.BudgetID != ""}
	if !r.Include {
		out.Class += " is-off"
	}
	out.Subtitle = t("track.rowSpent", FormatMoney(money.New(r.Cand.SpentMinor, base)), CandidateDate(r.Cand.LastSeen))
	if r.Cand.FromHint {
		out.Subtitle += " \u00b7 " + t("track.fromHistory")
	}
	return out
}

// TrackFooterPresentation contains optional plan and risk warnings.
type TrackFooterPresentation struct{ ToAssign, Risk string }

// PresentTrackFooter reports the plan-level consequence before any write.
func PresentTrackFooter(preview TrackPreview, base string, method budgeting.Methodology, t Translate) TrackFooterPresentation {
	out := TrackFooterPresentation{}
	if method == budgeting.MethodZeroBased {
		out.ToAssign = t("track.toAssign", FormatMoney(money.New(preview.Effect.ToAssignBeforeMinor, base)), FormatMoney(money.New(preview.Effect.ToAssignAfterMinor, base)))
	}
	if len(preview.RiskNames) > 0 {
		out.Risk = t("track.overspendRisk", strings.Join(preview.RiskNames, ", "))
	}
	return out
}

// AnnualDisclosure returns the caret tone and accessible action key.
func AnnualDisclosure(open bool) (class, key string) {
	if open {
		return "budget-annualgrid-caret is-open", "budgets.planYearHideAria"
	}
	return "budget-annualgrid-caret", "budgets.planYearShowAria"
}

// AnnualScenarioPresentation supplies scenario tone, status copy and signed delta.
type AnnualScenarioPresentation struct{ Class, StatusClass, Status, Delta string }

// PresentAnnualScenario retains positive/negative typography, without negating
// MinInt64. Formatting the signed value first keeps the full representable range.
func PresentAnnualScenario(on bool, delta int64, under int, base string, t Translate) AnnualScenarioPresentation {
	out := AnnualScenarioPresentation{Class: "budget-annualgrid-scenario", StatusClass: "budget-annualgrid-scenario-status is-clear", Status: t("budgets.scenarioAllFunded")}
	if on {
		out.Class += " is-on"
	}
	if under > 0 {
		out.StatusClass = "budget-annualgrid-scenario-status is-under"
		out.Status = t("budgets.scenarioUnderfunded", under)
	}
	sign := "+"
	value := money.FormatMinor(delta, currency.Decimals(base))
	if delta < 0 {
		sign = "\u2212"
		value = strings.TrimPrefix(value, "-")
	}
	out.Delta = sign + currency.Symbol(base) + value
	return out
}

// AnnualWindowClass returns the existing selection tone for a window button.
func AnnualWindowClass(current, value string) string {
	if current == value {
		return "budget-annualgrid-win is-on"
	}
	return "budget-annualgrid-win"
}

// AnnualFooterCell is a column total with its projected tone decided.
type AnnualFooterCell struct {
	Class, Amount string
	Projected     bool
}

// PresentAnnualFooter chooses actuals unless a future column has a positive projection.
func PresentAnnualFooter(grid budgeting.AnnualGrid, totals budgetplan.MonthAmounts, month, fromMonth int) AnnualFooterCell {
	out := AnnualFooterCell{Class: "budget-annualgrid-td is-foot" + GridColClass(month, grid.CurrentMonth, fromMonth), Amount: FormatMoney(grid.MonthActualTotals[month])}
	if month >= fromMonth && totals[month] > 0 {
		out.Projected = true
		out.Amount = FormatMoney(money.New(totals[month], grid.Currency))
	}
	return out
}

// ErrorColumnSpan includes the month window plus the row's Total column.
func ErrorColumnSpan(first, last int) int { return last - first + 2 }

// ApplyDisabled returns the bulk sheet's empty-selection decision.
func ApplyDisabled(effect untracked.Effect) bool { return effect.Categories == 0 }
