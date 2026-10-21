// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestWHO3UnassignedAndAssignedTransactionCellsUseAvatarSizingRules(t *testing.T) {
	f := render.New(t)
	f.Render(shorthand.Fragment(
		txnWhoCell(nil),
		txnWhoCell([]string{"Ada Lovelace"}),
	))

	unassigned := f.ByRole("img", "Unassigned")
	assigned := f.ByRole("img", "Ada Lovelace")
	if !unassigned.Exists() || !assigned.Exists() {
		t.Fatalf("both transaction owner cells must render: unassigned=%t assigned=%t", unassigned.Exists(), assigned.Exists())
	}
	unassignedClass, assignedClass := unassigned.Attr("class"), assigned.Attr("class")
	if !strings.Contains(unassignedClass, "cf-member-unassigned") || !strings.Contains(assignedClass, "cf-member-avatar") {
		t.Fatalf("owner cells must carry the classes styled by the shared avatar rules: unassigned=%q assigned=%q", unassignedClass, assignedClass)
	}
}
