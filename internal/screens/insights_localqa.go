// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

// insightsQASource implements localqa.Source using the live financial data that
// is already computed at the top of Insights(). It keeps the adapter close to
// the call-site so there are no circular imports; all the heavy lifting
// (LiquidBalance, NetWorth, category aggregation, goals, health score) reuses
// the same tested helpers already used on the screen.

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/healthscore"
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

// HealthScore delegates to healthscore.Evaluate, re-using buildHealthInputs
// from health.go (same package — no extra import needed).
func (s *insightsQASource) HealthScore() (score int, band string, ok bool) {
	in := buildHealthInputs(s.app, s.now)
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
