// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestForm3RegistersSharedResponsiveFieldGrid(t *testing.T) {
	resetSheet()
	registerForm3()
	cssText := Build()
	for _, want := range []string{
		".txn-form-grid{display:grid;grid-template-columns:repeat(2, minmax(0, 1fr))",
		".txn-form-grid .cf-field, .txn-form-grid .field, .txn-form-grid select{box-sizing:border-box;width:100%;min-width:0;}",
		"@media (max-width: 640px){.txn-form-grid{grid-template-columns:minmax(0, 1fr);}}",
	} {
		if !strings.Contains(cssText, want) {
			t.Errorf("FORM3 stylesheet is missing %q in %s", want, cssText)
		}
	}
}
