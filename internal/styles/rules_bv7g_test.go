// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBV7GDialogTypeAndMonthReviewLayout(t *testing.T) {
	resetSheet()
	registerBV7G()
	css := Build()
	for _, want := range []string{
		".acct-edit-form.cover-all, .mc-body{font-size:var(--fs-body, 14px);font-weight:400;font-variant-numeric:tabular-nums;",
		".cover-all .cover-all-row-over, .mc-body .budget-sub, .mc-leftover-head, .mc-leftover-outcome{font-size:var(--fs-meta, 12.5px);font-weight:400;",
		".mc-leftover-head, .mc-leftover-row{display:grid;grid-template-columns:minmax(8rem, 1.4fr) minmax(5rem, 0.7fr) minmax(8rem, 1fr) minmax(9rem, 1.2fr);column-gap:0.8rem;",
		".budget-edit .budget-edit-row{grid-template-columns:repeat(2, minmax(0, 1fr));column-gap:1rem;row-gap:0.8rem;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("BV7G stylesheet is missing %q", want)
		}
	}
}
