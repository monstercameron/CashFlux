// SPDX-License-Identifier: MIT

package networthview

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/attribution"
	"github.com/monstercameron/CashFlux/internal/balancesheet"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// ErrInvalidWindow reports a months value outside the supported window choices.
var ErrInvalidWindow = fmt.Errorf("networthview: invalid window")

// Input is the complete, explicit input to the Net Worth prepared model.
// Months is 1, 6, 12, 24, or 0 for all available history. Now and the adjustment
// classifier are passed by the caller so the model is deterministic and native.
type Input struct {
	Accounts     []domain.Account
	Transactions []domain.Transaction
	Rates        currency.Rates
	Months       int
	Now          time.Time
	IsAdjustment func(domain.Transaction) bool
}

// AccountFact is an account with its base-currency standing and selected-window
// facts. It includes every non-archived account, including chosen exclusions,
// because the account list explains the household's full balance sheet.
type AccountFact struct {
	Account       domain.Account
	Bucket        balancesheet.Bucket
	AsOf          time.Time
	Window        AccountWindow
	SideMagnitude uint64
}

// ExpenseFact reports the average spending across the previous three complete
// calendar months. Known distinguishes a true zero from a failed calculation.
type ExpenseFact struct {
	Amount money.Money
	Known  bool
	Err    error
}

// Prepared contains independently available Net Worth facts. Each engine keeps
// its own error so an unavailable bridge does not erase a valid snapshot, etc.
type Prepared struct {
	Base         string
	Months       int
	SpanMonths   int
	Since, Until time.Time
	Snapshot     Snapshot
	SnapshotErr  error
	Bridge       attribution.Bridge
	BridgeErr    error
	Points       []balancesheet.Point
	SeriesErr    error
	Cutoffs      []time.Time
	Accounts     []AccountFact
	Assets       []AccountFact
	Liabilities  []AccountFact
	Movers       []AccountFact
	Expense      ExpenseFact
	Cash         money.Money
	CashKnown    bool
	Health       balancesheet.Health
	HealthErr    error
	HealthKnown  bool
	RunwayKnown  bool
}

// Prepare builds all platform-independent Net Worth facts for the selected
// window. Every input slice is read-only. The engines' availability and errors
// stay independent in the returned model.
func Prepare(in Input) (Prepared, error) {
	if in.Now.IsZero() {
		return Prepared{}, fmt.Errorf("%w: reference time is required", ErrInvalidWindow)
	}
	if in.Months != 0 && in.Months != 1 && in.Months != 6 && in.Months != 12 && in.Months != 24 {
		return Prepared{}, fmt.Errorf("%w: %d months", ErrInvalidWindow, in.Months)
	}
	base := in.Rates.Base
	if base == "" {
		base = "USD"
	}
	rates := in.Rates
	rates.Base = base
	curMonth := dateutil.MonthStart(in.Now)
	span := in.Months
	since := time.Time{}
	if in.Months == 0 {
		since = dateutil.MonthStart(earliestRecord(in.Transactions, in.Now))
		span = monthsBetween(since, curMonth) + 1
	} else {
		since = dateutil.AddMonths(curMonth, -(in.Months - 1))
	}
	until := dateutil.DayStart(in.Now).AddDate(0, 0, 1)
	p := Prepared{Base: base, Months: in.Months, SpanMonths: span, Since: since, Until: until}
	p.Snapshot = Build(in.Accounts, in.Transactions, rates)
	p.SnapshotErr = p.Snapshot.Err
	p.Bridge, p.BridgeErr = attribution.BuildBridge(attribution.Input{
		Accounts: in.Accounts, Txns: in.Transactions, Rates: rates,
		Since: since, Until: until, IsAdjustment: in.IsAdjustment,
	})
	cutoffs := makeCutoffs(since, until, span)
	p.Cutoffs = append([]time.Time(nil), cutoffs...)
	checkedSeries, checkedErr := ledger.NetWorthSeries(in.Accounts, in.Transactions, cutoffs, rates)
	p.Points, p.SeriesErr = balancesheet.Series(in.Accounts, in.Transactions, cutoffs, rates)
	if checkedErr != nil {
		p.Points, p.SeriesErr = nil, checkedErr
	} else if p.SeriesErr == nil {
		if len(checkedSeries) != len(p.Points) {
			p.Points, p.SeriesErr = nil, fmt.Errorf("networthview: checked ledger series length %d differs from balance-sheet series length %d", len(checkedSeries), len(p.Points))
		}
		for i := range checkedSeries {
			if p.SeriesErr != nil {
				break
			}
			if p.Points[i].NetMinor != checkedSeries[i].Amount {
				p.Points, p.SeriesErr = nil, fmt.Errorf("networthview: balance-sheet series differs from checked ledger series at point %d", i)
				break
			}
		}
	}
	accountsByID := make(map[string]domain.Account, len(in.Accounts))
	for _, account := range in.Accounts {
		accountsByID[account.ID] = account
	}
	for _, window := range AccountWindows(in.Accounts, in.Transactions, rates, since, until, in.IsAdjustment) {
		if !window.BaseBalanceKnown() {
			continue
		}
		account, ok := accountsByID[window.AccountID]
		if !ok || account.Archived {
			continue
		}
		magnitude := uint64(window.BaseBalance.Amount)
		if window.BaseBalance.Amount < 0 {
			magnitude = uint64(-(window.BaseBalance.Amount + 1)) + 1
		}
		p.Accounts = append(p.Accounts, AccountFact{
			Account: account, Bucket: balancesheet.BucketOf(account), AsOf: account.BalanceAsOf,
			Window: window, SideMagnitude: magnitude,
		})
	}
	sortAccountFacts(p.Accounts)
	for _, account := range p.Accounts {
		switch account.Account.Class {
		case domain.ClassAsset:
			p.Assets = append(p.Assets, account)
		case domain.ClassLiability:
			p.Liabilities = append(p.Liabilities, account)
		}
		if !account.Window.WindowKnown() || account.Window.Move.Amount != 0 {
			p.Movers = append(p.Movers, account)
		}
	}
	sort.SliceStable(p.Movers, func(i, j int) bool {
		return magnitude(p.Movers[i].Window.Move.Amount) > magnitude(p.Movers[j].Window.Move.Amount)
	})
	p.Expense = typicalMonthlyExpense(in.Transactions, rates, in.Now)
	if p.SeriesErr == nil && len(p.Points) > 0 {
		last := p.Points[len(p.Points)-1]
		p.Cash, p.CashKnown = money.New(last.Assets[balancesheet.BucketCash], base), true
		if p.Expense.Known {
			p.Health, p.HealthErr = assessChecked(last.AssetsMinor, last.LiabilitiesMinor, p.Cash.Amount, p.Expense.Amount.Amount)
			p.HealthKnown = p.HealthErr == nil
			p.RunwayKnown = p.HealthKnown && p.Health.RunwayOK
		} else {
			p.Health, p.HealthErr = assessChecked(last.AssetsMinor, last.LiabilitiesMinor, p.Cash.Amount, 0)
			p.HealthKnown = p.HealthErr == nil
			p.RunwayKnown = false
		}
	}
	return p, nil
}

func earliestRecord(transactions []domain.Transaction, now time.Time) time.Time {
	earliest := now
	for _, transaction := range transactions {
		if !transaction.Date.IsZero() && transaction.Date.Before(earliest) {
			earliest = transaction.Date
		}
	}
	return earliest
}

func monthsBetween(a, b time.Time) int {
	months := int(b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
	if months < 0 {
		return 0
	}
	return months
}

func makeCutoffs(since, until time.Time, span int) []time.Time {
	step := 1
	if span > 36 {
		step = (span + 35) / 36
	}
	cutoffs := make([]time.Time, 0, span/step+2)
	for k := 0; k <= span-1; k += step {
		cutoffs = append(cutoffs, dateutil.AddMonths(since, k))
	}
	return append(cutoffs, until)
}

func sortAccountFacts(accounts []AccountFact) {
	sort.SliceStable(accounts, func(i, j int) bool { return accounts[i].SideMagnitude > accounts[j].SideMagnitude })
}

func magnitude(value int64) uint64 {
	if value < 0 {
		return uint64(-(value + 1)) + 1
	}
	return uint64(value)
}

func typicalMonthlyExpense(transactions []domain.Transaction, rates currency.Rates, now time.Time) ExpenseFact {
	const months = 3
	end := dateutil.MonthStart(now)
	start := dateutil.AddMonths(end, -months)
	_, expense, err := ledger.PeriodTotals(transactions, start, end, rates)
	if err != nil {
		return ExpenseFact{Err: err}
	}
	if expense.Amount > 0 {
		expense.Amount /= months
	}
	return ExpenseFact{Amount: expense, Known: true}
}

func assessChecked(assets, liabilities, cash, monthlyExpense int64) (balancesheet.Health, error) {
	if assets > 0 {
		if _, err := money.Mul(cash, 100); err != nil {
			return balancesheet.Health{}, fmt.Errorf("networthview: liquid share: %w", err)
		}
		if _, err := money.Mul(liabilities, 100); err != nil {
			return balancesheet.Health{}, fmt.Errorf("networthview: debt-to-asset: %w", err)
		}
	}
	if monthlyExpense > 0 && cash >= 0 {
		if _, err := money.Mul(cash, 10); err != nil {
			return balancesheet.Health{}, fmt.Errorf("networthview: runway: %w", err)
		}
	}
	return balancesheet.Assess(assets, liabilities, cash, monthlyExpense), nil
}
