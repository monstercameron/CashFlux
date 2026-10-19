// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "time"

// budgetPayCycleAnchor parses the household's stored ISO pay-cycle anchor.
// Invalid or unset values leave biweekly periods on their deterministic default.
func budgetPayCycleAnchor(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	anchor, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}
	}
	return anchor
}
