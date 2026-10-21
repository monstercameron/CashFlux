// SPDX-License-Identifier: MIT

package planningmodel

import (
	"errors"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/montecarlo"
	"github.com/monstercameron/CashFlux/internal/retirement"
)

// RetirementDrawdownYears is the Planning surface's horizon for the
// deterministic and probabilistic drawdown views. Forty years covers a
// retirement beginning at 60 through age 100.
const RetirementDrawdownYears = 40

// RetirementInputs are the assumptions that sit beside the household's
// retirement account balances. Amounts are in Base currency minor units.
type RetirementInputs struct {
	Base                    string
	FXRates                 map[string]float64
	SettingsErr             error
	YearsToRetirement       int
	AnnualContributionMinor int64
	AnnualSpendMinor        int64
	ReturnPct               float64
	InflationPct            float64
}

// RetirementFI is the target and reachability answer for the configured annual
// spending. A target can be available even when the current saving path cannot
// reach it within retirement.MaxYears.
type RetirementFI struct {
	TargetMinor int64
	Years       int
	Reachable   bool
}

// RetirementFacts contains checked, reusable facts for the Planning retirement
// card. The starting pot is all-or-nothing: a bad retirement account balance or
// a missing FX rate makes it unavailable instead of exposing a partial total.
type RetirementFacts struct {
	AccountCount int
	Assumptions  retirement.Assumptions
	StartingPot  Fact[money.Money]
	Projection   Fact[retirement.Projection]
	Drawdown     Fact[retirement.Drawdown]
	MonteCarlo   Fact[montecarlo.Result]
	FI           Fact[RetirementFI]
}

// PrepareRetirement computes the Planning retirement card from one checked
// household snapshot and its complete assumptions. It preserves the card's
// existing end-of-year contribution, start-of-year withdrawal, 4% SWR and
// deterministic Monte Carlo conventions.
func PrepareRetirement(snapshot Snapshot, in RetirementInputs) RetirementFacts {
	base := strings.ToUpper(strings.TrimSpace(in.Base))
	assumptions := retirement.Assumptions{ReturnPct: in.ReturnPct, InflationPct: in.InflationPct}
	result := RetirementFacts{
		Assumptions: assumptions,
		StartingPot: unavailable[money.Money]("ledger.Balances + currency.Rates.Convert + money.Add", "retirement account balances are unavailable", errors.Join(in.SettingsErr, validBase(base), snapshot.AccountsErr, snapshot.TransactionsErr)),
		Projection:  unavailable[retirement.Projection]("retirement.Project", "starting pot and a valid retirement horizon are required", nil),
		Drawdown:    unavailable[retirement.Drawdown]("retirement.RunDrawdown", "positive annual retirement spending is required", nil),
		MonteCarlo:  unavailable[montecarlo.Result]("montecarlo.Run", "positive annual retirement spending is required", nil),
		FI:          unavailable[RetirementFI]("retirement.FIRENumber + retirement.YearsToFI", "positive annual retirement spending is required", nil),
	}
	if result.StartingPot.Err != nil {
		return result
	}

	eligible := make([]domain.Account, 0, len(snapshot.Accounts))
	seen := make(map[string]struct{}, len(snapshot.Accounts))
	for _, account := range snapshot.Accounts {
		if account.Archived || account.Type != domain.TypeRetirement {
			continue
		}
		result.AccountCount++
		if _, ok := seen[account.ID]; ok {
			result.StartingPot = unavailable[money.Money](result.StartingPot.Source, "retirement account IDs are duplicated", fmt.Errorf("planningmodel: duplicate retirement account ID %q", account.ID))
			return result
		}
		seen[account.ID] = struct{}{}
		eligible = append(eligible, account)
	}

	baseTotal := money.Zero(base)
	if len(eligible) > 0 {
		balances, err := ledger.Balances(eligible, snapshot.Transactions)
		if err != nil {
			result.StartingPot = unavailable[money.Money](result.StartingPot.Source, "a retirement account balance could not be calculated", err)
			return result
		}
		rates := currency.Rates{Base: base, Rates: in.FXRates}
		for _, account := range eligible {
			balance, ok := balances[account.ID]
			if !ok {
				result.StartingPot = unavailable[money.Money](result.StartingPot.Source, "a retirement account balance is unavailable", fmt.Errorf("planningmodel: account %s has no computed balance", account.ID))
				return result
			}
			converted, err := rates.Convert(balance, base)
			if err != nil {
				result.StartingPot = unavailable[money.Money](result.StartingPot.Source, "a retirement account currency could not be converted", fmt.Errorf("planningmodel: account %s: %w", account.ID, err))
				return result
			}
			baseTotal, err = baseTotal.Add(converted)
			if err != nil {
				result.StartingPot = unavailable[money.Money](result.StartingPot.Source, "retirement account total overflowed", err)
				return result
			}
		}
	}
	result.StartingPot = available(result.StartingPot.Source, baseTotal)

	if in.AnnualContributionMinor < 0 {
		result.Projection = unavailable[retirement.Projection](result.Projection.Source, "annual retirement contributions cannot be negative", errors.New("planningmodel: negative annual contribution"))
	} else {
		projection := retirement.Project(baseTotal.Amount, in.AnnualContributionMinor, in.YearsToRetirement, assumptions)
		if projection.Known {
			result.Projection = available(result.Projection.Source, projection)
		} else {
			result.Projection = unavailable[retirement.Projection](result.Projection.Source, "a valid retirement horizon and assumptions are required", nil)
		}
	}
	if !result.Projection.Available {
		return result
	}
	if in.AnnualSpendMinor <= 0 {
		if in.AnnualSpendMinor < 0 {
			err := errors.New("planningmodel: negative annual retirement spending")
			result.Drawdown = unavailable[retirement.Drawdown](result.Drawdown.Source, "annual retirement spending cannot be negative", err)
			result.MonteCarlo = unavailable[montecarlo.Result](result.MonteCarlo.Source, "annual retirement spending cannot be negative", err)
			result.FI = unavailable[RetirementFI](result.FI.Source, "annual retirement spending cannot be negative", err)
		}
		return result
	}

	result.Drawdown = available(result.Drawdown.Source, retirement.RunDrawdown(
		result.Projection.Value.FinalNominalMinor, in.AnnualSpendMinor, RetirementDrawdownYears, assumptions))
	if !result.Drawdown.Value.Known {
		result.Drawdown = unavailable[retirement.Drawdown](result.Drawdown.Source, "drawdown assumptions are unavailable", nil)
	}
	if monteCarlo, ok := montecarlo.Run(result.Projection.Value.FinalNominalMinor, in.AnnualSpendMinor, RetirementDrawdownYears,
		montecarlo.Config{MeanReturnPct: assumptions.ReturnPct, InflationPct: assumptions.InflationPct}); ok {
		result.MonteCarlo = available(result.MonteCarlo.Source, monteCarlo)
	} else {
		result.MonteCarlo = unavailable[montecarlo.Result](result.MonteCarlo.Source, "simulation inputs are unavailable", nil)
	}

	if target, ok := retirement.FIRENumber(in.AnnualSpendMinor, retirement.DefaultSWRPct); ok {
		years, reachable := retirement.YearsToFI(baseTotal.Amount, in.AnnualContributionMinor, target, assumptions.RealReturnPct())
		result.FI = available(result.FI.Source, RetirementFI{TargetMinor: target, Years: years, Reachable: reachable})
	} else {
		result.FI = unavailable[RetirementFI](result.FI.Source, "a positive retirement spending target is required", nil)
	}
	return result
}
