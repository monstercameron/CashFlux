// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/goals"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// goalStatesWidget is the dashboard "Goals at a glance" tile: a three-count summary
// of how the household's objectives are tracking — Current (in flight, on time),
// Missed (a dated goal whose deadline passed unfunded), and Completed (reached, via
// the earmark-aware definition in internal/goals.Reached). Sinking funds are excluded
// (they're ongoing buckets, not finish-line goals). Each count opens the Goals page.
func goalStatesWidget(app *appstate.App) ui.Node {
	_ = uistate.UseDataRevision().Get() // re-render when goals/tasks change
	nav := router.UseNavigate()
	open := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/goals")) })

	c := goals.CountByState(app.Goals(), app.Tasks(), time.Now(), false)
	if c.Total() == 0 {
		return uiw.Widget(uiw.WidgetProps{
			ID: "goal-states", Title: uistate.T("dashboard.goalStates"), Draggable: true, Resizable: true,
			Body: ui.CreateElement(emptyAddCTA, emptyAddProps{
				Message: uistate.T("dashboard.noGoalsYet"), Label: uistate.T("dashboard.addGoal"), Path: "/goals",
			}),
		})
	}

	// A missed goal is the only one that should draw the eye — tint it red only when
	// there's something to flag, so a clean slate stays calm.
	missedMod := ""
	if c.Missed > 0 {
		missedMod = " is-missed"
	}
	body := Div(css.Class("dash-goalstates"),
		goalStateStat(open, "goal-states-current", c.Current, uistate.T("dashboard.goalsCurrent"), ""),
		goalStateStat(open, "goal-states-missed", c.Missed, uistate.T("dashboard.goalsMissed"), missedMod),
		goalStateStat(open, "goal-states-completed", c.Completed, uistate.T("dashboard.goalsCompleted"), " is-done"),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "goal-states", Title: uistate.T("dashboard.goalStates"), Draggable: true, Resizable: true,
		Body: body,
	})
}

// goalStateStat renders one count cell in the goal-states tile: a large serif number
// over a small uppercase label, as a button that opens the Goals page. mod is an
// optional tone modifier class (" is-missed" / " is-done"); open is the shared
// navigate handler (a fixed, non-looped set of three cells, so the hook order is stable).
func goalStateStat(open ui.Handler, testID string, n int, label, mod string) ui.Node {
	return Button(ClassStr("dgs-cell"+mod), Type("button"),
		Attr("data-testid", testID), Attr("aria-label", label),
		OnClick(open),
		Span(css.Class("dgs-n", tw.FontDisplay), strconv.Itoa(n)),
		Span(css.Class("dgs-k"), label),
	)
}
