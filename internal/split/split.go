// Package split is the pure core for splitting shared expenses between household
// members and settling up (B24). It computes each member's share of a cost,
// nets out who paid versus who owes across many shared expenses, and proposes a
// small set of transfers to settle the balances. Amounts are integer minor units
// (no floats), and splits distribute the rounding remainder so shares sum to the
// total exactly.
//
// Pure Go, no syscall/js; unit-tested on native Go. The transaction-level Split
// model and the Settle-up UI build on these functions.
package split

import "sort"

// Share is one member's portion of a split, in minor units.
type Share struct {
	MemberID string
	Amount   int64
}

// Equal splits a non-negative total evenly among memberIDs, handing the
// remainder out one minor unit at a time to the earliest members so the shares
// sum to total exactly (e.g. 1000¢ across three → 334, 333, 333). Order is
// preserved; an empty member list returns nil.
func Equal(total int64, memberIDs []string) []Share {
	n := int64(len(memberIDs))
	if n == 0 {
		return nil
	}
	base := total / n
	rem := total - base*n // 0 <= rem < n for non-negative total
	shares := make([]Share, len(memberIDs))
	for i, id := range memberIDs {
		amt := base
		if int64(i) < rem {
			amt++
		}
		shares[i] = Share{MemberID: id, Amount: amt}
	}
	return shares
}

// Expense is a shared cost: PayerID fronted Total, owed back by Shares (whose
// amounts are expected to sum to Total).
type Expense struct {
	PayerID string
	Total   int64
	Shares  []Share
}

// NetBalances returns each member's net position across the given expenses:
// positive means the group owes them (they paid more than their share), negative
// means they owe the group. Every member that appears as a payer or in a share
// is included, and (when each expense's shares sum to its total) the values sum
// to zero.
func NetBalances(expenses []Expense) map[string]int64 {
	net := map[string]int64{}
	for _, e := range expenses {
		net[e.PayerID] += e.Total
		for _, s := range e.Shares {
			net[s.MemberID] -= s.Amount
		}
	}
	return net
}

// Transfer is a settle-up payment: From pays To the given minor-unit Amount.
type Transfer struct {
	From   string
	To     string
	Amount int64
}

// SettleUp proposes transfers that zero out the given net balances (positive =
// owed money, negative = owes). It greedily pairs debtors with creditors in
// descending size, which keeps the number of transfers small. The result is
// deterministic (ties broken by member id) and, when the balances sum to zero,
// fully settles them.
func SettleUp(balances map[string]int64) []Transfer {
	type entry struct {
		id  string
		amt int64
	}
	var creditors, debtors []entry
	for id, a := range balances {
		switch {
		case a > 0:
			creditors = append(creditors, entry{id, a})
		case a < 0:
			debtors = append(debtors, entry{id, -a}) // store debt as a positive magnitude
		}
	}
	byAmountDesc := func(s []entry) {
		sort.Slice(s, func(i, j int) bool {
			if s[i].amt != s[j].amt {
				return s[i].amt > s[j].amt
			}
			return s[i].id < s[j].id
		})
	}
	byAmountDesc(creditors)
	byAmountDesc(debtors)

	var out []Transfer
	i, j := 0, 0
	for i < len(creditors) && j < len(debtors) {
		c, d := &creditors[i], &debtors[j]
		pay := c.amt
		if d.amt < pay {
			pay = d.amt
		}
		out = append(out, Transfer{From: d.id, To: c.id, Amount: pay})
		c.amt -= pay
		d.amt -= pay
		if c.amt == 0 {
			i++
		}
		if d.amt == 0 {
			j++
		}
	}
	return out
}
