// SPDX-License-Identifier: MIT

package styles

import (
	"strings"
	"testing"
)

func TestWHO3UnassignedAndAssignedAvatarsShareSizeAndType(t *testing.T) {
	resetSheet()
	registerUxbatch6()
	got := Build()
	for _, want := range []string{
		":root{--txn-avatar-size:22px;}",
		".txn-member-avatar{display:inline-flex;align-items:center;justify-content:center;width:var(--txn-avatar-size);height:var(--txn-avatar-size);",
		"font-size:10px;font-weight:700;line-height:1;}",
		".txn-table .td-user .cf-member-avatar, .cf-member-avatar-stack .cf-member-avatar{width:var(--txn-avatar-size);height:var(--txn-avatar-size);font-size:10px;line-height:1;}",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("WHO3 avatar stylesheet missing %q", want)
		}
	}
}
