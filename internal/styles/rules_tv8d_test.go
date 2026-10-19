// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestTV8DSplitRowsUseGridAndModalContentsDoNotShrink(t *testing.T) {
	resetSheet()
	registerTV8D()
	css := Build()
	for _, want := range []string{
		".split-row{display:grid;gap:0.5rem;align-items:center;}",
		".split-editor-modal .modal-scroll > *{flex-shrink:0;}",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("missing split dialog layout rule %q in %s", want, css)
		}
	}
}
