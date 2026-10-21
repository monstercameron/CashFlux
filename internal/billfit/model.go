// SPDX-License-Identifier: MIT

// Package billfit prepares checked budget-fit facts for upcoming bill rows.
package billfit

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Input is the immutable data snapshot used to prepare fits for displayed bills.
// Ledger must already be refund-netted when links apply. SourceError records a
// failed required read; a matched budget then produces an unavailable fact.
type Input struct {
	Candidates  []bills.Bill
	Accounts    []domain.Account
	Recurring   []domain.Recurring
	Budgets     []domain.Budget
	Categories  []domain.Category
	Ledger      budgeting.NettedLedger
	Rates       currency.Rates
	Now         time.Time
	WeekStart   time.Weekday
	SourceError error
	IndexError  error
}

// Fact distinguishes a healthy absence of a tracked budget from a failed
// calculation. HasBudget=false means there is no chip to show. A matched budget
// with Available=false must be rendered as unavailable, never as a zero result.
type Fact struct {
	BudgetID   string
	BudgetName string
	HasBudget  bool
	Available  bool
	Fits       bool
	Amount     money.Money // room left when Fits; overage otherwise
	Err        error
}

// Prepared holds indexes and the single upcoming-bills snapshot shared by all
// candidate rows. Treat it and its inputs as immutable after Prepare returns.
type Prepared struct {
	input               Input
	categoryByRecurring map[string]string
	budgetMatches       []budgetMatch
	upcoming            []bills.Bill
}

type budgetMatch struct {
	budget domain.Budget
	desc   map[string]bool
}

// Prepare calls UpcomingAll once and builds recurring-category and budget
// coverage indexes once. The returned model is safe for repeated Fit calls.
func Prepare(in Input) *Prepared {
	p := &Prepared{input: in, categoryByRecurring: make(map[string]string, len(in.Recurring))}
	for _, r := range in.Recurring {
		p.categoryByRecurring[r.ID] = r.CategoryID
	}
	p.budgetMatches = make([]budgetMatch, 0, len(in.Budgets))
	for _, b := range in.Budgets {
		p.budgetMatches = append(p.budgetMatches, budgetMatch{
			budget: b,
			desc:   categorytree.DescendantsOfAll(in.Categories, b.TrackedCategoryIDs()),
		})
	}
	p.upcoming = bills.UpcomingAll(in.Accounts, in.Recurring, in.Now)
	return p
}

// Candidates returns a copy of the displayed bill candidates supplied to Prepare.
func (p *Prepared) Candidates() []bills.Bill {
	if p == nil {
		return nil
	}
	return append([]bills.Bill(nil), p.input.Candidates...)
}

// Fit returns the budget-fit fact for one candidate bill.
func (p *Prepared) Fit(bill bills.Bill) Fact {
	if p == nil {
		return Fact{}
	}
	_, isRecurring := bills.RecurringIDFromAccount(bill.AccountID)
	if !isRecurring {
		return Fact{}
	}
	if p.input.IndexError != nil {
		return Fact{HasBudget: true, Err: fmt.Errorf("read bill-fit indexes: %w", p.input.IndexError)}
	}
	catID := recurringCategory(bill.AccountID, p.categoryByRecurring)
	if catID == "" {
		return Fact{}
	}
	var matched *budgetMatch
	for i := range p.budgetMatches {
		if p.budgetMatches[i].desc[catID] {
			matched = &p.budgetMatches[i]
			break // preserve the existing first matching budget policy
		}
	}
	if matched == nil {
		return Fact{}
	}
	limit := matched.budget.Limit
	if limit.Currency == "" {
		limit = money.New(limit.Amount, p.input.Rates.Base)
	}
	if limit.Amount <= 0 {
		return Fact{}
	}
	fact := Fact{BudgetID: matched.budget.ID, BudgetName: matched.budget.Name, HasBudget: true}
	if p.input.SourceError != nil {
		fact.Err = fmt.Errorf("read bill-fit inputs: %w", p.input.SourceError)
		return fact
	}
	if p.input.Rates.Base == "" {
		fact.Err = fmt.Errorf("bill-fit base currency is unavailable")
		return fact
	}
	start, end := budgeting.PeriodRange(matched.budget.Period, bill.DueDate, p.input.WeekStart)
	spent, err := budgeting.Spent(matched.budget, p.input.Ledger, start, end, p.input.Rates)
	if err != nil {
		fact.Err = fmt.Errorf("calculate budget spend: %w", err)
		return fact
	}
	pending := money.New(0, limit.Currency)
	for _, other := range p.upcoming {
		if other.AccountID == bill.AccountID && other.DueDate.Equal(bill.DueDate) {
			continue
		}
		if other.DueDate.Before(start) || !other.DueDate.Before(end) {
			continue // preserve the half-open budget period
		}
		otherCategory := recurringCategory(other.AccountID, p.categoryByRecurring)
		if otherCategory == "" || !matched.desc[otherCategory] {
			continue
		}
		converted, err := p.input.Rates.Convert(other.Amount, limit.Currency)
		if err != nil {
			fact.Err = fmt.Errorf("convert pending bill %q: %w", other.AccountID, err)
			return fact
		}
		pending, err = pending.Add(converted)
		if err != nil {
			fact.Err = fmt.Errorf("sum pending bills: %w", err)
			return fact
		}
	}
	booked, err := spent.Add(pending)
	if err != nil {
		fact.Err = fmt.Errorf("combine booked and pending spend: %w", err)
		return fact
	}
	charge, err := p.input.Rates.Convert(bill.Amount, limit.Currency)
	if err != nil {
		fact.Err = fmt.Errorf("convert bill: %w", err)
		return fact
	}
	fit, err := budgeting.FitBillChecked(limit.Amount, booked.Amount, charge.Amount)
	if err != nil {
		fact.Err = fmt.Errorf("evaluate bill fit: %w", err)
		return fact
	}
	fact.Available = true
	fact.Fits = fit.Fits
	if fit.Fits {
		fact.Amount = money.New(fit.LeftAfter, limit.Currency)
	} else {
		fact.Amount = money.New(fit.OverBy, limit.Currency)
	}
	return fact
}

func recurringCategory(accountID string, byID map[string]string) string {
	rid, ok := bills.RecurringIDFromAccount(accountID)
	if !ok || strings.TrimSpace(rid) == "" {
		return ""
	}
	return byID[rid]
}

// CategoryID returns the recurring category encoded by a bill identity. It is
// shared with screen-level filters so they use the same identity convention.
func CategoryID(accountID string, recurring []domain.Recurring) string {
	byID := make(map[string]string, len(recurring))
	for _, r := range recurring {
		byID[r.ID] = r.CategoryID
	}
	return recurringCategory(accountID, byID)
}
