// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/notify"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// notifyWhyDrawer renders a notification's structured evidence — "Why this
// fired" — or nothing when the alert carries none (C408).
//
// A title and a body are both conclusions. "Dining is over budget" does not say
// what the limit was, what was spent, or which rule decided it mattered, so a
// reader who disagrees has nowhere to look and a reader who wants the alert to
// stop has nothing to adjust. The drawer answers both where the claim was made.
//
// A native <details> for the same reasons as the report's methodology drawer: no
// hooks, so it is safe inside a row list; keyboard-operable for free; closed by
// default, because evidence is what you reach for when a claim surprises you,
// not something to read eleven times a day.
//
// Alerts written before this shipped carry no Reason and simply show no drawer.
// Backfilling one would mean recomputing against today's data, which answers a
// different question from the one the reader asked.
func notifyWhyDrawer(id string, r notify.Reason) ui.Node {
	if r.Empty() {
		return Fragment()
	}
	var rows []any
	add := func(labelKey, value string) {
		if value == "" {
			return
		}
		rows = append(rows, Div(css.Class("notif-why-row"),
			Span(css.Class("notif-why-label"), uistate.T(labelKey)),
			Span(css.Class("notif-why-val"), value),
		))
	}
	add("notifWhy.trigger", uistate.ResolveText(r.Trigger))
	add("notifWhy.threshold", uistate.ResolveText(r.Threshold))
	add("notifWhy.observed", uistate.ResolveText(r.Observed))
	add("notifWhy.about", uistate.ResolveText(r.EntityLabel))

	// The link goes last, after the evidence: the drawer's job is to explain, and
	// offering the exit before the explanation invites the reader to skip it.
	var link ui.Node = Fragment()
	if r.EntityHref != "" {
		link = A(css.Class("notif-why-link"), Href(uistate.RoutePath(r.EntityHref)),
			Attr("data-testid", "notif-why-link-"+id), uistate.T("notifWhy.open"))
	}

	return Details(css.Class("notif-why"), Attr("data-testid", "notif-why-"+id),
		Summary(css.Class("notif-why-sum"), uistate.T("notifWhy.label")),
		Div(css.Class("notif-why-body"), rows, link),
	)
}
