// SPDX-License-Identifier: MIT

package goalview

import (
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

// FundingSuggestion is additional money that can safely be reserved for a goal.
// Amount uses the goal's target currency and includes no existing reservation.
type FundingSuggestion struct {
	AccountID, AccountName string
	Amount                 money.Money
}

// Exposure is one account's convertible reservation and remaining balance.
type Exposure struct {
	Account                 domain.Account
	Balance, Reserved, Free money.Money
	Percent                 int
	Overbooked              bool
}

// EarmarkView holds the money map and account exposures for the Earmarks tab.
// Incomplete reports omitted invalid balances, reservations, or unavailable FX.
type EarmarkView struct {
	TotalBalance, TotalReserved, Free, Held money.Money
	Percent                                 int
	Accounts                                []Exposure
	Goals, Unearmarked                      []domain.Goal
	Incomplete                              bool
	ExcludedCurrencies                      []string
}

// FundingIndex shares reservation arithmetic and quick-funding suggestions.
// Its maps and slices are immutable after construction.
type FundingIndex struct {
	Suggestions map[string]FundingSuggestion
	Overbooked  map[string]bool
	Earmarks    EarmarkView
}

// LiquidSource reports whether an account type holds spendable cash.
func LiquidSource(t domain.AccountType) bool {
	switch t {
	case domain.TypeChecking, domain.TypeDebit, domain.TypeSavings, domain.TypeCash:
		return true
	default:
		return false
	}
}

// EarmarkSource reports whether an account can back a virtual reservation.
func EarmarkSource(a domain.Account) bool {
	return !a.Archived && a.Class != domain.ClassLiability && !a.Type.IsLiability()
}

// BuildFunding evaluates all household reservations once in each account's native
// currency. Unknown reservations block further funding from that account. Existing
// reservations from every goal, including the receiving goal, consume headroom.
func BuildFunding(all []domain.Goal, accounts []domain.Account, balances map[string]money.Money, rates currency.Rates, base string) FundingIndex {
	if base == "" {
		base = "USD"
	}
	f := FundingIndex{Suggestions: map[string]FundingSuggestion{}, Overbooked: map[string]bool{}}
	v := EarmarkView{TotalBalance: money.Zero(base), TotalReserved: money.Zero(base), Free: money.Zero(base), Held: money.Zero(base)}
	byID := make(map[string]domain.Account, len(accounts))
	for _, a := range accounts {
		byID[a.ID] = a
	}
	reserved, blocked, overflowed := map[string]int64{}, map[string]bool{}, map[string]bool{}
	byCurrency := map[string]map[string]int64{}
	excluded := map[string]bool{}
	omit := func(code string) {
		v.Incomplete = true
		if code != "" {
			excluded[code] = true
		}
	}
	for _, g := range all {
		has := false
		for _, allocation := range g.Allocations {
			if allocation.Amount.Amount == 0 {
				continue
			}
			has = true
			a, ok := byID[allocation.AccountID]
			if !ok {
				omit(allocation.Amount.Currency)
				continue
			}
			if allocation.Amount.Amount < 0 {
				blocked[a.ID] = true
				omit(allocation.Amount.Currency)
				continue
			}
			if byCurrency[a.ID] == nil {
				byCurrency[a.ID] = map[string]int64{}
			}
			total, err := money.Add(byCurrency[a.ID][allocation.Amount.Currency], allocation.Amount.Amount)
			if err != nil {
				blocked[a.ID] = true
				if allocation.Amount.Currency == a.Currency {
					overflowed[a.ID] = true
				}
				omit(a.Currency)
				continue
			}
			byCurrency[a.ID][allocation.Amount.Currency] = total
		}
		if has {
			v.Goals = append(v.Goals, g)
		} else if g.IsFinancial() && !g.Archived {
			v.Unearmarked = append(v.Unearmarked, g)
		}
	}
	// Aggregate in each reservation currency before FX, so many small earmarks
	// cannot each round down to zero and disappear from account headroom.
	for id, amounts := range byCurrency {
		for code, amount := range amounts {
			converted, err := rates.Convert(money.New(amount, code), byID[id].Currency)
			if err != nil {
				blocked[id] = true
				omit(code)
				continue
			}
			total, err := money.Add(reserved[id], converted.Amount)
			if err != nil {
				blocked[id], overflowed[id] = true, true
				omit(code)
				continue
			}
			reserved[id] = total
		}
	}
	freeNative := map[string]money.Money{}
	over := map[string]bool{}
	for _, a := range accounts {
		balance, valid := balances[a.ID]
		over[a.ID] = overflowed[a.ID] || valid && reserved[a.ID] > 0 && reserved[a.ID] > balance.Amount
		if !valid || balance.Currency != a.Currency || blocked[a.ID] {
			if EarmarkSource(a) || reserved[a.ID] > 0 {
				omit(a.Currency)
			}
			continue
		}
		free, err := money.Sub(balance.Amount, reserved[a.ID])
		if err != nil {
			omit(a.Currency)
			continue
		}
		if free > 0 && EarmarkSource(a) && LiquidSource(a.Type) {
			freeNative[a.ID] = money.New(free, a.Currency)
		}
		if !EarmarkSource(a) && reserved[a.ID] <= 0 {
			continue
		}
		b, be := rates.Convert(balance, base)
		r, re := rates.Convert(money.New(reserved[a.ID], a.Currency), base)
		if be != nil || re != nil {
			omit(a.Currency)
			continue
		}
		delta, err := b.Sub(r)
		if err != nil {
			omit(a.Currency)
			continue
		}
		// Commit each account atomically, keeping totals on one safely added subset.
		total, held, available := v.TotalBalance, v.Held, v.Free
		if EarmarkSource(a) {
			total, err = total.Add(b)
			if err == nil && LiquidSource(a.Type) {
				available, err = available.Add(delta)
			}
			if err == nil && !LiquidSource(a.Type) {
				held, err = held.Add(b)
			}
		}
		rsum, rerr := v.TotalReserved.Add(r)
		if err != nil || rerr != nil {
			omit(a.Currency)
			continue
		}
		v.TotalBalance, v.Held, v.Free, v.TotalReserved = total, held, available, rsum
		if reserved[a.ID] > 0 {
			freeBase := delta
			if freeBase.Amount < 0 {
				freeBase = money.Zero(base)
			}
			pct := sharePercent(r.Amount, b.Amount)
			if over[a.ID] {
				pct = 100
			}
			v.Accounts = append(v.Accounts, Exposure{a, b, r, freeBase, pct, over[a.ID]})
		}
	}
	for _, g := range all {
		for _, allocation := range g.Allocations {
			if over[allocation.AccountID] {
				f.Overbooked[g.ID] = true
				break
			}
		}
	}
	v.Percent = sharePercent(v.TotalReserved.Amount, v.TotalBalance.Amount)
	if v.Percent == 0 && v.TotalReserved.Amount > 0 && v.TotalBalance.Amount > 0 {
		v.Percent = 1
	}
	for code := range excluded {
		v.ExcludedCurrencies = append(v.ExcludedCurrencies, code)
	}
	sort.Strings(v.ExcludedCurrencies)
	f.Earmarks = v
	f.Suggestions = fundingSuggestions(all, accounts, freeNative, byCurrency, rates)
	return f
}

func fundingSuggestions(all []domain.Goal, accounts []domain.Account, freeNative map[string]money.Money, reservedByCurrency map[string]map[string]int64, rates currency.Rates) map[string]FundingSuggestion {
	suggestions := map[string]FundingSuggestion{}
	// Convert each account's free amount only once per goal currency.
	freeByCurrency := map[string]map[string]money.Money{}
	for _, g := range all {
		if g.Archived || !g.IsFinancial() || g.TargetAmount.Amount <= 0 {
			continue
		}
		gap, err := fundingGap(g, rates)
		if err != nil || gap.Amount <= 0 {
			continue
		}
		available, ok := freeByCurrency[gap.Currency]
		if !ok {
			available = map[string]money.Money{}
			for id, free := range freeNative {
				if c, err := rates.Convert(free, gap.Currency); err == nil && c.Amount > 0 {
					available[id] = c
				}
			}
			freeByCurrency[gap.Currency] = available
		}
		var best domain.Account
		var bestFree money.Money
		for _, a := range accounts {
			free := available[a.ID]
			if free.Amount <= 0 {
				continue
			}
			if a.ID == g.AccountID && free.Amount >= gap.Amount {
				best, bestFree = a, free
				break
			}
			if free.Amount > bestFree.Amount {
				best, bestFree = a, free
			}
		}
		if best.ID == "" {
			continue
		}
		amount := min(gap.Amount, bestFree.Amount)
		// Check the change in the account's TOTAL converted reservation. Checking
		// the new amount alone misses rounding carried by existing foreign money.
		mergeCurrency := gap.Currency
		for _, allocation := range g.Allocations {
			if allocation.AccountID == best.ID {
				mergeCurrency = allocation.Amount.Currency
				break
			}
		}
		existing := money.New(reservedByCurrency[best.ID][mergeCurrency], mergeCurrency)
		before, err := rates.Convert(existing, best.Currency)
		if err != nil {
			continue
		}
		fits := func(additional int64) bool {
			total, err := AddReservationAmount(existing, money.New(additional, gap.Currency), rates)
			if err != nil {
				return false
			}
			after, err := rates.Convert(total, best.Currency)
			if err != nil {
				return false
			}
			delta, err := after.Sub(before)
			return err == nil && delta.Amount <= freeNative[best.ID].Amount
		}
		if !fits(amount) {
			low, high := int64(0), amount
			for low < high {
				step := high - low
				mid := low + step/2 + step%2
				if fits(mid) {
					low = mid
				} else {
					high = mid - 1
				}
			}
			amount = low
		}
		if amount > 0 {
			suggestions[g.ID] = FundingSuggestion{best.ID, best.Name, money.New(amount, gap.Currency)}
		}
	}
	return suggestions
}

// AddReservationAmount adds a proposed increment to an existing reservation,
// preserving its stored currency and checking conversion and integer range.
func AddReservationAmount(existing, additional money.Money, rates currency.Rates) (money.Money, error) {
	converted, err := rates.Convert(additional, existing.Currency)
	if err != nil {
		return money.Money{}, fmt.Errorf("goalview: convert reservation increment: %w", err)
	}
	total, err := existing.Add(converted)
	if err != nil {
		return money.Money{}, fmt.Errorf("goalview: add reservation increment: %w", err)
	}
	return total, nil
}

func fundingGap(g domain.Goal, rates currency.Rates) (money.Money, error) {
	c, err := goals.EvaluateCoverage(g, rates)
	return c.Remaining, err
}

// allocationTotal sums each source currency before conversion, rounding once
// per currency. Storage accepts foreign-currency allocations even though the
// current allocation editor normally writes the goal's target currency.
func allocationTotal(g domain.Goal, rates currency.Rates, to string) (money.Money, error) {
	return goals.AllocationTotal(g, rates, to)
}

func sharePercent(n, d int64) int {
	if n <= 0 || d <= 0 {
		return 0
	}
	if n >= d {
		return 100
	}
	return ratioPercent(n, d)
}
