// SPDX-License-Identifier: MIT

// Package goalview prepares the Goals page's owner-scoped picture in native Go.
// Dates are explicit inputs so deadline and pause transitions can be tested.
package goalview

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/cashflow"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TimeKey identifies the current calendar day and the most recent goal deadline
// or pause transition. The view can change at those exact times within a day.
// Stored goal changes must still invalidate the caller's data-revision key.
func TimeKey(all []domain.Goal, now time.Time) string {
	var transition time.Time
	for _, g := range all {
		for _, at := range [...]time.Time{g.TargetDate, g.PausedUntil} {
			if !at.After(now) && at.After(transition) {
				transition = at
			}
		}
	}
	return now.Format("2006-01-02") + "|" + transition.UTC().Format(time.RFC3339Nano)
}

// Input contains the stored data and display scope needed to prepare a view.
type Input struct {
	Base, MemberID string
	Rates          currency.Rates
	Goals          []domain.Goal
	Accounts       []domain.Account
	Members        []domain.Member
	Categories     []domain.Category
	Tasks          []domain.Task
	Transactions   []domain.Transaction
	Now            time.Time
}

// Pace carries the shared funding verdict and the base-currency figures behind it.
type Pace struct {
	Health                                 goals.Health
	RequiredMinor, FairMinor, SurplusMinor int64
}

// View is a read-only, sectioned picture of the visible goals. ExcludedCurrencies
// names currencies omitted from the summary when conversion is unavailable.
type View struct {
	Base                                                         string
	Accounts                                                     []domain.Account
	Members                                                      []domain.Member
	Categories                                                   []domain.Category
	Tasks                                                        []domain.Task
	All, Active, Missed, Fund, Achieved                          []domain.Goal
	SavedTotal, SetAsideMin, TargetTotal, FundedTotal, LeftTotal money.Money
	OverallPct                                                   int
	Health                                                       map[string]Pace
	Overbooked                                                   map[string]bool
	ExcludedCurrencies                                           []string
}

// Build scopes, classifies, sorts and totals goals without mutating its inputs.
// Only financial, in-flight goals contribute to the money summary. All figures
// use the same convertible subset so the percentage reconciles with the totals.
func Build(in Input) View {
	if in.Base == "" {
		in.Base = "USD"
	}
	v := View{Base: in.Base, Accounts: in.Accounts, Members: in.Members, Categories: in.Categories, Tasks: in.Tasks}
	tasks := tasksByGoal(in.Tasks)
	for _, g := range in.Goals {
		if !accountview.Visible(g.OwnerID, in.MemberID) {
			continue
		}
		v.All = append(v.All, g)
		switch {
		case g.Archived:
			v.Achieved = append(v.Achieved, g)
		case g.IsSinkingFund:
			v.Fund = append(v.Fund, g)
		case goals.Classify(g, tasks[g.ID], in.Now) == goals.StateMissed:
			v.Missed = append(v.Missed, g)
		default:
			v.Active = append(v.Active, g)
		}
	}
	sort.SliceStable(v.Active, func(i, j int) bool { return goals.LessForList(v.Active[i], v.Active[j]) })
	sort.SliceStable(v.Missed, func(i, j int) bool { return v.Missed[i].TargetDate.Before(v.Missed[j].TargetDate) })
	sort.SliceStable(v.Fund, func(i, j int) bool { return v.Fund[i].Name < v.Fund[j].Name })
	sort.SliceStable(v.Achieved, func(i, j int) bool { return v.Achieved[i].Name < v.Achieved[j].Name })
	inFlight := append(append([]domain.Goal(nil), v.Active...), v.Missed...)
	excluded := map[string]bool{}
	v.SavedTotal, v.SetAsideMin, v.TargetTotal = money.Zero(in.Base), money.Zero(in.Base), money.Zero(in.Base)
	for _, g := range inFlight {
		if !g.IsFinancial() {
			continue
		}
		if g.CurrentAmount.Currency == "" && g.CurrentAmount.Amount == 0 {
			g.CurrentAmount = money.Zero(g.TargetAmount.Currency)
		}
		saved, savedErr := in.Rates.Convert(g.CurrentAmount, in.Base)
		target, targetErr := in.Rates.Convert(g.TargetAmount, in.Base)
		allocated := money.Zero(g.TargetAmount.Currency)
		var asideErr error
		for _, allocation := range g.Allocations {
			allocated, asideErr = allocated.Add(allocation.Amount)
			if asideErr != nil {
				excluded[allocation.Amount.Currency] = true
				break
			}
		}
		var setAside money.Money
		if asideErr == nil {
			// Sum in the stored target currency before converting, so rounding
			// occurs once per goal rather than once per reservation.
			setAside, asideErr = in.Rates.Convert(allocated, in.Base)
		}
		if savedErr != nil || targetErr != nil || asideErr != nil {
			if savedErr != nil {
				excluded[g.CurrentAmount.Currency] = true
			}
			if targetErr != nil || asideErr != nil {
				excluded[g.TargetAmount.Currency] = true
			}
			continue
		}
		// Checked money addition avoids showing a wrapped household total.
		s, se := v.SavedTotal.Add(saved)
		t, te := v.TargetTotal.Add(target)
		a, ae := v.SetAsideMin.Add(setAside)
		_, fundedErr := s.Add(a)
		if se != nil || te != nil || ae != nil || fundedErr != nil {
			excluded[g.TargetAmount.Currency] = true
			continue
		}
		v.SavedTotal, v.TargetTotal, v.SetAsideMin = s, t, a
	}
	for code := range excluded {
		v.ExcludedCurrencies = append(v.ExcludedCurrencies, code)
	}
	sort.Strings(v.ExcludedCurrencies)
	// Ratio computes in integer arithmetic without overflowing saved*100.
	if v.TargetTotal.Amount > 0 && v.SavedTotal.Amount > 0 {
		if v.SavedTotal.Amount >= v.TargetTotal.Amount {
			v.OverallPct = 100
		} else {
			v.OverallPct = ratioPercent(v.SavedTotal.Amount, v.TargetTotal.Amount)
		}
	}
	v.FundedTotal, _ = v.SavedTotal.Add(v.SetAsideMin)
	if v.FundedTotal.Currency == "" {
		v.FundedTotal = v.SavedTotal
	}
	v.LeftTotal, _ = v.TargetTotal.Sub(v.SavedTotal)
	if v.LeftTotal.Amount < 0 {
		v.LeftTotal = money.Zero(in.Base)
	}
	v.Health = Health(v.All, in.Transactions, in.Rates, in.Base, in.Now)
	v.Overbooked = Overbooked(in.Goals, in.Accounts, in.Transactions, in.Rates)
	return v
}

func ratioPercent(n, d int64) int {
	// n<d and both are positive: repeated remainder addition is bounded by d.
	p, remainder := 0, int64(0)
	for range 100 {
		if remainder >= d-n {
			p++
			remainder -= d - n
		} else {
			remainder += n
		}
	}
	return p
}

func tasksByGoal(tasks []domain.Task) map[string][]domain.Task {
	out := map[string][]domain.Task{}
	for _, task := range tasks {
		if task.RelatedType == domain.RelatedGoal && task.RelatedID != "" {
			out[task.RelatedID] = append(out[task.RelatedID], task)
		}
	}
	return out
}

// Health derives pace using the same monthly-surplus model as the assistant.
// A required amount with unavailable FX produces no verdict and consumes no
// fair-share slot; it cannot invent an on-track status or penalize other goals.
func Health(all []domain.Goal, txns []domain.Transaction, rates currency.Rates, base string, now time.Time) map[string]Pace {
	surplus := cashflow.TrailingMonthlySurplus(txns, rates, base, now, cashflow.DefaultTrailingMonths)
	type need struct {
		id     string
		amount int64
	}
	var needs []need
	for _, g := range all {
		if g.Archived || !g.IsFinancial() {
			continue
		}
		required, ok, err := goals.MonthlyNeeded(g, now)
		if err != nil || !ok {
			continue
		}
		if required.Currency == "" {
			required.Currency = base
		}
		converted, err := rates.Convert(required, base)
		if err != nil {
			continue
		}
		needs = append(needs, need{g.ID, converted.Amount})
	}
	fair := int64(0)
	if surplus > 0 && len(needs) > 0 {
		fair = surplus / int64(len(needs))
	}
	out := make(map[string]Pace, len(needs))
	for _, n := range needs {
		if verdict := goals.AssessHealth(n.amount, surplus, len(needs)); verdict != goals.HealthNone {
			out[n.id] = Pace{verdict, n.amount, fair, surplus}
		}
	}
	return out
}

// Overbooked finds goals reserving more money than an account currently holds.
// Reservations are compared in the account's native currency; an unknown FX
// rate is never treated as parity. Archived goals retain their reservations.
func Overbooked(all []domain.Goal, accounts []domain.Account, txns []domain.Transaction, rates currency.Rates) map[string]bool {
	byID := make(map[string]domain.Account, len(accounts))
	for _, a := range accounts {
		byID[a.ID] = a
	}
	earmarked := map[string]int64{}
	overflowed := map[string]bool{}
	for _, g := range all {
		for _, allocation := range g.Allocations {
			a, ok := byID[allocation.AccountID]
			if !ok {
				continue
			}
			amount, err := rates.Convert(allocation.Amount, a.Currency)
			if err != nil {
				continue
			}
			if amount.Amount <= 0 {
				continue
			}
			if total, err := money.Add(earmarked[a.ID], amount.Amount); err == nil {
				earmarked[a.ID] = total
			} else {
				// Positive reservations beyond int64 exceed any representable balance.
				overflowed[a.ID] = true
			}
		}
	}
	if len(earmarked) == 0 {
		return map[string]bool{}
	}
	balances, _ := ledger.Balances(accounts, txns)
	over := map[string]bool{}
	for _, a := range accounts {
		over[a.ID] = overflowed[a.ID] || earmarked[a.ID] > 0 && earmarked[a.ID] > balances[a.ID].Amount
	}
	out := map[string]bool{}
	for _, g := range all {
		for _, allocation := range g.Allocations {
			if over[allocation.AccountID] {
				out[g.ID] = true
				break
			}
		}
	}
	return out
}
