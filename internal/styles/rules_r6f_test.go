// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestR6FGhostTextUsesFullContrastToken(t *testing.T) {
	resetSheet()
	registerR6F()
	css := Build()
	if !strings.Contains(css, ".txn-scheduled-ghost{color:var(--text);font-style:normal;opacity:1;") {
		t.Fatalf("ghost row must use normal, full-strength primary text; CSS: %s", css)
	}
	if strings.Contains(css, ".txn-scheduled-ghost{color:var(--text-dim)") || strings.Contains(css, ".txn-scheduled-ghost{color:var(--text-faint)") {
		t.Fatal("ghost row text uses a dim token")
	}
	if !strings.Contains(css, ".txn-scheduled-ghost .td-acct{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}") {
		t.Fatal("scheduled account column does not truncate on one line")
	}
}
