// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestTxnWhoCellUnassignedAndSingleOwner(t *testing.T) {
	f := render.New(t)
	f.Render(txnWhoCell(nil))
	unassigned := f.ByRole("img", "Unassigned")
	if !unassigned.Exists() || unassigned.Text() != "—" {
		t.Fatalf("unassigned marker text=%q exists=%t", unassigned.Text(), unassigned.Exists())
	}
	cell := findTestID(f, "span", "txn-who-unassigned")
	if cell == nil || !cell.Exists() || cell.Attr("title") != "Unassigned" {
		t.Fatal("unassigned marker must expose the same tooltip and accessible name")
	}

	f.Render(txnWhoCell([]string{"Marcus Hartley"}))
	avatar := findTestID(f, "span", "txn-who-avatar")
	if avatar == nil || !avatar.Exists() || avatar.Text() != "MH" || avatar.Attr("aria-label") != "Marcus Hartley" {
		if avatar == nil {
			t.Fatal("single owner avatar not found")
		}
		t.Fatalf("single owner avatar text=%q aria-label=%q exists=%t", avatar.Text(), avatar.Attr("aria-label"), avatar.Exists())
	}
}

func TestTxnWhoCellStackKeepsAccessibleNameAndCapsVisibleOwners(t *testing.T) {
	names := []string{"Marcus Hartley", "Priya Hartley", "Nadia Hartley", "Alex Hartley", "Sam Hartley"}
	f := render.New(t)
	f.Render(txnWhoCell(names))
	stack := f.ByRole("img", "Shared by Marcus Hartley, Priya Hartley, Nadia Hartley, Alex Hartley, and Sam Hartley")
	if !stack.Exists() || !strings.Contains(stack.Text(), "+2") {
		t.Fatalf("stack text=%q exists=%t", stack.Text(), stack.Exists())
	}
	if got := stack.Attr("title"); got != "Shared by Marcus Hartley, Priya Hartley, Nadia Hartley, Alex Hartley, and Sam Hartley" {
		t.Fatalf("stack title=%q", got)
	}
	if strings.Count(stack.Text(), "MH") != 1 || !strings.Contains(stack.Text(), "PH") || !strings.Contains(stack.Text(), "NH") {
		t.Fatalf("stack should show the first three avatars and remainder count, got %q", stack.Text())
	}
}

func TestTxnWhoFormMultiPickerRoundTrip(t *testing.T) {
	owners := []string{"m1"}
	f := render.New(t)
	f.Render(ui.CreateElement(txnWhoRoundTripHarness, txnWhoRoundTripProps{Owners: owners}))
	if !f.ByRole("img", "Marcus Hartley").Exists() {
		t.Fatal("initial owner should appear in the selected owner summary")
	}
	owners = toggleTxnOwner(owners, "m2")
	f.Render(ui.CreateElement(txnWhoRoundTripHarness, txnWhoRoundTripProps{Owners: owners}))
	if !f.ByRole("img", "Shared by Marcus Hartley and Priya Hartley").Exists() {
		t.Fatal("round-tripped owners should appear in the shared-owner summary")
	}
}

func findTestID(f *render.Fixture, tag, id string) *render.QueryNode {
	for _, node := range f.AllByTag(tag) {
		if node.Attr("data-testid") == id {
			return node
		}
	}
	return nil
}

type txnWhoRoundTripProps struct {
	Owners []string
}

func txnWhoRoundTripHarness(props txnWhoRoundTripProps) ui.Node {
	options := []uiw.PickerOption{{Value: "m1", Label: "Marcus Hartley"}, {Value: "m2", Label: "Priya Hartley"}}
	return shorthand.Fragment(
		TxnWhoField(TxnWhoFieldProps{ID: "who-form", Owners: props.Owners, Options: options}),
	)
}
