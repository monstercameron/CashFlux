// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR4CopyCapsTransactionsProse(t *testing.T) {
	resetSheet()
	registerR4Copy()
	css := Build()
	if !strings.Contains(css, ".rvs-note, .filter-inline-panel.filter-sheet p{max-width:68ch;}") {
		t.Fatalf("Transactions prose width rule missing: %s", css)
	}
}
