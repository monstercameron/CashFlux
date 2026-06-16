//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// SettingsHost mounts at the shell root and renders the active settings panel
// (per-widget or global) as a FlipPanel overlay, driven by the shared settings
// atom. It renders nothing when no panel is open.
func SettingsHost() uic.Node {
	settings := uistate.UseSettings()
	target := settings.Get()
	if !target.Open() {
		return Fragment()
	}
	closePanel := func() { settings.Set(uistate.SettingsTarget{}) }

	switch target.Kind {
	case "global":
		// The global-settings body lands in a later feature; show the shell now.
		return ui.FlipPanel(ui.FlipPanelProps{
			Title:   "Settings",
			Width:   "760px",
			Height:  "560px",
			Back:    P(Class("text-dim text-[13px]"), "Household settings are coming soon."),
			OnClose: closePanel,
		})
	default: // "widget"
		return ui.FlipPanel(ui.FlipPanelProps{
			Title:   target.Title,
			Back:    uic.CreateElement(widgetSettingsForm, widgetSettingsFormProps{Title: target.Title}),
			OnClose: closePanel,
		})
	}
}

type widgetSettingsFormProps struct {
	Title string
}

// widgetSettingsForm is the per-widget settings back face: an editable title and
// the behavior toggles. State is local for now (persisting layout/visibility to
// the store arrives with the layout model); Save simply closes.
func widgetSettingsForm(props widgetSettingsFormProps) uic.Node {
	title := uic.UseState(props.Title)
	onDashboard := uic.UseState(true)
	allowMoving := uic.UseState(true)
	allowResizing := uic.UseState(true)
	compact := uic.UseState(false)

	onTitle := uic.UseEvent(func(v string) { title.Set(v) })

	return Div(
		Div(Class("set-label"), "Title"),
		Input(Class("set-input"), Type("text"), Value(title.Get()), OnInput(onTitle)),

		Div(Class("set-label"), "Behavior"),
		ui.ToggleRow(ui.ToggleRowProps{Label: "Show on dashboard", On: onDashboard.Get(), OnChange: func(v bool) { onDashboard.Set(v) }}),
		ui.ToggleRow(ui.ToggleRowProps{Label: "Allow moving", On: allowMoving.Get(), OnChange: func(v bool) { allowMoving.Set(v) }}),
		ui.ToggleRow(ui.ToggleRowProps{Label: "Allow resizing", On: allowResizing.Get(), OnChange: func(v bool) { allowResizing.Set(v) }}),
		ui.ToggleRow(ui.ToggleRowProps{Label: "Compact", On: compact.Get(), OnChange: func(v bool) { compact.Set(v) }}),
	)
}
