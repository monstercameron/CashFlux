// SPDX-License-Identifier: MIT

package allocate

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/split"
)

// MemberIncomeSplit is one household member's share of the total period income,
// in base-currency minor units. Shares from all members returned by
// SplitPeriodIncome sum to exactly the total period income.
type MemberIncomeSplit struct {
	MemberID string
	Name     string
	Amount   int64 // base-currency minor units; always ≥ 0
}

// SplitPeriodIncome apportions the total income for the half-open period
// [start, end) across household members, returning per-member minor-unit amounts
// that sum exactly to the total.
//
// Apportionment uses equal weights across non-group members (weight=1 each) as
// the fallback because no per-member income-weight configuration exists yet.
// When a future per-member weight field is added it should override this default.
//
// Domain transactions are converted to the base currency via rates before
// summing. Transfers are excluded; only positive non-transfer transactions
// (IsIncome() == true) count.
//
// Returns nil, nil when members is empty or when all members are the group
// pseudo-member (GroupOwnerID). When rates are unavailable for a transaction
// currency the error is propagated and the caller should suppress the card
// rather than display zeros.
func SplitPeriodIncome(txns []domain.Transaction, members []domain.Member, start, end time.Time, base string, rates currency.Rates) ([]MemberIncomeSplit, error) {
	// Collect non-group members only; the group pseudo-member is not a real
	// person and should not receive a share.
	nonGroup := make([]domain.Member, 0, len(members))
	for _, m := range members {
		if m.ID != domain.GroupOwnerID {
			nonGroup = append(nonGroup, m)
		}
	}
	if len(nonGroup) == 0 {
		return nil, nil
	}

	// Sort by ID for determinism — ByWeights preserves slice order and breaks
	// equal-remainder ties by index, so lexical pre-sort gives deterministic
	// tie-breaking identical to SplitByShares.
	sort.Slice(nonGroup, func(i, j int) bool { return nonGroup[i].ID < nonGroup[j].ID })

	// Build an allocate.RateConverter adapter from the currency.Rates table.
	convert := func(amount int64, from, to string) (int64, error) {
		result, err := rates.Convert(money.New(amount, from), to)
		if err != nil {
			return 0, err
		}
		return result.Amount, nil
	}

	// Convert domain.Transaction → allocate.Transaction for PeriodIncome.
	allocTxns := make([]Transaction, 0, len(txns))
	for _, t := range txns {
		if t.IsTransfer() {
			continue
		}
		allocTxns = append(allocTxns, Transaction{
			Amount:   t.Amount.Amount,
			Currency: t.Amount.Currency,
			IsIncome: t.IsIncome(),
			Date:     t.Date,
		})
	}

	totalIncome, err := PeriodIncome(allocTxns, start, end, base, convert)
	if err != nil {
		return nil, err
	}

	// Equal weights: each non-group member gets weight 1.
	weighted := make([]split.WeightedMember, len(nonGroup))
	for i, m := range nonGroup {
		weighted[i] = split.WeightedMember{MemberID: m.ID, Weight: 1}
	}

	shares := split.ByWeights(totalIncome, weighted)

	// Build name lookup.
	nameOf := make(map[string]string, len(members))
	for _, m := range members {
		nameOf[m.ID] = m.Name
	}

	out := make([]MemberIncomeSplit, len(nonGroup))
	for i, m := range nonGroup {
		var amt int64
		if shares != nil && i < len(shares) {
			amt = shares[i].Amount
		}
		out[i] = MemberIncomeSplit{
			MemberID: m.ID,
			Name:     nameOf[m.ID],
			Amount:   amt,
		}
	}
	return out, nil
}
