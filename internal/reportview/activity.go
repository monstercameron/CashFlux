// SPDX-License-Identifier: MIT

package reportview

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
)

// ActivityInput supplies the resolved report scope, current and comparison
// windows, and household metadata needed to classify recurring charges.
// Series must be built over Bounds. Now is explicit for daily report readings.
type ActivityInput struct {
	Series               Series
	Bounds               []time.Time
	PriorStart, PriorEnd time.Time
	Rates                currency.Rates
	Categories           []domain.Category
	HouseholdAccounts    []domain.Account
	Recurring            []domain.Recurring
	ConfirmedSubs        map[string]bool
	Now                  time.Time
}

// Activity is the report's expensive, unformatted activity picture. UI posture
// (summary/full, export popover, formulas and category rollup) is not an input,
// so a single memoized picture serves each posture without rescanning the ledger.
type Activity struct {
	// Error identifies an incomplete activity picture. Independent sections remain usable.
	Error                   error
	Spending                []reports.CategorySpend
	CategoryTrends          []reports.CategoryTrend
	PayeeTrends             map[string][]int64
	NetWorthSeries          []money.Money
	NetWorth                money.Money
	NetWorthChange          int64
	Costs                   reports.MoneyCosts
	Runway                  reports.Runway
	Payees                  []reports.PayeeTotal
	LargestExpenses         []reports.ExpenseItem
	LargestIncome           []reports.ExpenseItem
	Income                  []reports.CategorySpend
	Members                 []reports.MemberSpend
	Tags                    []reports.TagSpend
	SpendStats              reports.SpendStats
	NoSpendDays             int
	Weekday                 [7]int64
	Subscriptions           []subscriptions.Subscription
	SubscriptionAnnualMinor int64
	PriceRises              []subscriptions.PriceChange
	MonthsNegative          int
	HighMonth, LowMonth     int
	HasSeasonality          bool
	Trims                   []reports.TrimTarget
}

// BuildActivity delegates each financial rule to the existing reporting core.
// Errors identify unavailable sections while leaving independent sections usable;
// this preserves each core function's exclusions, refund and rounding semantics.
// The returned slices/maps are read-only and safe to retain in a view memo.
func BuildActivity(in ActivityInput) (Activity, error) {
	var out Activity
	if len(in.Bounds) < 2 {
		return out, nil
	}
	start, end := in.Bounds[0], in.Bounds[len(in.Bounds)-1]
	txns, accounts := in.Series.Transactions, in.Series.Accounts
	var failures []error
	keep := func(section string, err error) {
		if err != nil {
			failures = append(failures, fmt.Errorf("report %s: %w", section, err))
		}
	}
	var err error
	out.Spending, err = reports.SpendingByCategory(txns, start, end, true, in.PriorStart, in.PriorEnd, in.Rates)
	keep("categories", err)
	out.CategoryTrends, err = reports.CategoryTrends(txns, in.Bounds, in.Rates)
	keep("category trends", err)
	payees, err := reports.PayeeTrends(txns, in.Bounds, in.Rates, 10)
	keep("payee trends", err)
	out.PayeeTrends = make(map[string][]int64, len(payees))
	for _, trend := range payees {
		out.PayeeTrends[strings.ToLower(strings.TrimSpace(trend.Payee))] = trend.Spend
	}
	out.NetWorthSeries, err = ledger.NetWorthSeries(accounts, txns, in.Bounds, in.Rates)
	keep("net worth series", err)
	out.NetWorth, _, _, err = ledger.NetWorth(accounts, txns, in.Rates)
	keep("net worth", err)
	if n := len(out.NetWorthSeries); n >= 2 {
		out.NetWorthChange, err = money.Sub(out.NetWorthSeries[n-1].Amount, out.NetWorthSeries[0].Amount)
		keep("net worth movement", err)
	}
	out.Costs, err = reports.CostOfMoney(txns, in.Categories, start, end, in.Rates)
	keep("cost of money", err)
	liquid, err := ledger.LiquidBalance(accounts, txns, in.Rates)
	keep("liquid balance", err)
	flows := in.Series.MonthFlows
	if len(flows) > 6 {
		flows = flows[len(flows)-6:]
	}
	_, burn, _, err := reports.MonthlyAveragesChecked(flows)
	keep("monthly burn", err)
	out.Runway = reports.EstimateRunway(liquid.Amount, burn)
	out.Payees, err = reports.TopPayees(txns, start, end, in.Rates, 10)
	keep("top payees", err)
	out.LargestExpenses, err = reports.LargestExpenses(txns, start, end, in.Rates, 10)
	keep("largest expenses", err)
	out.LargestIncome, err = reports.LargestIncome(txns, start, end, in.Rates, 8)
	keep("largest income", err)
	out.Income, err = reports.IncomeByCategory(txns, start, end, in.Rates)
	keep("income sources", err)
	out.Members, err = reports.SpendingByMember(txns, start, end, in.Rates)
	keep("member spending", err)
	out.Tags, err = reports.SpendingByTag(txns, start, end, true, in.PriorStart, in.PriorEnd, in.Rates)
	keep("tag spending", err)
	out.SpendStats, err = reports.SpendingStats(txns, start, end, in.Rates)
	keep("spending statistics", err)
	out.NoSpendDays = reports.NoSpendDays(txns, start, end, in.Now)
	out.Weekday, err = reports.SpendingByWeekday(txns, start, end, in.Rates)
	keep("weekday spending", err)
	keep("subscriptions", buildActivitySubscriptions(&out, in))
	out.MonthsNegative = reports.MonthsNegative(in.Series.MonthFlows)
	partial := -1
	for i := 0; i+1 < len(in.Bounds); i++ {
		if dateutil.InRange(in.Now, in.Bounds[i], in.Bounds[i+1]) {
			partial = i
			break
		}
	}
	out.HighMonth, out.LowMonth, out.HasSeasonality = reports.SeasonalExtremesSkipping(in.Series.MonthFlows, partial)
	out.Trims = reports.TrimTargets(out.CategoryTrends, 2500, 3)
	out.Error = errors.Join(failures...)
	return out, out.Error
}

func buildActivitySubscriptions(out *Activity, in ActivityInput) error {
	planned := make(map[string]bool, len(in.Recurring))
	for _, recurring := range in.Recurring {
		if key := strings.ToLower(strings.TrimSpace(recurring.Label)); key != "" {
			planned[key] = true
		}
	}
	names := make(map[string]string, len(in.Categories))
	for _, category := range in.Categories {
		names[category.ID] = category.Name
	}
	nameOf := func(id string) string { return names[id] }
	txns := in.Series.Transactions
	subs, subErr := subscriptions.Detect(txns, in.Rates, 3)
	levels := make(map[string]subscriptions.Confidence, len(subs))
	for _, sub := range subs {
		level := subscriptions.Assess(sub, in.ConfirmedSubs).Level
		levels[subscriptions.ConfirmKey(sub.Name)] = level
		if sub.Lapsed(in.Now) || level == subscriptions.ConfidenceReview || !subscriptions.IsRealSubscriptionName(sub.Name, txns, in.HouseholdAccounts, nameOf, planned) {
			continue
		}
		out.Subscriptions = append(out.Subscriptions, sub)
	}
	sort.SliceStable(out.Subscriptions, func(i, j int) bool { return out.Subscriptions[i].AnnualAmount() > out.Subscriptions[j].AnnualAmount() })
	var totalErr error
	for _, sub := range out.Subscriptions {
		out.SubscriptionAnnualMinor, totalErr = money.Add(out.SubscriptionAnnualMinor, sub.AnnualAmount())
		if totalErr != nil {
			out.SubscriptionAnnualMinor = 0
			break
		}
	}
	changes, priceErr := subscriptions.DetectPriceChanges(txns, in.Rates, 3)
	for _, change := range changes {
		if change.Delta > 0 && levels[subscriptions.ConfirmKey(change.Name)] != subscriptions.ConfidenceReview && subscriptions.IsRealSubscriptionName(change.Name, txns, in.HouseholdAccounts, nameOf, planned) {
			out.PriceRises = append(out.PriceRises, change)
		}
	}
	return errors.Join(subErr, priceErr, totalErr)
}
