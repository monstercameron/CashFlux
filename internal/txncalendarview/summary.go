// SPDX-License-Identifier: MIT

package txncalendarview

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
)

// WeekInput is the weekly summary's raw data and accessibility identity.
type WeekInput struct {
	Cashflow  txncalendar.Cashflow
	Base      string
	Scheduled int
	WeekOf    string
	Future    bool
	SummaryID string
}

// Line is a preformatted summary line and its unchanged CSS classes.
type Line struct{ Class, Text string }

// WeekSummary contains the visible lines and the hidden complete description.
type WeekSummary struct {
	Lines []Line
	Aria  string
}

// WeekLastLine chooses the existing transaction, schedule or empty-week label.
func (l Labels) WeekLastLine(in WeekInput) string {
	if in.Cashflow.Count > 0 {
		if in.Cashflow.Count == 1 {
			return l.T("transactions.tq3.weekOne")
		}
		return l.T("transactions.tq3.weekCount", in.Cashflow.Count)
	}
	if in.Scheduled > 0 {
		return l.T("transactions.tq3.weekScheduled", in.Scheduled)
	}
	if in.Future {
		return l.T("transactions.tq3.weekEmpty")
	}
	return "\u2003"
}

// WeekSummary preserves visible zero-spend suppression and the full accessible
// explanation, including income, movements and pending scheduled occurrences.
func (l Labels) WeekSummary(in WeekInput) WeekSummary {
	out := WeekSummary{Lines: make([]Line, 0, 4)}
	spent := l.unsigned(in.Cashflow.Spent, in.Base)
	if in.Cashflow.Spent > 0 {
		out.Lines = append(out.Lines, Line{"txn-cal-week-spent text-down", l.T("r9c.weekSpent", spent)})
	} else if in.Scheduled > 0 || in.Future {
		out.Lines = append(out.Lines, Line{"sr-only txn-cal-empty-week text-dim", l.T("r9c.weekSpent", spent)})
	}
	if in.Cashflow.Income > 0 {
		out.Lines = append(out.Lines, Line{"txn-cal-week-income text-up", l.T("r9c.weekIncome", l.unsigned(in.Cashflow.Income, in.Base))})
	}
	if in.Cashflow.Moved > 0 {
		out.Lines = append(out.Lines, Line{"txn-cal-week-moved", l.T("transactions.tq3.weekMoved", l.unsigned(in.Cashflow.Moved, in.Base))})
	}
	class := "txn-cal-week-count"
	if in.Cashflow.Count == 0 && in.Scheduled == 0 {
		class += " text-dim"
	}
	out.Lines = append(out.Lines, Line{class, l.WeekLastLine(in)})
	count := l.T("transactions.tq3.weekManySummary", in.Cashflow.Count)
	if in.Cashflow.Count == 1 {
		count = l.T("transactions.tq3.weekOneSummary")
	}
	scheduled := ""
	if in.Scheduled > 0 {
		scheduled = l.T("transactions.tq3.weekSummaryScheduled", in.Scheduled)
	}
	out.Aria = l.T("transactions.tq3.weekSummary", in.WeekOf, spent, l.unsigned(in.Cashflow.Income, in.Base), l.unsigned(in.Cashflow.Moved, in.Base), count, scheduled)
	return out
}

// GhostSummary is a scheduled marker's class, status, name and day description.
type GhostSummary struct{ Class, Status, Label, Aria string }

// GhostAria chooses the precise recurring, liability or missed occurrence wording.
func (l Labels) GhostAria(ghost txncalendar.Ghost, amountText string) string {
	switch {
	case ghost.NotPosted:
		return l.T("transactions.calNotPostedSummary", ghost.Label, amountText)
	case ghost.Liability:
		return l.T("transactions.calAccountDueSummary", ghost.Label, amountText)
	default:
		return l.T("transactions.calScheduledDueSummary", ghost.Label, amountText)
	}
}

// GhostSummary converts a scheduled magnitude using checked money operations.
// A missing rate or an unrepresentable absolute magnitude retains the unavailable
// label. FX conversion retains the shared currency package's existing policy.
func (l Labels) GhostSummary(ghost txncalendar.Ghost, rates currency.Rates) GhostSummary {
	text := l.T("transactions.calScheduledAmountUnavailable")
	amount := money.New(ghost.Amount, ghost.Currency)
	var err error
	amount.Amount, err = money.Abs(amount.Amount)
	if err == nil {
		if converted, err := rates.Convert(amount, rates.Base); err == nil {
			text = l.unsigned(converted.Amount, converted.Currency)
		}
	}
	out := GhostSummary{Class: "txn-cal-ghost text-dim", Label: ghost.Label, Aria: l.GhostAria(ghost, text)}
	if ghost.NotPosted {
		out.Class = "txn-cal-ghost txn-cal-ghost-missed"
		out.Status = l.T("transactions.calNotPostedStatus", text)
	} else {
		out.Status = l.T("transactions.calDueStatus", text)
	}
	return out
}

// CellSummary contains all non-rendering decisions for a single calendar button.
type CellSummary struct {
	ShowNet, ShowMoved   bool
	Class, Aria, DayText string
	Dots                 int
	Ghosts               []GhostSummary
	Pickable, Current    bool
}

// CellSummary prepares density, schedule markers and the complete day name.
func (l Labels) CellSummary(day Day) CellSummary {
	// Every day in the grid opens its date in the ledger, including the days from the neighbouring months.
	out := CellSummary{ShowNet: day.NetStr != "", ShowMoved: day.MovedStr != "", Class: "cal-cell txn-cal-cell", DayText: strconv.Itoa(day.Day), Dots: DotCount(day.Count), Pickable: true, Current: day.InMonth && day.Today}
	if !day.InMonth {
		out.Class += " out"
	}
	if day.Today {
		out.Class += " today"
	}
	label := day.AriaLabel
	if label == "" {
		label = l.T("transactions.calDayLabel", out.DayText, day.NetStr)
	}
	if !day.InMonth && day.MonthName != "" {
		label = l.T("transactions.calDayOtherMonth", out.DayText, day.MonthName)
	}
	aria := make([]string, 0, len(day.Ghosts))
	out.Ghosts = make([]GhostSummary, 0, len(day.Ghosts))
	for _, ghost := range day.Ghosts {
		summary := l.GhostSummary(ghost, day.Rates)
		out.Ghosts = append(out.Ghosts, summary)
		aria = append(aria, summary.Aria)
	}
	if len(aria) > 0 {
		label = l.T("transactions.calDayWithScheduled", label, strings.Join(aria, "; "))
	}
	out.Aria = label
	return out
}
