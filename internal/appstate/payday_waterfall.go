// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/waterfall"
)

// This file implements the read + write sides of the payday waterfall (GL1): when
// income lands, propose funding goals in priority order until each goal's monthly
// quota is met, cascading the remainder. It composes the pure waterfall package
// (ordering + XC7 cap), the goals pace math (MonthlyAssignment), and the same
// virtual-earmark write path as the leftover sweep (XC6). No syscall/js — the
// wasm layer calls in, renders the preview card, and persists on approval.
//
// It reconciles with the R19 automated-savings plan (C183-C188): R19's workflow
// ActionTransfer MOVES money between accounts on a schedule; this waterfall is the
// approve-first, trigger-and-ordering layer that only writes virtual earmarks and
// never auto-commits. The two share the goal-funding intent but not the write:
// R19 posts transfer transactions, the waterfall reserves balance in place.

// waterfallStampKVKey is the SetKV key holding the RFC3339 timestamp of the last
// income date the user handled via the waterfall, so a handled paycheck stops
// re-proposing.
const waterfallStampKVKey = "gl1.waterfall.lastHandled"

// WaterfallProposal is the preview a caller renders: the priority-ordered funding
// lines, the income that triggered it, and the remainder left after every quota.
type WaterfallProposal struct {
	// IncomeMinor is the income detected since the last handled waterfall, in base
	// currency minor units — the pool being distributed.
	IncomeMinor int64
	// Currency is the base currency the amounts are expressed in.
	Currency string
	// Lines are the per-goal funding proposals in priority order.
	Lines []WaterfallLine
	// RemainderMinor is income left unallocated after every reachable quota.
	RemainderMinor int64
	// FundedMinor is the total proposed across all lines.
	FundedMinor int64
}

// WaterfallLine is one goal's proposed funding in the preview.
type WaterfallLine struct {
	GoalID      string
	GoalName    string
	AccountID   string
	AmountMinor int64
}

// HasProposal reports whether the proposal has any income to distribute and at
// least one fundable goal — the gate the /goals card uses before rendering.
func (p WaterfallProposal) HasProposal() bool {
	return p.IncomeMinor > 0 && len(p.Lines) > 0
}

// waterfallLastHandled returns the timestamp of the last handled waterfall, or the
// zero time when none is recorded (so the first run considers all income).
func (a *App) waterfallLastHandled() time.Time {
	raw, ok := a.GetKV(waterfallStampKVKey)
	if !ok || raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		a.log.Warn("payday waterfall: bad stamp, treating as unset", "value", raw, "err", err)
		return time.Time{}
	}
	return t
}

// incomeSinceStamp sums income transactions dated after the given stamp, converted
// to the base currency. Transfers and expenses are excluded (ledger's income
// contract). It is the "an income txn posted since last waterfall" detector.
func (a *App) incomeSinceStamp(stamp time.Time, base string, rates currency.Rates) int64 {
	var total int64
	for _, t := range a.Transactions() {
		if !t.IsIncome() || t.IsTransfer() {
			continue
		}
		if !t.Date.After(stamp) {
			continue
		}
		conv := convertMinor(t.Amount.Amount, t.Amount.Currency, base, rates)
		total += conv
	}
	if total < 0 {
		return 0
	}
	return total
}

// WaterfallPlan builds the payday funding proposal from live state: it detects
// income since the last handled waterfall, ranks the fundable goals in their
// stored order (the priority source the /goals list controls), computes each
// goal's monthly quota via goals.MonthlyAssignment, caps every account at its real
// free balance (XC7), and runs the pure waterfall. `now` anchors the pace math.
func (a *App) WaterfallPlan(now time.Time) WaterfallProposal {
	base := a.baseCurrency()
	rates := currency.Rates{Base: base, Rates: a.Settings().FXRates}
	income := a.incomeSinceStamp(a.waterfallLastHandled(), base, rates)

	proposal := WaterfallProposal{IncomeMinor: income, Currency: base, RemainderMinor: income}
	if income <= 0 {
		return proposal
	}

	allGoals := a.Goals()

	// Per-account free balance (XC7): real balance minus every existing earmark.
	free := waterfall.AccountFree{}
	quotas := make([]waterfall.GoalQuota, 0, len(allGoals))
	for _, g := range allGoals {
		if g.Archived || !g.IsFinancial() {
			continue
		}
		complete, err := goals.IsComplete(g)
		if err != nil || complete {
			continue
		}
		linked := g.LinkedAccountIDs()
		if len(linked) == 0 {
			continue
		}
		acctID := linked[0]
		m, ok, err := goals.MonthlyAssignment(g, now)
		if err != nil || !ok || m.Amount <= 0 {
			continue
		}
		quotaMinor := convertMinor(m.Amount, m.Currency, base, rates)
		if quotaMinor <= 0 {
			continue
		}
		if _, seen := free[acctID]; !seen {
			free[acctID] = a.accountFreeToEarmark(acctID, allGoals, base, rates)
		}
		quotas = append(quotas, waterfall.GoalQuota{
			GoalID:     g.ID,
			Name:       g.Name,
			QuotaMinor: quotaMinor,
			AccountID:  acctID,
		})
	}

	plan := waterfall.Compute(income, quotas, free)
	proposal.RemainderMinor = plan.RemainderMinor
	proposal.FundedMinor = plan.FundedMinor
	for _, l := range plan.Lines {
		proposal.Lines = append(proposal.Lines, WaterfallLine{
			GoalID:      l.GoalID,
			GoalName:    l.Name,
			AccountID:   l.AccountID,
			AmountMinor: l.AmountMinor,
		})
	}
	return proposal
}

// accountFreeToEarmark is the account's real balance (in base currency minor
// units) minus what every goal already earmarks against it — the XC7 ceiling the
// waterfall must not exceed. Never negative.
func (a *App) accountFreeToEarmark(acctID string, allGoals []domain.Goal, base string, rates currency.Rates) int64 {
	acc, ok := findAccount(a, acctID)
	if !ok {
		return 0
	}
	bal, err := ledger.Balance(acc, a.Transactions())
	if err != nil {
		a.log.Warn("payday waterfall: balance lookup failed", "account", acctID, "err", err)
		return 0
	}
	balMinor := convertMinor(bal.Amount, bal.Currency, base, rates)
	free := balMinor - goals.AccountEarmarkedMinor(allGoals, acctID, "")
	if free < 0 {
		return 0
	}
	return free
}

// ApplyWaterfall writes the proposal's funding lines as virtual earmarks (merging
// into an existing earmark against the same account, matching the leftover-sweep
// write path) and stamps the waterfall as handled so it stops re-proposing. It
// never posts a transaction and never auto-commits — the caller invokes it only on
// the user's explicit approval, then persists (RequestPersist). handledUpTo is the
// income cut-off to stamp (typically the caller's `now`).
func (a *App) ApplyWaterfall(proposal WaterfallProposal, handledUpTo time.Time) error {
	if len(proposal.Lines) == 0 {
		return fmt.Errorf("appstate: waterfall: nothing to fund")
	}
	for _, line := range proposal.Lines {
		if line.AmountMinor <= 0 {
			continue
		}
		var target domain.Goal
		found := false
		for _, g := range a.Goals() {
			if g.ID == line.GoalID {
				target = g
				found = true
				break
			}
		}
		if !found {
			a.log.Warn("payday waterfall: goal vanished before apply", "goal", line.GoalID)
			continue
		}
		cur := target.TargetAmount.Currency
		if cur == "" {
			cur = proposal.Currency
		}
		amtMinor := convertMinor(line.AmountMinor, proposal.Currency, cur,
			currency.Rates{Base: a.baseCurrency(), Rates: a.Settings().FXRates})
		merged := false
		for i, al := range target.Allocations {
			if al.AccountID == line.AccountID {
				target.Allocations[i].Amount = money.New(al.Amount.Amount+amtMinor, al.Amount.Currency)
				merged = true
				break
			}
		}
		if !merged {
			target.Allocations = append(target.Allocations, domain.GoalAllocation{
				AccountID: line.AccountID,
				Amount:    money.New(amtMinor, cur),
			})
		}
		if err := a.PutGoal(target); err != nil {
			return fmt.Errorf("appstate: waterfall: save goal %q: %w", line.GoalID, err)
		}
	}
	if err := a.SetKV(waterfallStampKVKey, handledUpTo.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("appstate: waterfall: stamp: %w", err)
	}
	a.log.Info("payday waterfall applied", "lines", len(proposal.Lines), "funded", proposal.FundedMinor)
	return nil
}

// DismissWaterfall stamps the waterfall as handled without writing any earmarks —
// the "not now" path, so a dismissed paycheck stops re-nagging until fresh income
// lands. The caller persists (RequestPersist).
func (a *App) DismissWaterfall(handledUpTo time.Time) error {
	if err := a.SetKV(waterfallStampKVKey, handledUpTo.Format(time.RFC3339)); err != nil {
		return fmt.Errorf("appstate: waterfall: dismiss stamp: %w", err)
	}
	return nil
}
