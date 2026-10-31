// SPDX-License-Identifier: MIT

package goals

import (
	"errors"
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// ErrReservationUnavailable identifies a missing account or balance.
var ErrReservationUnavailable = errors.New("goal reservation source is unavailable")

// ErrInvalidReservation identifies a negative stored reservation.
var ErrInvalidReservation = errors.New("goal reservation amount is negative")

// ReservationGoal identifies one goal claiming an account. Duplicate allocation
// rows, and even repeated records with the same goal ID, remain one identity.
type ReservationGoal struct{ ID, Name string }

// AccountReservation is one account's reservation position in its native
// currency. Amounts are usable only with their Known flag. Free is signed;
// Over is nonnegative. Overbooked can also establish a breach when positive
// native reservations overflow, even though its exact amount is unavailable.
type AccountReservation struct {
	Account                            domain.Account
	Exists                             bool
	Balance, Reserved, Free, Over      money.Money
	BalanceKnown, ReservedKnown, Known bool
	Overbooked                         bool
	GoalRefs                           []ReservationGoal
	Err                                error
}

// ReservationIndex is an immutable household reservation snapshot. ByCurrency
// retains checked source-currency sums for funding's rounding-aware increments.
// Incomplete/ExcludedCurrencies describe reservation errors, including references
// to missing accounts; unavailable unreserved balances are per-account only.
type ReservationIndex struct {
	ByAccount          map[string]AccountReservation
	ByCurrency         map[string]map[string]int64
	Incomplete         bool
	ExcludedCurrencies []string
}

// Warnings returns known breaches and unavailable claimed positions, ordered by
// account name then ID. minimumGoals counts unique goal identities, not rows.
func (index ReservationIndex) Warnings(minimumGoals int) []AccountReservation {
	var warnings []AccountReservation
	for _, p := range index.ByAccount {
		if len(p.GoalRefs) >= max(1, minimumGoals) && (!p.Known || p.Overbooked) {
			warnings = append(warnings, p)
		}
	}
	sort.Slice(warnings, func(i, j int) bool {
		if warnings[i].Account.Name != warnings[j].Account.Name {
			return warnings[i].Account.Name < warnings[j].Account.Name
		}
		return warnings[i].Account.ID < warnings[j].Account.ID
	})
	return warnings
}

// BuildAccountReservations aggregates all retained reservations, including
// archived goals and archived sources. Source currencies are summed before FX,
// preventing individually rounded small amounts from disappearing. Missing FX,
// invalid balances and arithmetic errors never become an available zero balance.
func BuildAccountReservations(all []domain.Goal, accounts []domain.Account, balances map[string]money.Money, rates currency.Rates) ReservationIndex {
	index := ReservationIndex{ByAccount: make(map[string]AccountReservation, len(accounts)), ByCurrency: map[string]map[string]int64{}}
	for _, a := range accounts {
		index.ByAccount[a.ID] = AccountReservation{Account: a, Exists: true, Reserved: money.Zero(a.Currency), Free: money.Zero(a.Currency), Over: money.Zero(a.Currency)}
	}
	excluded := map[string]bool{}
	broken := map[string]bool{}
	provenOverflow := map[string]bool{}
	seenGoals := map[string]map[string]bool{}
	fail := func(id, code string, err error) {
		p := index.ByAccount[id]
		if p.Err == nil {
			p.Err = fmt.Errorf("goals: account %q reservations: %w", id, err)
		}
		index.ByAccount[id] = p
		broken[id], index.Incomplete = true, true
		if code != "" {
			excluded[code] = true
		}
	}
	for _, g := range all {
		for _, allocation := range g.Allocations {
			if allocation.Amount.Amount == 0 {
				continue
			}
			id := allocation.AccountID
			p, exists := index.ByAccount[id]
			if !exists {
				p.Account.ID = id
			}
			if seenGoals[id] == nil {
				seenGoals[id] = map[string]bool{}
			}
			if !seenGoals[id][g.ID] {
				p.GoalRefs = append(p.GoalRefs, ReservationGoal{g.ID, g.Name})
				seenGoals[id][g.ID] = true
			}
			index.ByAccount[id] = p
			if !p.Exists {
				fail(id, allocation.Amount.Currency, ErrReservationUnavailable)
				continue
			}
			if allocation.Amount.Amount < 0 {
				fail(id, allocation.Amount.Currency, ErrInvalidReservation)
				continue
			}
			if index.ByCurrency[id] == nil {
				index.ByCurrency[id] = map[string]int64{}
			}
			total, err := money.Add(index.ByCurrency[id][allocation.Amount.Currency], allocation.Amount.Amount)
			if err != nil {
				if allocation.Amount.Currency == p.Account.Currency {
					provenOverflow[id] = true
				}
				fail(id, p.Account.Currency, err)
				continue
			}
			index.ByCurrency[id][allocation.Amount.Currency] = total
		}
	}
	for id, amounts := range index.ByCurrency {
		p := index.ByAccount[id]
		var codes []string
		for code := range amounts {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			converted, err := rates.Convert(money.New(amounts[code], code), p.Account.Currency)
			if err != nil {
				fail(id, code, err)
				continue
			}
			total, err := p.Reserved.Add(converted)
			if err != nil {
				provenOverflow[id] = true
				fail(id, code, err)
				continue
			}
			p.Reserved = total
		}
		// Preserve a failure recorded while converting without losing the sum.
		if p.Err == nil {
			p.Err = index.ByAccount[id].Err
		}
		index.ByAccount[id] = p
	}
	for id, p := range index.ByAccount {
		p.ReservedKnown = p.Exists && !broken[id]
		if !p.ReservedKnown {
			p.Reserved = money.Zero(p.Account.Currency)
		}
		balance, present := balances[id]
		p.BalanceKnown = p.Exists && present && balance.Currency == p.Account.Currency
		if p.BalanceKnown {
			p.Balance = balance
		} else if p.Err == nil {
			p.Err = fmt.Errorf("goals: account %q balance: %w", id, ErrReservationUnavailable)
		}
		p.Overbooked = provenOverflow[id] || p.ReservedKnown && p.BalanceKnown && p.Reserved.Amount > 0 && p.Reserved.Amount > p.Balance.Amount
		if p.ReservedKnown && p.BalanceKnown {
			free, err := p.Balance.Sub(p.Reserved)
			if err == nil {
				p.Free = free
				if p.Overbooked {
					p.Over, err = p.Reserved.Sub(p.Balance)
				}
			}
			if err != nil && p.Err == nil {
				p.Err = fmt.Errorf("goals: account %q headroom: %w", id, err)
			}
			p.Known = err == nil && p.Err == nil
		}
		if !p.Known && len(p.GoalRefs) > 0 {
			index.Incomplete = true
			if (!p.BalanceKnown || p.ReservedKnown) && p.Account.Currency != "" {
				excluded[p.Account.Currency] = true
			}
		}
		index.ByAccount[id] = p
	}
	for code := range excluded {
		index.ExcludedCurrencies = append(index.ExcludedCurrencies, code)
	}
	sort.Strings(index.ExcludedCurrencies)
	return index
}
