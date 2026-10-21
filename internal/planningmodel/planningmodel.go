// SPDX-License-Identifier: MIT

// Package planningmodel prepares the checked, data-dependent facts shown on the
// Planning screen. Presentation and user-input state stay at the screen boundary.
package planningmodel

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/afford"
	"github.com/monstercameron/CashFlux/internal/cashflow"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/forecast"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/planning"
	"github.com/monstercameron/CashFlux/internal/runway"
	"github.com/monstercameron/CashFlux/internal/safespend"
)

const (
	ForecastMonths = 12
	RunwayDays     = 60
	TrailingMonths = 3
)

// Snapshot contains one read-only set of household facts. Read errors remain
// attached to their source so independent Planning facts can still be prepared.
type Snapshot struct {
	Accounts        []domain.Account
	AccountsErr     error
	Transactions    []domain.Transaction
	TransactionsErr error
	Recurring       []domain.Recurring
	RecurringErr    error
	Goals           []domain.Goal
	GoalsErr        error
}

// Inputs are the settings and user-entered values used for one preparation.
type Inputs struct {
	Now                   time.Time
	Base                  string
	FXRates               map[string]float64
	SettingsErr           error
	TrimMinor             *int64
	TrimErr               error
	ComparePlan           *domain.Plan
	ComparePlanErr        error
	Affordability         *AffordabilityInput
	AffordabilityInputErr error
	RunwayBufferMinor     int64
	RunwayBufferErr       error
}

// AffordabilityInput is a validated purchase check in base-currency minor units.
type AffordabilityInput struct {
	AmountMinor  int64
	Months       int
	ReserveMinor int64
}

// Fact carries availability and provenance along with a value. A zero Value is
// meaningful only when Available is true.
type Fact[T any] struct {
	Available bool
	Value     T
	Source    string
	Err       error
	Reason    string
	Partial   bool
	Notice    string
}

// ForecastSeries contains checked end-of-month balances for each projected month
// and the zero-based index of the first projected month below zero, when present.
type ForecastSeries struct {
	Balances      []int64
	DipMonth      int
	DipsBelowZero bool
}

// Prepared contains independently available facts for the Planning surface.
type Prepared struct {
	NetWorth           Fact[ledger.NetWorthResult]
	TrailingMonthlyNet Fact[int64]
	Forecast           Fact[ForecastSeries]
	TrimForecast       Fact[ForecastSeries]
	TrimEndChange      Fact[int64]
	CompareForecast    Fact[ForecastSeries]
	CompareEndChange   Fact[int64]
	CurrentMonthNet    Fact[int64]
	LiquidBalance      Fact[money.Money]
	BillsDue           Fact[int64]
	GoalContributions  Fact[int64]
	SafeToSpend        Fact[safespend.Breakdown]
	Affordability      Fact[afford.Result]
	Runway             Fact[cashflow.Projection]
	CoverSuggestion    Fact[runway.CoverSuggestion]
	DaysToNextInflow   Fact[int]
}

// Prepare computes the Planning screen's checked financial facts from a single
// snapshot and one reference time. It does not mutate inputs or household data.
func Prepare(snapshot Snapshot, in Inputs) Prepared {
	base := strings.ToUpper(strings.TrimSpace(in.Base))
	rates := currency.Rates{Base: base, Rates: in.FXRates}
	configErr := errors.Join(in.SettingsErr, validBase(base))
	result := Prepared{
		NetWorth:           unavailable[ledger.NetWorthResult]("ledger.NetWorthExplained", "net-worth inputs are unavailable", errors.Join(configErr, snapshot.AccountsErr, snapshot.TransactionsErr)),
		TrailingMonthlyNet: unavailable[int64]("reports.TrailingMonthlyNetChecked", "transaction history is unavailable", errors.Join(configErr, snapshot.TransactionsErr, validNow(in.Now))),
		CurrentMonthNet:    unavailable[int64]("ledger.PeriodTotals", "current-month cash flow is unavailable", errors.Join(configErr, snapshot.TransactionsErr, validNow(in.Now))),
		LiquidBalance:      unavailable[money.Money]("ledger.LiquidBalance", "liquid balance is unavailable", errors.Join(configErr, snapshot.AccountsErr, snapshot.TransactionsErr)),
		BillsDue:           unavailable[int64]("safespend.BillsDueBeforeChecked", "bill totals are unavailable", errors.Join(configErr, snapshot.AccountsErr, snapshot.RecurringErr, validNow(in.Now))),
		GoalContributions:  unavailable[int64]("safespend.GoalContributionsProratedChecked", "goal contributions are unavailable", errors.Join(configErr, snapshot.GoalsErr, validNow(in.Now))),
		Runway:             unavailable[cashflow.Projection]("runway.ProjectChecked", "runway is unavailable", errors.Join(configErr, snapshot.RecurringErr, in.RunwayBufferErr, validNow(in.Now))),
		CoverSuggestion:    unavailable[runway.CoverSuggestion]("runway.SuggestCoverChecked", "cover suggestion is unavailable", errors.Join(configErr, snapshot.AccountsErr, snapshot.TransactionsErr)),
		DaysToNextInflow:   unavailable[int]("runway.DaysToNextInflow", "next inflow is unavailable", errors.Join(configErr, snapshot.RecurringErr, validNow(in.Now))),
		Forecast:           unavailable[ForecastSeries]("forecast.ProjectChecked", "forecast inputs are unavailable", nil),
		TrimForecast:       unavailable[ForecastSeries]("forecast.ProjectChecked", "no positive trim amount was provided", in.TrimErr),
		TrimEndChange:      unavailable[int64]("money.Sub(trim end, baseline end)", "trim comparison is unavailable", in.TrimErr),
		CompareForecast:    unavailable[ForecastSeries]("planning.MonthlyNetChecked + forecast.ProjectChecked", "no comparison plan was selected", in.ComparePlanErr),
		CompareEndChange:   unavailable[int64]("money.Sub(comparison end, baseline end)", "plan comparison is unavailable", in.ComparePlanErr),
		SafeToSpend:        unavailable[safespend.Breakdown]("safespend.ComputeChecked", "safe-to-spend inputs are unavailable", nil),
		Affordability:      unavailable[afford.Result]("afford.CanAffordChecked", "no positive purchase amount was provided", in.AffordabilityInputErr),
	}
	if base == "" || !currency.Valid(base) {
		return result
	}
	if !in.Now.IsZero() {
		result.nowFacts(snapshot, in, rates)
	}
	result.forecasts(in)
	result.safeSpend(snapshot, in, rates, base)
	result.affordability(in)
	result.runwayFacts(snapshot, in, rates)
	return result
}

func (p *Prepared) nowFacts(snapshot Snapshot, in Inputs, rates currency.Rates) {
	if p.NetWorth.Err == nil {
		value, err := ledger.NetWorthExplained(snapshot.Accounts, snapshot.Transactions, rates)
		if err != nil {
			p.NetWorth = unavailable[ledger.NetWorthResult](p.NetWorth.Source, "net worth could not be calculated", err)
		} else {
			p.NetWorth = available(p.NetWorth.Source, value)
			p.NetWorth.Partial = len(value.MissingCurrencies) > 0
			if p.NetWorth.Partial {
				p.NetWorth.Notice = "Some account currencies could not be converted: " + strings.Join(value.MissingCurrencies, ", ")
			}
		}
	}
	if p.TrailingMonthlyNet.Err == nil {
		value, err := trailingMonthlyNetChecked(snapshot.Transactions, in.Now, rates, TrailingMonths)
		if err != nil {
			p.TrailingMonthlyNet = unavailableFact(p.TrailingMonthlyNet, "trailing monthly net could not be calculated", err)
		} else {
			p.TrailingMonthlyNet = available(p.TrailingMonthlyNet.Source, value)
		}
	}
	start, end := dateutil.MonthRange(in.Now)
	if p.CurrentMonthNet.Err == nil {
		income, expense, err := ledger.PeriodTotals(snapshot.Transactions, start, end, rates)
		if err == nil {
			var net int64
			net, err = money.Sub(income.Amount, expense.Amount)
			if err == nil {
				p.CurrentMonthNet = available(p.CurrentMonthNet.Source, net)
			}
		}
		if err != nil {
			p.CurrentMonthNet = unavailableFact(p.CurrentMonthNet, "current-month cash flow could not be calculated", err)
		}
	}
}

func (p *Prepared) forecasts(in Inputs) {
	if !p.NetWorth.Available || !p.TrailingMonthlyNet.Available {
		err := errors.Join(p.NetWorth.Err, p.TrailingMonthlyNet.Err)
		if err == nil {
			err = errors.New("forecast inputs are incomplete")
		}
		p.Forecast = unavailableFact(p.Forecast, "net worth and trailing monthly net are both required", err)
	} else {
		p.Forecast = projectSeries(p.Forecast.Source, p.NetWorth.Value.Net.Amount, p.TrailingMonthlyNet.Value, 0)
		p.Forecast.Partial = p.NetWorth.Partial || p.TrailingMonthlyNet.Partial
		p.Forecast.Notice = strings.TrimSpace(strings.Join([]string{p.NetWorth.Notice, p.TrailingMonthlyNet.Notice}, " "))
		p.TrimEndChange = unavailable[int64](p.TrimEndChange.Source, "trim comparison is unavailable", nil)
		p.CompareEndChange = unavailable[int64](p.CompareEndChange.Source, "plan comparison is unavailable", nil)
	}
	if in.TrimErr != nil {
		p.TrimForecast = unavailableFact(p.TrimForecast, "trim amount is invalid", in.TrimErr)
	} else if in.TrimMinor != nil && *in.TrimMinor > 0 {
		if !p.NetWorth.Available || !p.TrailingMonthlyNet.Available {
			p.TrimForecast = unavailableFact(p.TrimForecast, "net worth and trailing monthly net are both required", errors.Join(p.NetWorth.Err, p.TrailingMonthlyNet.Err))
		} else {
			p.TrimForecast = projectSeries(p.TrimForecast.Source, p.NetWorth.Value.Net.Amount, p.TrailingMonthlyNet.Value, *in.TrimMinor)
			p.TrimForecast.Partial = p.Forecast.Partial
			p.TrimForecast.Notice = p.Forecast.Notice
			if p.Forecast.Available && len(p.Forecast.Value.Balances) > 0 && len(p.TrimForecast.Value.Balances) > 0 {
				change, err := money.Sub(p.TrimForecast.Value.Balances[len(p.TrimForecast.Value.Balances)-1], p.Forecast.Value.Balances[len(p.Forecast.Value.Balances)-1])
				if err != nil {
					p.TrimEndChange = unavailableFact(p.TrimEndChange, "trim end comparison overflowed", err)
				} else {
					p.TrimEndChange = available(p.TrimEndChange.Source, change)
				}
			}
		}
	}
	if in.ComparePlanErr != nil {
		p.CompareForecast = unavailableFact(p.CompareForecast, "saved plans could not be read", in.ComparePlanErr)
	} else if in.ComparePlan != nil {
		if !p.NetWorth.Available {
			p.CompareForecast = unavailableFact(p.CompareForecast, "net worth is required for the comparison", p.NetWorth.Err)
		} else {
			monthly, err := planning.MonthlyNetChecked(*in.ComparePlan)
			if err != nil {
				p.CompareForecast = unavailableFact(p.CompareForecast, "comparison plan monthly net is unavailable", err)
			} else {
				// Preserve Planning's existing comparison policy: saved plans overlay
				// their recurring monthly net against the same net-worth baseline.
				p.CompareForecast = projectSeries(p.CompareForecast.Source, p.NetWorth.Value.Net.Amount, monthly, 0)
				p.CompareForecast.Partial = p.NetWorth.Partial
				p.CompareForecast.Notice = p.NetWorth.Notice
				if p.Forecast.Available && p.CompareForecast.Available && len(p.Forecast.Value.Balances) > 0 && len(p.CompareForecast.Value.Balances) > 0 {
					change, err := money.Sub(p.CompareForecast.Value.Balances[len(p.CompareForecast.Value.Balances)-1], p.Forecast.Value.Balances[len(p.Forecast.Value.Balances)-1])
					if err != nil {
						p.CompareEndChange = unavailableFact(p.CompareEndChange, "plan end comparison overflowed", err)
					} else {
						p.CompareEndChange = available(p.CompareEndChange.Source, change)
					}
				}
			}
		}
	}
}

func (p *Prepared) safeSpend(snapshot Snapshot, in Inputs, rates currency.Rates, base string) {
	if p.LiquidBalance.Err == nil {
		value, err := ledger.LiquidBalance(snapshot.Accounts, snapshot.Transactions, rates)
		if err != nil {
			p.LiquidBalance = unavailableFact(p.LiquidBalance, "liquid balance could not be calculated", err)
		} else {
			p.LiquidBalance = available(p.LiquidBalance.Source, value)
		}
	}
	_, monthEnd := dateutil.MonthRange(in.Now)
	if p.BillsDue.Err == nil {
		value, err := safespend.BillsDueBeforeChecked(snapshot.Accounts, snapshot.Recurring, in.Now, monthEnd, rates)
		if err != nil {
			p.BillsDue = unavailableFact(p.BillsDue, "bill totals could not be calculated", err)
		} else {
			p.BillsDue = available(p.BillsDue.Source, value)
		}
	}
	if p.GoalContributions.Err == nil {
		value, err := safespend.GoalContributionsProratedChecked(snapshot.Goals, in.Now, rates)
		if err != nil {
			p.GoalContributions = unavailableFact(p.GoalContributions, "goal contributions could not be calculated", err)
		} else {
			p.GoalContributions = available(p.GoalContributions.Source, value)
		}
	}
	if p.LiquidBalance.Available && p.BillsDue.Available && p.GoalContributions.Available {
		value, err := safespend.ComputeChecked(p.LiquidBalance.Value.Amount, p.BillsDue.Value, p.GoalContributions.Value, 0, base)
		if err != nil {
			p.SafeToSpend = unavailableFact(p.SafeToSpend, "safe-to-spend total overflowed", err)
		} else {
			p.SafeToSpend = available(p.SafeToSpend.Source, value)
		}
	} else {
		p.SafeToSpend = unavailableFact(p.SafeToSpend, "liquid balance, bills and goals must all be available", errors.Join(p.LiquidBalance.Err, p.BillsDue.Err, p.GoalContributions.Err))
	}
}

func (p *Prepared) affordability(in Inputs) {
	if in.AffordabilityInputErr != nil {
		p.Affordability = unavailableFact(p.Affordability, "purchase inputs are invalid", in.AffordabilityInputErr)
		return
	}
	if in.Affordability == nil || in.Affordability.AmountMinor <= 0 {
		return
	}
	if !p.SafeToSpend.Available || !p.CurrentMonthNet.Available {
		p.Affordability = unavailableFact(p.Affordability, "safe-to-spend and current-month net are required", errors.Join(p.SafeToSpend.Err, p.CurrentMonthNet.Err))
		return
	}
	input := *in.Affordability
	if input.ReserveMinor < 0 {
		input.ReserveMinor = 0 // preserve the Planning input's existing floor.
	}
	value, err := afford.CanAffordChecked(input.AmountMinor, p.SafeToSpend.Value.SafeToSpend, p.CurrentMonthNet.Value, input.Months, input.ReserveMinor)
	if err != nil {
		p.Affordability = unavailableFact(p.Affordability, "purchase projection could not be calculated", err)
	} else {
		p.Affordability = available(p.Affordability.Source, value)
	}
}

func (p *Prepared) runwayFacts(snapshot Snapshot, in Inputs, rates currency.Rates) {
	if in.RunwayBufferErr != nil {
		p.Runway = unavailableFact(p.Runway, "runway buffer is invalid", in.RunwayBufferErr)
		p.DaysToNextInflow = unavailableFact(p.DaysToNextInflow, "runway buffer is invalid", in.RunwayBufferErr)
		p.CoverSuggestion = unavailableFact(p.CoverSuggestion, "runway buffer is invalid", in.RunwayBufferErr)
		return
	}
	if len(snapshot.Recurring) == 0 {
		p.Runway.Reason = "no recurring items are available to project"
		p.CoverSuggestion.Reason = "no runway projection is available"
		p.DaysToNextInflow.Reason = "no recurring items are available to project"
		return
	}
	if !p.LiquidBalance.Available {
		p.Runway = unavailableFact(p.Runway, "liquid balance is required", p.LiquidBalance.Err)
		p.CoverSuggestion = unavailableFact(p.CoverSuggestion, "liquid balance and source accounts are required", p.LiquidBalance.Err)
		p.DaysToNextInflow = unavailableFact(p.DaysToNextInflow, "recurring schedule is unavailable", p.LiquidBalance.Err)
		return
	}
	buffer := in.RunwayBufferMinor
	if buffer < 0 {
		buffer = 0
	}
	projection, err := runway.ProjectChecked(p.LiquidBalance.Value.Amount, snapshot.Recurring, in.Now, RunwayDays, buffer, rates)
	if err != nil {
		p.Runway = unavailableFact(p.Runway, "runway projection could not be calculated", err)
		p.CoverSuggestion = unavailableFact(p.CoverSuggestion, "runway projection is unavailable", err)
		p.DaysToNextInflow = unavailableFact(p.DaysToNextInflow, "recurring schedule could not be calculated", err)
		return
	}
	p.Runway = available(p.Runway.Source, projection)
	if projection.WillBreach() {
		suggestion, err := runway.SuggestCoverChecked(projection.BreachShortfall, snapshot.Accounts, snapshot.Transactions, rates)
		if err != nil {
			p.CoverSuggestion = unavailableFact(p.CoverSuggestion, "cover suggestion could not be calculated", err)
		} else {
			p.CoverSuggestion = available(p.CoverSuggestion.Source, suggestion)
		}
	} else {
		p.CoverSuggestion.Reason = "no runway shortfall needs a cover suggestion"
	}
	if p.SafeToSpend.Available && p.SafeToSpend.Value.SafeToSpend > 0 {
		days, err := runway.DaysToNextInflow(snapshot.Recurring, in.Now, RunwayDays, rates)
		if err != nil {
			p.DaysToNextInflow = unavailableFact(p.DaysToNextInflow, "next inflow could not be calculated", err)
		} else {
			p.DaysToNextInflow = available(p.DaysToNextInflow.Source, days)
		}
	} else if p.SafeToSpend.Err != nil {
		p.DaysToNextInflow = unavailableFact(p.DaysToNextInflow, "safe-to-spend must be known before daily allowance", p.SafeToSpend.Err)
	}
}

func trailingMonthlyNetChecked(txns []domain.Transaction, now time.Time, rates currency.Rates, months int) (int64, error) {
	if months <= 0 {
		months = 1
	}
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var sum int64
	for i := 1; i <= months; i++ {
		start := dateutil.AddMonths(monthStart, -i)
		end := dateutil.AddMonths(monthStart, -i+1)
		income, expense, err := ledger.PeriodTotals(txns, start, end, rates)
		if err != nil {
			return 0, fmt.Errorf("planningmodel: trailing month %d: %w", i, err)
		}
		monthNet, err := money.Sub(income.Amount, expense.Amount)
		if err != nil {
			return 0, fmt.Errorf("planningmodel: trailing month %d net: %w", i, err)
		}
		sum, err = money.Add(sum, monthNet)
		if err != nil {
			return 0, fmt.Errorf("planningmodel: trailing total: %w", err)
		}
	}
	return sum / int64(months), nil
}

func projectSeries(source string, start, monthlyNet, trim int64) Fact[ForecastSeries] {
	monthly, err := money.Add(monthlyNet, trim)
	if err != nil {
		return unavailable[ForecastSeries](source, "monthly projection flow overflowed", err)
	}
	balances, err := forecast.ProjectChecked(start, []forecast.Recurring{{Label: "net cash flow", Monthly: monthly}}, nil, ForecastMonths)
	if err != nil {
		return unavailable[ForecastSeries](source, "forecast could not be calculated", err)
	}
	return available(source, summarizeForecast(balances))
}

func summarizeForecast(balances []int64) ForecastSeries {
	series := ForecastSeries{Balances: balances, DipMonth: -1}
	for month, balance := range series.Balances {
		if balance < 0 {
			series.DipMonth, series.DipsBelowZero = month, true
			break
		}
	}
	return series
}

func available[T any](source string, value T) Fact[T] {
	return Fact[T]{Available: true, Value: value, Source: source}
}

func unavailable[T any](source, reason string, err error) Fact[T] {
	return Fact[T]{Source: source, Reason: reason, Err: err}
}

func unavailableFact[T any](current Fact[T], reason string, err error) Fact[T] {
	return unavailable[T](current.Source, reason, err)
}

func validBase(base string) error {
	if base == "" || !currency.Valid(base) {
		return fmt.Errorf("planningmodel: unsupported or missing base currency %q", base)
	}
	return nil
}

func validNow(now time.Time) error {
	if now.IsZero() {
		return errors.New("planningmodel: reference time is required")
	}
	return nil
}
