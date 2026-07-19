// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/state"
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
	// Seeded from persistence (QA task #44): the applied layout already survived
	// reloads, but this select fell back to "Choose a view…" — a control lying
	// about the state it controls. It now reads the active view's name.
	onPick := ui.UseEvent(func(e ui.Event) {
		applyDashPreset(layoutAtom, modeAtom, e.GetValue())
	})

	// Derived, not local state: the persisted key is the single source of truth,
	// so the select also follows a preset applied elsewhere (the Daily check-in
	// recommendation) instead of clinging to its mount-time value. applyDashPreset
	// writes the layout atom, which re-renders this component with the fresh key.
	cur := uistate.LoadDashPreset()
	return Label(css.Class("fctrl"), Attr("data-testid", "dash-preset-wrap"),
		Span(css.Class("fctrl-label"), uistate.T("dashboard.presetLabel")),
		Select(css.Class("fctrl-select"), Attr("data-testid", "dash-preset"),
			Attr("aria-label", uistate.T("dashboard.presetLabel")), Title(uistate.T("dashboard.presetTitle")),
			OnChange(onPick),
			// The full grid is one view named "Everything": both the never-chosen
			// state (cur=="") and an explicit restore (cur=="default") select it, and
			// picking it applies the default layout — so it truthfully names the
			// current state instead of a "Choose a view…" placeholder that lied while
			// Everything was active (2026-07-18 assessment).
			Option(Value("default"), SelectedIf(cur == "" || cur == "default"), uistate.T("dashboard.presetChoose")),
			Option(Value("daily"), SelectedIf(cur == "daily"), uistate.T("dashboard.presetDaily")),
			Option(Value("payday"), SelectedIf(cur == "payday"), uistate.T("dashboard.presetPayday")),
			Option(Value("monthend"), SelectedIf(cur == "monthend"), uistate.T("dashboard.presetMonthEnd")),
			Option(Value("debt"), SelectedIf(cur == "debt"), uistate.T("dashboard.presetDebt")),
			Option(Value("goals"), SelectedIf(cur == "goals"), uistate.T("dashboard.presetGoals")),
		),
	)
}

// applyDashPreset swaps the layout to the named preset (or restores the
// default), persists the pick + layout + mode, and flushes — the one shared
// apply path for the Focus select and the Daily check-in recommendation (#76).
// Reports whether the key was recognized and applied.
func applyDashPreset(layoutAtom state.Atom[[]dashlayout.Item], modeAtom state.Atom[dashlayout.Mode], key string) bool {
	var items []dashlayout.Item
	if key == "default" {
		items = dashlayout.DefaultLayoutItems()
	} else if preset, ok := dashlayout.PresetItems(key); ok {
		items = preset
	} else {
		return false
	}
	uistate.PersistDashPreset(key)
	layoutAtom.Set(items)
	uistate.PersistItems(items)
	modeAtom.Set(dashlayout.ModeCustom)
	uistate.PersistLayoutMode(dashlayout.ModeCustom)
	// kvSet only stages in the appstate snapshot; without an explicit flush
	// the preset pick AND the applied layout evaporate on reload (QA #44).
	uistate.RequestPersist()
	uistate.PostNotice(uistate.T("dashboard.presetApplied", uistate.T(dashPresetLabelKey(key))), false)
	return true
}

// The one-time Daily check-in recommendation (dashDailyNudge) was removed in the
// July 2026 UX pass: the review flagged it as noise on the dashboard's first
// viewport, since it only ever appeared for households already past their first
// week of use — the very people who don't need onboarding prompts. The Focus view
// picker (dashPresetPicker) remains the discoverable, opt-in way to switch to a
// calmer curated layout.

// layoutEditToggle flips the dashboard's edit-layout mode (#76): outside it the
// tiles hide their drag grips and resize handles and pointer drag is off, so
// the everyday surface stays calm; inside it the rearranging chrome returns.
func layoutEditToggle(_ struct{}) ui.Node {
	editAtom := uistate.UseLayoutEdit()
	onToggle := ui.UseEvent(func() { editAtom.Set(!editAtom.Get()) })
	editing := editAtom.Get()
	label := uistate.T("dashboard.editLayout")
	cls := "btn"
	if editing {
		label = uistate.T("dashboard.editLayoutDone")
		cls = "btn btn-primary"
	}
	pressed := "false"
	if editing {
		pressed = "true"
	}
	return Button(css.Class(cls), Type("button"),
		Attr("data-testid", "dash-edit-layout"),
		Attr("aria-pressed", pressed),
		Attr("title", uistate.T("dashboard.editLayoutTitle")),
		OnClick(onToggle),
		label,
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
