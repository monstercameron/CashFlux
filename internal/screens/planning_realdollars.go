// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/inflation"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// ─── FP-T2d: long-horizon figures, said in money the reader can price ────────
//
// A twelve-month projection is stated in the dollars of twelve months from now.
// Read as today's money — which is how everyone reads a number with a dollar
// sign in front of it — it overstates the result by a year of inflation. The
// error is small over one year and compounds badly over the horizons the rest of
// planning deals in, and the fix is the same in both cases: say what the figure
// is worth in money the reader has actually spent.
//
// The rate is the HOUSEHOLD's single assumption, not a per-card one. Two cards
// with independent inflation settings would eventually give two answers to the
// same question with no way to tell which the app meant.
//
// A plain render helper, not a component: it registers no hooks, so it can be
// called from anywhere in a card without adding a hook position.

// forecastRealLine states a nominal projection in today's money.
//
// It renders NOTHING when the discount is the identity — a zero rate, or a
// figure that rounds to itself. Printing "in today's money, $12,400" beside
// "$12,400" adds a line and no information, and a line that says nothing trains
// the reader to skip the ones that do.
func forecastRealLine(nominalMinor int64, base string) ui.Node {
	pct := uistate.LoadInflationPct()
	realMinor, ok := inflation.RealMinor(nominalMinor, pct, forecastHorizonYears)
	if !ok || realMinor == nominalMinor {
		return Fragment()
	}
	return P(css.Class("muted", tw.Text12), Attr("data-testid", "forecast-real"),
		uistate.T("planning.forecastReal", fmtMoney(money.New(realMinor, base)), pct))
}

// forecastHorizonYears is how far ahead the planning forecast projects, in
// years, matching the twelve months it charts. It is stated here rather than
// derived from the series length so the deflation and the chart cannot drift
// apart silently if the horizon changes.
const forecastHorizonYears = 1.0
