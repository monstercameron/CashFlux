// SPDX-License-Identifier: MIT

package store

import (
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
)

// backfillLegacyTransferGroups migrates additive transaction JSON on ingress.
// Call after backfillLiabilitySigns, before replacing the transaction table, so
// both legs become durable in the same Load transaction. No global marker is
// needed: group identity makes this idempotent while allowing later imports.
func backfillLegacyTransferGroups(ds Dataset) Dataset {
	base := ds.Settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	ds.Transactions = txnclassify.BackfillLegacyTransferGroups(ds.Transactions, ds.Accounts,
		currency.Rates{Base: base, Rates: ds.Settings.FXRates})
	return ds
}
