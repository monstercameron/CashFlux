// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestTV7BNarrowSummaryMovesBarAndLegendToOwnLine(t *testing.T) {
	resetSheet()
	registerUXT1()
	registerTV7B()
	css := Build()
	for _, want := range []string{
		"@media (max-width: 1180px){.txn-count-band{flex-wrap:wrap;max-height:none;",
		"@media (max-width: 1180px){.txn-category-breakdown{flex-basis:100%;order:3;",
		"@media (max-width: 1180px){.txn-category-list{flex-wrap:wrap;overflow:visible;",
		".txn-category-item{flex:0 1 auto;width:auto;max-width:none;min-width:0;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("TV7B responsive summary is missing %q", want)
		}
	}
}
