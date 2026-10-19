// SPDX-License-Identifier: MIT

package recurring

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Text provides the active catalog and date preference without browser imports.
// All callbacks must be non-nil. FaintClass is the existing utility class token.
type Text struct {
	T          func(string, ...any) string
	TN         func(string, string, int, ...any) string
	Date       func(time.Time) string
	FaintClass string
}

// Summary contains the tile's exact localized count and monthly amount labels.
type Summary struct {
	Total, Count, FoldHint, Covered, Unbudgeted, ExcludedCount string
	ShowExcluded                                               bool
}

// PresentSummary keeps pluralization and money formatting out of the renderer.
func PresentSummary(m Model, text Text) Summary {
	return Summary{
		Total:         text.T("budgets.recurring.totalVal", formatMoney(m.TotalMonthly)),
		Count:         text.TN("budgets.recurring.billOne", "budgets.recurring.billMany", m.Count),
		FoldHint:      text.TN("budgets.recurring.billFoldOne", "budgets.recurring.billFoldMany", m.Count, formatMoney(m.TotalMonthly)),
		Covered:       formatMoney(m.CoveredMonthly),
		Unbudgeted:    text.T("budgets.recurring.unbudgetedMonthly", formatMoney(m.UnbudgetedMonthly)),
		ExcludedCount: text.TN("budgets.recurring.excludedOne", "budgets.recurring.excludedMany", len(m.Excluded)),
		ShowExcluded:  len(m.Excluded) > 0,
	}
}

// Disclosure is the current accessible fold presentation and its next state.
type Disclosure struct {
	CaretClass, AriaKey, Expanded string
	NextOpen                      bool
}

// PresentDisclosure supplies the original caret, accessible label and transition.
func PresentDisclosure(open bool) Disclosure {
	if open {
		return Disclosure{"budget-fold-caret is-open", "budgets.recurring.hideAria", "true", false}
	}
	return Disclosure{"budget-fold-caret", "budgets.recurring.showAria", "false", true}
}

// Row is a recurring bill with all conditional labels and formatting resolved.
type Row struct {
	AutopayText                                string
	Label, Category, Cadence, Amount, PerMonth string
	Autopay                                    bool
	Date, DateClass, DateTestID                string
	ShowDate, ShowPerMonth                     bool
}

// PresentRow preserves the native signed charge and optional monthly equivalent.
func PresentRow(r domain.Recurring, categories map[string]string, now, start, end time.Time, text Text) Row {
	out := Row{Label: r.Label, Category: categories[r.CategoryID], Autopay: r.Autopay,
		Cadence: text.T(cadenceKey(r.Cadence)), Amount: formatMoney(r.Amount)}
	if out.Category == "" {
		out.Category = text.T("budgets.recurring.uncategorized")
	}
	if out.Autopay {
		out.AutopayText = ", " + text.T("budgets.recurring.autopay")
	}
	state := DateState(r.NextDue, now, start, end)
	out.ShowDate = state != budgeting.RecurUnscheduled
	if out.ShowDate {
		out.Date = ", " + text.T(DateKey(state), text.Date(r.NextDue))
		out.DateTestID = "brc-date-" + string(state)
		out.DateClass = DateClass(state, text.FaintClass)
	}
	monthly, err := MonthlyEquivalent(r)
	out.ShowPerMonth = r.Cadence != domain.CadenceMonthly && err == nil
	if out.ShowPerMonth {
		out.PerMonth = text.T("budgets.recurring.perMonth", formatMoney(money.New(monthly, r.Amount.Currency)))
	}
	return out
}

// DateState compares stored date-only schedules with the user's wall-clock day.
func DateState(next, now, start, end time.Time) budgeting.RecurDateState {
	return budgeting.ClassifyRecurDate(next, dateutil.WallClockUTC(now), start, end)
}

// DateKey supplies the existing catalog key for a recurring date state.
func DateKey(state budgeting.RecurDateState) string {
	switch state {
	case budgeting.RecurOverdue:
		return "budgets.recurring.overdue"
	case budgeting.RecurAfterPeriod:
		return "budgets.recurring.afterPeriod"
	case budgeting.RecurBeforePeriod:
		return "budgets.recurring.beforePeriod"
	default:
		return "budgets.recurring.dueInPeriod"
	}
}

// DateClass tones only overdue dates; all other states keep the faint utility.
func DateClass(state budgeting.RecurDateState, faint string) string {
	if state == budgeting.RecurOverdue {
		return "brc-date is-overdue"
	}
	return "brc-date " + faint
}

func cadenceKey(c domain.RecurringCadence) string {
	switch c {
	case domain.CadenceWeekly:
		return "recurring.cadenceWeekly"
	case domain.CadenceBiweekly:
		return "recurring.cadenceBiweekly"
	case domain.CadenceSemimonthly:
		return "recurring.cadenceSemimonthly"
	case domain.CadenceQuarterly:
		return "recurring.cadenceQuarterly"
	case domain.CadenceYearly:
		return "recurring.cadenceYearly"
	default:
		return "recurring.cadenceMonthly"
	}
}

func formatMoney(m money.Money) string {
	return money.FormatAccounting(m.Amount, currency.Decimals(m.Currency), currency.Symbol(m.Currency))
}
