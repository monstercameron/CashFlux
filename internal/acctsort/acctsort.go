// SPDX-License-Identifier: MIT

// Package acctsort holds the pure ordering rules for the accounts list. It is
// platform-independent (no syscall/js) so the risk-first comparison is unit-tested
// on native Go and reused by the wasm accounts screen.
package acctsort

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
