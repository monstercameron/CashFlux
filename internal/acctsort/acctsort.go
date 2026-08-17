// SPDX-License-Identifier: MIT

// Package acctsort holds the pure ordering rules for the accounts list. It is
// platform-independent (no syscall/js) so the risk-first comparison is unit-tested
// on native Go and reused by the wasm accounts screen.
package acctsort

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// RiskFirstLess reports whether account i should sort before account j under the
// accounts page's default "risk-first" ordering: an account that needs attention
// (stale — past its freshness window, or otherwise flagged) always leads a healthy
// one, so the rows worth acting on sit at the top. Within the same freshness state
// the ordering falls back to net-worth contribution, larger first (balI/balJ are the
// signed, base-converted balances the list already computes: assets positive,
// liabilities negative), preserving the familiar "biggest holdings first" layout.
func RiskFirstLess(staleI, staleJ bool, balI, balJ int64) bool {
	if staleI != staleJ {
		// The stale account is "less" (sorts earlier) than the healthy one.
		return staleI
	}
	return balI > balJ
}

// ForPicker returns accounts ordered the way a DROPDOWN should present them:
// alphabetically by name, case-insensitively, with ties broken by id so the order
// is stable across renders.
//
// Deliberately different from RiskFirstLess above, which orders the accounts PAGE.
// There, the question is "what needs my attention", so the urgent and the large
// lead. In a picker the question is "where is the one I already have in mind",
// and the only ordering that answers it is the one the reader can predict. A
// risk-first dropdown reshuffles itself as balances move, which means the account
// you picked last week is somewhere else today.
//
// Store order — which is what every account picker in the app used before this —
// answers neither question: it is the order the accounts happened to be created.
//
// The input slice is not modified; a sorted copy is returned.
func ForPicker(accounts []domain.Account) []domain.Account {
	out := make([]domain.Account, len(accounts))
	copy(out, accounts)
	sort.SliceStable(out, func(i, j int) bool {
		li, lj := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if li != lj {
			return li < lj
		}
		// Two accounts genuinely named the same thing ("Checking" at two banks) are
		// ordered by id so the list does not shuffle between renders.
		return out[i].ID < out[j].ID
	})
	return out
}
