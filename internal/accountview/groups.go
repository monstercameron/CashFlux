// SPDX-License-Identifier: MIT

package accountview

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// GroupSection is a named group of visible accounts and its net subtotal.
// Empty Group.ID identifies the trailing ungrouped section.
type GroupSection struct {
	Group              domain.AccountGroup
	Accounts           []domain.Account
	Subtotal           money.Money
	ExcludedCurrencies []string
	Incomplete         bool
}

// Groups claims accounts once in stored group order and appends the unclaimed
// accounts in their incoming order. Empty groups are omitted. Subtotals include
// only convertible, valid balances; exclusions are explicit on each section.
func Groups(shown []domain.Account, groups []domain.AccountGroup, balances map[string]money.Money, rates currency.Rates, base string) []GroupSection {
	byID := make(map[string]domain.Account, len(shown))
	for _, a := range shown {
		byID[a.ID] = a
	}
	claimed := map[string]bool{}

	var out []GroupSection
	for _, g := range groups {
		var members []domain.Account
		for _, aid := range g.AccountIDs {
			a, ok := byID[aid]
			if !ok || claimed[aid] {
				continue
			}
			claimed[aid] = true
			members = append(members, a)
		}
		if len(members) == 0 {
			continue
		}
		out = append(out, section(g, members, balances, rates, base))
	}

	// Ungrouped catch-all: every visible account not claimed by a group, in the
	// order the caller already sorted them.
	var rest []domain.Account
	for _, a := range shown {
		if claimed[a.ID] {
			continue
		}
		rest = append(rest, a)
	}
	if len(rest) > 0 {
		out = append(out, section(domain.AccountGroup{}, rest, balances, rates, base))
	}
	return out
}

func section(group domain.AccountGroup, accounts []domain.Account, balances map[string]money.Money, rates currency.Rates, base string) GroupSection {
	s := GroupSection{Group: group, Accounts: accounts, Subtotal: money.Zero(base)}
	excluded := map[string]bool{}
	for _, a := range accounts {
		balance, valid := balances[a.ID]
		if !valid || balance.Currency == "" {
			s.Incomplete = true
			continue
		}
		converted, err := rates.Convert(balance, base)
		if err != nil {
			excluded[a.Currency] = true
			s.Incomplete = true
			continue
		}
		if a.Class == domain.ClassLiability && converted.Amount > 0 {
			converted.Amount = -converted.Amount
		}
		total, err := s.Subtotal.Add(converted)
		if err != nil {
			s.Incomplete = true
			continue
		}
		s.Subtotal = total
	}
	for c := range excluded {
		s.ExcludedCurrencies = append(s.ExcludedCurrencies, c)
	}
	sort.Strings(s.ExcludedCurrencies)
	return s
}
