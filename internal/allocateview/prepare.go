// SPDX-License-Identifier: MIT

// Package allocateview prepares the native, deterministic facts rendered by
// the Allocate screen. It keeps eligibility, ledger work, validation, ranking,
// and distribution out of the WebAssembly view.
package allocateview

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/marginal"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Input supplies one immutable view of the data and the user's allocation
// choices. SourceErr must be set when any snapshot read failed.
type Input struct {
	Accounts      []domain.Account
	Transactions  []domain.Transaction
	Goals         []domain.Goal
	SourceErr     error
	Now           time.Time
	ActiveMember  string
	Mode          string
	Weights       allocate.Weights
	Excluded      map[string]bool
	Decimals      int
	Amount        string
	Reserve       string
	MaxPer        string
	StandingFloor int64
}

// GoalFacts reports whether goal progress and the remaining target are
// meaningful. Currency mismatches are represented as unavailable facts.
type GoalFacts struct {
	GoalID             string
	Progress           int
	Remaining          int64
	ProgressAvailable  bool
	RemainingAvailable bool
	Err                error
}

// Result is the prepared Allocate view model. Parse errors remain associated
// with their field so callers can render the input and error without inventing
// a zero amount.
type Result struct {
	Candidates          []allocate.Candidate
	Ranked              []allocate.Ranked
	HiddenZero          bool
	PlanByID            map[string]int64
	TotalMinor          int64
	TotalErr            error
	ReserveMinor        int64
	ReserveErr          error
	AllocatableMinor    int64
	ReserveFromStanding bool
	MaxPerMinor         int64
	MaxPerErr           error
	Remainder           int64
	Marginal            []marginal.Destination
	GoalFacts           map[string]GoalFacts
	BalanceErrors       map[string]error
}

// Prepare derives allocation facts from one input snapshot. It fails before
// preparing anything if a source snapshot was incomplete.
func Prepare(in Input) (Result, error) {
	if in.SourceErr != nil {
		return Result{}, fmt.Errorf("allocateview: source snapshot: %w", in.SourceErr)
	}
	res := Result{PlanByID: make(map[string]int64), GoalFacts: make(map[string]GoalFacts), BalanceErrors: make(map[string]error)}
	balances, balanceErr := ledger.Balances(in.Accounts, in.Transactions)
	if balanceErr != nil {
		// Balances returns valid account values alongside its first account error.
		// Per-account unavailability is retained below; do not discard valid rows.
		_ = balanceErr
	}
	for _, a := range in.Accounts {
		if _, ok := balances[a.ID]; !ok {
			res.BalanceErrors[a.ID] = fmt.Errorf("ledger: account %s: balance unavailable", a.ID)
		}
	}
	visible := func(owner string) bool {
		return in.ActiveMember == "" || owner == in.ActiveMember || owner == domain.GroupOwnerID
	}
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	for _, a := range in.Accounts {
		if a.Archived || !visible(a.OwnerID) {
			continue
		}
		if a.Class == domain.ClassLiability {
			if rate, ok := a.RateAPR(); ok && rate > 0 {
				res.Candidates = append(res.Candidates, allocate.Candidate{ID: a.ID, Name: a.Name, ExpectedReturnAPR: rate, StabilityScore: 100, DebtReduction: true})
			}
			bal, ok := balances[a.ID]
			capacity := int64(0)
			if ok {
				capacity = bal.Amount
				if capacity < 0 {
					if capacity == math.MinInt64 {
						capacity = math.MaxInt64
					} else {
						capacity = -capacity
					}
				}
			}
			res.Marginal = append(res.Marginal, marginal.Destination{ID: a.ID, Name: a.Name, Kind: marginal.KindDebt, AnnualRatePct: a.RateAPROrZero(), CapacityMinor: capacity})
			continue
		}
		if !a.LockUntil.IsZero() && a.LockUntil.After(now) {
			continue
		}
		res.Candidates = append(res.Candidates, allocate.Candidate{ID: a.ID, Name: a.Name, ExpectedReturnAPR: a.ExpectedReturnAPR, StabilityScore: a.StabilityScore, LiquidityScore: a.LiquidityScore})
		res.Marginal = append(res.Marginal, marginal.Destination{ID: a.ID, Name: a.Name, Kind: marginal.KindSavings, AnnualRatePct: a.ExpectedReturnAPR})
	}
	for _, g := range in.Goals {
		if g.Archived || !visible(g.OwnerID) {
			continue
		}
		complete, completeErr := goals.IsComplete(g)
		remaining, remainErr := goals.Remaining(g)
		facts := GoalFacts{GoalID: g.ID, ProgressAvailable: completeErr == nil, RemainingAvailable: remainErr == nil}
		if completeErr == nil {
			facts.Progress = goals.Percent(g)
		}
		if completeErr != nil {
			facts.Err = completeErr
		}
		if remainErr != nil {
			facts.Err = remainErr
		}
		if remainErr == nil {
			facts.Remaining = remaining.Amount
		}
		res.GoalFacts[g.ID] = facts
		if completeErr != nil || complete {
			continue
		}
		if in.Mode == "fill" && remainErr != nil {
			continue
		}
		toTarget := int64(0)
		if in.Mode == "fill" {
			toTarget = remaining.Amount
		}
		res.Candidates = append(res.Candidates, allocate.Candidate{ID: "goal:" + g.ID, Name: g.Name, StabilityScore: 80, LiquidityScore: 60, GoalProgress: float64(facts.Progress) / 100, RemainingToTarget: toTarget})
		res.Marginal = append(res.Marginal, marginal.Destination{ID: "goal:" + g.ID, Name: g.Name, Kind: marginal.KindOther})
	}
	ranked := allocate.RankWith(res.Candidates, in.Weights, allocate.Constraints{Exclude: in.Excluded})
	res.Ranked = make([]allocate.Ranked, 0, len(ranked))
	for _, r := range ranked {
		if r.Score > 0 {
			res.Ranked = append(res.Ranked, r)
		}
	}
	res.HiddenZero = len(res.Ranked) < len(ranked)
	res.TotalMinor, res.TotalErr = parse(in.Amount, in.Decimals)
	res.ReserveMinor, res.ReserveErr = parse(in.Reserve, in.Decimals)
	if strings.TrimSpace(in.Reserve) == "" && in.StandingFloor > 0 {
		res.ReserveMinor, res.ReserveErr, res.ReserveFromStanding = in.StandingFloor, nil, true
	}
	res.MaxPerMinor, res.MaxPerErr = parse(in.MaxPer, in.Decimals)
	if res.TotalErr == nil && res.ReserveErr == nil {
		// Amount and reserve are parsed as nonnegative int64 values. Their
		// difference therefore fits in int64; floor the result at zero for display.
		res.AllocatableMinor = res.TotalMinor - res.ReserveMinor
		if res.AllocatableMinor < 0 {
			res.AllocatableMinor = 0
		}
	}
	if res.TotalErr != nil || res.ReserveErr != nil || res.MaxPerErr != nil {
		return res, nil
	}
	if res.TotalMinor > 0 {
		opts := allocate.SplitOptions{Reserve: res.ReserveMinor, MaxPer: res.MaxPerMinor}
		var plans []allocate.Plan
		if in.Mode == "fill" {
			plans, res.Remainder = allocate.DistributeFillToTarget(res.Ranked, res.TotalMinor, opts)
		} else {
			plans, res.Remainder = allocate.Distribute(res.Ranked, res.TotalMinor, opts)
		}
		for _, p := range plans {
			res.PlanByID[p.Candidate.ID] = p.Amount
		}
	}
	return res, nil
}

func parse(s string, decimals int) (int64, error) {
	if strings.TrimSpace(s) == "" {
		return 0, nil
	}
	amount, err := money.ParseMinor(strings.TrimSpace(s), decimals)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", s, err)
	}
	if amount < 0 {
		return 0, fmt.Errorf("invalid amount %q: amount cannot be negative", s)
	}
	return amount, nil
}
