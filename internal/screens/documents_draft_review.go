// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/importdraft"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"

	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// draftSumItem renders one figure of the statement summary (a small uppercase
// label above a bold amount). tone "in"/"out" tints the amount with the app's
// money-positive/negative tokens; anything else leaves it neutral.
func draftSumItem(label, value, tone string) ui.Node {
	valCls := "draft-sum-val fig"
	switch tone {
	case "in":
		valCls += " amount-income"
	case "out":
		valCls += " amount-expense"
	}
	return Div(css.Class("draft-sum-item"),
		Span(css.Class("draft-sum-label"), label),
		Span(css.Class(valCls), value),
	)
}

// draftSumNet renders the net figure — the summary's emphasized close — tinted by
// its own sign and pushed to the right so it foots the amount column like a
// statement's bottom line.
func draftSumNet(netMinor int64, cur string) ui.Node {
	cls := "draft-sum-val draft-sum-net fig"
	cls += importdraft.SignClass(netMinor)
	return Div(css.Class("draft-sum-item draft-sum-item-net"),
		Span(css.Class("draft-sum-label"), uistate.T("documents.sumNet")),
		Span(css.Class(cls), fmtMoney(money.New(netMinor, cur))),
	)
}
