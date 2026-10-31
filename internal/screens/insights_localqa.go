// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

// insightsQASource is the local assistant's checked bridge to current app data.
// It reads the bound store directly so a closed store or missing FX cannot become
// an empty result through App's logging-only convenience accessors.

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/assistantdata"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	"github.com/monstercameron/CashFlux/internal/insights/localqa"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/safespend"
	"github.com/monstercameron/CashFlux/internal/scope"
)

type insightsQASource struct {
	app     *appstate.App
	now     time.Time
	base    string
	rates   currency.Rates
	lastErr error
}

func newInsightsQASource(app *appstate.App, base string, rates currency.Rates) *insightsQASource {
	now := time.Now()
	if app != nil {
		now = app.ClockNow()
	}
	return &insightsQASource{app: app, now: now, base: base, rates: rates}
}

func (s *insightsQASource) LastError() error { return s.lastErr }

func (s *insightsQASource) begin() error {
	s.lastErr = nil
	if s.app == nil || s.app.Store() == nil {
		return fmt.Errorf("local dataset is unavailable")
	}
	settings, err := s.app.Store().GetSettings()
	if err != nil {
		return fmt.Errorf("read local settings: %w", err)
	}
	s.base = settings.BaseCurrency
	if s.base == "" {
		s.base = "USD"
	}
	s.rates = currency.Rates{Base: s.base, Rates: settings.FXRates}
	if s.now.IsZero() {
		s.now = time.Now()
	}
	return nil
}

func (s *insightsQASource) fail(err error) {
	if err != nil {
		s.lastErr = err
	}
}

func (s *insightsQASource) accounts() ([]domain.Account, error) {
	rows, err := s.app.Store().ListAccounts()
	if err != nil {
		return nil, fmt.Errorf("read accounts: %w", err)
	}
	return rows, nil
}
func (s *insightsQASource) transactions() ([]domain.Transaction, error) {
	rows, err := s.app.Store().ListTransactions()
	if err != nil {
		return nil, fmt.Errorf("read transactions: %w", err)
	}
	return rows, nil
}
func (s *insightsQASource) recurring() ([]domain.Recurring, error) {
	return s.app.Recurring(), nil
}
func (s *insightsQASource) goals() ([]domain.Goal, error) {
	rows, err := s.app.Store().ListGoals()
	if err != nil {
		return nil, fmt.Errorf("read goals: %w", err)
	}
	return rows, nil
}
func (s *insightsQASource) LiquidBalanceMinor() int64 {
	if err := s.begin(); err != nil {
		s.fail(err)
		return 0
	}
	accounts, err := s.accounts()
	if err != nil {
		s.fail(err)
		return 0
	}
	txns, err := s.transactions()
	if err != nil {
		s.fail(err)
		return 0
	}
	liq, err := ledger.LiquidBalance(accounts, txns, s.rates)
	if err != nil {
		s.fail(err)
		return 0
	}
	return liq.Amount
}

func (s *insightsQASource) NetWorthMinor() (int64, int64) {
	if err := s.begin(); err != nil {
		s.fail(err)
		return 0, 0
	}
	accounts, err := s.accounts()
	if err != nil {
		s.fail(err)
		return 0, 0
	}
	txns, err := s.transactions()
	if err != nil {
		s.fail(err)
		return 0, 0
	}
	_, assets, liabilities, err := ledger.NetWorth(accounts, txns, s.rates)
	if err != nil {
		s.fail(err)
		return 0, 0
	}
	return assets.Amount, liabilities.Amount
}

func (s *insightsQASource) SafeToSpendMinor() int64 {
	if err := s.begin(); err != nil {
		s.fail(err)
		return 0
	}
	accounts, err := s.accounts()
	if err != nil {
		s.fail(err)
		return 0
	}
	txns, err := s.transactions()
	if err != nil {
		s.fail(err)
		return 0
	}
	recurring, err := s.recurring()
	if err != nil {
		s.fail(err)
		return 0
	}
	goals, err := s.goals()
	if err != nil {
		s.fail(err)
		return 0
	}
	liq, err := ledger.LiquidBalance(accounts, txns, s.rates)
	if err != nil {
		s.fail(err)
		return 0
	}
	_, monthEnd := dateutil.MonthRange(s.now)
	billsDue, err := safespend.BillsDueBeforeChecked(accounts, recurring, s.now, monthEnd, s.rates)
	if err != nil {
		s.fail(err)
		return 0
	}
	goalNeeds, err := safespend.GoalContributionsProratedChecked(goals, s.now, s.rates)
	if err != nil {
		s.fail(err)
		return 0
	}
	partial, err := money.Sub(liq.Amount, max64(billsDue, 0))
	if err != nil {
		s.fail(err)
		return 0
	}
	if _, err = money.Sub(partial, max64(goalNeeds, 0)); err != nil {
		s.fail(err)
		return 0
	}
	return safespend.Compute(liq.Amount, billsDue, goalNeeds, 0, s.base).SafeToSpend
}

func (s *insightsQASource) SpendingOnCategoryMinor(category string) int64 {
	if err := s.begin(); err != nil {
		s.fail(err)
		return 0
	}
	accounts, err := s.accounts()
	if err != nil {
		s.fail(err)
		return 0
	}
	txns, err := s.transactions()
	if err != nil {
		s.fail(err)
		return 0
	}
	categories, err := s.app.CategoriesChecked()
	if err != nil {
		s.fail(err)
		return 0
	}
	links, err := s.app.Store().ListTxnLinks()
	if err != nil {
		s.fail(fmt.Errorf("read transaction links: %w", err))
		return 0
	}
	finance := assistantdata.BuildFinance(assistantdata.FinanceInput{
		Accounts: accounts, Transactions: txns, Categories: categories, TxnLinks: links,
		Now: s.now, Scope: scope.ReportScope{}, Base: s.base, Rates: s.rates,
	})
	window, err := finance.ResolveWindow("this_month", "", "")
	if err != nil {
		s.fail(err)
		return 0
	}
	result, err := finance.Category(window, category)
	if err != nil {
		s.fail(err)
		return 0
	}
	return result.Amount.Amount
}

func (s *insightsQASource) UpcomingBillsMinor() (int, int64) {
	if err := s.begin(); err != nil {
		s.fail(err)
		return 0, 0
	}
	accounts, err := s.accounts()
	if err != nil {
		s.fail(err)
		return 0, 0
	}
	recurring, err := s.recurring()
	if err != nil {
		s.fail(err)
		return 0, 0
	}
	var count int
	var total int64
	for _, bill := range bills.UpcomingAll(accounts, recurring, s.now) {
		if bill.DaysUntil > 30 {
			continue
		}
		magnitude, err := bill.Amount.AbsChecked()
		if err != nil {
			s.fail(err)
			return 0, 0
		}
		converted, err := s.rates.Convert(magnitude, s.base)
		if err != nil {
			s.fail(err)
			return 0, 0
		}
		total, err = money.Add(total, converted.Amount)
		if err != nil {
			s.fail(err)
			return 0, 0
		}
		count++
	}
	return count, total
}

func (s *insightsQASource) TopGoal() (string, int64, int64, bool) {
	if err := s.begin(); err != nil {
		s.fail(err)
		return "", 0, 0, false
	}
	goals, err := s.goals()
	if err != nil {
		s.fail(err)
		return "", 0, 0, false
	}
	accounts, err := s.accounts()
	if err != nil {
		s.fail(err)
		return "", 0, 0, false
	}
	txns, err := s.transactions()
	if err != nil {
		s.fail(err)
		return "", 0, 0, false
	}
	var best domain.Goal
	found := false
	for _, goal := range goals {
		if goal.Archived {
			continue
		}
		if !found || (!goal.TargetDate.IsZero() && (best.TargetDate.IsZero() || goal.TargetDate.Before(best.TargetDate))) ||
			(goal.TargetDate.Equal(best.TargetDate) && goal.ID < best.ID) {
			best, found = goal, true
		}
	}
	if !found {
		return "", 0, 0, false
	}
	current := best.CurrentAmount
	if best.AccountID != "" {
		matched := false
		for _, account := range accounts {
			if account.ID == best.AccountID {
				matched = true
				balance, err := ledger.Balance(account, txns)
				if err != nil {
					s.fail(err)
					return "", 0, 0, false
				}
				current, err = s.rates.Convert(balance, s.base)
				if err != nil {
					s.fail(err)
					return "", 0, 0, false
				}
				break
			}
		}
		if !matched {
			s.fail(fmt.Errorf("linked goal account is unavailable"))
			return "", 0, 0, false
		}
	} else {
		current, err = s.rates.Convert(current, s.base)
		if err != nil {
			s.fail(err)
			return "", 0, 0, false
		}
	}
	target, err := s.rates.Convert(best.TargetAmount, s.base)
	if err != nil {
		s.fail(err)
		return "", 0, 0, false
	}
	return best.Name, current.Amount, target.Amount, true
}

func (s *insightsQASource) HealthScore() (int, string, bool) {
	if err := s.begin(); err != nil {
		s.fail(err)
		return 0, "", false
	}
	accounts, err := s.accounts()
	if err != nil {
		s.fail(err)
		return 0, "", false
	}
	txns, err := s.transactions()
	if err != nil {
		s.fail(err)
		return 0, "", false
	}
	links, err := s.app.Store().ListTxnLinks()
	if err != nil {
		s.fail(fmt.Errorf("read transaction links: %w", err))
		return 0, "", false
	}
	budgets, err := s.app.Store().ListBudgets()
	if err != nil {
		s.fail(fmt.Errorf("read budgets: %w", err))
		return 0, "", false
	}
	categories, err := s.app.CategoriesChecked()
	if err != nil {
		s.fail(err)
		return 0, "", false
	}
	financialPrefs, _, err := assistantDataPreferences(s.app.Store())
	if err != nil {
		s.fail(err)
		return 0, "", false
	}
	in := engineenv.HealthInputs(engineenv.Data{Ledger: budgeting.NewNettedLedger(txns, links), Accounts: accounts, Transactions: txns,
		Budgets: budgets, Categories: categories, WeekStart: financialPrefs.WeekStartWeekday(), Rates: s.rates, Now: s.now})
	result := healthscore.Evaluate(in)
	if result.Band == healthscore.BandNoData {
		return 0, "", false
	}
	return result.Score, string(result.Band), true
}

func (s *insightsQASource) BudgetStatus() (int, int, string, int64, bool) {
	if err := s.begin(); err != nil {
		s.fail(err)
		return 0, 0, "", 0, false
	}
	budgets, err := s.app.Store().ListBudgets()
	if err != nil {
		s.fail(fmt.Errorf("read budgets: %w", err))
		return 0, 0, "", 0, false
	}
	if len(budgets) == 0 {
		return 0, 0, "", 0, false
	}
	accounts, err := s.accounts()
	if err != nil {
		s.fail(err)
		return len(budgets), 0, "", 0, false
	}
	txns, err := s.transactions()
	if err != nil {
		s.fail(err)
		return len(budgets), 0, "", 0, false
	}
	recurring, err := s.recurring()
	if err != nil {
		s.fail(err)
		return len(budgets), 0, "", 0, false
	}
	categories, err := s.app.CategoriesChecked()
	if err != nil {
		s.fail(err)
		return len(budgets), 0, "", 0, false
	}
	links, err := s.app.Store().ListTxnLinks()
	if err != nil {
		s.fail(fmt.Errorf("read transaction links: %w", err))
		return len(budgets), 0, "", 0, false
	}
	financialPrefs, _, err := assistantDataPreferences(s.app.Store())
	if err != nil {
		s.fail(err)
		return len(budgets), 0, "", 0, false
	}
	anchor := time.Time{}
	if financialPrefs.PayCycleAnchor != "" {
		anchor, err = dateutil.ParseDate(financialPrefs.PayCycleAnchor)
		if err != nil {
			s.fail(fmt.Errorf("read pay-cycle anchor: %w", err))
			return len(budgets), 0, "", 0, false
		}
	}
	finance := assistantdata.BuildFinance(assistantdata.FinanceInput{
		Accounts: accounts, Transactions: txns, Categories: categories, Budgets: budgets,
		Recurring: recurring, TxnLinks: links, Now: s.now, Scope: scope.ReportScope{}, Base: s.base, Rates: s.rates,
		WeekStart: financialPrefs.WeekStartWeekday(), PayCycleAnchor: anchor, PoolRollover: financialPrefs.BudgetRolloverLeftover,
	})
	window, err := finance.ResolveWindow("this_month", "", "")
	if err != nil {
		s.fail(err)
		return len(budgets), 0, "", 0, false
	}
	facts := finance.Budgets(window)
	catName := make(map[string]string, len(categories))
	for _, c := range categories {
		catName[c.ID] = c.Name
	}
	over := 0
	var worst int64
	var worstName string
	for _, fact := range facts {
		st := fact.Status
		if fact.Error != nil || st.Error != nil {
			if fact.Error != nil {
				s.fail(fact.Error)
			} else {
				s.fail(st.Error)
			}
			return len(facts), 0, "", 0, false
		}
		if st.State != budgeting.StateOver {
			continue
		}
		over++
		amount := st.Remaining
		if !st.Budget.IsSaving() {
			amount, err = amount.NegChecked()
			if err != nil {
				s.fail(err)
				return len(facts), 0, "", 0, false
			}
		}
		converted, err := s.rates.Convert(amount, s.base)
		if err != nil {
			s.fail(err)
			return len(facts), 0, "", 0, false
		}
		if converted.Amount > worst {
			worst, worstName = converted.Amount, budgetDisplayName(st, catName)
		}
	}
	return len(facts), over, worstName, worst, true
}

func (s *insightsQASource) RecentTransactions() []localqa.RecentTxn {
	if err := s.begin(); err != nil {
		s.fail(err)
		return nil
	}
	txns, err := s.transactions()
	if err != nil {
		s.fail(err)
		return nil
	}
	sort.SliceStable(txns, func(i, j int) bool {
		if txns[i].Date.Equal(txns[j].Date) {
			return txns[i].ID < txns[j].ID
		}
		return txns[i].Date.After(txns[j].Date)
	})
	out := make([]localqa.RecentTxn, 0, len(txns))
	for _, txn := range txns {
		if txn.IsTransfer() {
			continue
		}
		magnitude, err := txn.Amount.AbsChecked()
		if err != nil {
			s.fail(err)
			return nil
		}
		converted, err := s.rates.Convert(magnitude, s.base)
		if err != nil {
			s.fail(err)
			return nil
		}
		out = append(out, localqa.RecentTxn{Payee: txnDisplayPayee(txn), AmountMinor: converted.Amount})
	}
	return out
}

func (s *insightsQASource) Subscriptions() (int, int64) {
	if err := s.begin(); err != nil {
		s.fail(err)
		return 0, 0
	}
	rows, err := s.recurring()
	if err != nil {
		s.fail(err)
		return 0, 0
	}
	count := 0
	var total int64
	for _, row := range rows {
		if row.Paused || row.Amount.Amount >= 0 {
			continue
		}
		magnitude, err := row.Amount.AbsChecked()
		if err != nil {
			s.fail(err)
			return 0, 0
		}
		num, den := int64(1), int64(1)
		switch row.Cadence {
		case domain.CadenceWeekly:
			num, den = 52, 12
		case domain.CadenceBiweekly:
			num, den = 26, 12
		case domain.CadenceSemimonthly:
			num = 2
		case domain.CadenceQuarterly:
			den = 3
		case domain.CadenceYearly:
			den = 12
		}
		monthly, err := magnitude.MulDiv(num, den, money.RoundTowardZero)
		if err != nil {
			s.fail(err)
			return 0, 0
		}
		converted, err := s.rates.Convert(monthly, s.base)
		if err != nil {
			s.fail(err)
			return 0, 0
		}
		total, err = money.Add(total, converted.Amount)
		if err != nil {
			s.fail(err)
			return 0, 0
		}
		count++
	}
	return count, total
}

func (s *insightsQASource) LargestExpense() (string, int64, bool) {
	if err := s.begin(); err != nil {
		s.fail(err)
		return "", 0, false
	}
	txns, err := s.transactions()
	if err != nil {
		s.fail(err)
		return "", 0, false
	}
	start, end := dateutil.MonthRange(s.now)
	var payee string
	var largest int64
	found := false
	for _, txn := range txns {
		if !txn.IsExpense() || txn.IsTransfer() || !txn.CountsInReports() || !dateutil.InRange(txn.Date, start, end) {
			continue
		}
		magnitude, err := txn.Amount.AbsChecked()
		if err != nil {
			s.fail(err)
			return "", 0, false
		}
		converted, err := s.rates.Convert(magnitude, s.base)
		if err != nil {
			s.fail(err)
			return "", 0, false
		}
		if !found || converted.Amount > largest {
			largest, payee, found = converted.Amount, txnDisplayPayee(txn), true
		}
	}
	return payee, largest, found
}

func insightsMoneyFmt(minor int64, base string) string { return fmtMoney(money.New(minor, base)) }

func txnDisplayPayee(t domain.Transaction) string {
	if p := strings.TrimSpace(t.Payee); p != "" {
		return p
	}
	if d := strings.TrimSpace(t.Desc); d != "" {
		return d
	}
	return "an unnamed transaction"
}

func budgetDisplayName(st budgeting.Status, catName map[string]string) string {
	if name := strings.TrimSpace(st.Budget.Name); name != "" {
		return name
	}
	if name := strings.TrimSpace(catName[st.Budget.CategoryID]); name != "" {
		return name
	}
	return "a budget"
}

func max64(v, floor int64) int64 {
	if v < floor {
		return floor
	}
	return v
}
