// SPDX-License-Identifier: MIT

package assistantdata

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/afford"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/liquidity"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/networthview"
	"github.com/monstercameron/CashFlux/internal/periodview"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/scope"
)

// FinanceInput names the immutable dataset, scope, policy and clock behind one
// Assistant finance invocation. Raw transactions remain the source of balances;
// a scope-local refund ledger supplies reporting and budget activity.
type FinanceInput struct {
	Accounts       []domain.Account
	Transactions   []domain.Transaction
	Categories     []domain.Category
	Budgets        []domain.Budget
	Recurring      []domain.Recurring
	TxnLinks       []domain.TxnLink
	Now            time.Time
	Scope          scope.ReportScope
	Base           string
	Rates          currency.Rates
	WeekStart      time.Weekday
	PayCycleAnchor time.Time
	PoolRollover   bool
}

// Finance prepares scope selection once. Its methods delegate financial rules
// to canonical native models and retain calculation errors independently.
type Finance struct {
	input        FinanceInput
	accounts     []domain.Account
	transactions []domain.Transaction
	netted       budgeting.NettedLedger
	err          error
}

// BuildFinance resolves the selected active accounts before refund netting.
// A caller must rebuild it from current inputs for each tool invocation.
func BuildFinance(in FinanceInput) Finance {
	in.Base = strings.ToUpper(strings.TrimSpace(in.Base))
	if in.Base == "" {
		in.Base = strings.ToUpper(strings.TrimSpace(in.Rates.Base))
	}
	if in.Base == "" {
		in.Base = "USD"
	}
	f := Finance{input: in}
	if in.Now.IsZero() {
		f.err = fmt.Errorf("finance: reference time is required")
	}
	if !currency.Valid(in.Base) {
		f.err = fmt.Errorf("finance: unsupported base currency %q", in.Base)
	}
	if in.Rates.Base != "" && !strings.EqualFold(in.Rates.Base, in.Base) {
		f.err = fmt.Errorf("finance: rate base %q differs from display base %q", in.Rates.Base, in.Base)
	}
	f.input.Rates.Base = in.Base
	ids := scope.ResolveScope(in.Accounts, in.Scope, func(a domain.Account) string { return a.Institution })
	f.accounts = scope.ApplyScopeToAccounts(in.Accounts, ids)
	f.transactions = scope.ApplyScopeToTxns(in.Transactions, ids)
	f.netted = budgeting.NewNettedLedger(f.transactions, in.TxnLinks)
	return f
}

// Base returns the currency used for aggregated money.
func (f Finance) Base() string { return f.input.Base }

// ReferenceTime returns the single instant used by calendar-dependent models.
func (f Finance) ReferenceTime() time.Time { return f.input.Now }

// Err reports an invalid invocation context before any financial claims are made.
func (f Finance) Err() error { return f.err }

// ScopeLabel states whether activity follows a selected account scope.
func (f Finance) ScopeLabel() string {
	if f.input.Scope.IsAll() {
		return "all active accounts"
	}
	return fmt.Sprintf("selected active-account scope (%d accounts)", len(f.accounts))
}

// ResolveWindow returns exact half-open bounds and rejects malformed supplied
// dates rather than silently answering a different period.
func (f Finance) ResolveWindow(preset, from, to string) (reports.AskedWindow, error) {
	if f.err != nil {
		return reports.AskedWindow{}, f.err
	}
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	for _, raw := range []string{from, to} {
		if strings.TrimSpace(raw) != "" {
			if _, err := dateutil.ParseDate(strings.TrimSpace(raw)); err != nil {
				return reports.AskedWindow{}, fmt.Errorf("finance: invalid date %q: %w", raw, err)
			}
		}
	}
	if from == "" && to == "" {
		switch strings.ToLower(strings.TrimSpace(preset)) {
		case "", "this_month", "last_month", "this_quarter", "last_quarter", "this_year", "last_year", "year_to_date", "last_12_months", "all":
		default:
			return reports.AskedWindow{}, fmt.Errorf("finance: unknown period %q", preset)
		}
	}
	w := reports.ResolveWindow(preset, from, to, f.input.Now)
	if !w.Start.IsZero() && !w.End.After(w.Start) {
		return reports.AskedWindow{}, fmt.Errorf("finance: end date precedes start date")
	}
	return w, nil
}

// Category resolves an exact ID/name or one unambiguous name fragment, then
// returns the category's checked spending. A defined category with no activity
// is a known zero; a missing or ambiguous name is a query error.
func (f Finance) Category(w reports.AskedWindow, query string) (CategorySpending, error) {
	q := strings.TrimSpace(query)
	var matches []domain.Category
	for _, c := range f.input.Categories {
		if c.ID == q || strings.EqualFold(c.Name, q) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 && q != "" {
		for _, c := range f.input.Categories {
			if strings.Contains(strings.ToLower(c.Name), strings.ToLower(q)) {
				matches = append(matches, c)
			}
		}
	}
	id, name := "", "Uncategorized"
	if !strings.EqualFold(q, "uncategorized") {
		if len(matches) != 1 {
			return CategorySpending{}, fmt.Errorf("finance: category %q matches %d categories; use an exact name or ID", q, len(matches))
		}
		id, name = matches[0].ID, matches[0].Name
	}
	rows, err := f.Spending(w)
	if err != nil {
		return CategorySpending{}, err
	}
	for _, row := range rows {
		if row.CategoryID == id {
			return row, nil
		}
	}
	return CategorySpending{CategoryID: id, Name: name, Amount: money.Zero(f.Base())}, nil
}

// Totals returns checked, refund-netted activity in the selected scope/window.
func (f Finance) Totals(w reports.AskedWindow) periodview.Totals {
	if f.err != nil {
		return periodview.Totals{Error: f.err}
	}
	return periodview.BuildTotals(f.netted, w.Start, w.End, f.input.Rates)
}

// HouseholdNetWorth deliberately uses the entire raw household balance sheet.
// Missing FX is a disclosed subtotal, while fatal errors make all money unknown.
func (f Finance) HouseholdNetWorth() networthview.Snapshot {
	if f.err != nil {
		return networthview.Snapshot{Err: f.err}
	}
	return networthview.Build(f.input.Accounts, f.input.Transactions, f.input.Rates)
}

// CategorySpending is one category's exact amount and number of contributing
// expense transactions; a split parent counts once in each relevant category.
type CategorySpending struct {
	CategoryID, Name string
	Amount           money.Money
	Count            int
}

// Spending delegates exclusions, split directions, conversion and accumulation
// to the checked report model over the explicit scope-local refund ledger.
func (f Finance) Spending(w reports.AskedWindow) ([]CategorySpending, error) {
	if f.err != nil {
		return nil, f.err
	}
	rows, err := reports.SpendingByCategoryChecked(f.netted, w.Start, w.End, f.input.Rates)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, c := range f.input.Categories {
		names[c.ID] = c.Name
	}
	counts := map[string]int{}
	for _, txn := range f.netted {
		if !txn.IsExpense() || !txn.CountsInReports() || !dateutil.InRange(txn.Date, w.Start, w.End) {
			continue
		}
		if txn.HasSplits() {
			seen := map[string]bool{}
			for _, split := range txn.Splits {
				if split.Amount.Amount < 0 && !seen[split.CategoryID] {
					counts[split.CategoryID]++
					seen[split.CategoryID] = true
				}
			}
		} else {
			counts[txn.CategoryID]++
		}
	}
	out := make([]CategorySpending, 0, len(rows))
	for _, row := range rows {
		name := names[row.CategoryID]
		if name == "" {
			name = "Uncategorized"
		}
		out = append(out, CategorySpending{CategoryID: row.CategoryID, Name: name, Amount: money.New(row.Amount, f.Base()), Count: counts[row.CategoryID]})
	}
	return out, nil
}

// AccountBalance retains each selected account even when its own ledger fails.
type AccountBalance struct {
	Account domain.Account
	Balance money.Money
	Err     error
}

// AccountBalances returns account-currency balances without silently hiding a
// corrupt account or mixing its units with the aggregate base currency.
func (f Finance) AccountBalances() []AccountBalance {
	out := make([]AccountBalance, 0, len(f.accounts))
	for _, account := range f.accounts {
		balance, err := ledger.Balance(account, f.input.Transactions)
		if f.err != nil {
			err = f.err
		}
		out = append(out, AccountBalance{Account: account, Balance: balance, Err: err})
	}
	return out
}

// BudgetFact includes the canonical view and its category/tag coverage labels.
type BudgetFact struct {
	budgeting.ViewBudget
	CategoryNames []string
	Tags          []string
}

// Budgets evaluates visible budgets on each budget's cadence, with the same
// rollover, boost, coverage and FX models as its card. Spending follows account
// scope; the standing/effective plan is retained and must be labeled separately.
func (f Finance) Budgets(w reports.AskedWindow) []BudgetFact {
	member := ""
	if len(f.input.Scope.Owners) == 1 {
		member = f.input.Scope.Owners[0]
	}
	budgets := budgeting.ViewBudgets(f.input.Budgets, member)
	in := budgeting.ViewInput{Ledger: &f.netted, Transactions: f.netted, Categories: f.input.Categories, Recurring: f.input.Recurring,
		Period: budgeting.ResolveViewPeriod(w.Start, w.End, f.input.Now, f.input.WeekStart, f.input.PayCycleAnchor),
		Rates:  f.input.Rates, Base: f.Base(), PoolRollover: f.input.PoolRollover}
	names := map[string]string{}
	for _, category := range f.input.Categories {
		names[category.ID] = category.Name
	}
	out := make([]BudgetFact, 0, len(budgets))
	for _, b := range budgets {
		view := budgeting.EvaluateViewBudget(b, in)
		if f.err != nil {
			view.Valid = false
			view.Error = &budgeting.BudgetEvaluationError{BudgetID: b.ID, Operation: "prepare finance context", Err: f.err}
			view.Status.Error, view.Status.State = view.Error, budgeting.StateUnavailable
		}
		covered := budgeting.ScopeOf(b, view.Covers)
		fact := BudgetFact{ViewBudget: view, Tags: covered.Tags}
		for _, id := range covered.CategoryIDs {
			name := names[id]
			if name == "" {
				name = id
			}
			fact.CategoryNames = append(fact.CategoryNames, name)
		}
		out = append(out, fact)
	}
	return out
}

// ParseAmount parses decimal major units exactly at the currency's precision.
// Excess precision and amounts outside int64 are rejected without float coercion.
func ParseAmount(decimal, base string) (money.Money, error) {
	if !currency.Valid(base) {
		return money.Money{}, fmt.Errorf("finance: unsupported currency %q", base)
	}
	minor, err := money.ParseMinor(decimal, currency.Decimals(base))
	if err != nil {
		return money.Money{}, err
	}
	return money.New(minor, base), nil
}

// Affordability contains a simple cash projection, not a safe-to-spend promise.
// Liquid and monthly net follow available cash assets within the selected account scope. Reserved is only
// the caller's explicit reserve; scheduled bills/goals are not deducted here.
type Affordability struct {
	Known                                  bool
	Liquid, MonthlyNet, Purchase, Reserved money.Money
	Months                                 int
	Result                                 afford.Result
	Err                                    error
}

// Affordability projects canonical cash-type balances using this calendar
// month's checked net activity. Unknown FX/overflow prevents a yes/no claim.
func (f Finance) Affordability(amount, reserve money.Money, months int) Affordability {
	out := Affordability{Purchase: amount, Reserved: reserve, Months: months}
	if f.err != nil {
		out.Err = f.err
		return out
	}
	if amount.Currency != f.Base() || reserve.Currency != f.Base() {
		out.Err = money.ErrCurrencyMismatch
		return out
	}
	if amount.Amount <= 0 || reserve.Amount < 0 {
		out.Err = fmt.Errorf("finance: positive purchase and nonnegative reserve required")
		return out
	}
	if months < 0 {
		out.Err = fmt.Errorf("finance: months must be nonnegative")
		return out
	}
	var sources []domain.Account
	var sourceIDs []string
	for _, account := range f.accounts {
		if !account.IsLiability() && liquidity.Of(account, f.input.Now) == liquidity.Available {
			sources = append(sources, account)
			sourceIDs = append(sourceIDs, account.ID)
		}
	}
	liquid, err := ledger.LiquidBalance(sources, f.input.Transactions, f.input.Rates)
	if err != nil {
		out.Err = err
		return out
	}
	start, end := dateutil.MonthRange(f.input.Now)
	cashTransactions := scope.ApplyScopeToTxns(f.transactions, sourceIDs)
	cashLedger := budgeting.NewNettedLedger(cashTransactions, f.input.TxnLinks)
	totals := periodview.BuildTotals(cashLedger, start, end, f.input.Rates)
	if !totals.Known {
		out.Err = totals.Error
		return out
	}
	result, err := afford.CanAffordChecked(amount.Amount, liquid.Amount, totals.Net.Amount, months, reserve.Amount)
	if err != nil {
		out.Err = err
		return out
	}
	out.Known, out.Liquid, out.MonthlyNet, out.Result = true, liquid, totals.Net, result
	return out
}
