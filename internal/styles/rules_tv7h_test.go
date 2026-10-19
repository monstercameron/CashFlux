// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestTV7HPressedTransactionViewUsesContrastAwareInk(t *testing.T) {
	resetSheet()
	registerTV7H()
	got := Build()
	want := `.txn-viewtoggle-btn[aria-pressed="true"]{color:var(--primary-button-ink);}`
	if !strings.Contains(got, want) {
		t.Fatalf("pressed Transactions view should use contrast-aware text; missing %q in %s", want, got)
	}
}
