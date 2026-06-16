//go:build js && wasm

package app

import (
	"sort"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
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

// freshnessTypes lists the account types whose staleness window is editable, with
// friendly labels. Keyed by the domain account-type string used in settings.
var freshnessTypes = []struct {
	Label string
	Type  domain.AccountType
}{
	{"Credit cards", domain.TypeCreditCard},
	{"Checking", domain.TypeChecking},
	{"Savings", domain.TypeSavings},
	{"Investments", domain.TypeInvestment},
	{"Loans", domain.TypeLoan},
	{"Cash", domain.TypeCash},
}

type freshnessRowProps struct {
	Label   string
	TypeKey string
	Days    int
	OnSet   func(typeKey string, days int)
}

// freshnessRow is one editable staleness-window row. Its own component so the
// number input's change hook stays at a stable position across the list.
func freshnessRow(props freshnessRowProps) uic.Node {
	on := uic.UseEvent(func(v string) {
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		props.OnSet(props.TypeKey, n)
	})
	return Div(Class("rate-row"),
		Span(Style(map[string]string{"width": "110px"}), props.Label),
		Input(Class("rate-in"), Type("number"), Value(strconv.Itoa(props.Days)), OnInput(on)),
		Span(Class("text-faint"), "days (0 = never)"),
	)
}

// hideableScreens lists the screens a user can show or hide from the sidebar.
// The dashboard and settings are intentionally omitted — they are locked visible
// in internal/modules.
var hideableScreens = []struct{ Label, Path string }{
	{"Accounts", "/accounts"},
	{"Transactions", "/transactions"},
	{"Budgets", "/budgets"},
	{"Goals", "/goals"},
	{"To-do", "/todo"},
	{"Members", "/members"},
	{"Categories", "/categories"},
}

// globalSettingsForm is the two-column household/global settings back face:
// members, base currency and FX rows (left) and AI, appearance, and data
// actions (right). Members, base currency, and rates are read live from app
// state; appearance controls hold local state for now (persisting preferences
// and wiring data actions land in their own features).
func globalSettingsForm() uic.Node {
	aiOn := uic.UseState(false)
	prefsAtom := uistate.UsePrefs()
	savePrefs := func(p prefs.Prefs) {
		p = p.Normalize()
		prefsAtom.Set(p)
		uistate.PersistPrefs(p)
		uistate.ApplyPrefs(p)
	}
	onDateStyle := uic.UseEvent(func(e uic.Event) {
		p := prefsAtom.Get()
		p.DateStyle = prefs.DateStyle(e.GetValue())
		savePrefs(p)
	})
	hiddenAtom := uistate.UseHiddenModules()
	toggleModule := func(path string) {
		nh := hiddenAtom.Get().Toggle(path)
		hiddenAtom.Set(nh)
		uistate.PersistHiddenModules(nh)
	}
	dataRev := uistate.UseDataRevision()
	bump := func() { dataRev.Update(func(n int) int { return n + 1 }) }
	nav := router.UseNavigate()
	settingsAtom := uistate.UseSettings()
	goManageMembers := func() { settingsAtom.Set(uistate.SettingsTarget{}); nav.Navigate("/members") }

	curKey, curModel := "", ""
	if a := appstate.Default; a != nil {
		s := a.Settings()
		curKey, curModel = s.OpenAIKey, s.OpenAIModel
	}
	aiKey := uic.UseState(curKey)
	onKey := uic.UseEvent(func(v string) {
		aiKey.Set(v)
		if a := appstate.Default; a != nil {
			s := a.Settings()
			s.OpenAIKey = v
			_ = a.PutSettings(s)
		}
	})
	onModel := uic.UseEvent(func(e uic.Event) {
		if a := appstate.Default; a != nil {
			s := a.Settings()
			s.OpenAIModel = e.GetValue()
			_ = a.PutSettings(s)
		}
	})

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
	memberChips = append(memberChips, Button(Class("member-add"), Type("button"), OnClick(goManageMembers), "+ Add member"))

	pr := prefsAtom.Get().Normalize()

	// Freshness window editor: per-type day inputs writing Settings.FreshnessOverrides.
	setFreshness := func(typeKey string, days int) {
		a := appstate.Default
		if a == nil {
			return
		}
		s := a.Settings()
		if s.FreshnessOverrides == nil {
			s.FreshnessOverrides = map[string]int{}
		}
		s.FreshnessOverrides[typeKey] = days
		_ = a.PutSettings(s)
		bump()
	}
	var freshnessRows []uic.Node
	if a := appstate.Default; a != nil {
		fw := a.FreshnessWindows()
		for _, ft := range freshnessTypes {
			freshnessRows = append(freshnessRows, uic.CreateElement(freshnessRow, freshnessRowProps{
				Label: ft.Label, TypeKey: string(ft.Type), Days: fw[ft.Type], OnSet: setFreshness,
			}))
		}
	}

	hidden := hiddenAtom.Get()
	screenToggles := make([]uic.Node, 0, len(hideableScreens))
	for _, sc := range hideableScreens {
		path := sc.Path
		screenToggles = append(screenToggles, ui.ToggleRow(ui.ToggleRowProps{
			Label:    "Show " + sc.Label,
			On:       !hidden.IsHidden(path),
			OnChange: func(bool) { toggleModule(path) },
		}))
	}

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
		Div(Class("set-label"), "Screens"),
		P(Class("text-faint text-[12px]"), "Hide screens you don't use. Dashboard and Settings always stay."),
		Div(screenToggles),
		Div(Class("set-label"), "Freshness reminders"),
		P(Class("text-faint text-[12px]"), "How many days before a balance looks stale, by account type."),
		Div(freshnessRows),
	)

	right := Div(
		Div(Class("set-label"), "AI (OpenAI · bring your own key)"),
		ui.ToggleRow(ui.ToggleRowProps{Label: "Enable AI features", On: aiOn.Get(), OnChange: func(v bool) { aiOn.Set(v) }}),
		Input(Class("set-input mt-[0.45rem]"), Type("password"), Placeholder("OpenAI API key (sk-…)"), Value(aiKey.Get()), OnInput(onKey)),
		Select(Class("set-input mt-[0.45rem]"), OnChange(onModel),
			Option(Value("gpt-4o-mini"), SelectedIf(curModel == "gpt-4o-mini" || curModel == ""), "GPT-4o mini"),
			Option(Value("gpt-4o"), SelectedIf(curModel == "gpt-4o"), "GPT-4o"),
		),
		Div(Class("set-label"), "Appearance"),
		ui.Segmented(ui.SegmentedProps{
			Options:  []ui.SegOption{{Value: string(prefs.ThemeDark), Label: "Dark"}, {Value: string(prefs.ThemeLight), Label: "Light"}, {Value: string(prefs.ThemeSystem), Label: "System"}},
			Selected: string(pr.Theme),
			OnSelect: func(v string) {
				p := prefsAtom.Get()
				p.Theme = prefs.Theme(v)
				savePrefs(p)
			},
		}),
		Div(Class("toggle-row"),
			Span("Accent"),
			ui.SwatchPicker(ui.SwatchPickerProps{
				Colors:   []string{"#54b884", "#cfa14e", "#7c83ff", "#d8716f"},
				Selected: pr.Accent,
				OnSelect: func(c string) {
					p := prefsAtom.Get()
					p.Accent = c
					savePrefs(p)
				},
			}),
		),
		ui.ToggleRow(ui.ToggleRowProps{Label: "Compact density", On: pr.Compact, OnChange: func(v bool) {
			p := prefsAtom.Get()
			p.Compact = v
			savePrefs(p)
		}}),
		Div(Class("set-label"), "Preferences"),
		Div(Class("toggle-row"),
			Span("Week starts on"),
			ui.Segmented(ui.SegmentedProps{
				Options:  []ui.SegOption{{Value: string(prefs.WeekSunday), Label: "Sunday"}, {Value: string(prefs.WeekMonday), Label: "Monday"}},
				Selected: string(pr.WeekStart),
				OnSelect: func(v string) {
					p := prefsAtom.Get()
					p.WeekStart = prefs.WeekStart(v)
					savePrefs(p)
				},
			}),
		),
		Select(Class("set-input mt-[0.45rem]"), Title("Date format"), OnChange(onDateStyle),
			Option(Value(string(prefs.DateISO)), SelectedIf(pr.DateStyle == prefs.DateISO), "2026-06-05  (ISO)"),
			Option(Value(string(prefs.DateUS)), SelectedIf(pr.DateStyle == prefs.DateUS), "06/05/2026  (US)"),
			Option(Value(string(prefs.DateEU)), SelectedIf(pr.DateStyle == prefs.DateEU), "05/06/2026  (European)"),
			Option(Value(string(prefs.DateLong)), SelectedIf(pr.DateStyle == prefs.DateLong), "Jun 5, 2026  (Long)"),
		),
		Div(Class("set-label"), "Data"),
		Div(Class("flex flex-wrap gap-2 py-1"),
			dataBtn("Export JSON", false, exportJSON),
			dataBtn("Export CSV", false, exportCSV),
			dataBtn("Import…", false, func() { importJSON(bump) }),
			dataBtn("Load sample", false, func() { loadSample(bump) }),
			dataBtn("Wipe data", true, func() { wipeData(bump) }),
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

// exportCSV downloads all transactions as a CSV file.
func exportCSV() {
	app := appstate.Default
	if app == nil {
		return
	}
	data, err := app.ExportCSV()
	if err != nil {
		return
	}
	downloadBytes("transactions.csv", "text/csv", data)
}

// importJSON picks a JSON dataset file and replaces all data with it, then
// bumps the data revision so screens refresh.
func importJSON(onChange func()) {
	pickFile(".json", func(data []byte) {
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.ImportJSON(data); err != nil {
			return
		}
		onChange()
	})
}

// loadSample replaces all data with the built-in sample dataset and refreshes.
func loadSample(onChange func()) {
	app := appstate.Default
	if app == nil {
		return
	}
	if err := app.LoadSample(); err != nil {
		return
	}
	onChange()
}

// wipeData clears all data after a confirmation, then refreshes.
func wipeData(onChange func()) {
	if !confirmAction("Erase all CashFlux data on this device? This cannot be undone.") {
		return
	}
	app := appstate.Default
	if app == nil {
		return
	}
	if err := app.Wipe(); err != nil {
		return
	}
	onChange()
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
