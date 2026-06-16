//go:build js && wasm

package app

import (
	"sort"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
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
		return ui.FlipPanel(ui.FlipPanelProps{
			Title:   "Settings",
			Width:   "760px",
			Height:  "560px",
			Back:    uic.CreateElement(globalSettingsForm),
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

// globalSettingsForm is the two-column household/global settings back face:
// members, base currency and FX rows (left) and AI, appearance, and data
// actions (right). Members, base currency, and rates are read live from app
// state; appearance controls hold local state for now (persisting preferences
// and wiring data actions land in their own features).
func globalSettingsForm() uic.Node {
	theme := uic.UseState("dark")
	accent := uic.UseState("#54b884")
	compact := uic.UseState(false)
	aiOn := uic.UseState(false)

	var members []domain.Member
	base := "USD"
	var fxRows []uic.Node
	if app := appstate.Default; app != nil {
		members = app.Members()
		s := app.Settings()
		if s.BaseCurrency != "" {
			base = s.BaseCurrency
		}
		codes := make([]string, 0, len(s.FXRates))
		for code := range s.FXRates {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			fxRows = append(fxRows, rateRow(code, s.FXRates[code], base))
		}
	}

	memberChips := make([]uic.Node, 0, len(members)+1)
	for _, m := range members {
		memberChips = append(memberChips, memberChip(m))
	}
	memberChips = append(memberChips, Button(Class("member-add"), Type("button"), "+ Add member"))

	left := Div(
		Div(Class("set-label"), "Household members"),
		Div(Class("flex flex-wrap gap-2 py-1"), memberChips),
		Div(Class("set-label"), "Base currency"),
		Select(Class("set-input"),
			Option(Value("USD"), SelectedIf(base == "USD"), "USD — US Dollar"),
			Option(Value("EUR"), SelectedIf(base == "EUR"), "EUR — Euro"),
			Option(Value("GBP"), SelectedIf(base == "GBP"), "GBP — British Pound"),
		),
		Div(Class("set-label"), "Exchange rates"),
		If(len(fxRows) == 0, P(Class("text-faint text-[12px]"), "No custom rates.")),
		Div(fxRows),
	)

	right := Div(
		Div(Class("set-label"), "AI (OpenAI · bring your own key)"),
		ui.ToggleRow(ui.ToggleRowProps{Label: "Enable AI features", On: aiOn.Get(), OnChange: func(v bool) { aiOn.Set(v) }}),
		Input(Class("set-input mt-[0.45rem]"), Type("password"), Placeholder("OpenAI API key (sk-…)")),
		Select(Class("set-input mt-[0.45rem]"),
			Option("Model — latest"),
			Option("Model — mini"),
		),
		Div(Class("set-label"), "Appearance"),
		ui.Segmented(ui.SegmentedProps{
			Options:  []ui.SegOption{{Value: "dark", Label: "Dark"}, {Value: "light", Label: "Light"}, {Value: "system", Label: "System"}},
			Selected: theme.Get(),
			OnSelect: func(v string) { theme.Set(v) },
		}),
		Div(Class("toggle-row"),
			Span("Accent"),
			ui.SwatchPicker(ui.SwatchPickerProps{
				Colors:   []string{"#54b884", "#cfa14e", "#7c83ff", "#d8716f"},
				Selected: accent.Get(),
				OnSelect: func(c string) { accent.Set(c) },
			}),
		),
		ui.ToggleRow(ui.ToggleRowProps{Label: "Compact density", On: compact.Get(), OnChange: func(v bool) { compact.Set(v) }}),
		Div(Class("set-label"), "Data"),
		Div(Class("flex flex-wrap gap-2 py-1"),
			dataBtn("Export JSON", false, exportJSON),
			dataBtn("Export CSV", false, nil),
			dataBtn("Import…", false, nil),
			dataBtn("Load sample", false, nil),
			dataBtn("Wipe data", true, nil),
		),
	)

	return Div(Class("grid grid-cols-2 gap-x-7 content-start"), left, right)
}

// memberChip renders a household member as a colored chip.
func memberChip(m domain.Member) uic.Node {
	color := m.Color
	if color == "" {
		color = "#7c83ff"
	}
	return Span(Class("member-chip"),
		Span(Style(map[string]string{"width": "9px", "height": "9px", "border-radius": "50%", "background": color})),
		m.Name,
	)
}

// rateRow renders one editable FX rate row (1 <code> = <rate> <base>).
func rateRow(code string, rate float64, base string) uic.Node {
	return Div(Class("rate-row"),
		Span(Style(map[string]string{"width": "40px"}), code),
		Span(Class("text-faint"), "1 "+code+" ="),
		Input(Class("rate-in"), Value(strconv.FormatFloat(rate, 'f', -1, 64))),
		Span(Class("text-faint"), base),
	)
}

// exportJSON downloads the full dataset as a JSON file (the portable
// export/import + sync payload), via the pure appstate export.
func exportJSON() {
	app := appstate.Default
	if app == nil {
		return
	}
	data, err := app.ExportJSON()
	if err != nil {
		return
	}
	downloadBytes("cashflux.json", "application/json", data)
}

// dataBtnProps configures a data-action button.
type dataBtnProps struct {
	Label   string
	Danger  bool
	OnClick func()
}

// dataBtn renders a data-action button (danger variant for destructive actions).
// It is its own component so each click hook stays stable across the row.
func dataBtn(label string, danger bool, onClick func()) uic.Node {
	return uic.CreateElement(dataButton, dataBtnProps{Label: label, Danger: danger, OnClick: onClick})
}

func dataButton(props dataBtnProps) uic.Node {
	args := []any{Class("data-btn"), Type("button")}
	if props.Danger {
		args = append(args, Style(map[string]string{"color": "#d8716f", "border-color": "#5a2a2a"}))
	}
	onClick := props.OnClick
	args = append(args, OnClick(func() {
		if onClick != nil {
			onClick()
		}
	}), props.Label)
	return Button(args...)
}
