// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestTV8BCheckboxFitsSelectionCell(t *testing.T) {
	resetSheet()
	registerTV8B()
	css := Build()
	for _, want := range []string{
		".bento-ledger .txn-table td.td-select{text-align:center;}",
		".bento-ledger .txn-table td.td-select input[type=checkbox]{margin:0 auto;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("TV8B CSS missing %q", want)
		}
	}
}
