// SPDX-License-Identifier: MIT

// Package txncalendar buckets transactions into calendar days and projects
// recurring cash flows forward as "ghost" occurrences, so the /transactions
// calendar view mode can render a month grid whose cells show each day's net
// amount, transaction-count density, and the bills due that day.
//
// It is a pure PROJECTION of whatever set of transactions it is handed — the
// caller passes the already-filtered ledger, so active filter chips continue to
// scope the calendar exactly as they scope the table. Pure Go, no syscall/js;
// unit-tested on native Go.
package txncalendar

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// Cashflow contains converted cash-flow figures for a day or calendar week.
// Spent, Income, and Moved are non-negative minor-unit amounts in the base
// currency; Count includes every transaction, including transfers.
type Cashflow struct {
	Spent  int64
	Income int64
	Moved  int64
	Count  int
	Net    int64
}

// CashflowCell pairs a calendar day with its cash-flow figures and recurring
// markers. Padding days have zero figures.
type CashflowCell struct {
	Date     time.Time
	InMonth  bool
	Cashflow Cashflow
	Ghosts   []Ghost
}

// CashflowWeek contains seven calendar cells and their checked total.
type CashflowWeek struct {
	Days  []CashflowCell
	Total Cashflow
}

// RateIssue identifies a transaction whose amount could not be converted to
// the selected base currency. Its count remains represented in the calendar.
type RateIssue struct {
	TransactionID string
	Date          time.Time
	Currency      string
	Err           error
}

// CashflowMonth is the month calendar projection. RateIssues makes any partial
// subtotal explicit instead of silently dropping unconvertible transactions.
type CashflowMonth struct {
	Weeks      []CashflowWeek
	RateIssues []RateIssue
}

// MonthCashflow folds transactions once into day cash-flow cells, then derives
// week totals from those cells. Transfers contribute to Moved, never Spent or
// Income. Amounts use rates.Base and every sum is checked for overflow.
func MonthCashflow(anchor time.Time, weekStart time.Weekday, txns []domain.Transaction, recurring []domain.Recurring, rates currency.Rates) (CashflowMonth, error) {
	monthStart := dateutil.MonthStart(anchor)
	monthEnd := dateutil.AddMonths(monthStart, 1)
	grid := monthGrid(anchor, weekStart, recurring)
	byDay := make(map[string]Cashflow, len(txns))
	transferRows := txnclassify.SummarizeVisibleTransfers(txns, rates).CountedRows
	result := CashflowMonth{Weeks: make([]CashflowWeek, len(grid))}
	for i, txn := range txns {
		if txn.Date.Before(monthStart) || !txn.Date.Before(monthEnd) {
			continue
		}
		key := DayKey(txn.Date)
		day := byDay[key]
		if day.Count == math.MaxInt {
			return CashflowMonth{}, fmt.Errorf("count transactions on %s: %w", key, money.ErrOverflow)
		}
		day.Count++
		if txn.ExcludeFromReports || (txn.IsTransfer() && !transferRows[i]) {
			byDay[key] = day
			continue
		}
		converted, err := rates.Convert(txn.Amount, rates.Base)
		if err != nil {
			result.RateIssues = append(result.RateIssues, RateIssue{TransactionID: txn.ID, Date: txn.Date, Currency: txn.Amount.Currency, Err: err})
			byDay[key] = day
			continue
		}
		var amount int64
		switch {
		case txn.IsTransfer():
			if !transferRows[i] {
				byDay[key] = day
				continue
			}
			amount, err = money.Abs(converted.Amount)
			if err == nil {
				day.Moved, err = money.Add(day.Moved, amount)
			}
		case converted.Amount < 0:
			amount, err = money.Neg(converted.Amount)
			if err == nil {
				day.Spent, err = money.Add(day.Spent, amount)
			}
		case converted.Amount > 0:
			day.Income, err = money.Add(day.Income, converted.Amount)
		}
		if err != nil {
			return CashflowMonth{}, fmt.Errorf("sum transaction %q on %s: %w", txn.ID, key, err)
		}
		byDay[key] = day
	}
	for wi, week := range grid {
		out := CashflowWeek{Days: make([]CashflowCell, len(week))}
		for di, cell := range week {
			cash := Cashflow{}
			if cell.InMonth {
				cash = byDay[DayKey(cell.Date)]
				var err error
				cash.Net, err = money.Sub(cash.Income, cash.Spent)
				if err != nil {
					return CashflowMonth{}, fmt.Errorf("calculate net cash flow on %s: %w", DayKey(cell.Date), err)
				}
			}
			out.Days[di] = CashflowCell{Date: cell.Date, InMonth: cell.InMonth, Cashflow: cash, Ghosts: cell.Ghosts}
			if err := addCashflow(&out.Total, cash); err != nil {
				return CashflowMonth{}, fmt.Errorf("sum week %s: %w", DayKey(week[0].Date), err)
			}
		}
		result.Weeks[wi] = out
	}
	return result, nil
}

// MonthCashflowWithAccounts projects recurring rules and liability statement
// payments into the calendar. Liability payments already represented by a
// matching monthly recurring rule are omitted by bills.OccurrencesWithin, so a
// due item appears once even when both sources describe it.
func MonthCashflowWithAccounts(anchor time.Time, weekStart time.Weekday, txns []domain.Transaction, recurring []domain.Recurring, accounts []domain.Account, rates currency.Rates) (CashflowMonth, error) {
	result, err := MonthCashflow(anchor, weekStart, txns, recurring, rates)
	if err != nil {
		return CashflowMonth{}, err
	}
	start := dateutil.MonthStart(anchor)
	end := dateutil.AddMonths(start, 1).AddDate(0, 0, -1)
	liabilityBills := bills.OccurrencesWithin(accounts, recurring, start, end)
	byDay := make(map[string][]Ghost)
	for _, bill := range liabilityBills {
		if _, recurringBill := bills.RecurringIDFromAccount(bill.AccountID); recurringBill {
			continue
		}
		amount, err := money.Neg(bill.Amount.Amount)
		if err != nil {
			return CashflowMonth{}, fmt.Errorf("project liability payment %q: %w", bill.AccountID, err)
		}
		byDay[DayKey(bill.DueDate)] = append(byDay[DayKey(bill.DueDate)], Ghost{
			RecurringID: "account:" + bill.AccountID,
			Label:       bill.Name,
			Amount:      amount,
			Currency:    bill.Amount.Currency,
			Date:        bill.DueDate,
			Liability:   true,
		})
	}
	for wi := range result.Weeks {
		for di := range result.Weeks[wi].Days {
			day := &result.Weeks[wi].Days[di]
			if day.InMonth {
				day.Ghosts = append(day.Ghosts, byDay[DayKey(day.Date)]...)
			}
		}
	}
	return result, nil
}

func addCashflow(total *Cashflow, day Cashflow) error {
	var err error
	if total.Spent, err = money.Add(total.Spent, day.Spent); err != nil {
		return fmt.Errorf("spent: %w", err)
	}
	if total.Income, err = money.Add(total.Income, day.Income); err != nil {
		return fmt.Errorf("income: %w", err)
	}
	if total.Moved, err = money.Add(total.Moved, day.Moved); err != nil {
		return fmt.Errorf("moved: %w", err)
	}
	if total.Net, err = money.Add(total.Net, day.Net); err != nil {
		return fmt.Errorf("net: %w", err)
	}
	if day.Count > math.MaxInt-total.Count {
		return fmt.Errorf("count: %w", money.ErrOverflow)
	}
	total.Count += day.Count
	return nil
}

// DayKey returns the canonical calendar-day key (YYYY-MM-DD, UTC) used to bucket
// a transaction or a ghost onto a day. Transaction dates are UTC-midnight
// calendar dates (dateutil), so bucketing on the UTC date keeps a day cell in
// step with the ledger row's rendered date.
func DayKey(t time.Time) string {
	return dateutil.CalendarDayUTC(t).Format(dateutil.Layout)
}

// DayStat aggregates the transactions that fall on one calendar day: the signed
// net amount in minor units (income positive, spend negative) and the count of
// transactions (the dot-density source).
type DayStat struct {
	Net   int64
	Count int
}

// ScopeRecurring applies the account, category, and member dimensions that can
// be represented by recurring definitions. Text, tag, source, and amount filters
// have no matching fields on Recurring and therefore do not alter the projection.
func ScopeRecurring(recurring []domain.Recurring, criteria txnfilter.Criteria, accounts []domain.Account) []domain.Recurring {
	accountIDs := splitScope(criteria.Accounts, criteria.Account)
	categoryIDs := splitScope(criteria.Categories, criteria.Category)
	memberIDs := splitScope(criteria.Members, criteria.Member)

	accountOwners := make(map[string]bool, len(accounts))
	for _, account := range accounts {
		ownerMatches := len(memberIDs) == 0
		if len(memberIDs) > 0 {
			if len(account.OwnershipShares) > 0 {
				for owner := range account.OwnershipShares {
					if contains(memberIDs, owner) {
						ownerMatches = true
						break
					}
				}
			} else {
				ownerMatches = contains(memberIDs, account.OwnerID)
			}
		}
		accountOwners[account.ID] = ownerMatches
	}

	filtered := make([]domain.Recurring, 0, len(recurring))
	for _, item := range recurring {
		if !matchesScope(accountIDs, item.AccountID) || !matchesScope(categoryIDs, item.CategoryID) {
			continue
		}
		if len(memberIDs) > 0 && !accountOwners[item.AccountID] {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

// ScopeLiabilityAccounts returns liability accounts represented by the active
// account and member filters. Category filters are handled by the recurring
// rules that carry a category; inferred account statements have no category.
func ScopeLiabilityAccounts(accounts []domain.Account, criteria txnfilter.Criteria) []domain.Account {
	accountIDs := splitScope(criteria.Accounts, criteria.Account)
	memberIDs := splitScope(criteria.Members, criteria.Member)
	filtered := make([]domain.Account, 0, len(accounts))
	for _, account := range accounts {
		if account.Archived || account.Class != domain.ClassLiability || !matchesScope(accountIDs, account.ID) {
			continue
		}
		if len(memberIDs) > 0 {
			matchesOwner := false
			if len(account.OwnershipShares) > 0 {
				for owner := range account.OwnershipShares {
					if contains(memberIDs, owner) {
						matchesOwner = true
						break
					}
				}
			} else {
				matchesOwner = contains(memberIDs, account.OwnerID)
			}
			if !matchesOwner {
				continue
			}
		}
		filtered = append(filtered, account)
	}
	return filtered
}

func splitScope(multi, single string) []string {
	value := single
	if strings.TrimSpace(multi) != "" {
		value = multi
	}
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if id := strings.TrimSpace(part); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func matchesScope(ids []string, value string) bool {
	return len(ids) == 0 || contains(ids, value)
}

func contains(ids []string, value string) bool {
	for _, id := range ids {
		if id == value {
			return true
		}
	}
	return false
}

// Ghost is a projected recurring occurrence on a due date — a dimmed, read-only
// entry in a day cell that names the recurring and its signed amount. It carries
// no interaction beyond a title/tooltip.
type Ghost struct {
	RecurringID string
	Label       string
	Amount      int64 // signed minor units (negative = money out)
	Currency    string
	Date        time.Time
	NotPosted   bool
	Liability   bool
}

// FoldSettled removes scheduled markers with a durable bill-match and marks
// unpaid occurrences before asOf as not posted yet. The caller supplies the
// settlement state and date so the fold remains deterministic and testable.
func FoldSettled(weeks []CashflowWeek, settled map[string]bool, asOf time.Time) []CashflowWeek {
	today := DayKey(asOf)
	folded := make([]CashflowWeek, len(weeks))
	for wi, week := range weeks {
		folded[wi] = week
		folded[wi].Days = make([]CashflowCell, len(week.Days))
		for di, day := range week.Days {
			folded[wi].Days[di] = day
			folded[wi].Days[di].Ghosts = make([]Ghost, 0, len(day.Ghosts))
			for _, ghost := range day.Ghosts {
				if settled[domain.OccurrenceKey(ghost.RecurringID, ghost.Date)] {
					continue
				}
				ghost.NotPosted = DayKey(ghost.Date) < today
				folded[wi].Days[di].Ghosts = append(folded[wi].Days[di].Ghosts, ghost)
			}
		}
	}
	return folded
}

const liabilitySettlementWindowDays = 31

// SettledLiabilityOccurrences finds calendar statement occurrences settled by
// a linked bill payment or a signed transfer into that liability account. A
// payment applies only to an occurrence due within the preceding 31 calendar
// days; a payment after the due date never rewrites that occurrence's history.
// The supplied weeks and accounts must already be scoped to the active view.
func SettledLiabilityOccurrences(weeks []CashflowWeek, txns []domain.Transaction, accounts []domain.Account, asOf time.Time) map[string]bool {
	accountByID := make(map[string]domain.Account, len(accounts))
	for _, account := range accounts {
		if account.IsLiability() {
			accountByID[account.ID] = account
		}
	}

	type occurrence struct {
		accountID string
		due       time.Time
	}
	var occurrences []occurrence
	for _, week := range weeks {
		for _, day := range week.Days {
			for _, ghost := range day.Ghosts {
				if !ghost.Liability || !strings.HasPrefix(ghost.RecurringID, "account:") {
					continue
				}
				accountID := strings.TrimPrefix(ghost.RecurringID, "account:")
				if _, ok := accountByID[accountID]; ok {
					occurrences = append(occurrences, occurrence{accountID: accountID, due: dateutil.CalendarDayUTC(ghost.Date)})
				}
			}
		}
	}

	asOfDay := dateutil.CalendarDayUTC(asOf)
	settled := make(map[string]bool)
	for _, txn := range txns {
		paymentDay := dateutil.CalendarDayUTC(txn.Date)
		if paymentDay.After(asOfDay) {
			continue
		}
		for _, item := range occurrences {
			if paymentDay.Before(item.due.AddDate(0, 0, -liabilitySettlementWindowDays)) || paymentDay.After(item.due) {
				continue
			}
			account := accountByID[item.accountID]
			linkedBillPayment := txn.BillAccountID == item.accountID
			paymentIntoAccount := txn.AccountID == item.accountID && inflowAmount(txn.Amount.Amount, account.InflowSign())
			if linkedBillPayment || paymentIntoAccount {
				settled[domain.OccurrenceKey("account:"+item.accountID, item.due)] = true
			}
		}
	}
	return settled
}

func inflowAmount(amount, inflowSign int64) bool {
	if inflowSign < 0 {
		return amount < 0
	}
	return amount > 0
}

// Ghosts projects every recurring's occurrences into the half-open range
// [from, to), stepping from each recurring's NextDue by its cadence. A recurring
// whose NextDue is zero (never scheduled) or before the range is jumped forward
// arithmetically to the first due date in the window. Paused recurrings are not
// active cash flows and are omitted.
func Ghosts(recurring []domain.Recurring, from, to time.Time) []Ghost {
	var out []Ghost
	for _, r := range recurring {
		if !r.Active() || r.NextDue.IsZero() || !to.After(from) {
			continue
		}
		due := jumpToRange(r.Cadence, r.NextDue, from)
		for due.Before(to) {
			out = append(out, Ghost{
				RecurringID: r.ID,
				Label:       r.Label,
				Amount:      r.Amount.Amount,
				Currency:    r.Amount.Currency,
				Date:        due,
			})
			next := r.Cadence.Next(due)
			if !next.After(due) {
				break
			}
			due = next
		}
	}
	return out
}

func jumpToRange(cadence domain.RecurringCadence, due, from time.Time) time.Time {
	if !due.Before(from) {
		return due
	}
	switch cadence {
	case domain.CadenceDaily, domain.CadenceWeekly, domain.CadenceBiweekly:
		stepDays := int64(1)
		if cadence == domain.CadenceWeekly {
			stepDays = 7
		} else if cadence == domain.CadenceBiweekly {
			stepDays = 14
		}
		dayDelta := calendarDayOrdinal(from) - calendarDayOrdinal(due)
		steps := dayDelta / stepDays
		if dayDelta%stepDays != 0 {
			steps++
		}
		return due.AddDate(0, 0, int(steps*stepDays))
	case domain.CadenceSemimonthly:
		if due.Day() != 1 && due.Day() != 15 {
			due = cadence.Next(due)
		}
		dueIndex := (due.Year()*12 + int(due.Month()) - 1) * 2
		if due.Day() == 15 {
			dueIndex++
		}
		fromIndex := (from.Year()*12 + int(from.Month()) - 1) * 2
		switch {
		case from.Day() <= 1:
		case from.Day() <= 15:
			fromIndex++
		default:
			fromIndex += 2
		}
		if fromIndex > dueIndex {
			monthIndex := fromIndex / 2
			day := 1
			if fromIndex%2 == 1 {
				day = 15
			}
			due = time.Date(monthIndex/12, time.Month(monthIndex%12+1), day, due.Hour(), due.Minute(), due.Second(), due.Nanosecond(), due.Location())
		}
	case domain.CadenceQuarterly:
		due = jumpMonthCadence(cadence, due, from, 3)
	case domain.CadenceYearly:
		due = jumpMonthCadence(cadence, due, from, 12)
	default: // monthly, including unknown legacy cadence strings
		due = jumpMonthCadence(cadence, due, from, 1)
	}
	for due.Before(from) {
		next := cadence.Next(due)
		if !next.After(due) {
			return time.Time{}
		}
		due = next
	}
	return due
}

func jumpMonthCadence(cadence domain.RecurringCadence, due, from time.Time, monthsPerStep int) time.Time {
	// Go's AddDate normalizes days beyond the target month's length. One real
	// cadence step first preserves that behavior (and stabilizes dates such as
	// Jan 31 -> Mar 3); the remaining jump is then equivalent to repeated steps.
	if due.Day() > 28 {
		due = cadence.Next(due)
	}
	months := (from.Year()-due.Year())*12 + int(from.Month()-due.Month())
	steps := months / monthsPerStep
	if steps > 0 {
		due = due.AddDate(0, steps*monthsPerStep, 0)
	}
	return due
}

func calendarDayOrdinal(t time.Time) int64 {
	return dateutil.CalendarDayUTC(t).Unix() / int64(24*time.Hour/time.Second)
}

// calendarCell is one day square in a month grid. Padding days belong to an
// adjacent month and carry no recurring markers.
type calendarCell struct {
	Date    time.Time
	InMonth bool
	Ghosts  []Ghost
}

func monthGrid(anchor time.Time, weekStart time.Weekday, recurring []domain.Recurring) [][]calendarCell {
	monthStart := dateutil.MonthStart(anchor)
	monthEnd := dateutil.AddMonths(monthStart, 1) // exclusive

	gridStart := dateutil.WeekStart(monthStart, weekStart)
	// Last day of the month, then the end of its week (exclusive grid end).
	lastDay := monthEnd.AddDate(0, 0, -1)
	gridEnd := dateutil.WeekStart(lastDay, weekStart).AddDate(0, 0, 7)

	ghosts := Ghosts(recurring, monthStart, monthEnd)
	ghostsByDay := make(map[string][]Ghost, len(ghosts))
	for _, g := range ghosts {
		k := DayKey(g.Date)
		ghostsByDay[k] = append(ghostsByDay[k], g)
	}

	var weeks [][]calendarCell
	for day := gridStart; day.Before(gridEnd); {
		week := make([]calendarCell, 0, 7)
		for i := 0; i < 7; i++ {
			k := DayKey(day)
			inMonth := !day.Before(monthStart) && day.Before(monthEnd)
			cell := calendarCell{Date: day, InMonth: inMonth}
			if inMonth {
				cell.Ghosts = ghostsByDay[k]
			}
			week = append(week, cell)
			day = day.AddDate(0, 0, 1)
		}
		weeks = append(weeks, week)
	}
	return weeks
}
