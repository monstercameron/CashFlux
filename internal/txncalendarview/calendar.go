// SPDX-License-Identifier: MIT

// Package txncalendarview prepares the transaction calendar's scope, labels and
// display decisions without depending on a browser or component framework.
package txncalendarview

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnscope"
)

// Labels supplies the existing catalog and shared money formatter. Both callbacks
// must be deterministic; keeping them at the boundary preserves the app's exact
// localized copy and ledger convention without importing its UI package.
type Labels struct {
	T      func(string, ...any) string
	Ledger func(money.Money, bool) (string, string)
	Prefs  prefs.Prefs
	// ScopePrefs preserves the caption's persisted date style independently of live preferences.
	ScopePrefs prefs.Prefs
}

// Input is a read-only snapshot for one calendar projection. Filter is the raw
// ledger filter; MemberLens is applied to recurring and liability definitions.
type Input struct {
	Now                 time.Time
	WeekStart           time.Weekday
	Filter              txnfilter.Criteria
	MemberLens          []string
	Shown, Transactions []domain.Transaction
	Recurring           []domain.Recurring
	Accounts            []domain.Account
	Settled             map[string]bool
	Rates               currency.Rates
}

// Month contains ready-to-render calendar rows and its original projection error.
type Month struct {
	Caption       string
	Weeks         []Week
	ShowScheduled bool
	RateIssues    int
	Err           error
}

// Week contains ready-to-render days and its summary inputs.
type Week struct {
	Days  []Day
	Total WeekInput
}

// Day contains a calendar button's data, before its event handler is attached.
type Day struct {
	DateKey                              string
	Day                                  int
	InMonth                              bool
	MonthName                            string
	Today                                bool
	NetStr, NetTone, MovedStr, AriaLabel string
	Count                                int
	Ghosts                               []txncalendar.Ghost
	Rates                                currency.Rates
}

// Build projects the ledger and scheduled definitions into the same visible month.
// It copies settlement flags before combining them, so nil and shared maps are safe.
func (l Labels) Build(in Input) Month {
	now := dateutil.WallClockUTC(in.Now)
	scope := txnscope.Of(in.Filter.From, in.Filter.To, now)
	effective, _ := in.Filter.WithOwnerLens(in.MemberLens)
	recurring := txncalendar.ScopeRecurring(in.Recurring, effective, in.Accounts)
	var accounts []domain.Account
	var projection txncalendar.CashflowMonth
	var err error
	if strings.TrimSpace(effective.Categories) == "" && strings.TrimSpace(effective.Category) == "" {
		accounts = txncalendar.ScopeLiabilityAccounts(in.Accounts, effective)
		projection, err = txncalendar.MonthCashflowWithAccounts(scope.Anchor, in.WeekStart, in.Shown, recurring, accounts, in.Rates)
	} else {
		projection, err = txncalendar.MonthCashflow(scope.Anchor, in.WeekStart, in.Shown, recurring, in.Rates)
	}
	out := Month{Caption: l.Caption(scope), ShowScheduled: strings.TrimSpace(in.Filter.Text) == "", RateIssues: len(projection.RateIssues), Err: err}
	weeks := projection.Weeks
	if out.ShowScheduled {
		settled := make(map[string]bool, len(in.Settled))
		for key, value := range in.Settled {
			settled[key] = value
		}
		for key := range txncalendar.SettledLiabilityOccurrences(weeks, in.Transactions, accounts, now) {
			settled[key] = true
		}
		weeks = txncalendar.FoldSettled(weeks, settled, now)
	}
	out.Weeks = make([]Week, 0, len(weeks))
	for _, week := range weeks {
		row := Week{Days: make([]Day, 0, len(week.Days))}
		scheduled := 0
		for _, day := range week.Days {
			ghosts := ScheduledGhosts(day.Ghosts, out.ShowScheduled)
			scheduled += len(ghosts)
			row.Days = append(row.Days, Day{
				DateKey: txncalendar.DayKey(day.Date), Day: day.Date.Day(), InMonth: day.InMonth,
				MonthName: l.MonthYear(day.Date), Today: txncalendar.DayKey(day.Date) == txncalendar.DayKey(now),
				NetStr: l.DayNet(day.Cashflow, in.Rates.Base), NetTone: NetTone(day.Cashflow.Net),
				MovedStr: l.Moved(day.Cashflow.Moved, in.Rates.Base), AriaLabel: l.DayAria(day.Date.Day(), day.Cashflow, in.Rates.Base),
				Count: day.Cashflow.Count, Ghosts: ghosts, Rates: in.Rates,
			})
		}
		first := week.Days[0].Date // txncalendar guarantees seven cells per week.
		row.Total = WeekInput{Cashflow: week.Total, Base: in.Rates.Base, Scheduled: scheduled,
			WeekOf: l.Prefs.FormatDate(first), Future: dateutil.WeekStart(first, in.WeekStart).After(dateutil.WeekStart(now, in.WeekStart)),
			SummaryID: "txn-cal-week-" + txncalendar.DayKey(first)}
		out.Weeks = append(out.Weeks, row)
	}
	return out
}

// Caption states the ledger scope and, when needed, the grid's month as well.
func (l Labels) Caption(scope txnscope.Scope) string {
	var caption string
	switch scope.Kind {
	case txnscope.All:
		caption = l.T("txnperiod.allLabel")
	case txnscope.Month:
		caption = scope.Anchor.Format("January 2006")
	case txnscope.Day:
		caption = l.T("txnperiod.oneDay", l.ScopePrefs.FormatDate(scope.From))
	default:
		from, to := "…", "…"
		if !scope.From.IsZero() {
			from = l.ScopePrefs.FormatDate(scope.From)
		}
		if !scope.To.IsZero() {
			to = l.ScopePrefs.FormatDate(scope.To)
		}
		caption = l.T("txnperiod.customRange", from, to)
	}
	if scope.Kind != txnscope.Month {
		caption = l.T("transactions.calShowingMonth", caption, l.MonthYear(scope.Anchor))
	}
	return caption
}

// PickNotice describes the day-filter transition using the previous ledger scope.
func (l Labels) PickNotice(dateKey, from, to string) string {
	day := dateKey
	if d, err := dateutil.ParseDate(dateKey); err == nil {
		day = l.Prefs.FormatDate(d)
	}
	if from == "" && to == "" {
		return l.T("transactions.calPickedFromAll", day)
	}
	return l.T("transactions.calPicked", day)
}

// ScheduledGhosts hides scheduled definitions during text searches.
func ScheduledGhosts(ghosts []txncalendar.Ghost, visible bool) []txncalendar.Ghost {
	if !visible {
		return nil
	}
	return ghosts
}

// Balance returns a register balance's accounting text and negative-only tone.
func Balance(balances map[string]money.Money, id string) (text, tone string) {
	m, ok := balances[id]
	if !ok {
		return "", ""
	}
	text = money.FormatAccounting(m.Amount, currency.Decimals(m.Currency), currency.Symbol(m.Currency))
	if m.IsNegative() {
		tone = "text-down"
	}
	return text, tone
}

// Net formats an active day's signed net, leaving inactive days blank.
func (l Labels) Net(stat txncalendar.DayStat, base string) string {
	if stat.Count == 0 {
		return ""
	}
	text, _ := l.Ledger(money.New(stat.Net, base), false)
	return text
}

// DayNet hides net figures for transfer-only and inactive days.
func (l Labels) DayNet(cash txncalendar.Cashflow, base string) string {
	if cash.Count == 0 || (cash.Spent == 0 && cash.Income == 0) {
		return ""
	}
	return l.Net(txncalendar.DayStat{Net: cash.Net, Count: cash.Count}, base)
}

// Moved formats positive transfer magnitudes with the existing localized label.
func (l Labels) Moved(amount int64, base string) string {
	if amount <= 0 {
		return ""
	}
	return l.T("transactions.tq3.dayMoved", l.unsigned(amount, base))
}

// NetTone returns the existing signed cash-flow tone class.
func NetTone(net int64) string {
	if net > 0 {
		return "text-up"
	}
	if net < 0 {
		return "text-down"
	}
	return ""
}

func (l Labels) unsigned(amount int64, base string) string {
	text, _ := l.Ledger(money.New(amount, base), false)
	return strings.TrimPrefix(text, "+")
}

// DayAria describes every cash-flow category, including zero figures.
func (l Labels) DayAria(day int, cash txncalendar.Cashflow, base string) string {
	if cash.Count == 0 {
		return l.T("transactions.tq3.dayEmpty", strconv.Itoa(day))
	}
	count := l.T("transactions.tq3.dayMany", cash.Count)
	if cash.Count == 1 {
		count = l.T("transactions.tq3.dayOne")
	}
	return l.T("transactions.tq3.dayCashflow", strconv.Itoa(day), l.unsigned(cash.Spent, base), l.unsigned(cash.Income, base), l.unsigned(cash.Moved, base), count)
}

// MonthYear uses the existing full-month catalog keys and literal year separator.
func (l Labels) MonthYear(t time.Time) string {
	keys := [...]string{"january", "february", "march", "april", "mayFull", "june", "july", "august", "september", "october", "november", "december"}
	return l.T("budgets.month."+keys[int(t.Month())-1]) + " " + strconv.Itoa(t.Year())
}

// Weekday returns the existing short or full weekday catalog label.
func (l Labels) Weekday(day time.Weekday, full bool) string {
	keys := [...]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
	suffix := ""
	if full {
		suffix = ".full"
	}
	return l.T("transactions.tq3.weekday." + keys[int(day)] + suffix)
}

// Header is a weekday column's short label and accessible full name.
type Header struct{ Text, Aria string }

// Headers returns seven weekday columns starting on the chosen day.
func (l Labels) Headers(start time.Weekday) []Header {
	headers := make([]Header, 7)
	for i := range headers {
		day := time.Weekday((int(start) + i) % 7)
		headers[i] = Header{Text: l.Weekday(day, false), Aria: l.Weekday(day, true)}
	}
	return headers
}

// DotCount maps transaction density to at most five dots without overflowing.
func DotCount(count int) int {
	if count <= 0 {
		return 0
	}
	dots := 1 + (count-1)/5
	if dots > 5 {
		return 5
	}
	return dots
}

// DaySelection describes the existing transition back to a single-day ledger.
type DaySelection struct {
	DateKey, ViewMode, Notice string
}

// PickDay prepares a day selection and its notice from the previous scope.
func (l Labels) PickDay(dateKey, from, to string) DaySelection {
	return DaySelection{DateKey: dateKey, ViewMode: "table", Notice: l.PickNotice(dateKey, from, to)}
}

// Apply changes only the date range of the latest filter. Other criteria,
// including a previously chosen preset, retain the calendar's existing behavior.
func (selection DaySelection) Apply(filter txnfilter.Criteria) txnfilter.Criteria {
	filter.From, filter.To = selection.DateKey, selection.DateKey
	return filter
}
