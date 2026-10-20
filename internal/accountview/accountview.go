// SPDX-License-Identifier: MIT

// Package accountview prepares account lists without scanning the full ledger
// during comparisons or once per row. It has no browser dependencies.
package accountview

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/acctsort"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Snapshot indexes transactions and calculates each account's exact ledger
// balance once. Error results match ledger.Balance, including its zero result.
// Transactions retain their original order, which matters for overflow checks.
type Snapshot struct {
	Transactions map[string][]domain.Transaction
	Balances     map[string]money.Money
	Cleared      map[string]money.Money
	Earliest     map[string]time.Time
	BillPayments map[string]ledger.BillPaymentInfo
}

// Build folds the ledger into per-account slices, balances and earliest dates.
// The returned values are read-only inputs for row rendering and list selection.
func Build(accounts []domain.Account, txns []domain.Transaction) Snapshot {
	counts := make(map[string]int, len(accounts))
	for _, t := range txns {
		counts[t.AccountID]++
	}
	byAccount := make(map[string][]domain.Transaction, len(counts))
	for id, count := range counts {
		byAccount[id] = make([]domain.Transaction, 0, count)
	}
	for _, t := range txns {
		byAccount[t.AccountID] = append(byAccount[t.AccountID], t)
	}
	return BuildIndexed(accounts, txns, byAccount)
}

// BuildIndexed prepares a snapshot using an existing immutable transaction index.
// Each indexed slice must contain every transaction for that account, in original
// order. The snapshot shares the index, so callers must not mutate it afterward.
func BuildIndexed(accounts []domain.Account, txns []domain.Transaction, byAccount map[string][]domain.Transaction) Snapshot {
	s := Snapshot{
		Transactions: byAccount,
		Balances:     make(map[string]money.Money, len(accounts)),
		Cleared:      make(map[string]money.Money, len(accounts)),
		Earliest:     make(map[string]time.Time, len(accounts)),
		BillPayments: make(map[string]ledger.BillPaymentInfo),
	}
	paymentDates := map[string]time.Time{}
	for _, t := range txns {
		if date, ok := s.Earliest[t.AccountID]; !ok || t.Date.Before(date) {
			s.Earliest[t.AccountID] = t.Date
		}
		// Payments are indexed by the liability they pay, which can differ from
		// the account that books the transaction (e.g. paying a card from checking).
		if t.BillAccountID != "" {
			info := s.BillPayments[t.BillAccountID]
			info.Count++
			info.HasAny = true
			if info.LatestTxn == "" || !t.Date.Before(paymentDates[t.BillAccountID]) {
				info.LatestTxn, info.Latest = t.ID, t.Amount.Abs()
				paymentDates[t.BillAccountID] = t.Date
			}
			s.BillPayments[t.BillAccountID] = info
		}
	}
	for _, a := range accounts {
		s.Balances[a.ID], _ = ledger.Balance(a, s.Transactions[a.ID])
		s.Cleared[a.ID], _ = ledger.ClearedBalance(a, s.Transactions[a.ID])
	}
	return s
}

// Visible reports whether a member view includes an entity's owner.
func Visible(ownerID, memberID string) bool {
	return memberID == "" || ownerID == memberID || ownerID == domain.GroupOwnerID
}

// Contribution returns the account's sortable net-worth contribution. A missing
// FX rate retains the historical native-amount ordering; this value must never
// be displayed as a base-currency total.
func Contribution(a domain.Account, balance money.Money, rates currency.Rates, base string) int64 {
	v := balance.Amount
	if converted, err := rates.Convert(balance, base); err == nil {
		v = converted.Amount
	}
	if a.Class == domain.ClassLiability {
		if v > 0 {
			return -v
		}
	}
	return v
}

// Assets partitions owner-visible non-liability accounts. Active assets sort
// largest first, with stable ties; archived accounts preserve stored order.
func Assets(accounts []domain.Account, balances map[string]money.Money, rates currency.Rates, base, memberID string) (active, archived []domain.Account) {
	values := make(map[string]int64, len(accounts))
	for _, a := range accounts {
		if !Visible(a.OwnerID, memberID) || a.Class == domain.ClassLiability {
			continue
		}
		if a.Archived {
			archived = append(archived, a)
			continue
		}
		active = append(active, a)
		values[a.ID] = Contribution(a, balances[a.ID], rates, base)
	}
	sort.SliceStable(active, func(i, j int) bool { return values[active[i].ID] > values[active[j].ID] })
	return active, archived
}

// SortRiskFirst sorts a list in place by freshness then net-worth contribution.
// Each freshness check and currency conversion is evaluated once per account.
func SortRiskFirst(accounts []domain.Account, balances map[string]money.Money, windows map[domain.AccountType]int, rates currency.Rates, base string, now time.Time) {
	type rank struct {
		stale bool
		value int64
	}
	ranks := make(map[string]rank, len(accounts))
	for _, a := range accounts {
		ranks[a.ID] = rank{freshness.IsStale(a, windows, now), Contribution(a, balances[a.ID], rates, base)}
	}
	sort.SliceStable(accounts, func(i, j int) bool {
		a, b := ranks[accounts[i].ID], ranks[accounts[j].ID]
		return acctsort.RiskFirstLess(a.stale, b.stale, a.value, b.value)
	})
}
