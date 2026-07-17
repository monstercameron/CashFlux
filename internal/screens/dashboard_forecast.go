// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/liquidity"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// forecastWidget is the 30/60/90-day cash forecast tile: the projected total of
// the household's AVAILABLE cash (internal/liquidity) at each horizon, summed
// from every cash account's own recurring-driver projection — the same
// acctproject curve the accounts page's AC13 low-dip line uses, so the dashboard
// and the account rows can never tell different stories. Figures are
// base-converted; rate-less accounts are skipped (consistent with net worth's
// exclusion disclosure). Each horizon tones by its delta against today.
func forecastWidget(c widgetrender.RenderCtx) ui.Node {
	app := c.App
	now := time.Now()
	horizons := [3]int{30, 60, 90}
	var totals [3]int64
	var today int64
	counted := 0
	for _, a := range c.ScopedAccounts {
		if a.Archived || a.Class == domain.ClassLiability || liquidity.Of(a, now) != liquidity.Available {
			continue
		}
		// Convert this account's projected figures; skip the account wholly on a
		// missing rate so a partial sum never masquerades as the full picture.
		proj0 := app.ProjectAccount(a.ID, now, horizons[0])
		st, err := c.Rates.Convert(money.New(proj0.Start, a.Currency), c.Base)
		if err != nil {
			continue
		}
		today += st.Amount
		counted++
		for i, h := range horizons {
			proj := proj0
			if i > 0 {
				proj = app.ProjectAccount(a.ID, now, h)
			}
			if cv, cerr := c.Rates.Convert(money.New(proj.End, a.Currency), c.Base); cerr == nil {
				totals[i] += cv.Amount
			}
		}
	}

	var body ui.Node
	if counted == 0 {
		body = P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.forecastEmpty"))
	} else {
		stat := func(i int, labelKey string) ui.Node {
			tone := "text-up"
			if totals[i] < today {
				tone = "text-down"
			}
			return Div(Style(map[string]string{"display": "grid", "gap": "0.1rem"}),
				Span(css.Class("t-caption", tw.TextDim), uistate.T(labelKey)),
				Span(ClassStr("fig "+tw.Fold(tw.FontDisplay, tw.LeadingTight)+" "+tw.ColorClass(tone)),
					Attr("data-testid", "forecast-"+[3]string{"30", "60", "90"}[i]),
					fmtMoney(money.New(totals[i], c.Base))),
			)
		}
		body = Div(
			Div(Style(map[string]string{"display": "flex", "gap": "1.5rem", "flex-wrap": "wrap"}),
				stat(0, "dashboard.forecast30"),
				stat(1, "dashboard.forecast60"),
				stat(2, "dashboard.forecast90"),
			),
			P(css.Class("t-caption", tw.TextDim, tw.Pt15),
				uistate.T("dashboard.forecastSub", fmtMoney(money.New(today, c.Base)))),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "forecast", Title: uistate.T("dashboard.forecast"), Draggable: !c.Preview, Resizable: !c.Preview,
		Preview: c.Preview, GridColumn: "1 / span 2",
		Body: Div(Attr("data-testid", "dash-forecast"), body),
	})
}
