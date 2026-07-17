// SPDX-License-Identifier: MIT

// Package widgetsource is the engine's data layer for collection/visualization
// widgets: pure resolvers that reduce the fundamental data (transactions,
// accounts, budgets, goals, bills) into RICH, typed domain.Frames a widget renders
// from — status bars, signed-balance grids, and charts (as label/value columns).
//
// "Rich" means the Frame carries everything the visualization needs without the
// renderer recomputing it: money columns hold minor-unit int64 (+ a currency
// column where rows differ), percent columns hold the used-%, and a tone column
// carries the ok/near/over (or up/down) state so a bar/row can be colored straight
// from the data. This lets the dashboard's rich widgets be data-driven through the
// engine instead of computing inline. Pure Go, no syscall/js — unit-tested.
package widgetsource

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// BudgetStatus produces a Frame for the Budgets widget: one row per budget with
// columns name (string), percent (percent), state (tone: ok|near|over), over
// (bool). Parent budgets roll up sub-categories. atRisk drops on-track budgets;
// limit caps rows (0 = no cap). The tone column lets the renderer color the bar
// straight from the data.
func BudgetStatus(budgets []domain.Budget, cats []domain.Category, txns []domain.Transaction, rates currency.Rates, start, end time.Time, atRisk bool, limit int) domain.Frame {
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}
	var names, percents, states, over []any
	for _, b := range budgets {
		st, err := budgeting.EvaluateRollup(b, txns, start, end, rates, budgeting.DefaultNearThreshold, categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs()))
		if err != nil {
			continue
		}
		if atRisk && st.State != budgeting.StateNear && st.State != budgeting.StateOver {
			continue
		}
		if limit > 0 && len(names) >= limit {
			break
		}
		label := st.Budget.Name
		if label == "" {
			label = catName[st.Budget.CategoryID]
		}
		names = append(names, label)
		percents = append(percents, float64(st.Percent))
		states = append(states, stateTone(st.State))
		over = append(over, st.State == budgeting.StateOver)
	}
	return domain.NewFrame(
		domain.Field{Name: "name", Type: domain.FieldString, Values: names},
		domain.Field{Name: "percent", Type: domain.FieldPercent, Values: percents},
		domain.Field{Name: "state", Type: domain.FieldTone, Values: states},
		domain.Field{Name: "over", Type: domain.FieldBool, Values: over},
	)
}

// stateTone maps a budgeting.State to a Frame tone token.
func stateTone(s budgeting.State) string {
	switch s {
	case budgeting.StateOver:
		return "over"
	case budgeting.StateNear:
		return "near"
	}
	return "ok"
}

// AccountBalances produces a Frame for the Accounts widget: one row per
// non-archived account with columns name (string), balance (money minor units),
// currency (string, since accounts may differ) and tone (down when negative).
// cleared selects the cleared balance. Accounts whose balance can't be computed
// (FX mismatch) are skipped. limit caps rows (0 = no cap).
func AccountBalances(accounts []domain.Account, txns []domain.Transaction, cleared bool, limit int) domain.Frame {
	var names, balances, currencies, tones []any
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		if limit > 0 && len(names) >= limit {
			break
		}
		var bal money.Money
		var err error
		if cleared {
			bal, err = ledger.ClearedBalance(a, txns)
		} else {
			bal, err = ledger.Balance(a, txns)
		}
		if err != nil {
			continue
		}
		// A liability presents as the owed magnitude, negative — accounting parens
		// + down tone — regardless of its at-rest sign convention (the sample data
		// stores debts negative; the "amount you owe" add form stores them
		// positive). A positive-stored $550 loan used to render as a healthy
		// "$550.00" here while /accounts showed "($550.00)" (QA CF-09).
		if a.Class == domain.ClassLiability {
			m := bal.Amount
			if m < 0 {
				m = -m
			}
			bal = money.New(-m, bal.Currency)
		}
		tone := ""
		if bal.IsNegative() {
			tone = "down"
		}
		names = append(names, a.Name)
		balances = append(balances, bal.Amount)
		currencies = append(currencies, bal.Currency)
		tones = append(tones, tone)
	}
	return domain.NewFrame(
		domain.Field{Name: "name", Type: domain.FieldString, Values: names},
		domain.Field{Name: "balance", Type: domain.FieldMoney, Values: balances},
		domain.Field{Name: "currency", Type: domain.FieldString, Values: currencies},
		domain.Field{Name: "tone", Type: domain.FieldTone, Values: tones},
	)
}

// NetWorthSeries produces a chart Frame for the Trend widget: one row per cutoff
// with columns t (cutoff time, unix seconds as number) and value (money minor
// units). The renderer formats labels and builds the chart from these.
func NetWorthSeries(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, cutoffs []time.Time) domain.Frame {
	series, _ := ledger.NetWorthSeries(accounts, txns, cutoffs, rates)
	var ts, values []any
	for i := range series {
		var when time.Time
		if i < len(cutoffs) {
			when = cutoffs[i]
		}
		ts = append(ts, float64(when.Unix()))
		values = append(values, series[i].Amount)
	}
	return domain.NewFrame(
		domain.Field{Name: "t", Type: domain.FieldNumber, Values: ts},
		domain.Field{Name: "value", Type: domain.FieldMoney, Values: values},
	)
}

// RecentTransactions produces a Frame for the Recent widget: every transaction,
// newest first, with columns date (unix seconds as number), desc (string), amount
// (money minor units — the transaction's own currency, signed) and currency. The
// row cap is a limit transform applied by the pipeline.
func RecentTransactions(txns []domain.Transaction) domain.Frame {
	recent := ledger.Recent(txns, len(txns))
	var dates, descs, amounts, currencies []any
	for _, t := range recent {
		dates = append(dates, float64(t.Date.Unix()))
		descs = append(descs, t.Desc)
		amounts = append(amounts, t.Amount.Amount)
		currencies = append(currencies, t.Amount.Currency)
	}
	return domain.NewFrame(
		domain.Field{Name: "date", Type: domain.FieldNumber, Values: dates},
		domain.Field{Name: "desc", Type: domain.FieldString, Values: descs},
		domain.Field{Name: "amount", Type: domain.FieldMoney, Values: amounts},
		domain.Field{Name: "currency", Type: domain.FieldString, Values: currencies},
	)
}

// RichTransactions produces the fuller Frame the widgetized /transactions table
// renders: every transaction newest-first with the columns the ledger view needs —
// id (for drill-to-edit), date (unix seconds), payee, desc, amount (minor units,
// signed, own currency), currency, account + category display names (resolved from
// the supplied lists), cleared (bool) and tags (comma-joined). Row trimming is a
// limit transform applied by the pipeline. Pure: no syscall/js.
func RichTransactions(txns []domain.Transaction, accounts []domain.Account, cats []domain.Category) domain.Frame {
	acctName := make(map[string]string, len(accounts))
	for _, a := range accounts {
		acctName[a.ID] = a.Name
	}
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}
	// Preserve the INPUT order — the caller (the widgetized ledger) passes rows already
	// sorted by the active sort column/direction, so re-sorting here would override the
	// user's chosen sort. (Contrast RecentTransactions, which forces newest-first for
	// the dashboard "recent" tile.) A pipeline sort transform can still reorder it.
	var ids, dates, payees, descs, amounts, currencies, accountsCol, categories, cleared, tagsCol, sources []any
	for _, t := range txns {
		ids = append(ids, t.ID)
		dates = append(dates, float64(t.Date.Unix()))
		payees = append(payees, t.Payee)
		descs = append(descs, t.Desc)
		amounts = append(amounts, t.Amount.Amount)
		currencies = append(currencies, t.Amount.Currency)
		accountsCol = append(accountsCol, acctName[t.AccountID])
		categories = append(categories, catName[t.CategoryID])
		cleared = append(cleared, t.Cleared)
		tagsCol = append(tagsCol, strings.Join(t.Tags, ", "))
		// Display label for the provenance column ("Manual"/"Imported"/…, "—" if unset).
		sources = append(sources, t.Source.Label())
	}
	return domain.NewFrame(
		domain.Field{Name: "id", Type: domain.FieldString, Values: ids},
		domain.Field{Name: "date", Type: domain.FieldNumber, Values: dates},
		domain.Field{Name: "payee", Type: domain.FieldString, Values: payees},
		domain.Field{Name: "desc", Type: domain.FieldString, Values: descs},
		domain.Field{Name: "amount", Type: domain.FieldMoney, Values: amounts},
		domain.Field{Name: "currency", Type: domain.FieldString, Values: currencies},
		domain.Field{Name: "account", Type: domain.FieldString, Values: accountsCol},
		domain.Field{Name: "category", Type: domain.FieldString, Values: categories},
		domain.Field{Name: "cleared", Type: domain.FieldBool, Values: cleared},
		domain.Field{Name: "tags", Type: domain.FieldString, Values: tagsCol},
		domain.Field{Name: "source", Type: domain.FieldString, Values: sources},
	)
}

// UpcomingBills produces a Frame for the Bills widget: the next recurring charges
// (via the same bills.UpcomingAll derivation the Bills screen uses), with columns
// name (string), due (unix seconds as number), days (whole days until due, number)
// and amount (money minor units, positive). The row cap is a limit transform.
func UpcomingBills(accounts []domain.Account, recurring []domain.Recurring, now time.Time) domain.Frame {
	upcoming := bills.UpcomingAll(accounts, recurring, now)
	var names, dues, days, amounts, currencies []any
	for _, b := range upcoming {
		names = append(names, b.Name)
		dues = append(dues, float64(b.DueDate.Unix()))
		days = append(days, float64(b.DaysUntil))
		amounts = append(amounts, b.Amount.Amount)
		currencies = append(currencies, b.Amount.Currency)
	}
	return domain.NewFrame(
		domain.Field{Name: "name", Type: domain.FieldString, Values: names},
		domain.Field{Name: "due", Type: domain.FieldNumber, Values: dues},
		domain.Field{Name: "days", Type: domain.FieldNumber, Values: days},
		domain.Field{Name: "amount", Type: domain.FieldMoney, Values: amounts},
		domain.Field{Name: "currency", Type: domain.FieldString, Values: currencies},
	)
}

// CashFlowSeries produces a chart Frame for the Cash flow widget: income vs expense
// per month for a trailing window ending at the current month, columns t (month
// start, unix seconds as number), income (money minor, base currency) and expense
// (money minor, base currency). Totals via ledger.PeriodTotals.
func CashFlowSeries(txns []domain.Transaction, rates currency.Rates, now time.Time, months int) domain.Frame {
	if months < 1 {
		months = 4
	}
	start := dateutil.MonthStart(now)
	var ts, incomes, expenses []any
	for i := 0; i < months; i++ {
		ms := dateutil.AddMonths(start, i-(months-1)) // (months-1) ago … current
		s, e := dateutil.MonthRange(ms)
		inc, exp, _ := ledger.PeriodTotals(txns, s, e, rates)
		ts = append(ts, float64(ms.Unix()))
		incomes = append(incomes, inc.Amount)
		expenses = append(expenses, exp.Amount)
	}
	return domain.NewFrame(
		domain.Field{Name: "t", Type: domain.FieldNumber, Values: ts},
		domain.Field{Name: "income", Type: domain.FieldMoney, Values: incomes},
		domain.Field{Name: "expense", Type: domain.FieldMoney, Values: expenses},
	)
}

// SpendingBreakdown produces a Frame for the Spending breakdown widget: period
// expenses rolled up to each root category and ranked by spend (descending), with
// columns name (string, "" → uncategorized), amount (money minor, base currency)
// and percent (share of total spend). The widget collapses the tail into "Other"
// at render time per its top-N setting; the engine supplies the full ranked rollup.
func SpendingBreakdown(cats []domain.Category, txns []domain.Transaction, rates currency.Rates, start, end time.Time) domain.Frame {
	catName := make(map[string]string, len(cats))
	parent := make(map[string]string, len(cats))
	exists := make(map[string]bool, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
		parent[c.ID] = c.ParentID
		exists[c.ID] = true
	}
	rootOf := func(id string) string {
		seen := map[string]bool{}
		for {
			p := parent[id]
			if p == "" || !exists[p] || seen[id] {
				break
			}
			seen[id] = true
			id = p
		}
		return id
	}
	totals := make(map[string]int64)
	var total int64
	for _, t := range txns {
		if !t.IsExpense() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			continue
		}
		amt := conv.Amount
		if amt < 0 {
			amt = -amt
		}
		totals[rootOf(t.CategoryID)] += amt
		total += amt
	}
	// Rank all root categories by spend (no tail collapse — that's the widget's
	// top-N presentation choice). RankSpending with n == len keeps every category.
	ranked, _ := ledger.RankSpending(totals, len(totals))
	var names, amounts, percents []any
	for _, ct := range ranked {
		name := catName[ct.CategoryID]
		names = append(names, name)
		amounts = append(amounts, ct.Amount)
		pct := 0.0
		if total > 0 {
			pct = float64(ct.Amount) * 100 / float64(total)
		}
		percents = append(percents, pct)
	}
	return domain.NewFrame(
		domain.Field{Name: "name", Type: domain.FieldString, Values: names},
		domain.Field{Name: "amount", Type: domain.FieldMoney, Values: amounts},
		domain.Field{Name: "percent", Type: domain.FieldPercent, Values: percents},
	)
}

// TxnFilterMatcher parses a flow-series filter into a per-transaction
// predicate. Three selector forms cover the household's own vocabulary:
//
//	tag:<tag>        — the transaction carries the tag (case-insensitive)
//	cat:<id or name> — the category, by stable id or display name
//	cf:<key>=<value> — a custom-field value on the transaction
//
// Transfers never match (they're money moving, not flow).
func TxnFilterMatcher(filter string, cats []domain.Category) (func(domain.Transaction) bool, error) {
	f := strings.TrimSpace(filter)
	sel, arg, ok := strings.Cut(f, ":")
	if !ok || strings.TrimSpace(arg) == "" {
		return nil, fmt.Errorf("flow series: unsupported filter %q (want tag:, cat:, or cf:key=value)", filter)
	}
	arg = strings.TrimSpace(arg)
	switch strings.ToLower(sel) {
	case "tag":
		return func(t domain.Transaction) bool {
			if t.TransferAccountID != "" {
				return false
			}
			for _, tg := range t.Tags {
				if strings.EqualFold(tg, arg) {
					return true
				}
			}
			return false
		}, nil
	case "cat":
		ids := map[string]bool{}
		for _, c := range cats {
			if c.ID == arg || strings.EqualFold(c.Name, arg) {
				ids[c.ID] = true
			}
		}
		if len(ids) == 0 {
			ids[arg] = true // fall back to a literal id (cats may be absent in tests)
		}
		return func(t domain.Transaction) bool {
			return t.TransferAccountID == "" && ids[t.CategoryID]
		}, nil
	case "cf":
		key, want, ok := strings.Cut(arg, "=")
		key, want = strings.TrimSpace(key), strings.TrimSpace(want)
		if !ok || key == "" || want == "" {
			return nil, fmt.Errorf("flow series: cf filter needs key=value, got %q", arg)
		}
		return func(t domain.Transaction) bool {
			if t.TransferAccountID != "" || t.Custom == nil {
				return false
			}
			v, has := t.Custom[key]
			if !has {
				return false
			}
			return strings.EqualFold(strings.TrimSpace(fmt.Sprint(v)), want)
		}, nil
	}
	return nil, fmt.Errorf("flow series: unknown selector %q", sel)
}

// FilteredFlowSeries plots the matching transactions' monthly sums (base
// currency) for the trailing `months` months — the "graph MY tag / category /
// custom value" series behind Metric=="flow". Sums are signed unless abs is set,
// in which case each month plots its magnitude (so a pure-expense "costs" series
// reads as positive dollars rather than a chart of negatives).
func FilteredFlowSeries(txns []domain.Transaction, rates currency.Rates, now time.Time, months int, match func(domain.Transaction) bool, abs bool) domain.Frame {
	if months < 1 {
		months = 12
	}
	// End at the last COMPLETE month: a mid-month partial reads as a cliff to
	// zero on a trend chart, which looks broken rather than honest.
	start := dateutil.AddMonths(dateutil.MonthStart(now), -1)
	var ts, values []any
	for i := 0; i < months; i++ {
		ms := dateutil.AddMonths(start, i-(months-1))
		s, e := dateutil.MonthRange(ms)
		var sum int64
		for _, t := range txns {
			if !match(t) || t.Date.Before(s) || !t.Date.Before(e) {
				continue
			}
			conv, err := rates.ToBase(t.Amount)
			if err != nil {
				continue
			}
			sum += conv.Amount
		}
		if abs && sum < 0 {
			sum = -sum
		}
		ts = append(ts, float64(ms.Unix()))
		values = append(values, sum)
	}
	return domain.NewFrame(
		domain.Field{Name: "t", Type: domain.FieldNumber, Values: ts},
		domain.Field{Name: "value", Type: domain.FieldMoney, Values: values},
	)
}

// FormulaSeries plots a formula's value for each trailing month window — the
// series behind Metric=="formula". eval receives each window's bounds and
// returns the formula's value against that month's variable surface (ok=false
// plots the month as zero rather than dropping it, keeping the x-axis dense).
// format selects the value column's type: "percent", "number", or currency
// (the default — values arrive in major units and are stored as minor).
func FormulaSeries(now time.Time, months int, format, base string, eval func(start, end time.Time) (float64, bool)) domain.Frame {
	if months < 1 {
		months = 12
	}
	mul := 1.0
	for i := 0; i < currency.Decimals(base); i++ {
		mul *= 10
	}
	fieldType := domain.FieldMoney
	switch format {
	case "percent":
		fieldType, mul = domain.FieldPercent, 1
	case "number":
		fieldType, mul = domain.FieldNumber, 1
	}
	// Same last-complete-month window as FilteredFlowSeries: a formula over a
	// five-day partial month plots a misleading spike or cliff.
	start := dateutil.AddMonths(dateutil.MonthStart(now), -1)
	var ts, values []any
	for i := 0; i < months; i++ {
		ms := dateutil.AddMonths(start, i-(months-1))
		s, e := dateutil.MonthRange(ms)
		v, ok := eval(s, e)
		if !ok {
			v = 0
		}
		ts = append(ts, float64(ms.Unix()))
		if fieldType == domain.FieldMoney {
			values = append(values, int64(math.Round(v*mul)))
		} else {
			values = append(values, v)
		}
	}
	return domain.NewFrame(
		domain.Field{Name: "t", Type: domain.FieldNumber, Values: ts},
		domain.Field{Name: "value", Type: fieldType, Values: values},
	)
}
