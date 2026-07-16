// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestPutPayeeAliasRecordsRenameHistory verifies that renaming a merchant's clean name
// appends the PRIOR display to the alias's History (oldest first), preserves it across
// updates, and does not record a no-op re-save of the same name.
func TestPutPayeeAliasRecordsRenameHistory(t *testing.T) {
	a, err := New(nil, false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const raw = "SQ *BLUE BOTTLE #47"
	put := func(disp string) {
		t.Helper()
		if err := a.PutPayeeAlias(domain.PayeeAlias{RawPayee: raw, Display: disp}); err != nil {
			t.Fatalf("put %q: %v", disp, err)
		}
	}

	// First learn: no history yet.
	put("Blue Bottle")
	got := a.PayeeAliases()
	if len(got) != 1 || len(got[0].History) != 0 {
		t.Fatalf("after first learn: want 1 alias, 0 history; got %d aliases, %d history", len(got), len(got[0].History))
	}

	// Re-saving the SAME name must not pad the history.
	put("Blue Bottle")
	if h := a.PayeeAliases()[0].History; len(h) != 0 {
		t.Fatalf("re-saving same name padded history: %+v", h)
	}

	// Rename → the previous name is recorded.
	put("Blue Bottle Coffee")
	got = a.PayeeAliases()
	if len(got) != 1 {
		t.Fatalf("rename duplicated the alias: %d rows", len(got))
	}
	if got[0].Display != "Blue Bottle Coffee" {
		t.Fatalf("display = %q, want Blue Bottle Coffee", got[0].Display)
	}
	if len(got[0].History) != 1 || got[0].History[0].Display != "Blue Bottle" {
		t.Fatalf("history = %+v, want [Blue Bottle]", got[0].History)
	}

	// Second rename → lineage grows, oldest first.
	put("Blue Bottle SF")
	hist := a.PayeeAliases()[0].History
	if len(hist) != 2 || hist[0].Display != "Blue Bottle" || hist[1].Display != "Blue Bottle Coffee" {
		t.Fatalf("history = %+v, want [Blue Bottle, Blue Bottle Coffee]", hist)
	}
	// Each entry carries a supersession timestamp.
	for i, h := range hist {
		if h.At.IsZero() {
			t.Errorf("history[%d] has zero timestamp", i)
		}
	}
}
