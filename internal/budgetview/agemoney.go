// SPDX-License-Identifier: MIT

package budgetview

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/agemoney"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// AgeOfMoney converts the household's real cash movements into an ordered FIFO
// history. Transfers and zeroes are skipped; reporting exclusions still move real
// money and remain included. Unknown FX is omitted.
// Equal-date transactions retain their original ordering.
func AgeOfMoney(txns []domain.Transaction, base string, rates currency.Rates) agemoney.Result {
	flows := make([]agemoney.Flow, 0, len(txns))
	for _, txn := range txns {
		if txn.IsTransfer() || txn.Amount.Amount == 0 {
			continue
		}
		minor := txn.Amount.Amount
		if txn.Amount.Currency != base {
			converted, err := currency.ConvertBetween(minor, txn.Amount.Currency, base, rates)
			if err != nil {
				continue
			}
			minor = converted
		}
		flows = append(flows, agemoney.Flow{Date: txn.Date, AmountMinor: minor})
	}
	sort.SliceStable(flows, func(i, j int) bool { return flows[i].Date.Before(flows[j].Date) })
	return agemoney.Compute(flows, agemoney.Opts{})
}
