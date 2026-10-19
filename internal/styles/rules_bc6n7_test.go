// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestBC6N7TemplateBucketSelectStaysCompact(t *testing.T) {
	resetSheet()
	registerBC6N7()
	css := Build()
	for _, want := range []string{
		".budget-tmpl-row .budget-tmpl-bucket{",
		"width:auto",
		"min-width:7.5rem",
		"flex:0 0 auto",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("template bucket rule missing %q: %s", want, css)
		}
	}
}
