// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/provisional"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// provisionalNotice captions a report's period with how settled its figures are
// (C693).
//
// A month whose statements have all arrived will not move again. The month you
// are living in holds a few weeks of transactions, whatever has not posted yet,
// and any balance checkpoint standing in for the gap — and it renders in exactly
// the same type as a finished month, which invites the one comparison that means
// least. A month-to-date total always looks like a collapse in spending, because
// it is three weeks short.
//
// It states rather than warns, and it is absent entirely for a closed period that
// rests on nothing: a banner that appears on every report is one nobody reads.
func provisionalNotice(app *appstate.App, start, end time.Time) ui.Node {
	if app == nil {
		return Fragment()
	}
	st := provisional.StandingOf(app.Transactions(), start, end, app.Accounts())
	if st.Status == provisional.Closed && !st.RestsOnAGuess() {
		return Fragment()
	}

	base := "USD"
	if s := app.Settings(); s.BaseCurrency != "" {
		base = s.BaseCurrency
	}
	dec := currency.Decimals(base)

	var lines []ui.Node
	if st.Status == provisional.Provisional {
		through, ok := provisional.ClosedThrough(app.Accounts())
		msg := uistate.T("reports.provisionalNoStatements")
		if ok {
			msg = uistate.T("reports.provisionalThrough", through.Format("January 2, 2006"))
		}
		lines = append(lines, P(css.Class("t-body"), Attr("data-testid", "reports-provisional-status"), msg))
	}
	if st.RestsOnAGuess() {
		lines = append(lines, P(css.Class("muted", tw.Text12), Attr("data-testid", "reports-provisional-checkpoints"),
			uistate.TN("reports.provisionalCheckpointOne", "reports.provisionalCheckpointMany", st.Checkpoints,
				st.Checkpoints, money.FormatMinor(abs64(st.CheckpointMinor), dec))))
	}
	return Div(css.Class("card", tw.P3, tw.FlexCol, tw.Gap1), Attr("data-testid", "reports-provisional-notice"),
		Attr("role", "status"), lines)
}

// abs64 is the magnitude of a signed minor amount — a checkpoint can correct a
// balance in either direction, and the caption names how much is resting on a
// guess rather than which way it leans.
func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
