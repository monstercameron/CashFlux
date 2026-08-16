// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

// insightsQASource implements localqa.Source using the live financial data that
// is already computed at the top of Insights(). It keeps the adapter close to
// the call-site so there are no circular imports; all the heavy lifting
// (LiquidBalance, NetWorth, category aggregation, goals, health score) reuses
// the same tested helpers already used on the screen.

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	"github.com/monstercameron/CashFlux/internal/insights/localqa"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/safespend"
)

// insightsQASource satisfies localqa.Source using live appstate data.
type insightsQASource struct {
	app   *appstate.App
	now   time.Time
	base  string
	rates currency.Rates
}

// newInsightsQASource constructs the adapter. base and rates must match those
// already computed by the Insights() component so money figures are consistent.
func newInsightsQASource(app *appstate.App, base string, rates currency.Rates) *insightsQASource {
	return &insightsQASource{app: app, now: time.Now(), base: base, rates: rates}
}

// LiquidBalanceMinor returns total liquid (checking + savings) balance in minor units.
func (s *insightsQASource) LiquidBalanceMinor() int64 {
	liq, err := ledger.LiquidBalance(s.app.Accounts(), s.app.Transactions(), s.rates)
	if err != nil {
		return 0
	}
	return liq.Amount
}

// NetWorthMinor returns (assetsMinor, liabilitiesMinor) both in minor units.
func (s *insightsQASource) NetWorthMinor() (int64, int64) {
	_, assets, liabilities, err := ledger.NetWorth(s.app.Accounts(), s.app.Transactions(), s.rates)
	if err != nil {
		return 0, 0
	}
	return assets.Amount, liabilities.Amount
}

// SafeToSpendMinor computes liquid cash minus upcoming bills and goal
// contributions, using the same approach as the Planning screen.
func (s *insightsQASource) SafeToSpendMinor() int64 {
	liq, err := ledger.LiquidBalance(s.app.Accounts(), s.app.Transactions(), s.rates)
	if err != nil {
		return 0
	}
	toBase := safespend.ToBaseFunc(s.rates)
	_, mEnd := dateutil.MonthRange(s.now)
	billsDue := safespend.BillsDueBefore(s.app.Accounts(), s.app.Recurring(), s.now, mEnd, toBase)
	goalNeeds := safespend.GoalContributionsProrated(s.app.Goals(), s.now, toBase)
	return safespend.Compute(liq.Amount, billsDue, goalNeeds, 0, s.base).SafeToSpend
}

// SpendingOnCategoryMinor returns month-to-date spend in the named category
// (case-insensitive partial name match, current month only).
func (s *insightsQASource) SpendingOnCategoryMinor(category string) int64 {
	mStart, mEnd := dateutil.MonthRange(s.now)
	catLower := strings.ToLower(strings.TrimSpace(category))
	// First category whose lowercased name contains the query substring.
	matchedID := ""
	for _, c := range s.app.Categories() {
		if strings.Contains(strings.ToLower(c.Name), catLower) {
			matchedID = c.ID
			break
		}
	}
	if matchedID == "" {
		return 0
	}
	var total int64
	for _, t := range s.app.Transactions() {
		if t.CategoryID != matchedID || !t.IsExpense() {
			continue
		}
		if !dateutil.InRange(t.Date, mStart, mEnd) {
			continue
		}
		conv, err := s.rates.Convert(t.Amount.Abs(), s.base)
		if err != nil {
			conv = t.Amount.Abs()
		}
		total += conv.Amount
	}
	return total
}

// UpcomingBillsMinor returns the count and combined total of bills due within
// 30 days using the same bills.UpcomingAll derivation as the dashboard widget.
func (s *insightsQASource) UpcomingBillsMinor() (int, int64) {
	toBase := safespend.ToBaseFunc(s.rates)
	const horizonDays = 30
	var count int
	var total int64
	for _, b := range bills.UpcomingAll(s.app.Accounts(), s.app.Recurring(), s.now) {
		if b.DaysUntil > horizonDays {
			continue
		}
		count++
		total += toBase(b.Amount.Amount, b.Amount.Currency)
	}
	return count, total
}

// TopGoal returns the highest-priority active goal (earliest target date) with
// its name, current saved amount, and target — all in minor units.
func (s *insightsQASource) TopGoal() (name string, currentMinor, targetMinor int64, ok bool) {
	var bestName string
	var bestCurrent, bestTarget int64
	var bestDate time.Time
	found := false
	for _, g := range s.app.Goals() {
		if g.Archived {
			continue
		}
		current := g.CurrentAmount.Amount
		// If the goal is linked to an account, use the live ledger balance as the
		// current saved amount so the answer reflects the actual account value.
		if g.AccountID != "" {
			for _, a := range s.app.Accounts() {
				if a.ID != g.AccountID {
					continue
				}
				if bal, err := ledger.Balance(a, s.app.Transactions()); err == nil {
					if conv, cerr := s.rates.Convert(bal, s.base); cerr == nil {
						current = conv.Amount
					}
				}
				break
			}
		}
		date := g.TargetDate
		if !found || (!date.IsZero() && (bestDate.IsZero() || date.Before(bestDate))) {
			bestName = g.Name
			bestCurrent = current
			bestTarget = g.TargetAmount.Amount
			bestDate = date
			found = true
		}
	}
	if !found {
		return "", 0, 0, false
	}
	return bestName, bestCurrent, bestTarget, true
}

// HealthScore delegates to healthscore.Evaluate, re-using liveHealthInputs
// from health.go (same package — no extra import needed).
func (s *insightsQASource) HealthScore() (score int, band string, ok bool) {
	in := liveHealthInputs(s.app, s.now)
	result := healthscore.Evaluate(in)
	if result.Band == healthscore.BandNoData {
		return 0, "", false
	}
	return result.Score, string(result.Band), true
}

// insightsMoneyFmt wraps fmtMoney for a plain minor-unit + currency pair, used
// as the fmtMoney closure supplied to localqa.Answer inside sendText.
func insightsMoneyFmt(minor int64, base string) string {
	return fmtMoney(money.Money{Amount: minor, Currency: base})
}

// BudgetStatus reports how the household's budgets are doing this month: how many
// exist, how many are over, and which one is furthest over (G2-C8). The overspend
// is what the answer leads with, because "over by $4" and "over by $400" are
// different situations that a count alone flattens.
func (s *insightsQASource) BudgetStatus() (total, over int, worstName string, worstOverMinor int64, ok bool) {
	budgets := s.app.Budgets()
	if len(budgets) == 0 {
		return 0, 0, "", 0, false
	}
	mStart, mEnd := dateutil.MonthRange(s.now)
	statuses, err := budgeting.EvaluateAll(budgets, s.app.Transactions(), mStart, mEnd, s.rates, budgetNearThreshold)
	if err != nil {
		return 0, 0, "", 0, false
	}
	catName := map[string]string{}
	for _, c := range s.app.Categories() {
		catName[c.ID] = c.Name
	}
	for _, st := range statuses {
		if st.State != budgeting.StateOver {
			continue
		}
		over++
		// Remaining is negative once a budget is over, so the overspend is its
		// magnitude.
		amountOver := -st.Remaining.Amount
		if amountOver > worstOverMinor {
			worstOverMinor = amountOver
			worstName = budgetDisplayName(st, catName)
		}
	}
	return len(statuses), over, worstName, worstOverMinor, true
}

// budgetNearThreshold matches the fraction the budgets screen uses for its "close
// to the limit" state, so a keyless answer and the screen never disagree about
// which budgets are over.
const budgetNearThreshold = 0.9

// budgetDisplayName names a budget the way its screen does: by its own name when
// it has one, otherwise by the category it governs.
func budgetDisplayName(st budgeting.Status, catName map[string]string) string {
	if n := strings.TrimSpace(st.Budget.Name); n != "" {
		return n
	}
	if n := strings.TrimSpace(catName[st.Budget.CategoryID]); n != "" {
		return n
	}
	return "a budget"
}

// RecentTransactions returns the newest transactions, newest first, converted to
// the base currency so the figures in an answer are comparable with each other.
func (s *insightsQASource) RecentTransactions() []localqa.RecentTxn {
	txns := s.app.Transactions()
	sorted := make([]domain.Transaction, 0, len(txns))
	for _, t := range txns {
		if t.IsTransfer() {
			// A transfer moved money between the household's own accounts; listing
			// it as something they bought would be wrong.
			continue
		}
		sorted = append(sorted, t)
	}
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date.After(sorted[j].Date) })
	out := make([]localqa.RecentTxn, 0, len(sorted))
	for _, t := range sorted {
		out = append(out, localqa.RecentTxn{
			Payee:       txnDisplayPayee(t),
			AmountMinor: s.toBaseMinor(t.Amount.Abs()),
		})
	}
	return out
}

// txnDisplayPayee names a transaction the way the ledger does: its payee, falling
// back to its description.
func txnDisplayPayee(t domain.Transaction) string {
	if p := strings.TrimSpace(t.Payee); p != "" {
		return p
	}
	if d := strings.TrimSpace(t.Desc); d != "" {
		return d
	}
	return "an unnamed transaction"
}

// Subscriptions counts the active recurring charges and their combined monthly
// cost. It counts the household's own recurring SCHEDULE rather than guessing at
// subscriptions from transaction patterns: a keyless answer should be a fact about
// what was set up, not an inference presented as one.
func (s *insightsQASource) Subscriptions() (count int, monthlyMinor int64) {
	for _, r := range s.app.Recurring() {
		if r.Paused || r.Amount.Amount >= 0 {
			// A paused schedule is not currently costing anything, and an income
			// schedule was never a subscription.
			continue
		}
		count++
		monthly := r.MonthlyEquivalent()
		if monthly < 0 {
			monthly = -monthly
		}
		monthlyMinor += monthly
	}
	return count, monthlyMinor
}

// LargestExpense returns the biggest single expense of the current month.
func (s *insightsQASource) LargestExpense() (payee string, amountMinor int64, ok bool) {
	mStart, mEnd := dateutil.MonthRange(s.now)
	for _, t := range s.app.Transactions() {
		if !t.IsExpense() || t.IsTransfer() || !dateutil.InRange(t.Date, mStart, mEnd) {
			continue
		}
		amount := s.toBaseMinor(t.Amount.Abs())
		if amount > amountMinor {
			amountMinor, payee, ok = amount, txnDisplayPayee(t), true
		}
	}
	return payee, amountMinor, ok
}

// toBaseMinor converts an amount into the base currency, falling back to the raw
// figure when the rate table cannot (an unmixed single-currency household is the
// common case, where the two are the same).
func (s *insightsQASource) toBaseMinor(m money.Money) int64 {
	conv, err := s.rates.Convert(m, s.base)
	if err != nil {
		return m.Amount
	}
	return conv.Amount
}
