// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/nwchange"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// nwChangeSub is the ONE way this app says how far net worth moved (C341).
//
// Every surface that prints a net-worth delta — the dashboard hero, the
// /accounts summary, the Reports net-worth tab, the /networth page — renders it
// through here, so the same window can never be described in two different
// wordings, and a delta can never appear without the window that produced it.
// Returns the sentence and the tone class the caller should colour it with.
//
// The arrow carries the direction and the amount is always printed as a
// magnitude: fmtMoney renders negatives in accounting parentheses, so a signed
// amount inside the "(…)" of a percentage phrase used to read "▼ 3% (($120.00))".
// A flat 0% is spelled out rather than shown as a misleading "▲ 0%": nothing
// moved and nothing moving are different facts (G1 §7).
func nwChangeSub(c nwchange.Change) (text, tone string) {
	window := uistate.T("nw.windowMonthToDate")
	if !c.Known {
		return uistate.T("nw.changeUnknown", window), "text-dim"
	}
	delta := c.Delta()
	abs := fmtMoney(money.New(absMinor(delta.Amount), delta.Currency))
	pct, havePct := c.PercentChange()
	switch {
	case delta.Amount == 0:
		return uistate.T("nw.changeNone", window), "text-dim"
	case delta.Amount < 0 && havePct && pct < 0:
		return uistate.T("nw.changeDown", -pct, abs, window), "text-down"
	case delta.Amount > 0 && havePct && pct > 0:
		return uistate.T("nw.changeUp", pct, abs, window), "text-up"
	case delta.Amount < 0:
		// Real movement the percentage cannot describe (the window opened at
		// zero, or it rounded to 0%): show the amount, skip the false precision.
		return uistate.T("nw.changeDownAmount", abs, window), "text-down"
	default:
		return uistate.T("nw.changeUpAmount", abs, window), "text-up"
	}
}
