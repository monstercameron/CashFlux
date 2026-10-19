// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

// These CSS strings are the interaction contract: hover is capability-gated,
// keyboard focus is visible, and row state surfaces use the theme tokens.
func TestSOL1LedgerHoverAndFocusStyleContract(t *testing.T) {
	resetSheet()
	registerSOL1()
	got := Build()
	for _, want := range []string{
		"@media (hover: hover){.txn-ledger-row:not(.selected):hover{",
		"@media (hover: hover){.txn-table .txn-description-edit:hover{",
		"background:var(--txn-row-surface, var(--bg-card))",
		"--txn-row-surface:color-mix(in srgb, var(--hover) 45%, var(--bg-card))",
		".txn-table .txn-description-edit:focus-visible{",
		"outline:2px solid var(--accent)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing style contract %q", want)
		}
	}
	for _, forbidden := range []string{"!important", "z-index:", "position:fixed", "#"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("unexpected style %q", forbidden)
		}
	}
}
