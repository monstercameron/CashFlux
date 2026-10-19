// SPDX-License-Identifier: MIT

package txnclassify

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// BackfillLegacyTransferGroups returns a copy with durable identities for
// unambiguous legacy transfers. It uses the reporting matcher, including account
// booking conventions and optional FX conversion. Ambiguous, incomplete, and
// already grouped rows are untouched. IDs depend on row identity, never clocks
// or input order, so repeating a migration or importing the same rows is safe.
func BackfillLegacyTransferGroups(rows []domain.Transaction, accounts []domain.Account, rateTables ...currency.Rates) []domain.Transaction {
	if rows == nil {
		return nil
	}
	result := slices.Clone(rows)
	summary := SummarizeVisibleTransfersWithAccounts(rows, accounts, rateTables...)
	used := make(map[string]bool)
	identities := make(map[string]int, len(rows))
	for _, row := range rows {
		identities[row.ID]++
		if row.TransferGroupID != "" {
			used[row.TransferGroupID] = true
		}
	}
	for _, pair := range summary.Pairs {
		a, b := rows[pair.Outgoing], rows[pair.Incoming]
		if a.TransferGroupID != "" || b.TransferGroupID != "" || a.ID == "" || b.ID == "" || identities[a.ID] != 1 || identities[b.ID] != 1 {
			continue
		}
		first, second := a.ID, b.ID
		if first > second {
			first, second = second, first
		}
		// Length prefixes avoid collisions from IDs containing delimiters.
		digest := sha256.Sum256([]byte(strconv.Itoa(len(first)) + ":" + first + strconv.Itoa(len(second)) + ":" + second))
		base := fmt.Sprintf("legacy-transfer-v1-%x", digest)
		group := base
		for suffix := 1; used[group]; suffix++ {
			group = base + "-" + strconv.Itoa(suffix)
		}
		used[group] = true
		result[pair.Outgoing].TransferGroupID = group
		result[pair.Incoming].TransferGroupID = group
	}
	return result
}

// TransferOutgoing reports whether a booked transfer leg moves money out of
// its account under that account's fixed liability sign convention.
func TransferOutgoing(row domain.Transaction, account domain.Account) bool {
	return row.Amount.Amount != 0 && !incoming(row.Amount.Amount, account.InflowSign())
}
