// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTXU6CountLineOmitsPageRange(t *testing.T) {
	const want = "1–25 of 3,292"
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{
		Shown: 25, Total: 3292, Range: txnPageRange(1, 25, 3292),
	}))
	if got := strings.Count(f.Text(), want); got != 0 {
		t.Fatalf("count line contains page range %d times, want none per R2A B4: %q", got, f.Text())
	}
}

func TestTXU6SavedViewsIsAReachableMoreMenuEntry(t *testing.T) {
	f := render.New(t)
	f.Render(uiw.OverflowMenu(uiw.OverflowMenuProps{
		TriggerText: "More",
		ExtraContent: ui.CreateElement(TxnSavedViewsMenu, txnSavedViewsMenuProps{
			Embedded: true,
		}),
	}))
	var more *render.QueryNode
	for _, button := range f.AllByTag("button") {
		if button.Attr("aria-haspopup") == "menu" {
			more = button
			break
		}
	}
	if more == nil {
		t.Fatal("More menu trigger is missing")
	}
	more.Click()
	f.Stabilize()
	if got := more.Attr("aria-expanded"); got != "true" {
		t.Fatalf("More expanded state = %q after activation, want true", got)
	}
	for _, button := range f.AllByTag("button") {
		if button.Attr("data-testid") == "txn-views-btn" {
			if button.Attr("role") != "menuitem" {
				t.Fatalf("Views entry role = %q, want menuitem", button.Attr("role"))
			}
			return
		}
	}
	t.Fatal("Saved Views entry is not present inside the More menu")
}
