// SPDX-License-Identifier: MIT

package planningmodel

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/montecarlo"
)

func TestPrepareRetirementAggregatesBalancesAndPreservesProjectionAssumptions(t *testing.T) {
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	snapshot := Snapshot{
		Accounts: []domain.Account{
			{ID: "401k", Type: domain.TypeRetirement, Currency: "USD", OpeningBalance: money.New(100_000, "USD")},
			{ID: "ira", Type: domain.TypeRetirement, Currency: "EUR", OpeningBalance: money.New(50_000, "EUR")},
			{ID: "archived", Archived: true, Type: domain.TypeRetirement, Currency: "USD", OpeningBalance: money.New(900_000, "USD")},
			{ID: "brokerage", Type: domain.TypeInvestment, Currency: "USD", OpeningBalance: money.New(800_000, "USD")},
		},
		Transactions: []domain.Transaction{
			{ID: "401k-growth", AccountID: "401k", Date: now, Amount: money.New(10_000, "USD")},
			{ID: "ira-growth", AccountID: "ira", Date: now, Amount: money.New(5_000, "EUR")},
		},
	}
	facts := PrepareRetirement(snapshot, RetirementInputs{
		Base: "USD", FXRates: map[string]float64{"EUR": 2},
		YearsToRetirement: 2, AnnualContributionMinor: 10_000,
		AnnualSpendMinor: 10_000, ReturnPct: 0, InflationPct: 0,
	})
	if !facts.StartingPot.Available || facts.AccountCount != 2 {
		t.Fatalf("retirement starting pot unavailable or account count wrong: %+v", facts)
	}
	if got, want := facts.StartingPot.Value, money.New(220_000, "USD"); got != want {
		t.Fatalf("starting pot = %v, want %v", got, want)
	}
	if !facts.Projection.Available {
		t.Fatalf("projection unavailable: %+v", facts.Projection)
	}
	if got, want := facts.Projection.Value.FinalNominalMinor, int64(240_000); got != want {
		t.Fatalf("final nominal pot = %d, want %d", got, want)
	}
	if got, want := facts.Projection.Value.FinalRealMinor, int64(240_000); got != want {
		t.Fatalf("final real pot = %d, want %d", got, want)
	}
	if got, want := facts.Projection.Value.ContributedMinor, int64(20_000); got != want {
		t.Fatalf("contributions = %d, want %d", got, want)
	}
	if !facts.Drawdown.Available || facts.Drawdown.Value.LastsYears != 23 {
		t.Fatalf("deterministic drawdown does not preserve configured horizon/spend: %+v", facts.Drawdown)
	}
	if !facts.MonteCarlo.Available || facts.MonteCarlo.Value.Iterations != montecarlo.DefaultIterations || facts.MonteCarlo.Value.Seed != montecarlo.DefaultSeed {
		t.Fatalf("simulation defaults were not preserved: %+v", facts.MonteCarlo)
	}
	if !facts.FI.Available || facts.FI.Value.TargetMinor != 250_000 || !facts.FI.Value.Reachable || facts.FI.Value.Years != 3 {
		t.Fatalf("FIRE target or time-to-FI changed: %+v", facts.FI)
	}
}

func TestPrepareRetirementRequiresCompleteFXAndAccountBalances(t *testing.T) {
	snapshot := Snapshot{Accounts: []domain.Account{
		{ID: "usd", Type: domain.TypeRetirement, Currency: "USD", OpeningBalance: money.New(100_000, "USD")},
		{ID: "eur", Type: domain.TypeRetirement, Currency: "EUR", OpeningBalance: money.New(50_000, "EUR")},
	}}
	facts := PrepareRetirement(snapshot, RetirementInputs{
		Base: "USD", YearsToRetirement: 10, AnnualContributionMinor: 10_000,
		AnnualSpendMinor: 10_000, ReturnPct: 7, InflationPct: 3,
	})
	if facts.StartingPot.Available || facts.Projection.Available {
		t.Fatalf("missing FX must not produce a partial starting pot or projection: %+v", facts)
	}
	if facts.StartingPot.Err == nil || !strings.Contains(facts.StartingPot.Err.Error(), "rate") {
		t.Fatalf("missing FX should retain the conversion error: %+v", facts.StartingPot)
	}

	facts = PrepareRetirement(Snapshot{
		Accounts: []domain.Account{{ID: "broken", Type: domain.TypeRetirement, Currency: "USD", OpeningBalance: money.New(1, "EUR")}},
	}, RetirementInputs{Base: "USD", FXRates: map[string]float64{"EUR": 2}, YearsToRetirement: 10, ReturnPct: 7, InflationPct: 3})
	if facts.StartingPot.Available || facts.Projection.Available || facts.StartingPot.Err == nil {
		t.Fatalf("invalid account balance must make the whole pot unavailable: %+v", facts)
	}
}

func TestPrepareRetirementRejectsIncompleteSourcesAndArithmeticOverflow(t *testing.T) {
	readErr := errors.New("transaction snapshot failed")
	facts := PrepareRetirement(Snapshot{TransactionsErr: readErr}, RetirementInputs{Base: "USD", YearsToRetirement: 5, ReturnPct: 7, InflationPct: 3})
	if facts.StartingPot.Available || facts.StartingPot.Err == nil || !errors.Is(facts.StartingPot.Err, readErr) {
		t.Fatalf("source failure must remain unavailable: %+v", facts.StartingPot)
	}

	facts = PrepareRetirement(Snapshot{Accounts: []domain.Account{
		{ID: "one", Type: domain.TypeRetirement, Currency: "USD", OpeningBalance: money.New(math.MaxInt64, "USD")},
		{ID: "two", Type: domain.TypeRetirement, Currency: "USD", OpeningBalance: money.New(1, "USD")},
	}}, RetirementInputs{Base: "USD", YearsToRetirement: 5, ReturnPct: 7, InflationPct: 3})
	if facts.StartingPot.Available || facts.Projection.Available || facts.StartingPot.Err == nil {
		t.Fatalf("overflow must not return a plausible partial pot: %+v", facts)
	}
}

func TestPrepareRetirementHandlesNoAccountsAndInvalidPlanInputs(t *testing.T) {
	facts := PrepareRetirement(Snapshot{}, RetirementInputs{Base: "USD", ReturnPct: 7, InflationPct: 3})
	if !facts.StartingPot.Available || facts.StartingPot.Value != money.Zero("USD") || facts.AccountCount != 0 {
		t.Fatalf("an empty but readable retirement set should be a known zero with no accounts: %+v", facts.StartingPot)
	}
	if facts.Projection.Available {
		t.Fatalf("no horizon should not project: %+v", facts.Projection)
	}

	facts = PrepareRetirement(Snapshot{}, RetirementInputs{
		Base: "USD", YearsToRetirement: 5, AnnualContributionMinor: -1,
		AnnualSpendMinor: -1, ReturnPct: 7, InflationPct: 3,
	})
	if !facts.StartingPot.Available || facts.Projection.Available || facts.Drawdown.Available || facts.MonteCarlo.Available || facts.FI.Available {
		t.Fatalf("invalid plan amounts must not create derived retirement figures: %+v", facts)
	}
}
