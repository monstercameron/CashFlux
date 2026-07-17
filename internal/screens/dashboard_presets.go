// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// dashPresetPicker is the dashboard "Focus" select: applying a preset replaces
// the layout with a curated widget set for the moment (daily / payday / month
// end / debt / goals) via dashlayout.PresetItems, or restores the default
// arrangement. Applying pins the layout mode to Custom so an auto mode doesn't
// immediately re-sort the curated order. Session state remembers the last pick
// for the select's label; a manual drag simply diverges from it (the preset is
// a starting point, not a lock).
func dashPresetPicker(props struct{}) ui.Node {
	layoutAtom := uistate.UseLayoutItems()
	modeAtom := uistate.UseLayoutMode()
	sel := ui.UseState("")
	onPick := ui.UseEvent(func(e ui.Event) {
		key := e.GetValue()
		sel.Set(key)
		var items []dashlayout.Item
		if key == "default" {
			items = dashlayout.DefaultLayoutItems()
		} else if preset, ok := dashlayout.PresetItems(key); ok {
			items = preset
		} else {
			return
		}
		layoutAtom.Set(items)
		uistate.PersistItems(items)
		modeAtom.Set(dashlayout.ModeCustom)
		uistate.PersistLayoutMode(dashlayout.ModeCustom)
		uistate.PostNotice(uistate.T("dashboard.presetApplied", uistate.T(dashPresetLabelKey(key))), false)
	})

	cur := sel.Get()
	return Label(css.Class("fctrl"), Attr("data-testid", "dash-preset-wrap"),
		Span(css.Class("fctrl-label"), uistate.T("dashboard.presetLabel")),
		Select(css.Class("fctrl-select"), Attr("data-testid", "dash-preset"),
			Attr("aria-label", uistate.T("dashboard.presetLabel")), Title(uistate.T("dashboard.presetTitle")),
			OnChange(onPick),
			Option(Value(""), SelectedIf(cur == ""), uistate.T("dashboard.presetChoose")),
			Option(Value("daily"), SelectedIf(cur == "daily"), uistate.T("dashboard.presetDaily")),
			Option(Value("payday"), SelectedIf(cur == "payday"), uistate.T("dashboard.presetPayday")),
			Option(Value("monthend"), SelectedIf(cur == "monthend"), uistate.T("dashboard.presetMonthEnd")),
			Option(Value("debt"), SelectedIf(cur == "debt"), uistate.T("dashboard.presetDebt")),
			Option(Value("goals"), SelectedIf(cur == "goals"), uistate.T("dashboard.presetGoals")),
			Option(Value("default"), SelectedIf(cur == "default"), uistate.T("dashboard.presetDefault")),
		),
	)
}

// dashPresetLabelKey maps a preset key to its display-label i18n key.
func dashPresetLabelKey(key string) string {
	switch key {
	case "daily":
		return "dashboard.presetDaily"
	case "payday":
		return "dashboard.presetPayday"
	case "monthend":
		return "dashboard.presetMonthEnd"
	case "debt":
		return "dashboard.presetDebt"
	case "goals":
		return "dashboard.presetGoals"
	default:
		return "dashboard.presetDefault"
	}
}
