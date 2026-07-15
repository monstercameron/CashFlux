// SPDX-License-Identifier: MIT

package ledger

import (
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// RegisterBalances returns the running account balance AFTER each of the
// account's transactions, keyed by transaction ID. The fold runs over the
// account's FULL history in chronological order — date ascending, ties broken by
// transaction ID ascending (the same deterministic order the register view
// enforces) — starting from the opening balance. A caller showing a filtered or
// paginated subset of the account's rows can therefore look up each visible row's
// TRUE running balance by ID: the figure reflects every prior transaction on the
// account, not just the rows currently on screen.
//
// Only transactions booked on the account (AccountID == account.ID) participate;
// bill-linked payments booked on other accounts are ignored, matching the balance
// semantics in Balance. Amounts must be in the account's currency — a transaction
// in a different currency (a multi-currency account) makes the fold error, and the
// caller should suppress the running-balance column rather than show wrong figures.
func RegisterBalances(account domain.Account, all []domain.Transaction) (map[string]money.Money, error) {
	bal, err := openingBalance(account)
	if err != nil {
		return nil, err
	}

	ordered := make([]domain.Transaction, 0, len(all))
	for _, t := range all {
		if t.AccountID == account.ID {
			ordered = append(ordered, t)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Date.Equal(ordered[j].Date) {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].Date.Before(ordered[j].Date)
	})

	out := make(map[string]money.Money, len(ordered))
	for _, t := range ordered {
		next, err := bal.Add(t.Amount)
		if err != nil {
			return nil, fmt.Errorf("ledger: account %s register: %w", account.ID, err)
		}
		bal = next
		out[t.ID] = bal
	}
	return out, nil
}
