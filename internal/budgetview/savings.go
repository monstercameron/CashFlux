// SPDX-License-Identifier: MIT

package budgetview

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

// SavingsAccount is one member-visible savings account and its funded goal's
// timeline. Monetary figures retain their explicit source and base currencies.
type SavingsAccount struct {
	AccountID       string
	Name            string
	Type            string // raw account type; callers choose its display label
	Currency        string // account currency (also the monthly-savings currency)
	Monthly         int64  // monthly savings budget, account-currency minor units
	AssignedBase    int64  // Monthly converted to base currency (0 when unconvertible)
	ConversionError error

	// Plan-vs-reality against the account's funded goal (populated only when the
	// account funds a financial goal visible to the active member).
	HasGoal       bool
	GoalID        string
	GoalName      string
	GoalComplete  bool   // every linked goal is already met — show the win, not a projection
	MoreGoals     int    // additional incomplete linked goals beyond the one shown (0 = none)
	PlannedMonths int    // whole months from the viewed date to the goal's target (0 = undated)
	RateMonths    int64  // months to reach the goal at this monthly rate (0 = can't project)
	DeltaMonths   int64  // RateMonths − PlannedMonths (>0 later than planned, <0 sooner)
	SyncMinor     int64  // this monthly expressed in the goal's currency — what Sync writes
	SyncCurrency  string // the goal's currency (money-correct target for MonthlyContribution)
	Synced        bool   // goal.MonthlyContribution already equals SyncMinor/SyncCurrency
}

// MonthsUntil counts whole calendar months from `from` to `target`, rounding a
// partial final month up and flooring at 1, matching goals.MonthlyNeeded's
// convention. It returns 0 for an undated or already-past target so callers can
// treat "no plan" uniformly.
func MonthsUntil(from, target time.Time) int {
	if target.IsZero() || !target.After(from) {
		return 0
	}
	m := (target.Year()-from.Year())*12 + int(target.Month()) - int(from.Month())
	if target.Day() > from.Day() {
		m++ // a partial final month still needs a contribution
	}
	if m < 1 {
		m = 1
	}
	return m
}

// SavingsAccounts builds the per-account savings lines for the zero-based
// view: every non-archived savings/investment account VISIBLE TO THE ACTIVE MEMBER,
// its monthly savings budget converted to base, and — when the account funds a
// financial goal — the plan-vs-reality timeline (how many months the goal is planned
// for vs. how many it takes at this monthly rate). asOf is the viewed reference date
// (the page's period anchor), so paging months moves the "planned" horizon with the
// rest of the view. Accounts are returned sorted by name for a stable list.
func SavingsAccounts(accounts []domain.Account, goals []domain.Goal, activeMemberID string, asOf time.Time, base string, rates currency.Rates) []SavingsAccount {
	// Index active, member-visible financial goals by the account they fund, nearest
	// target first, so each account picks its most time-sensitive goal. Sinking-fund
	// goals are excluded here — they have their own monthly set-aside line, so counting
	// them a second time via the account row would double-report the same commitment.
	goalsByAcct := map[string][]domain.Goal{}
	for _, g := range goals {
		if g.Archived || g.IsSinkingFund || !g.IsFinancial() || g.AccountID == "" {
			continue
		}
		if !savingsOwnerVisible(g.OwnerID, activeMemberID) {
			continue
		}
		goalsByAcct[g.AccountID] = append(goalsByAcct[g.AccountID], g)
	}
	for id, gs := range goalsByAcct {
		sort.SliceStable(gs, func(i, j int) bool { return goalsvc.LessForList(gs[i], gs[j]) })
		goalsByAcct[id] = gs
	}

	var out []SavingsAccount
	for _, ac := range accounts {
		if ac.Archived || !ac.Type.IsSavingsLike() || !savingsOwnerVisible(ac.OwnerID, activeMemberID) {
			continue
		}
		cur := ac.Currency
		if cur == "" {
			cur = base
		}
		monthly := ac.MonthlySavings.Amount
		mCur := ac.MonthlySavings.Currency
		if mCur == "" {
			mCur = cur
		}
		assignedBase, conversionErr := convertSavingsMonthly(monthly, mCur, base, rates)
		sa := SavingsAccount{
			AccountID: ac.ID, Name: ac.Name, Type: string(ac.Type),
			Currency: cur, Monthly: monthly, AssignedBase: assignedBase, ConversionError: conversionErr,
		}
		// Plan-vs-reality against this account's funded goals: prefer the nearest still-
		// incomplete goal for the projection; if every linked goal is already met, show a
		// "fully funded" state rather than silently reverting to an unlinked-looking row.
		linked := goalsByAcct[ac.ID]
		incomplete := 0
		var picked domain.Goal
		var pickedRem money.Money
		havePick := false
		for _, g := range linked {
			rem, err := goalsvc.CoveredRemaining(g)
			if err != nil || rem.Amount <= 0 {
				continue
			}
			incomplete++
			if !havePick {
				picked, pickedRem, havePick = g, rem, true
			}
		}
		switch {
		case havePick:
			sa.HasGoal = true
			sa.GoalID = picked.ID
			sa.GoalName = picked.Name
			sa.MoreGoals = incomplete - 1
			sa.PlannedMonths = MonthsUntil(asOf, picked.TargetDate)
			// The monthly rate expressed in the goal's currency for the projection. When
			// an FX rate is missing the conversion fails; rather than silently treat the
			// raw amount as if it were in the goal's currency (which would then be
			// PERSISTED by Sync), leave the projection unset so the row shows no estimate.
			rate, rateOK := monthly, true
			if mCur != pickedRem.Currency {
				if conv, err := currency.ConvertBetween(monthly, mCur, pickedRem.Currency, rates); err == nil {
					rate = conv
				} else {
					rateOK = false
				}
			}
			if rateOK && rate > 0 {
				sa.RateMonths = 1 + (pickedRem.Amount-1)/rate // overflow-safe ceil division
				sa.SyncMinor = rate
				sa.SyncCurrency = pickedRem.Currency
				sa.Synced = monthly > 0 && picked.MonthlyContribution.Amount == rate && picked.MonthlyContribution.Currency == pickedRem.Currency
				if sa.PlannedMonths > 0 {
					sa.DeltaMonths = sa.RateMonths - int64(sa.PlannedMonths)
				}
			}
		case len(linked) > 0:
			// Every linked goal is already met — surface the win, don't vanish.
			sa.HasGoal = true
			sa.GoalComplete = true
			sa.GoalName = linked[0].Name
		}
		out = append(out, sa)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func savingsOwnerVisible(owner, member string) bool {
	return member == "" || owner == member || owner == domain.GroupOwnerID
}

func convertSavingsMonthly(monthly int64, from, base string, rates currency.Rates) (int64, error) {
	if monthly == 0 {
		return 0, nil
	}
	return currency.ConvertBetween(monthly, from, base, rates)
}
