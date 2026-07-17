// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// reportEventChips annotates the report with the household's life events
// (domain.Event — trips, projects, weddings) that intersect the report window:
// the "why does this period look like that?" context the charts alone can't
// carry. Chips are read-only here; Manage routes to /events where the entities
// live. Renders nothing when no event touches the window.
func reportEventChips(props struct{}) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()
	w := uistate.UsePeriod().Get()
	nav := router.UseNavigate()
	goEvents := ui.UseEvent(Prevent(func() { nav.Navigate(uistate.RoutePath("/events")) }))

	start, end := w.Range()
	evs := reports.EventsIn(app.Events(), start, end)
	if len(evs) == 0 {
		return Fragment()
	}
	kids := []any{css.Class("t-caption"), Attr("data-testid", "report-event-chips"),
		Style(map[string]string{"display": "flex", "flex-wrap": "wrap", "gap": "0.35rem 0.75rem",
			"align-items": "center", "margin-top": "0.4rem"}),
		Span(css.Class("text-dim"), uistate.T("reports.eventsHeading"))}
	for _, e := range evs {
		rng := uistate.T("reports.eventOpenRange", e.Start.Format("Jan 2"))
		if !e.End.IsZero() {
			rng = uistate.T("reports.eventRange", e.Start.Format("Jan 2"), e.End.AddDate(0, 0, -1).Format("Jan 2"))
		}
		kids = append(kids, Span(css.Class("badge"), Attr("data-testid", "report-event-chip"),
			Title(e.Note), "◈ "+e.Name+" · "+rng))
	}
	kids = append(kids, Button(css.Class("btn-link"), Type("button"),
		Attr("data-testid", "report-events-manage"), OnClick(goEvents), uistate.T("reports.eventsManage")))
	return Div(kids...)
}
