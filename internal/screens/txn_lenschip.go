// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// lensChipKey marks the transactions toolbar chip that represents the TOP BAR's
// member perspective rather than one of the page's own filters.
//
// The distinction is the whole point of C574. Every other chip is a value inside
// the persisted filter, so its ✕ removes that value. The perspective is not in the
// filter at all — it is ambient app state layered on at render time — so a ✕ that
// went through the filter's RemoveValue removed nothing and left the chip sitting
// there. Keying it apart lets the one handler route each chip to the state that
// actually owns it.
const lensChipKey = "lens"

// toolbarIconBtnTitled is a toolbar button whose accessible name carries MORE than
// its visible label.
//
// It exists for counts whose scope cannot fit on the button. "Review inbox (249)"
// beside a ledger reading "122 of 3,227" is two populations rendered identically;
// the button has room for the number but not for "across the whole household,
// ignoring this page's filters". Putting that in the title alone would leave it to
// hover only, so it is the accessible name as well — the sentence is the point.
func toolbarIconBtnTitled(testID string, ic icon.Name, label, title string, onClick ui.Handler, variant string) ui.Node {
	cls := "btn btn-tool"
	switch variant {
	case "primary":
		cls += " btn-primary"
	case "danger":
		cls += " bt-danger"
	}
	args := []any{
		css.Class(cls), Type("button"),
		Attr("aria-label", label+" — "+title), Attr("title", title), OnClick(onClick),
		uiw.Icon(ic, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(label),
	}
	if testID != "" {
		args = append(args, Attr("data-testid", testID))
	}
	return Button(args...)
}

// clearAllLabel is the text for the filter summary's reset, or "" to hide it.
//
// The count is the number of the PAGE's own filters, which is not always the
// number of chips on screen: the member-lens chip is cleared from the top bar, so
// with a lens on and no filters set the reset would read "Clear all 0 filters" and
// do nothing when pressed. A control with nothing to do should not be there.
func clearAllLabel(n int) string {
	if n == 0 {
		return ""
	}
	return uistate.T("transactions.clearAllFiltersN", plural(n, "filter"))
}

// customFieldLabel resolves a transaction custom-field key to the label the user
// gave it, falling back to the raw key so a chip never renders as a bare value
// with no field name attached.
func customFieldLabel(defs []customfields.Def, key string) string {
	for _, d := range defs {
		if d.Key == key {
			return d.Label
		}
	}
	return key
}
