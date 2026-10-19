// SPDX-License-Identifier: MIT

// Package settle turns a set of shared expenses (who paid, and each member's
// share) plus any recorded settlements into each member's net balance and a
// minimal set of "X pays Y $Z" transfers that zero everyone out — the classic
// debt-simplification a roommate "settle up" view needs.
//
// Pure Go, no platform dependencies; all arithmetic is on integer minor units, so
// no cents are lost or created. It is unit-tested on native Go.
package settle

import (
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Expense is one shared cost: the member who paid, and each member's share of it.
// The shares are expected to sum to the amount paid (use SplitEqually or a weighted
// split to build them); the payer typically has a share too.
type Expense struct {
	Payer  string
	Shares map[string]money.Money
}

// Settlement is a payment already made from one member to another, reducing the
// outstanding balance between them.
type Settlement struct {
	From   string
	To     string
	Amount money.Money
}

// Transfer is a suggested payment: From should pay To the given Amount.
type Transfer struct {
	From   string
	To     string
	Amount money.Money
}

// Net returns each member's net balance in minor units: positive means the group
// owes them (they fronted more than their share), negative means they owe the
// group. The balances always sum to zero. currencyCode labels the returned amounts.
// It returns nil if the input contains an amount that cannot be converted to that
// currency; callers that need the reason should use NetWithRates.
//
// For each expense the payer is credited the full amount (the sum of its shares)
// and every member is debited their own share; each settlement moves its amount
// from the payer's debt toward zero and reduces the recipient's credit.
func Net(expenses []Expense, settlements []Settlement, currencyCode string) map[string]money.Money {
	net, err := NetWithRates(expenses, settlements, currency.Rates{Base: currencyCode}, currencyCode)
	if err != nil {
		return nil
	}
	return net
}

// NetWithRates converts every recorded share and settlement into targetCurrency
// before adding minor units. It returns an error when a conversion cannot be
// performed, so callers can withhold a misleading mixed-currency result.
func NetWithRates(expenses []Expense, settlements []Settlement, rates currency.Rates, targetCurrency string) (map[string]money.Money, error) {
	bal := map[string]int64{}
	for _, e := range expenses {
		var total int64
		for member, share := range e.Shares {
			converted, err := rates.Convert(share, targetCurrency)
			if err != nil {
				return nil, fmt.Errorf("settle: convert share for %q: %w", member, err)
			}
			bal[member] -= converted.Amount
			total += converted.Amount
		}
		bal[e.Payer] += total
	}
	for _, s := range settlements {
		converted, err := rates.Convert(s.Amount, targetCurrency)
		if err != nil {
			return nil, fmt.Errorf("settle: convert settlement %q to %q: %w", s.From, s.To, err)
		}
		bal[s.From] += converted.Amount
		bal[s.To] -= converted.Amount
	}
	out := make(map[string]money.Money, len(bal))
	for member, amt := range bal {
		out[member] = money.New(amt, targetCurrency)
	}
	return out, nil
}

// Minimize returns a minimal set of transfers that settles the given net balances:
// it repeatedly pays the largest debtor's debt to the largest creditor. The result
// has at most n-1 transfers for n non-zero members, each a positive amount in the
// net's currency. Members already at zero are ignored. Ties break by member ID so
// the output is deterministic.
func Minimize(net map[string]money.Money) []Transfer {
	currency := ""
	type bal struct {
		id  string
		amt int64
	}
	var debtors, creditors []bal // debtors: amt>0 = magnitude owed; creditors: amt>0 = magnitude owed to them
	for id, m := range net {
		if currency == "" && m.Currency != "" {
			currency = m.Currency
		}
		switch {
		case m.Amount < 0:
			debtors = append(debtors, bal{id, -m.Amount})
		case m.Amount > 0:
			creditors = append(creditors, bal{id, m.Amount})
		}
	}
	// Largest magnitude first; ID breaks ties for determinism.
	byMagnitude := func(s []bal) {
		sort.Slice(s, func(i, j int) bool {
			if s[i].amt != s[j].amt {
				return s[i].amt > s[j].amt
			}
			return s[i].id < s[j].id
		})
	}
	byMagnitude(debtors)
	byMagnitude(creditors)

	var transfers []Transfer
	di, ci := 0, 0
	for di < len(debtors) && ci < len(creditors) {
		pay := debtors[di].amt
		if creditors[ci].amt < pay {
			pay = creditors[ci].amt
		}
		transfers = append(transfers, Transfer{From: debtors[di].id, To: creditors[ci].id, Amount: money.New(pay, currency)})
		debtors[di].amt -= pay
		creditors[ci].amt -= pay
		if debtors[di].amt == 0 {
			di++
		}
		if creditors[ci].amt == 0 {
			ci++
		}
	}
	return transfers
}

// Simplify is the convenience pipeline: the minimal transfers that settle the net
// of the given expenses and settlements.
func Simplify(expenses []Expense, settlements []Settlement, currency string) []Transfer {
	return Minimize(Net(expenses, settlements, currency))
}

// SplitEqually divides total across the members into per-member shares that sum
// exactly to total (no lost or created minor units): the remainder cents are
// handed to the first members in sorted order, so the split is deterministic.
func SplitEqually(total money.Money, members []string) map[string]money.Money {
	out := make(map[string]money.Money, len(members))
	if len(members) == 0 {
		return out
	}
	sorted := append([]string(nil), members...)
	sort.Strings(sorted)
	n := int64(len(sorted))
	base := total.Amount / n
	rem := total.Amount % n
	for i, m := range sorted {
		share := base
		if int64(i) < rem {
			share++
		}
		out[m] = money.New(share, total.Currency)
	}
	return out
}
