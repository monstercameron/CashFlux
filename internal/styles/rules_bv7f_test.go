// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBV7FSavingStatusUsesTheMetaTypeScale(t *testing.T) {
	resetSheet()
	registerBV7F()
	css := Build()
	for _, want := range []string{
		".toast-msg{font-size:var(--fs-meta);font-weight:400;font-variant-numeric:tabular-nums;color:var(--text-dim);",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("budget-saving toast CSS is missing %q", want)
		}
	}
}
