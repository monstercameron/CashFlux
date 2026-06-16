//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Settings is the configuration page: a household summary and the in-app debug
// log viewer. Most editing (currency, FX, AI, data) lives in the household panel
// (the card at the bottom of the sidebar) and the dedicated screens.
func Settings() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}

	rev := state.UseAtom("rev:settings-log", 0)
	refresh := ui.UseEvent(func() { rev.Set(rev.Get() + 1) })

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	summary := Section(Class("card"),
		H2(Class("card-title"), uistate.T("settings.household")),
		Div(Class("rows"),
			settingRow(uistate.T("settings.baseCurrency"), base),
			settingRow(uistate.T("nav.members"), fmt.Sprintf("%d", len(app.Members()))),
			settingRow(uistate.T("nav.accounts"), fmt.Sprintf("%d", len(app.Accounts()))),
			settingRow(uistate.T("nav.categories"), fmt.Sprintf("%d", len(app.Categories()))),
		),
		P(Class("muted"), uistate.T("settings.manageHint")),
	)

	entries := app.LogRing().Entries()
	var logBody ui.Node
	if len(entries) == 0 {
		logBody = P(Class("empty"), uistate.T("settings.noLog"))
	} else {
		rows := make([]ui.Node, 0, len(entries))
		for i := len(entries) - 1; i >= 0; i-- { // newest first
			e := entries[i]
			rows = append(rows, Div(Class("row"),
				Div(Class("row-main"),
					Span(Class("row-desc"), e.Message),
					Span(Class("row-meta"), e.Level.String()),
				),
			))
		}
		logBody = Div(Class("rows"), rows)
	}

	logCard := Section(Class("card"),
		Div(Class("budget-head"),
			H2(Class("card-title"), uistate.T("settings.debugLog")),
			Button(Class("btn"), Type("button"), Title(uistate.T("settings.refreshLog")), OnClick(refresh), uistate.T("settings.refresh")),
		),
		logBody,
	)

	return Div(summary, logCard)
}

// settingRow renders a label/value pair as a simple row.
func settingRow(label, value string) ui.Node {
	return Div(Class("row"),
		Span(Class("row-desc"), label),
		Span(Class("amount"), value),
	)
}
