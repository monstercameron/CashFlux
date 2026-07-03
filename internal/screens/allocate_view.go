// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/allocate"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// computeAllocInput carries everything computeAllocView needs to build the model — the scoring
// weights, the split mode/controls, and the current member scope.
type computeAllocInput struct {
	Base         string
	Dec          int
	Rates        currency.Rates
	ActiveMember string
	Mode         string
	Weights      allocate.Weights
	Excluded     map[string]bool
	MonthIncome  money.Money
	AmountStr    string
	ReserveStr   string
	MaxPerStr    string
}

// computeAllocView builds the shared allocate model over the live store: candidate destinations
// (asset accounts to grow, interest-bearing debts to pay down, unfinished goals to fund), the
// ranking under the chosen weights, and — when an amount is entered — the split plan. Pure (no
// hooks), so every tile renders from one consistent model.
func computeAllocView(app *appstate.App, in computeAllocInput) allocView {
	v := allocView{Base: in.Base, Dec: in.Dec, MonthIncome: in.MonthIncome, PlanByID: map[string]int64{}}

	var cands []allocate.Candidate
	for _, a := range app.Accounts() {
		if a.Archived || !ownerVisibleTo(a.OwnerID, in.ActiveMember) {
			continue
		}
		if a.Class == domain.ClassLiability {
			if a.InterestRateAPR > 0 {
				cands = append(cands, allocate.Candidate{
					ID: a.ID, Name: uistate.T("allocate.payDown", a.Name), ExpectedReturnAPR: a.InterestRateAPR,
					StabilityScore: 100, LiquidityScore: 0, DebtReduction: true,
				})
			}
			continue
		}
		// A locked account (e.g. a CD) can't take new money until its lock lifts.
		if !a.LockUntil.IsZero() && a.LockUntil.After(time.Now()) {
			continue
		}
		cands = append(cands, allocate.Candidate{
			ID: a.ID, Name: a.Name, ExpectedReturnAPR: a.ExpectedReturnAPR,
			StabilityScore: a.StabilityScore, LiquidityScore: a.LiquidityScore,
		})
	}
	for _, g := range app.Goals() {
		if done, _ := goalsvc.IsComplete(g); done {
			continue
		}
		if !ownerVisibleTo(g.OwnerID, in.ActiveMember) {
			continue
		}
		var remaining int64
		if in.Mode == "fill" {
			if r := g.TargetAmount.Amount - g.CurrentAmount.Amount; r > 0 {
				remaining = r
			}
		}
		cands = append(cands, allocate.Candidate{
			ID: "goal:" + g.ID, Name: uistate.T("allocate.goalPrefix", g.Name),
			StabilityScore: 80, LiquidityScore: 60,
			GoalProgress:      float64(goalsvc.Percent(g)) / 100,
			RemainingToTarget: remaining,
		})
	}
	v.Candidates = cands

	ranked := allocate.RankWith(cands, in.Weights, allocate.Constraints{Exclude: in.Excluded})
	scored := make([]allocate.Ranked, 0, len(ranked))
	for _, r := range ranked {
		if r.Score > 0 {
			scored = append(scored, r)
		}
	}
	v.HiddenZero = len(scored) < len(ranked)
	v.Ranked = scored

	v.TotalMinor, _ = money.ParseMinor(strings.TrimSpace(in.AmountStr), in.Dec)
	v.ReserveMinor, _ = money.ParseMinor(strings.TrimSpace(in.ReserveStr), in.Dec)
	v.MaxPerMinor, _ = money.ParseMinor(strings.TrimSpace(in.MaxPerStr), in.Dec)
	if v.TotalMinor > 0 {
		var plans []allocate.Plan
		opts := allocate.SplitOptions{Reserve: v.ReserveMinor, MaxPer: v.MaxPerMinor}
		if in.Mode == "fill" {
			plans, v.Remainder = allocate.DistributeFillToTarget(v.Ranked, v.TotalMinor, opts)
		} else {
			plans, v.Remainder = allocate.Distribute(v.Ranked, v.TotalMinor, opts)
		}
		for _, p := range plans {
			v.PlanByID[p.Candidate.ID] = p.Amount
		}
	}
	return v
}

// Allocatable is the amount that actually gets split (total minus the reserve, floored at 0).
func (v allocView) Allocatable() int64 {
	if a := v.TotalMinor - v.ReserveMinor; a > 0 {
		return a
	}
	return 0
}
