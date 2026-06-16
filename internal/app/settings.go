//go:build js && wasm

package app

import (
	"sort"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/i18n"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
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
			Title:   uistate.T("settings.panelTitle"),
			Width:   "760px",
			Height:  "560px",
			Back:    uic.CreateElement(globalSettingsForm),
			OnClose: closePanel,
		})
	default: // "widget"
		return ui.FlipPanel(ui.FlipPanelProps{
			Title:   target.Title,
			Back:    uic.CreateElement(widgetSettingsForm, widgetSettingsFormProps{ID: target.ID, Title: target.Title}),
			OnClose: closePanel,
		})
	}
}

type widgetSettingsFormProps struct {
	ID    string
	Title string
}

// widgetSettingsForm is the per-widget settings back face. It renders the
// widget's registered widgetcfg.Schema generically (toggle/number/select),
// bound to the persisted WidgetConfigs atom so changes survive reloads. Widgets
// with no schema yet show a friendly placeholder.
func widgetSettingsForm(props widgetSettingsFormProps) uic.Node {
	cfgAtom := uistate.UseWidgetConfigs()
	schema, ok := widgetcfg.SchemaFor(props.ID)
	if !ok {
		return Div(
			Div(Class("set-label"), props.Title),
			P(Class("muted"), uistate.T("settings.noWidgetSettings")),
		)
	}
	all := cfgAtom.Get()
	cfg := all.For(props.ID)
	set := func(key, val string) {
		next := all.WithField(props.ID, key, val)
		cfgAtom.Set(next)
		uistate.PersistWidgetConfigs(next)
	}
	rows := make([]any, 0, len(schema.Fields)+1)
	rows = append(rows, Div(Class("set-label"), schema.Title))
	for _, f := range schema.Fields {
		rows = append(rows, uic.CreateElement(widgetFieldRow, widgetFieldRowProps{Field: f, Cfg: cfg, OnSet: set}))
	}
	return Div(rows...)
}

type widgetFieldRowProps struct {
	Field widgetcfg.Field
	Cfg   widgetcfg.Config
	OnSet func(key, val string)
}

// widgetFieldRow renders one schema field as the right control. Its own
// component so each field's input hook stays at a stable position (the
// On*-hooks-in-loops rule).
func widgetFieldRow(props widgetFieldRowProps) uic.Node {
	f := props.Field
	switch f.Type {
	case widgetcfg.Toggle:
		return ui.ToggleRow(ui.ToggleRowProps{
			Label: f.Label, On: f.Bool(props.Cfg),
			OnChange: func(v bool) { props.OnSet(f.Key, strconv.FormatBool(v)) },
		})
	case widgetcfg.Number:
		on := uic.UseEvent(func(v string) { props.OnSet(f.Key, strings.TrimSpace(v)) })
		label := f.Label
		if f.Unit != "" {
			label += " (" + f.Unit + ")"
		}
		return Div(Class("toggle-row"),
			Span(label),
			Input(Class("rate-in"), Type("number"), Value(strconv.Itoa(f.Int(props.Cfg))), OnInput(on)),
		)
	case widgetcfg.Select:
		on := uic.UseEvent(func(e uic.Event) { props.OnSet(f.Key, e.GetValue()) })
		cur := f.Str(props.Cfg)
		opts := make([]any, 0, len(f.Options)+2)
		opts = append(opts, Class("set-input"), OnChange(on))
		for _, o := range f.Options {
			opts = append(opts, Option(Value(o.Value), SelectedIf(cur == o.Value), o.Label))
		}
		return Div(Class("toggle-row"), Span(f.Label), Select(opts...))
	default:
		return Fragment()
	}
}

// freshnessTypes lists the account types whose staleness window is editable, with
// friendly labels. Keyed by the domain account-type string used in settings.
var freshnessTypes = []struct {
	Key  string // i18n key resolved at render
	Type domain.AccountType
}{
	{"settings.freshCredit", domain.TypeCreditCard},
	{"settings.freshChecking", domain.TypeChecking},
	{"settings.freshSavings", domain.TypeSavings},
	{"settings.freshInvestments", domain.TypeInvestment},
	{"settings.freshLoans", domain.TypeLoan},
	{"settings.freshCash", domain.TypeCash},
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
	periodAtom := uistate.UsePeriod()
	noticeAtom := uistate.UseNotice()
	notify := func(text string, isErr bool) { noticeAtom.Set(noticeAtom.Get().With(text, isErr)) }
	savePrefs := func(p prefs.Prefs) {
		p = p.Normalize()
		prefsAtom.Set(p)
		uistate.PersistPrefs(p)
		uistate.ApplyPrefs(p)
		// Keep the dashboard window's week boundaries in lockstep with the
		// week-start preference (no-op for any non-week-start change).
		if w := periodAtom.Get(); w.WeekStart != p.WeekStartWeekday() {
			periodAtom.Set(w.WithWeekStart(p.WeekStartWeekday()))
		}
	}
	onDateStyle := uic.UseEvent(func(e uic.Event) {
		p := prefsAtom.Get()
		p.DateStyle = prefs.DateStyle(e.GetValue())
		savePrefs(p)
	})
	onLang := uic.UseEvent(func(e uic.Event) { uistate.SetActiveLanguage(i18n.Lang(e.GetValue())) })
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
	memberChips = append(memberChips, Button(Class("member-add"), Type("button"), OnClick(goManageMembers), uistate.T("settings.addMember")))

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
				Label: uistate.T(ft.Key), TypeKey: string(ft.Type), Days: fw[ft.Type], OnSet: setFreshness,
			}))
		}
	}

	hidden := hiddenAtom.Get()
	screenToggles := make([]uic.Node, 0, len(hideableScreens))
	for _, sc := range hideableScreens {
		path := sc.Path
		screenToggles = append(screenToggles, ui.ToggleRow(ui.ToggleRowProps{
			Label:    uistate.T("settings.showScreen", sc.Label),
			On:       !hidden.IsHidden(path),
			OnChange: func(bool) { toggleModule(path) },
		}))
	}

	left := Div(
		Div(Class("set-label"), uistate.T("settings.householdMembers")),
		Div(Class("flex flex-wrap gap-2 py-1"), memberChips),
		Div(Class("set-label"), uistate.T("settings.baseCurrency")),
		Select(Class("set-input"),
			Option(Value("USD"), SelectedIf(base == "USD"), "USD — US Dollar"),
			Option(Value("EUR"), SelectedIf(base == "EUR"), "EUR — Euro"),
			Option(Value("GBP"), SelectedIf(base == "GBP"), "GBP — British Pound"),
		),
		Div(Class("set-label"), uistate.T("settings.exchangeRates")),
		If(len(fxRows) == 0, P(Class("text-faint text-[12px]"), uistate.T("settings.noRates"))),
		Div(fxRows),
		Div(Class("set-label"), uistate.T("settings.screens")),
		P(Class("text-faint text-[12px]"), uistate.T("settings.screensHint")),
		Div(screenToggles),
		Div(Class("set-label"), uistate.T("settings.freshnessTitle")),
		P(Class("text-faint text-[12px]"), uistate.T("settings.freshnessHint")),
		Div(freshnessRows),
	)

	activeLang := uistate.ActiveLanguage()
	langOptions := make([]uic.Node, 0)
	for _, l := range uistate.Languages() {
		langOptions = append(langOptions, Option(Value(string(l)), SelectedIf(activeLang == l), langDisplay(l)))
	}

	right := Div(
		Div(Class("set-label"), uistate.T("settings.aiTitle")),
		ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("settings.aiEnable"), On: aiOn.Get(), OnChange: func(v bool) { aiOn.Set(v) }}),
		Input(Class("set-input mt-[0.45rem]"), Type("password"), Placeholder(uistate.T("settings.aiKeyPlaceholder")), Value(aiKey.Get()), OnInput(onKey)),
		If(strings.TrimSpace(aiKey.Get()) == "", P(Class("text-faint text-[12px] mt-1"), uistate.T("settings.aiNoKey"))),
		Select(Class("set-input mt-[0.45rem]"), Title(uistate.T("settings.aiModel")), OnChange(onModel),
			Option(Value("gpt-4o-mini"), SelectedIf(curModel == "gpt-4o-mini" || curModel == ""), "GPT-4o mini"),
			Option(Value("gpt-4.1-nano"), SelectedIf(curModel == "gpt-4.1-nano"), "GPT-4.1 nano"),
			Option(Value("gpt-4.1-mini"), SelectedIf(curModel == "gpt-4.1-mini"), "GPT-4.1 mini"),
			Option(Value("gpt-4o"), SelectedIf(curModel == "gpt-4o"), "GPT-4o"),
			Option(Value("gpt-4.1"), SelectedIf(curModel == "gpt-4.1"), "GPT-4.1"),
			Option(Value("o4-mini"), SelectedIf(curModel == "o4-mini"), "o4-mini (reasoning)"),
		),
		Div(Class("set-label"), uistate.T("settings.appearance")),
		ui.Segmented(ui.SegmentedProps{
			Options:  []ui.SegOption{{Value: string(prefs.ThemeDark), Label: uistate.T("settings.themeDark")}, {Value: string(prefs.ThemeLight), Label: uistate.T("settings.themeLight")}, {Value: string(prefs.ThemeSystem), Label: uistate.T("settings.themeSystem")}},
			Selected: string(pr.Theme),
			OnSelect: func(v string) {
				p := prefsAtom.Get()
				p.Theme = prefs.Theme(v)
				savePrefs(p)
			},
		}),
		Div(Class("toggle-row"),
			Span(uistate.T("settings.accent")),
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
		ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("settings.compact"), On: pr.Compact, OnChange: func(v bool) {
			p := prefsAtom.Get()
			p.Compact = v
			savePrefs(p)
		}}),
		Div(Class("set-label"), uistate.T("settings.preferences")),
		Div(Class("toggle-row"),
			Span(uistate.T("settings.weekStart")),
			ui.Segmented(ui.SegmentedProps{
				Options:  []ui.SegOption{{Value: string(prefs.WeekSunday), Label: uistate.T("settings.sunday")}, {Value: string(prefs.WeekMonday), Label: uistate.T("settings.monday")}},
				Selected: string(pr.WeekStart),
				OnSelect: func(v string) {
					p := prefsAtom.Get()
					p.WeekStart = prefs.WeekStart(v)
					savePrefs(p)
				},
			}),
		),
		Select(Class("set-input mt-[0.45rem]"), Title(uistate.T("settings.dateFormat")), OnChange(onDateStyle),
			Option(Value(string(prefs.DateISO)), SelectedIf(pr.DateStyle == prefs.DateISO), "2026-06-05  (ISO)"),
			Option(Value(string(prefs.DateUS)), SelectedIf(pr.DateStyle == prefs.DateUS), "06/05/2026  (US)"),
			Option(Value(string(prefs.DateEU)), SelectedIf(pr.DateStyle == prefs.DateEU), "05/06/2026  (European)"),
			Option(Value(string(prefs.DateLong)), SelectedIf(pr.DateStyle == prefs.DateLong), "Jun 5, 2026  (Long)"),
		),
		Div(Class("set-label"), uistate.T("settings.data")),
		Div(Class("flex flex-wrap gap-2 py-1"),
			dataBtn(uistate.T("settings.exportJSON"), false, func() { exportJSON(notify) }),
			dataBtn(uistate.T("settings.exportCSV"), false, func() { exportCSV(notify) }),
			dataBtn(uistate.T("settings.import"), false, func() { importJSON(bump, notify) }),
			dataBtn(uistate.T("settings.loadSample"), false, func() { loadSample(bump, notify) }),
			dataBtn(uistate.T("settings.wipe"), true, func() { wipeData(bump, notify) }),
		),
		Div(Class("set-label"), uistate.T("settings.languages")),
		Select(Class("set-input"), Title(uistate.T("settings.language")), OnChange(onLang), langOptions),
		Div(Class("flex flex-wrap gap-2 py-1"),
			dataBtn(uistate.T("settings.exportLangs"), false, func() { exportLanguages(notify) }),
			dataBtn(uistate.T("settings.importLangs"), false, func() { importLanguages(notify) }),
		),
	)

	return Div(Class("grid grid-cols-2 gap-x-7 content-start"), left, right)
}

// langDisplay gives a human label for a language code: English by name, any
// other code uppercased (e.g. "es" → "ES") until it ships a localized name.
func langDisplay(l i18n.Lang) string {
	if l == i18n.English {
		return "English"
	}
	return strings.ToUpper(string(l))
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
func exportJSON(notify func(string, bool)) {
	app := appstate.Default
	if app == nil {
		return
	}
	data, err := app.ExportJSON()
	if err != nil {
		notify("Couldn't export your data: "+err.Error(), true)
		return
	}
	downloadBytes("cashflux.json", "application/json", data)
	notify("Exported your data as cashflux.json.", false)
}

// exportCSV downloads all transactions as a CSV file.
func exportCSV(notify func(string, bool)) {
	app := appstate.Default
	if app == nil {
		return
	}
	data, err := app.ExportCSV()
	if err != nil {
		notify("Couldn't export your transactions: "+err.Error(), true)
		return
	}
	downloadBytes("transactions.csv", "text/csv", data)
	notify("Exported your transactions as transactions.csv.", false)
}

// exportLanguages downloads the whole language bundle (every supported language)
// as JSON — the file translators edit and re-import.
func exportLanguages(notify func(string, bool)) {
	data, err := uistate.ExportLanguages()
	if err != nil {
		notify("Couldn't export languages: "+err.Error(), true)
		return
	}
	downloadBytes("cashflux-languages.json", "application/json", data)
	notify("Exported the language bundle.", false)
}

// importLanguages picks a language-bundle JSON file and merges it into the app,
// persisting it for next launch.
func importLanguages(notify func(string, bool)) {
	pickFile(".json", func(data []byte) {
		if err := uistate.ImportLanguages(data); err != nil {
			notify("Couldn't import languages: "+err.Error(), true)
			return
		}
		notify("Imported languages — reload to apply.", false)
	})
}

// importJSON picks a JSON dataset file and replaces all data with it, then
// bumps the data revision so screens refresh.
func importJSON(onChange func(), notify func(string, bool)) {
	pickFile(".json", func(data []byte) {
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.ImportJSON(data); err != nil {
			notify("Couldn't import that file: "+err.Error(), true)
			return
		}
		onChange()
		notify("Imported your data.", false)
	})
}

// loadSample replaces all data with the built-in sample dataset and refreshes.
func loadSample(onChange func(), notify func(string, bool)) {
	app := appstate.Default
	if app == nil {
		return
	}
	if err := app.LoadSample(); err != nil {
		notify("Couldn't load the sample data: "+err.Error(), true)
		return
	}
	onChange()
	notify("Loaded the sample data.", false)
}

// wipeData clears all data after a confirmation, then refreshes.
func wipeData(onChange func(), notify func(string, bool)) {
	if !confirmAction("Erase all CashFlux data on this device? This cannot be undone.") {
		return
	}
	app := appstate.Default
	if app == nil {
		return
	}
	if err := app.Wipe(); err != nil {
		notify("Couldn't erase your data: "+err.Error(), true)
		return
	}
	onChange()
	notify("Erased all data on this device.", false)
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
