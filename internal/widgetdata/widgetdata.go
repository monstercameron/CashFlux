// SPDX-License-Identifier: MIT

// Package widgetdata is the pure data logic behind custom-page widgets: turning a
// widget's binding + the dataset into the rows a List shows, the formatted text a
// KPI shows, and the month window a Chart plots. It has no platform or UI
// dependencies, so the real logic a user sees — newest-first ordering, row caps,
// money/percent/currency formatting — unit-tests on native Go. The wasm renderers
// in internal/screens are thin shells over these functions.
package widgetdata

import (
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/widgetspec"
)

// DefaultListRows is how many rows a List widget shows by default.
const DefaultListRows = 5

// Row is one label/value pair a List widget renders.
type Row struct {
	Label string
	Value string
}

// Data is the dataset a List widget reads from, plus the rate table used by any
// money aggregation.
type Data struct {
	Transactions []domain.Transaction
	Accounts     []domain.Account
	Budgets      []domain.Budget
	Goals        []domain.Goal
	Tasks        []domain.Task
	Recurring    []domain.Recurring // for SourceBills (L63 GAP-A)
	Rates        currency.Rates
	Now          time.Time // reference time for SourceBills due-date calculation
}

// fmtMoney renders money the same way the rest of the app does (accounting style:
// grouped, symbol, parentheses for negatives). Kept here so list rows format
// identically to every other figure without depending on the wasm UI layer.
func fmtMoney(m money.Money) string {
	return money.FormatAccounting(m.Amount, currency.Decimals(m.Currency), currency.Symbol(m.Currency))
}

// ListRows returns up to n rows for the given source. ok is false for an unknown
// source (the widget should prompt the user to pick one). n <= 0 falls back to the
// default. Transactions are returned newest-first; the input slices are not
// mutated. Deterministic for a given Data.
func ListRows(source string, d Data, n int) (rows []Row, ok bool) {
	if n <= 0 {
		n = DefaultListRows
	}
	switch source {
	case widgetspec.SourceTransactions:
		txns := append([]domain.Transaction(nil), d.Transactions...)
		sort.SliceStable(txns, func(i, j int) bool { return txns[i].Date.After(txns[j].Date) })
		for i, t := range txns {
			if i >= n {
				break
			}
			rows = append(rows, Row{Label: firstNonEmpty(t.Desc, t.Payee), Value: fmtMoney(t.Amount)})
		}
	case widgetspec.SourceAccounts:
		for i, a := range d.Accounts {
			if i >= n {
				break
			}
			bal, _ := ledger.Balance(a, d.Transactions)
			rows = append(rows, Row{Label: a.Name, Value: fmtMoney(bal)})
		}
	case widgetspec.SourceBudgets:
		for i, b := range d.Budgets {
			if i >= n {
				break
			}
			rows = append(rows, Row{Label: b.Name, Value: fmtMoney(b.Limit)})
		}
	case widgetspec.SourceGoals:
		for i, g := range d.Goals {
			if i >= n {
				break
			}
			rows = append(rows, Row{Label: g.Name, Value: strconv.Itoa(goals.Percent(g)) + "%"})
		}
	case widgetspec.SourceTasks:
		for i, tk := range d.Tasks {
			if i >= n {
				break
			}
			rows = append(rows, Row{Label: tk.Title, Value: string(tk.Status)})
		}
	case widgetspec.SourceBills:
		// Upcoming bills: account-due-day bills + negative recurring items, soonest
		// first. "Due today" / "in N days" / "due <date>" as the value. Uses the
		// Data.Now reference time; falls back to the real clock when zero.
		now := d.Now
		if now.IsZero() {
			now = time.Now()
		}
		upcoming := bills.UpcomingAll(d.Accounts, d.Recurring, now)
		for i, b := range upcoming {
			if i >= n {
				break
			}
			var when string
			switch b.DaysUntil {
			case 0:
				when = "due today"
			case 1:
				when = "due tomorrow"
			default:
				when = "due in " + strconv.Itoa(b.DaysUntil) + " days"
			}
			rows = append(rows, Row{Label: b.Name, Value: when})
		}
	default:
		return nil, false
	}
	return rows, true
}

// KPIText formats a KPI's numeric value for display per its format: currency
// renders in the base currency (rounded to the nearest minor unit — a bare
// truncation could drop a cent), otherwise number/percent via widgetspec.Format.
func KPIText(value float64, format, base string) string {
	if format == widgetspec.FormatCurrency {
		div := minorPerMajor(base)
		return fmtMoney(money.New(int64(math.Round(value*div)), base))
	}
	return widgetspec.Format(value, format)
}

// ChartWindow returns the month-start cutoffs for a `months`-long net-worth trend
// ending at the month after `now` (matching the dashboard trend). Pure given now.
func ChartWindow(now time.Time, months int) []time.Time {
	if months < 1 {
		months = 1
	}
	start := dateutil.MonthStart(now)
	cutoffs := make([]time.Time, 0, months)
	for i := 0; i < months; i++ {
		cutoffs = append(cutoffs, dateutil.AddMonths(start, i-(months-2)))
	}
	return cutoffs
}

func minorPerMajor(cur string) float64 {
	div := 1.0
	for i := 0; i < currency.Decimals(cur); i++ {
		div *= 10
	}
	return div
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
