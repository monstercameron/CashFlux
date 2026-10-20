// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dashboardview"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// forecastWidget is the 30/60/90-day cash forecast tile: the projected total of
// the household's AVAILABLE cash (internal/liquidity) at each horizon, summed
// from every cash account's own recurring-driver projection — the same
// acctproject curve the accounts page's AC13 low-dip line uses, so the dashboard
// and the account rows can never tell different stories. Figures are
// base-converted; rate-less accounts are skipped (consistent with net worth's
// exclusion disclosure). Each horizon tones by its delta against today.
func forecastWidget(c widgetrender.RenderCtx) ui.Node {
	return ui.CreateElement(forecastCard, c)
}

// forecastCard owns its memo hook independently of the variable-length widget
// registry loop, so rearranging or hiding tiles cannot shift the hook order.
func forecastCard(c widgetrender.RenderCtx) ui.Node {
	app := c.App
	now := time.Now()
	model := ui.UseMemo(func() dashboardview.CashForecast {
		return dashboardview.BuildCashForecast(c.ScopedAccounts, c.Rates, now, app.ProjectAccount)
	}, revKey(app), acctSig(c.ScopedAccounts), now.Format(dateutil.Layout))

	var body ui.Node
	if model.Error != nil {
		body = P(css.Class("empty t-body", tw.TextDim), Attr("data-testid", "forecast-unavailable"), uistate.T("dashboard.forecastUnavailable"))
	} else if model.Counted == 0 {
		body = P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.forecastEmpty"))
	} else {
		stat := func(i int, labelKey string) ui.Node {
			tone := "text-up"
			if model.Totals[i] < model.Today {
				tone = "text-down"
			}
			return Div(Style(map[string]string{"display": "grid", "gap": "0.1rem"}),
				Span(css.Class("t-caption", tw.TextDim), uistate.T(labelKey)),
				Span(ClassStr("fig "+tw.Fold(tw.FontDisplay, tw.LeadingTight)+" "+tw.ColorClass(tone)),
					Attr("data-testid", "forecast-"+[3]string{"30", "60", "90"}[i]),
					fmtMoney(money.New(model.Totals[i], c.Base))),
			)
		}
		body = Div(
			Div(Style(map[string]string{"display": "flex", "gap": "1.5rem", "flex-wrap": "wrap"}),
				stat(0, "dashboard.forecast30"),
				stat(1, "dashboard.forecast60"),
				stat(2, "dashboard.forecast90"),
			),
			P(css.Class("t-caption", tw.TextDim, tw.Pt15),
				uistate.T("dashboard.forecastSub", fmtMoney(money.New(model.Today, c.Base)))),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "forecast", Title: uistate.T("dashboard.forecast"), Draggable: !c.Preview, Resizable: !c.Preview,
		Preview: c.Preview, GridColumn: "1 / span 2",
		Body: Div(Attr("data-testid", "dash-forecast"), body,
			If(model.Excluded > 0, P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "forecast-excluded"), uistate.T("dashboard.forecastExcluded")))),
	})
}
