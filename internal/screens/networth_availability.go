// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/networthview"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func netWorthFigure(view networthview.Snapshot) string {
	if !view.Known {
		return "—"
	}
	return fmtMoney(view.Net)
}

func netWorthAvailabilityNote(view networthview.Snapshot) string {
	if !view.Known {
		return uistate.T("networth.unavailable")
	}
	if view.Partial {
		return uistate.T("networth.partial", plural(len(view.ExcludedAccounts), "account"), strings.Join(view.MissingCurrencies, ", "))
	}
	return ""
}

func netWorthDisclosure(view networthview.Snapshot) string {
	note := netWorthAvailabilityNote(view)
	if n := len(view.ExcludedByChoice); view.Known && n > 0 {
		if note != "" {
			note += " "
		}
		note += uistate.T("networth.byChoice", plural(n, "account"))
	}
	return note
}

// netWorthContext contains only aggregate amounts/counts/currency codes. Account
// names and the native error context must not leak into the Assistant's prompt.
func netWorthContext(view networthview.Snapshot) string {
	if !view.Known {
		return uistate.T("calculation.unavailable")
	}
	text := fmtMoney(view.Net)
	if note := netWorthDisclosure(view); note != "" {
		text += " (" + note + ")"
	}
	return text
}
