// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/backendauth"
	"github.com/monstercameron/CashFlux/internal/backup"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/i18n"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/notify"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/version"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// backendHost extracts the lowercased host[:port] from a server URL, ignoring
// scheme, path, query, and fragment. Two URLs that differ only in path/query
// share a host, so editing the path of the same server doesn't drop the session;
// pointing at a different host does (§3.4 switch-server flow).
func backendHost(raw string) string {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return strings.ToLower(s)
}

// settingsPage hosts the tabbed settings form as the routed /settings page —
// the same component the flip-modal host mounts, laid out for the content
// column.
func settingsPage() uic.Node {
	return Div(css.Class("settings-page"), uic.CreateElement(globalSettingsForm))
}

// The screens registry can't import this package (app imports screens), so the
// /settings route's view is injected at boot.
func init() {
	screens.SettingsView = func() uic.Node { return uic.CreateElement(settingsPage) }
}

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
			Title: uistate.T("settings.panelTitle"),
			// FlipLarge width (the standard for global settings); a taller responsive
			// height for this, the densest panel in the app.
			Width:  ui.FlipLargeW,
			Height: "min(90vh, 900px)",
			Back:   uic.CreateElement(globalSettingsForm),
			// Every setting in this panel applies live on change (currency, FX, screens,
			// freshness, prefs, etc.), so a Save/Cancel footer is misleading — use a
			// single Close button instead (§6.17).
			CloseOnly: true,
			OnClose:   closePanel,
		})
	default: // "widget"
		return ui.FlipPanel(ui.FlipPanelProps{
			Title: target.Title,
			Back:  uic.CreateElement(widgetSettingsForm, widgetSettingsFormProps{ID: target.ID, Title: target.Title}),
			// A widget with no settings schema has nothing to save — show a single
			// Close button instead of a misleading Cancel/Save (C11).
			CloseOnly: !widgetcfg.Has(target.ID),
			OnClose:   closePanel,
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
// notifySettings is the Settings-modal section for notifications (C75, C263):
//   - A browser-notification opt-in toggle.
//   - A "Manage alerts" group with one on/off row per alert type (C263), so the
//     user can silence specific event types without affecting others.
func notifySettings() uic.Node {
	on := uic.UseState(uistate.BrowserNotifyEnabled())
	// Load the persisted rule config once at render time; each alertRow
	// re-reads it on toggle so concurrent changes compose correctly.
	cfg := notify.UnmarshalRuleConfig(uistate.SettingKVGet(notify.RuleConfigKey()))
	rules := notify.DefaultRules()
	rows := make([]any, 0, len(rules))
	for _, r := range rules {
		props := alertRowProps{
			RuleID:  r.ID,
			Label:   uistate.T(alertLabelKey(r.ID)),
			Enabled: cfg.IsEnabled(r.ID),
		}
		// Attach threshold controls for rules that expose a user-tunable threshold.
		switch r.ID {
		case "default-large", "default-low-balance", "default-paycheck":
			// Money rule: display in dollars (major units); store in cents (minor units).
			defaultDollars := int(r.Threshold) / 100
			overrideMinor := cfg.Thresholds[r.ID]
			displayVal := defaultDollars
			if overrideMinor > 0 {
				displayVal = int(overrideMinor / 100)
			}
			props.ThresholdLabel = "$"
			props.ThresholdValue = displayVal
			props.ThresholdIsMoney = true
		case "default-bill-due":
			// Days rule: display and store directly as integer days.
			defaultDays := int(r.Threshold)
			overrideDays := cfg.Thresholds[r.ID]
			displayVal := defaultDays
			if overrideDays > 0 {
				displayVal = int(overrideDays)
			}
			props.ThresholdLabel = "days"
			props.ThresholdValue = displayVal
			props.ThresholdIsMoney = false
		}
		rows = append(rows, uic.CreateElement(alertRow, props))
	}
	alertChildren := make([]any, 0, len(rules)+1)
	alertChildren = append(alertChildren, Attr("data-testid", "settings-manage-alerts"))
	alertChildren = append(alertChildren, rows...)
	return Div(Attr("data-testid", "settings-notifications"),
		H4(css.Class("set-label"), uistate.T("settings.notifyTitle")),
		ui.ToggleRow(ui.ToggleRowProps{
			Label: uistate.T("settings.notifyBrowser"),
			On:    on.Get(),
			OnChange: func(v bool) {
				on.Set(v)
				uistate.SetBrowserNotifyEnabled(v)
				if v {
					if N := js.Global().Get("Notification"); N.Truthy() && N.Get("permission").String() == "default" {
						N.Call("requestPermission")
					}
				}
			},
		}),
		H4(css.Class("set-label"), uistate.T("settings.manageAlerts")),
		Div(alertChildren...),
	)
}

// alertLabelKey maps a notify rule ID to its i18n key. Unknown IDs fall back to
// the raw ID so a newly added rule is still labelled rather than blank.
func alertLabelKey(ruleID string) string {
	switch ruleID {
	case "default-bill-due":
		return "settings.alert.billDue"
	case "default-budget":
		return "settings.alert.budgetThreshold"
	case "default-stale":
		return "settings.alert.staleBalance"
	case "default-digest":
		return "settings.alert.digest"
	case "default-backup":
		return "settings.alert.backupDue"
	case "default-large":
		return "settings.alert.largeTransaction"
	case "default-low-balance":
		return "settings.alert.lowBalance"
	case "default-paycheck":
		return "settings.alert.paycheckLanded"
	default:
		return ruleID
	}
}

type alertRowProps struct {
	RuleID string
	Label  string
	// Enabled is the initial enabled state read from the persisted config.
	Enabled bool
	// ThresholdLabel is the unit label shown next to the threshold input
	// ("$" for money rules, "days" for bill-due). Empty means no threshold input.
	ThresholdLabel string
	// ThresholdValue is the currently persisted threshold in display units
	// (dollars for money rules, days for bill-due).
	ThresholdValue int
	// ThresholdIsMoney indicates the input is in dollars and should be stored
	// as minor units (cents = dollars × 100).
	ThresholdIsMoney bool
}

// alertRow is a single per-alert-type toggle row, optionally followed by a
// threshold input for rules that expose one. It is its own component so the
// OnChange and OnInput hooks are registered at stable positions — not inside a
// loop (the GoWebComponents On*-in-loop rule). Local state atoms drive the
// toggle and threshold input so changes reflect immediately.
func alertRow(props alertRowProps) uic.Node {
	enabled := uic.UseState(props.Enabled)
	thresh := uic.UseState(props.ThresholdValue)

	onToggle := func(v bool) {
		enabled.Set(v)
		// Re-read, mutate, and write back so concurrent toggles compose
		// (each row owns exactly one key).
		cfg := notify.UnmarshalRuleConfig(uistate.SettingKVGet(notify.RuleConfigKey()))
		if cfg.Enabled == nil {
			cfg.Enabled = map[string]bool{}
		}
		cfg.Enabled[props.RuleID] = v
		uistate.SettingKVSet(notify.RuleConfigKey(), notify.MarshalRuleConfig(cfg))
	}
	onThresh := uic.UseEvent(func(v string) {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 0 {
			return
		}
		thresh.Set(n)
		cfg := notify.UnmarshalRuleConfig(uistate.SettingKVGet(notify.RuleConfigKey()))
		if cfg.Thresholds == nil {
			cfg.Thresholds = map[string]int64{}
		}
		var stored int64
		if props.ThresholdIsMoney {
			stored = int64(n) * 100 // dollars → cents
		} else {
			stored = int64(n)
		}
		cfg.Thresholds[props.RuleID] = stored
		uistate.SettingKVSet(notify.RuleConfigKey(), notify.MarshalRuleConfig(cfg))
	})

	toggle := ui.ToggleRow(ui.ToggleRowProps{
		Label:    props.Label,
		On:       enabled.Get(),
		OnChange: onToggle,
	})
	if props.ThresholdLabel == "" {
		return toggle
	}
	ariaLabel := props.Label + " threshold (" + props.ThresholdLabel + ")"
	threshInput := Div(css.Class("toggle-row"),
		Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.alert.threshold", props.ThresholdLabel)),
		Input(css.Class("rate-in"), Type("number"), Attr("min", "0"), Attr("step", "1"),
			Attr("aria-label", ariaLabel),
			Value(strconv.Itoa(thresh.Get())),
			OnChange(onThresh)),
	)
	return Fragment(toggle, threshInput)
}

// learnThresholdRow renders a small number-input control (C35) that lets the
// user tune how many payee→category corrections must accumulate before a
// suggestion chip appears in Quick-Add. Writes directly to the preserved KV
// store via uistate.SaveLearnThreshold. Its own component so the input's
// UseEvent hook stays at a stable render position.
func learnThresholdRow() uic.Node {
	cur := uistate.LoadLearnThreshold()
	thresh := uic.UseState(cur)
	onThresh := uic.UseEvent(func(v string) {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || n < 1 {
			return
		}
		thresh.Set(n)
		uistate.SaveLearnThreshold(n)
	})
	ariaLabel := uistate.T("settings.learnThresholdLabel")
	return Div(
		H4(css.Class("set-label"), uistate.T("settings.learnThresholdLabel")),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.learnThresholdHint")),
		Div(css.Class("toggle-row"),
			Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.learnThresholdLabel")),
			Input(css.Class("rate-in"), Type("number"), Attr("min", "1"), Attr("step", "1"),
				Attr("aria-label", ariaLabel), Attr("data-testid", "settings-learn-threshold"),
				Value(strconv.Itoa(thresh.Get())),
				OnChange(onThresh)),
		),
	)
}

// musicSettings is the Settings-modal control group for the background music: an
// on/off toggle and a volume slider. It writes the shared muzak atoms (the same
// ones the top-bar speaker button uses), so changes apply live and persist. Its
// own component so the slider's input hook stays at a stable position.
func musicSettings() uic.Node {
	enabled := uistate.UseMuzakEnabled()
	vol := uistate.UseMuzakVolume()
	pct := int(vol.Get()*100 + 0.5)
	apply := func(e uic.Event) {
		f, err := strconv.ParseFloat(strings.TrimSpace(e.GetValue()), 64)
		if err != nil {
			return
		}
		v := f / 100
		vol.Set(v)
		uistate.PersistMuzakVolume(v)
	}
	// Live while dragging (no DB write); checkpoint the dataset on release only, so
	// dragging the slider doesn't re-serialize the whole dataset on every step.
	onVol := uic.UseEvent(apply)
	onVolCommit := uic.UseEvent(func(e uic.Event) { apply(e); checkpointMusic() })
	return Div(
		H4(css.Class("set-label"), uistate.T("settings.music")),
		ui.ToggleRow(ui.ToggleRowProps{
			Label:    uistate.T("settings.musicOn"),
			On:       enabled.Get(),
			OnChange: func(on bool) { enabled.Set(on); uistate.PersistMuzakEnabled(on); checkpointMusic() },
		}),
		Div(css.Class("toggle-row"),
			Span(uistate.T("settings.musicVolume")),
			Input(Type("range"), css.Class("set-range"), Attr("min", "0"), Attr("max", "100"), Attr("step", "1"),
				Attr("aria-label", uistate.T("settings.musicVolume")),
				Value(strconv.Itoa(pct)), OnInput(onVol), OnChange(onVolCommit)),
		),
	)
}

func widgetSettingsForm(props widgetSettingsFormProps) uic.Node {
	cfgAtom := uistate.UseWidgetConfigs()
	all := cfgAtom.Get()
	cfg := all.For(props.ID)
	set := func(key, val string) {
		next := all.WithField(props.ID, key, val)
		cfgAtom.Set(next)
		uistate.PersistWidgetConfigs(next)
	}
	// Every tile can be ranked for the auto-importance layout mode and tinted with
	// a per-widget accent color (B20), so both controls always render — a tile's
	// panel is never empty (C21/C24).
	importance := uic.CreateElement(importanceRow, importanceRowProps{ID: props.ID})
	colorRow := uic.CreateElement(widgetColorRow, widgetColorRowProps{
		Current: cfg.Accent(),
		OnSet:   func(hex string) { set(widgetcfg.AccentKey, hex) },
	})
	schema, ok := widgetcfg.SchemaFor(props.ID)
	if !ok {
		return Div(
			H4(css.Class("set-label"), props.Title),
			colorRow,
			importance,
		)
	}
	rows := make([]any, 0, len(schema.Fields)+3)
	rows = append(rows, H4(css.Class("set-label"), schema.Title))
	for _, f := range schema.Fields {
		rows = append(rows, uic.CreateElement(widgetFieldRow, widgetFieldRowProps{Field: f, Cfg: cfg, OnSet: set}))
	}
	rows = append(rows, colorRow, importance)
	return Div(rows...)
}

type widgetColorRowProps struct {
	Current string
	OnSet   func(string)
}

// widgetColorRow lets the user tint this tile with an accent color (shown as a
// colored top strip) or clear it back to the theme default. Own component so the
// color input's change hook stays at a stable position.
func widgetColorRow(props widgetColorRowProps) uic.Node {
	on := uic.UseEvent(func(e uic.Event) {
		if props.OnSet != nil {
			props.OnSet(e.GetValue())
		}
	})
	val := props.Current
	if val == "" {
		val = "#7c83ff"
	}
	return Div(css.Class("toggle-row"),
		Span(uistate.T("settings.tileColor")),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Input(Type("color"), Style(map[string]string{"width": "2rem", "height": "1.6rem", "padding": "0", "border": "none", "background": "none"}), Attr("aria-label", uistate.T("settings.tileColor")), Value(val), OnChange(on)),
			If(props.Current != "", Button(css.Class("btn"), Type("button"), OnClick(func() {
				if props.OnSet != nil {
					props.OnSet("")
				}
			}), uistate.T("action.clear"))),
		),
	)
}

type importanceRowProps struct {
	ID string
}

// importanceLevels are the friendly importance choices shown in the per-tile
// settings panel; the value is the priority the auto-importance layout mode
// sorts by (higher first), with Normal = 0 the default.
var importanceLevels = []struct {
	Label string // i18n key
	Value int
}{
	{"widget.importanceHighest", 2},
	{"widget.importanceHigh", 1},
	{"widget.importanceNormal", 0},
	{"widget.importanceLow", -1},
}

// importanceRow lets the user rank this tile for the auto-importance layout mode
// (C24). Its own component so the select's change hook stays at a stable
// position; it writes the shared layout-items atom and persists.
func importanceRow(props importanceRowProps) uic.Node {
	itemsAtom := uistate.UseLayoutItems()
	cur := dashlayout.ImportanceOf(itemsAtom.Get(), props.ID)
	on := uic.UseEvent(func(e uic.Event) {
		n, err := strconv.Atoi(e.GetValue())
		if err != nil {
			return
		}
		next := dashlayout.SetImportance(itemsAtom.Get(), props.ID, n)
		itemsAtom.Set(next)
		uistate.PersistItems(next)
	})
	opts := []any{css.Class("set-input"), Title(uistate.T("widget.importance")), Attr("aria-label", uistate.T("widget.importance")), OnChange(on)}
	for _, lvl := range importanceLevels {
		opts = append(opts, Option(Value(strconv.Itoa(lvl.Value)), SelectedIf(cur == lvl.Value), uistate.T(lvl.Label)))
	}
	return Div(css.Class("toggle-row"), Span(uistate.T("widget.importance")), Select(opts...))
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
		return Div(css.Class("toggle-row"),
			Span(label),
			// aria-label mirrors the visible Span so the number field has an accessible name (WCAG 4.1.2).
			Input(css.Class("rate-in"), Type("number"), Attr("aria-label", label), Value(strconv.Itoa(f.Int(props.Cfg))), OnInput(on)),
		)
	case widgetcfg.Select:
		on := uic.UseEvent(func(e uic.Event) { props.OnSet(f.Key, e.GetValue()) })
		cur := f.Str(props.Cfg)
		opts := make([]any, 0, len(f.Options)+2)
		// aria-label mirrors the visible Span(f.Label) so the select has an accessible name (WCAG 4.1.2).
		opts = append(opts, css.Class("set-input"), Attr("aria-label", f.Label), OnChange(on))
		for _, o := range f.Options {
			opts = append(opts, Option(Value(o.Value), SelectedIf(cur == o.Value), o.Label))
		}
		return Div(css.Class("toggle-row"), Span(f.Label), Select(opts...))
	case widgetcfg.Text:
		// Free text — e.g. a configurable formula expression. Programmable: the user
		// can rewrite a KPI's formula (over the engine variable surface) right here.
		on := uic.UseEvent(func(v string) { props.OnSet(f.Key, v) })
		return Div(css.Class("toggle-row", tw.FlexCol, tw.ItemsStart, tw.Gap1),
			Span(f.Label),
			Input(css.Class("set-input", tw.WFull), Type("text"), Attr("aria-label", f.Label),
				Attr("spellcheck", "false"), Value(f.Str(props.Cfg)), OnInput(on)),
		)
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
	{"settings.freshRetirement", domain.TypeRetirement},
	{"settings.freshCrypto", domain.TypeCrypto},
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
	return Div(css.Class("rate-row"),
		Span(Style(map[string]string{"width": "110px"}), props.Label),
		// aria-label mirrors the row's type label so the staleness-window field has an accessible name.
		Input(css.Class("rate-in"), Type("number"), Attr("aria-label", uistate.T("settings.freshnessAria", props.Label)), Value(strconv.Itoa(props.Days)), OnInput(on)),
		Span(css.Class(tw.TextFaint), uistate.T("settings.freshNever")),
	)
}

// hideableScreens lists the screens a user can show or hide from the sidebar.
// The dashboard is intentionally omitted — it is locked visible in
// internal/modules. Covers every routed main-line screen (primary nav, the Tools
// group, and the System group).
// LabelKey is an i18n key (reuses the nav.* catalog) so screen names localize
// with the active language instead of being hardcoded English (§6.12).
var hideableScreens = []struct{ LabelKey, Path string }{
	{"nav.accounts", "/accounts"},
	{"nav.transactions", "/transactions"},
	{"nav.budgets", "/budgets"},
	{"nav.goals", "/goals"},
	{"nav.todo", "/todo"},
	{"nav.planning", "/planning"},
	{"nav.allocate", "/allocate"},
	{"nav.insights", "/insights"},
	{"nav.documents", "/documents"},
	{"nav.customize", "/customize"},
	{"nav.fields", "/fields"},
	{"nav.members", "/members"},
	{"nav.categories", "/categories"},
	{"nav.rules", "/rules"},
}

// globalSettingsForm is the two-column household/global settings back face:
// members, base currency and FX rows (left) and AI, appearance, and data
// actions (right). Members, base currency, and rates are read live from app
// state; appearance controls hold local state for now (persisting preferences
// and wiring data actions land in their own features).
func globalSettingsForm() uic.Node {
	aiOn := uic.UseState(false)
	// The active settings tab. The panel was one dense two-column form with 14
	// stacked sections and a jump-nav; tabs give each cluster its own room.
	// A pending deep-link (OpenGlobalSettingsAt) picks the opening tab; the
	// consume only matters on mount, which is exactly when UseState reads it.
	initTab := uistate.ConsumeRequestedSettingsTab()
	if initTab == "" {
		initTab = "household"
	}
	setTab := uic.UseState(initTab)
	prefsAtom := uistate.UsePrefs()
	periodAtom := uistate.UsePeriod()
	noticeAtom := uistate.UseNotice()
	notify := func(text string, isErr bool) { noticeAtom.Set(noticeAtom.Get().With(text, isErr)) }
	logRev := uic.UseState(0)
	refreshLog := func() { logRev.Set(logRev.Get() + 1) }
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
	onBackupCadence := uic.UseEvent(func(e uic.Event) {
		saveBackupCadence(backup.ParseCadence(e.GetValue()))
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
	goManageMembers := func() { settingsAtom.Set(uistate.SettingsTarget{}); nav.Navigate(uistate.RoutePath("/members")) }

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
		// Persist the key across reloads by default: entering a key means you want to
		// use it, and the dataset autosave deliberately REDACTS the key, so the
		// on-device browser store is its only persistence path. Turning a key ON also
		// turns "Remember AI key" on (its opt-out still lets you keep it session-only —
		// toggling it off clears the stored copy). Clearing the key clears storage.
		if strings.TrimSpace(v) != "" {
			if p := prefsAtom.Get(); !p.RememberAIKey {
				p.RememberAIKey = true
				savePrefs(p)
			}
			uistate.PersistAIKey(v)
		} else {
			uistate.ClearAIKey()
		}
		// A newly configured key can now drive the lock-screen quote-of-the-day —
		// generate + cache it right away so the user doesn't have to wait for a reload.
		refreshDailyLockQuote()
	})
	onModel := uic.UseEvent(func(e uic.Event) {
		if a := appstate.Default; a != nil {
			s := a.Settings()
			s.OpenAIModel = e.GetValue()
			_ = a.PutSettings(s)
		}
	})
	// Optional web-search API key for the chat's web_search tool (paid/higher-limit
	// access); kept on-device in its own localStorage entry.
	wsKey := uic.UseState(uistate.LoadWebSearchKey())
	onWsKey := uic.UseEvent(func(v string) {
		wsKey.Set(v)
		uistate.PersistWebSearchKey(v)
	})
	curMethod := budgeting.MethodSimple
	if a := appstate.Default; a != nil {
		curMethod = budgeting.ParseMethodology(a.Settings().BudgetMethodology)
	}
	onMethod := uic.UseEvent(func(e uic.Event) {
		if a := appstate.Default; a != nil {
			s := a.Settings()
			s.BudgetMethodology = e.GetValue()
			_ = a.PutSettings(s)
			bump() // re-render the budgets view with the new methodology
		}
	})
	onBase := uic.UseEvent(func(e uic.Event) {
		if a := appstate.Default; a != nil {
			s := a.Settings()
			s.BaseCurrency = e.GetValue()
			_ = a.PutSettings(s)
			bump() // re-window every currency-aware figure to the new base
		}
	})
	// setRate writes (or clears, when 0/blank) one currency's rate against the base.
	setRate := func(code string, rate float64) {
		a := appstate.Default
		if a == nil {
			return
		}
		s := a.Settings()
		if s.FXRates == nil {
			s.FXRates = map[string]float64{}
		}
		if s.FXUpdatedAt == nil {
			s.FXUpdatedAt = map[string]time.Time{}
		}
		if rate > 0 {
			s.FXRates[code] = rate
			s.FXUpdatedAt[code] = time.Now() // stamp so staleness can be shown (L4)
		} else {
			delete(s.FXRates, code)
			delete(s.FXUpdatedAt, code)
		}
		_ = a.PutSettings(s)
		bump()
	}

	var members []domain.Member
	base := "USD"
	var fxRows []uic.Node
	var fxNonBaseCodes []string
	var fxCurrentRates map[string]float64
	if app := appstate.Default; app != nil {
		members = app.Members()
		s := app.Settings()
		if s.BaseCurrency != "" {
			base = s.BaseCurrency
		}
		fxCurrentRates = s.FXRates
		for _, code := range currency.Codes() {
			if code == base {
				continue
			}
			fxNonBaseCodes = append(fxNonBaseCodes, code)
			stale := false
			asOf := ""
			if s.FXUpdatedAt != nil {
				if t, ok := s.FXUpdatedAt[code]; ok && !t.IsZero() {
					stale = currency.RateStale(t, time.Now(), currency.DefaultRateMaxAge)
					asOf = uistate.LoadPrefs().FormatDate(t) // C80: respect the user's date format
				}
			}
			fxRows = append(fxRows, uic.CreateElement(fxRateRow, fxRateRowProps{
				Code: code, Base: base, Rate: s.FXRates[code], OnSet: setRate, Stale: stale, AsOf: asOf,
			}))
		}
	}

	memberChips := make([]uic.Node, 0, len(members)+1)
	for _, m := range members {
		memberChips = append(memberChips, memberChip(m))
	}
	memberChips = append(memberChips, Button(css.Class("member-add"), Type("button"), OnClick(goManageMembers), uistate.T("settings.addMember")))

	pr := prefsAtom.Get().Normalize()
	serverMode := uic.UseState(string(pr.ServerMode))
	serverURL := uic.UseState(pr.ServerURL)
	serverToken := uic.UseState(pr.ServerToken)
	// backendOn drives the clear "connect to a backend" switch. It's the inverse of
	// the persisted BackendDisabled flag, so off cleanly stops every sync/AI-proxy
	// connection even with a URL/token saved.
	backendOn := uic.UseState(!pr.BackendDisabled)
	onBackendToggle := func(v bool) {
		backendOn.Set(v)
		p := prefsAtom.Get()
		p.BackendDisabled = !v
		savePrefs(p)
		if v {
			requestBackendSyncNow()
		}
	}
	initialAuth := backendauth.Discovery{AuthMode: backendauth.ModeToken}
	if pr.ServerMode == prefs.ServerCloud {
		initialAuth = backendauth.Discovery{AuthMode: backendauth.ModeOAuth, AuthProviders: []string{"google", "github"}}
	}
	serverAuthMode := uic.UseState(initialAuth.AuthMode)
	serverAuthProviders := uic.UseState(strings.Join(initialAuth.AuthProviders, ","))
	billingInterval := uic.UseState("annual")
	saveOAuthSession := func(token, csrf, userID string) {
		p := prefsAtom.Get()
		p.ServerToken = token
		p.ServerCSRF = csrf
		savePrefs(p)
		serverToken.Set(token)
		if strings.TrimSpace(userID) == "" {
			notify(uistate.T("settings.oauthSignedIn"), false)
		} else {
			notify(uistate.T("settings.oauthSignedInAs", userID), false)
		}
		requestBackendSyncNow()
	}
	onServerMode := func(v string) {
		serverMode.Set(v)
		if prefs.ServerMode(v) == prefs.ServerCloud {
			serverAuthMode.Set(backendauth.ModeOAuth)
			serverAuthProviders.Set("google,github")
		} else {
			serverAuthMode.Set(backendauth.ModeToken)
			serverAuthProviders.Set("")
		}
		p := prefsAtom.Get()
		p.ServerMode = prefs.ServerMode(v)
		savePrefs(p)
	}
	// keySet tracks whether a cloud AI key is stored server-side, so the panel can
	// show "Key set" + a Remove action (§7.11). Persisted locally; set on a
	// successful upload, cleared on remove or on switching servers.
	keySet := uic.UseState(lsGet("cashflux:cloud-ai-key-set") != "")
	onServerURL := uic.UseEvent(func(v string) {
		serverURL.Set(v)
		p := prefsAtom.Get()
		next := strings.TrimSpace(v)
		// Switch-server flow (§3.4): pointing at a different server signs out of the
		// old one — a token/session issued by one server is meaningless to another —
		// and re-points sync at the new URL. Local data is untouched; only the cloud
		// session is cleared. We compare hosts so editing the path/query of the same
		// server doesn't needlessly drop the session.
		if backendHost(next) != backendHost(p.ServerURL) && p.ServerToken != "" {
			p.ServerToken = ""
			p.ServerCSRF = ""
			serverToken.Set("")
			lsSet("cashflux:cloud-ai-key-set", "")
			keySet.Set(false)
			setSyncStatus(syncStatus{State: "offline"})
			notify(uistate.T("settings.serverSwitched"), false)
		}
		p.ServerURL = next
		savePrefs(p)
	})
	onServerToken := uic.UseEvent(func(v string) {
		serverToken.Set(v)
		p := prefsAtom.Get()
		p.ServerToken = strings.TrimSpace(v)
		savePrefs(p)
	})
	cloudPrice := uistate.T("settings.cloudPriceAnnual")
	if billingInterval.Get() == "monthly" {
		cloudPrice = uistate.T("settings.cloudPriceMonthly")
	}
	cloudSelected := prefs.ServerMode(serverMode.Get()) == prefs.ServerCloud
	authDiscovery := backendauth.Discovery{
		AuthMode:      serverAuthMode.Get(),
		AuthProviders: strings.Split(serverAuthProviders.Get(), ","),
	}.Normalize()
	oauthProviders := authDiscovery.OAuthProvidersOrFallback(nil)
	showTokenAuth := authDiscovery.UsesToken()
	showGoogleOAuth := containsString(oauthProviders, "google")
	showGitHubOAuth := containsString(oauthProviders, "github")
	uploadKey := uic.UseEvent(func() {
		uploadOpenAIKeyToBackend(serverURL.Get(), serverToken.Get(), aiKey.Get(), func() {
			lsSet("cashflux:cloud-ai-key-set", "1")
			keySet.Set(true)
			notify(uistate.T("settings.serverKeyStored"), false)
		}, func(msg string) {
			notify(uistate.T("settings.serverKeyFailed", strings.TrimSpace(msg)), true)
		})
	})
	removeKey := uic.UseEvent(func() {
		removeOpenAIKeyFromBackend(serverURL.Get(), serverToken.Get(), func() {
			lsSet("cashflux:cloud-ai-key-set", "")
			keySet.Set(false)
			notify(uistate.T("settings.serverKeyRemoved"), false)
		}, func(msg string) {
			notify(uistate.T("settings.serverKeyFailed", strings.TrimSpace(msg)), true)
		})
	})
	testBackend := uic.UseEvent(func() {
		testBackendConnection(serverURL.Get(), serverToken.Get(), func(discovery backendauth.Discovery) {
			discovery = discovery.Normalize()
			serverAuthMode.Set(discovery.AuthMode)
			serverAuthProviders.Set(strings.Join(discovery.AuthProviders, ","))
			notify(uistate.T("settings.serverTestOK", discovery.AuthMode), false)
		}, func(msg string) {
			notify(uistate.T("settings.serverTestFailed", strings.TrimSpace(msg)), true)
		})
	})
	startCheckout := uic.UseEvent(func() {
		startBillingCheckout(serverURL.Get(), serverToken.Get(), billingInterval.Get(), func(msg string) {
			notify(uistate.T("settings.billingFailed", strings.TrimSpace(msg)), true)
		})
	})
	openPortal := uic.UseEvent(func() {
		openBillingPortal(serverURL.Get(), serverToken.Get(), func(msg string) {
			notify(uistate.T("settings.billingFailed", strings.TrimSpace(msg)), true)
		})
	})
	signInGoogle := uic.UseEvent(func() {
		startOAuthLogin(serverURL.Get(), "google", saveOAuthSession, func(msg string) {
			notify(uistate.T("settings.oauthFailed", strings.TrimSpace(msg)), true)
		})
	})
	signInGitHub := uic.UseEvent(func() {
		startOAuthLogin(serverURL.Get(), "github", saveOAuthSession, func(msg string) {
			notify(uistate.T("settings.oauthFailed", strings.TrimSpace(msg)), true)
		})
	})
	signOut := uic.UseEvent(func() {
		p := prefsAtom.Get()
		signOutBackendOAuth(serverURL.Get(), p.ServerToken, p.ServerCSRF, func() {
			p.ServerToken = ""
			p.ServerCSRF = ""
			savePrefs(p)
			serverToken.Set("")
			notify(uistate.T("settings.oauthSignedOut"), false)
		})
	})
	syncNow := uic.UseEvent(func() {
		requestBackendSyncNow()
		notify(uistate.T("settings.syncRequested"), false)
	})
	// C309: restore / discard a local edit that lost an LWW conflict.
	activeWsID := loadRegistry().ActiveID
	restoreConflict := uic.UseEvent(func() {
		if restoreConflictBackup(activeWsID) {
			notify(uistate.T("sync.conflictRestored"), false)
		}
		bump()
	})
	discardConflict := uic.UseEvent(func() {
		clearConflictBackup(activeWsID)
		notify(uistate.T("sync.conflictDiscarded"), false)
		bump()
	})

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
			Label:    uistate.T("settings.showScreen", uistate.T(sc.LabelKey)),
			On:       !hidden.IsHidden(path),
			OnChange: func(bool) { toggleModule(path) },
		}))
	}

	// fxAIFetchNode is the live-rate fetch panel; it owns its async hooks so it
	// must be created via CreateElement (not inlined). Pass the resolved API key
	// and base URL directly so the component never reads appstate inside a hook.
	fxAIFetchAPIKey := ""
	fxAIFetchBaseURL := ai.DefaultBaseURL
	if a := appstate.Default; a != nil {
		fxAIFetchAPIKey = a.Settings().OpenAIKey
	}
	fxAIFetchNode := uic.CreateElement(fxAIFetch, fxAIFetchProps{
		APIKey:       fxAIFetchAPIKey,
		BaseURL:      fxAIFetchBaseURL,
		Base:         base,
		Codes:        fxNonBaseCodes,
		CurrentRates: fxCurrentRates,
		OnSet:        setRate,
	})

	leftProps := settingsLeftProps{
		MemberChips:   memberChips,
		OnBase:        onBase,
		Base:          base,
		OnMethod:      onMethod,
		CurMethod:     curMethod,
		FXRows:        fxRows,
		FXAIFetch:     fxAIFetchNode,
		ScreenToggles: screenToggles,
		FreshnessRows: freshnessRows,
	}

	activeLang := uistate.ActiveLanguage()
	langOptions := make([]uic.Node, 0)
	for _, l := range uistate.Languages() {
		langOptions = append(langOptions, Option(Value(string(l)), SelectedIf(activeLang == l), langDisplay(l)))
	}

	// Tab panes: Preferences → AI → Cloud → Data → Advanced (plus Household from
	// leftProps). Ordered by usage frequency: everyday preferences first, AI is
	// setup-once, Cloud is power-user.
	rightProps := settingsRightProps{
		Pr: pr,
		// Appearance is its own tab now — the Preferences link just switches to it.
		OnAppearanceLink: func() { setTab.Set("appearance") },
		OnDateStyle:      onDateStyle,
		OnWeekStart:      func(v string) { p := prefsAtom.Get(); p.WeekStart = prefs.WeekStart(v); savePrefs(p) },
		OnPayCycleAnchor: func(v string) { p := prefsAtom.Get(); p.PayCycleAnchor = strings.TrimSpace(v); savePrefs(p) },
		OnMonthlyIncome: func(v string) {
			amt, err := money.ParseMinor(strings.TrimSpace(v), currency.Decimals(base))
			if err != nil || amt < 0 {
				amt = 0
			}
			p := prefsAtom.Get()
			p.MonthlyIncomeMinor = amt
			savePrefs(p)
		},

		AiOn:       aiOn.Get(),
		OnAiToggle: func(v bool) { aiOn.Set(v) },
		AiKey:      aiKey.Get(),
		OnKey:      onKey,
		OnRememberKey: func(v bool) {
			p := prefsAtom.Get()
			p.RememberAIKey = v
			savePrefs(p)
			if v {
				uistate.PersistAIKey(aiKey.Get())
			} else {
				uistate.ClearAIKey()
			}
		},
		OnModel:  onModel,
		CurModel: curModel,
		WsKey:    wsKey.Get(),
		OnWsKey:  onWsKey,

		BackendOn:         backendOn.Get(),
		OnBackendToggle:   onBackendToggle,
		ServerMode:        serverMode.Get(),
		OnServerMode:      onServerMode,
		ServerURL:         serverURL.Get(),
		OnServerURL:       onServerURL,
		ServerToken:       serverToken.Get(),
		OnServerToken:     onServerToken,
		CloudSelected:     cloudSelected,
		AuthDiscovery:     authDiscovery,
		ShowTokenAuth:     showTokenAuth,
		ShowGoogleOAuth:   showGoogleOAuth,
		ShowGitHubOAuth:   showGitHubOAuth,
		OnSignInGoogle:    signInGoogle,
		OnSignInGitHub:    signInGitHub,
		OnSignOut:         signOut,
		OnTestBackend:     testBackend,
		OnSyncNow:         syncNow,
		HasConflictBackup: hasConflictBackup(activeWsID),
		OnRestoreConflict: restoreConflict,
		OnDiscardConflict: discardConflict,
		OnUploadKey:       uploadKey,
		KeySet:            keySet.Get(),
		OnRemoveKey:       removeKey,
		BillingInterval:   billingInterval.Get(),
		OnBillingInterval: func(v string) { billingInterval.Set(v) },
		CloudPrice:        cloudPrice,
		OnStartCheckout:   startCheckout,
		OnOpenPortal:      openPortal,

		OnExportJSON:    func() { exportJSON(notify) },
		OnExportCSV:     func() { exportCSV(notify) },
		OnBackupAll:     backupEverything,
		OnImportJSON:    func() { importJSON(bump, notify) },
		OnLoadSample:    func() { loadSample(bump, notify) },
		OnResetSample:   func() { resetSampleData(notify) },
		OnWipe:          func() { wipeData(bump, notify) },
		OnBackupCadence: onBackupCadence,

		LangOptions:   langOptions,
		LangCount:     len(langOptions),
		OnLang:        onLang,
		OnExportLangs: func() { exportLanguages(notify) },
		OnImportLangs: func() { importLanguages(notify) },
		Bump:          bump,
	}

	// Debug log viewer (moved here from the old /settings screen): the last entries
	// of the in-app log ring, newest first, with a refresh.
	_ = logRev.Get() // re-render when refreshed
	var logBody uic.Node = P(css.Class("empty"), uistate.T("settings.noLog"))
	if app := appstate.Default; app != nil {
		entries := app.LogRing().Entries()
		if n := len(entries); n > 0 {
			const maxShown = 25
			rows := make([]uic.Node, 0, maxShown)
			for i := n - 1; i >= 0 && len(rows) < maxShown; i-- {
				e := entries[i]
				rows = append(rows, Div(css.Class("row"),
					Div(css.Class("row-main"),
						Span(css.Class("row-desc"), e.Message),
						Span(css.Class("row-meta"), e.Level.String()),
					),
				))
			}
			logBody = Div(css.Class("rows"), rows)
		}
	}
	// Developer debug log behind a collapsed <details> disclosure so it doesn't
	// clutter the user-facing settings panel (§6.12) — power users expand it on demand.
	// Copy a bug report (R34-feedback): bundle the app version + the in-app log ring
	// into the clipboard so the user can paste it into a message. Local-first — it
	// copies to the clipboard, nothing is uploaded.
	copyBugReport := func() {
		var sb strings.Builder
		sb.WriteString("CashFlux " + version.Label() + "\n")
		if a := appstate.Default; a != nil {
			es := a.LogRing().Entries()
			sb.WriteString(strconv.Itoa(len(es)) + " log entries (oldest first):\n")
			for _, e := range es {
				sb.WriteString("[" + e.Level.String() + "] " + e.Message + "\n")
			}
		}
		if nv := js.Global().Get("navigator"); nv.Truthy() {
			if cb := nv.Get("clipboard"); cb.Truthy() {
				cb.Call("writeText", sb.String())
			}
		}
		notify(uistate.T("settings.bugReportCopied"), false)
	}
	debugLog := Details(css.Class(tw.Mt5),
		Summary(css.Class("set-label"), Style(map[string]string{"cursor": "pointer"}), uistate.T("settings.debugLog")),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2), Style(map[string]string{"justify-content": "flex-end"}),
			dataBtn(uistate.T("settings.copyReport"), false, copyBugReport),
			dataBtn(uistate.T("settings.refresh"), false, refreshLog),
		),
		logBody,
	)

	about := Div(css.Class("set-about", tw.Mt5, tw.Pt3, tw.BorderT, tw.BorderLine, tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.TextFaint, tw.Text12),
		Span("CashFlux "+version.Label()),
		A(Attr("href", "https://github.com/monstercameron/CashFlux/blob/main/CHANGELOG.md"), Attr("target", "_blank"),
			Attr("rel", "noopener noreferrer"), css.Class(tw.HoverTextFg, tw.Underline), uistate.T("settings.changelog")),
	)

	// One pane at a time behind the tab strip. Panes are pure renderers (their
	// embedded components own their hooks and mount/unmount cleanly per tab).
	var pane uic.Node
	switch setTab.Get() {
	case "prefs":
		pane = settingsPreferencesPane(rightProps)
	case "appearance":
		// The full appearance surface (mode/motion/accent + theme editor) mounts
		// as a child component — its hooks are its own, same as the other
		// pane-embedded components.
		pane = uic.CreateElement(screens.Appearance)
	case "alerts":
		pane = settingsAlertsPane(leftProps)
	case "ai":
		pane = settingsAIPane(rightProps)
	case "cloud":
		pane = settingsCloudPane(rightProps)
	case "data":
		pane = settingsDataPane(rightProps)
	case "advanced":
		pane = Div(settingsAdvancedPane(rightProps), debugLog, about)
	default: // "household"
		pane = settingsHouseholdPane(leftProps)
	}
	return Div(
		Div(css.Class("set-tab-strip", tw.Mb3, tw.Pb2, tw.BorderB, tw.BorderLine),
			ui.Segmented(ui.SegmentedProps{
				Label:    uistate.T("settings.tabsAria"),
				Selected: setTab.Get(),
				OnSelect: func(v string) { setTab.Set(v) },
				Options: []ui.SegOption{
					{Value: "household", Label: uistate.T("settings.tabHousehold")},
					{Value: "prefs", Label: uistate.T("settings.tabPrefs")},
					{Value: "appearance", Label: uistate.T("settings.tabAppearance")},
					{Value: "alerts", Label: uistate.T("settings.tabAlerts")},
					{Value: "ai", Label: uistate.T("settings.tabAI")},
					{Value: "cloud", Label: uistate.T("settings.tabCloud")},
					{Value: "data", Label: uistate.T("settings.tabData")},
					{Value: "advanced", Label: uistate.T("settings.tabAdvanced")},
				},
			}),
		),
		pane,
	)
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
	return Span(css.Class("member-chip"),
		Span(Style(map[string]string{"width": "9px", "height": "9px", "border-radius": "50%", "background": color})),
		m.Name,
	)
}

func containsString(items []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

// baseCurrencyOptions builds the base-currency <option> list from the registered
// currencies (code + name), marking the current base selected.
func baseCurrencyOptions(base string) []uic.Node {
	opts := make([]uic.Node, 0)
	for _, code := range currency.Codes() {
		name := code
		if c, ok := currency.Lookup(code); ok {
			name = c.Name
		}
		opts = append(opts, Option(Value(code), SelectedIf(base == code), code+" — "+name))
	}
	return opts
}

type fxRateRowProps struct {
	Code, Base string
	Rate       float64
	OnSet      func(code string, rate float64)
	Stale      bool   // the rate hasn't been refreshed in a while (L4)
	AsOf       string // C80: formatted date the rate was last set (empty if never)
}

// fxRateRow is one editable FX rate (1 <code> = <rate> <base>). Its own component
// so the input's change hook stays stable across the currency list. The value
// commits on change (blur), so a decimal like 1.08 isn't mangled mid-typing; an
// empty/zero value clears the rate.
func fxRateRow(props fxRateRowProps) uic.Node {
	on := uic.UseEvent(func(e uic.Event) {
		r, _ := strconv.ParseFloat(strings.TrimSpace(e.GetValue()), 64)
		props.OnSet(props.Code, r)
	})
	val := ""
	if props.Rate > 0 {
		val = strconv.FormatFloat(props.Rate, 'f', -1, 64)
	}
	// C81/C82: when a rate is set, disclose the inverse direction so the user can
	// confirm they entered it the right way round and see the conversion at a glance
	// — e.g. "1 USD = 0.92 EUR" (the row) ⇒ "1 EUR = 1.0870 USD" (this hint).
	var inverseHint uic.Node = Fragment()
	if props.Rate > 0 {
		inv := strconv.FormatFloat(1/props.Rate, 'f', 4, 64)
		inverseHint = Span(css.Class(tw.TextXs, tw.TextFaint), Attr("data-testid", "fx-inverse"),
			Style(map[string]string{"margin-left": "0.5rem"}),
			uistate.T("settings.fxInverse", props.Base, inv, props.Code))
	}
	return Div(css.Class("rate-row"),
		Span(Style(map[string]string{"width": "40px"}), props.Code),
		Span(css.Class(tw.TextFaint), uistate.T("settings.fxRateLabel", props.Code)),
		Input(css.Class("rate-in"), Type("number"), Attr("step", "any"), Attr("min", "0"), Attr("placeholder", "—"), Attr("aria-label", uistate.T("settings.fxRateAria", props.Code, props.Base)), Value(val), OnChange(on)),
		Span(css.Class(tw.TextFaint), props.Base),
		inverseHint,
		// C80: show when this rate was last set so the user can judge its freshness.
		If(props.AsOf != "", Span(css.Class(tw.TextXs, tw.TextFaint), Attr("data-testid", "fx-asof"), Style(map[string]string{"margin-left": "0.5rem"}), uistate.T("settings.fxAsOf", props.AsOf))),
		If(props.Stale, Span(css.Class(tw.TextXs), Attr("data-testid", "fx-stale"), Attr("title", uistate.T("settings.fxStaleTitle")), Style(map[string]string{"color": "var(--warn)", "margin-left": "0.5rem"}), uistate.T("settings.fxStale"))),
	)
}

// fxAIFetchProps configures the AI-powered exchange rate fetcher panel.
type fxAIFetchProps struct {
	// APIKey is the user's OpenAI key; empty means no key is configured.
	APIKey string
	// BaseURL is the OpenAI API base URL (for proxy compatibility).
	BaseURL string
	// Base is the base currency code (e.g. "USD").
	Base string
	// Codes is the list of non-base currency codes to fetch rates for.
	Codes []string
	// CurrentRates holds the rates already stored (code → current rate).
	CurrentRates map[string]float64
	// OnSet applies a rate; mirrors the outer setRate signature.
	OnSet func(code string, rate float64)
}

// proposedRate holds one AI-proposed rate and its per-row apply state.
type proposedRate struct {
	Code     string
	Proposed float64
}

// fxAIFetch is the "Fetch live rates with AI" panel inside the Settings FX
// editor. It owns all state for the async request cycle (loading, error,
// proposed rates) so its hooks sit at stable positions and never appear in
// a loop. The button is gated on a configured OpenAI key — when no key is set
// the panel shows a friendly hint instead.
func fxAIFetch(props fxAIFetchProps) uic.Node {
	loading := uic.UseState(false)
	errMsg := uic.UseState("")
	// proposed holds the AI's suggested rates, keyed and ordered for stable render.
	proposed := uic.UseState[[]proposedRate](nil)
	asOf := uic.UseState("")
	costStr := uic.UseState("")

	// cancelRef holds the in-flight abort func; replaced on each new request.
	cancelRef := uic.UseState[func()](nil)

	onFetch := uic.UseEvent(func() {
		// Abort any previous in-flight request.
		if c := cancelRef.Get(); c != nil {
			c()
		}
		loading.Set(true)
		errMsg.Set("")
		proposed.Set(nil)
		asOf.Set("")
		costStr.Set("")

		prompt := currency.BuildFXPrompt(props.Base, props.Codes)
		model := "gpt-5.5"
		cancel := ai.SendResponsesWebSearch(
			props.APIKey,
			props.BaseURL,
			model,
			prompt,
			func(text string, u ai.Usage) {
				loading.Set(false)
				rates, date, err := currency.ParseFXReply(text, props.Base)
				if err != nil {
					errMsg.Set(uistate.T("settings.fxAIError", err.Error()))
					return
				}
				result := make([]proposedRate, 0, len(rates))
				for _, code := range props.Codes {
					if r, ok := rates[code]; ok {
						result = append(result, proposedRate{Code: code, Proposed: r})
					}
				}
				proposed.Set(result)
				asOf.Set(date)
				if cost, ok := ai.EstimateCostUSD(model, u); ok {
					costStr.Set(uistate.T("settings.fxAICost", ai.FormatCostUSD(cost)))
				}
			},
			func(msg string) {
				loading.Set(false)
				errMsg.Set(uistate.T("settings.fxAIError", msg))
			},
		)
		cancelRef.Set(cancel)
	})

	// Key is not configured — show a friendly hint, no button.
	if props.APIKey == "" {
		return P(css.Class(tw.TextFaint, tw.Text12, tw.Mt2), uistate.T("settings.fxAINoKey"))
	}

	rows := proposed.Get()
	onApplyAll := uic.UseEvent(func() {
		for _, r := range proposed.Get() {
			props.OnSet(r.Code, r.Proposed)
		}
		proposed.Set(nil)
	})

	return Div(css.Class(tw.Mt2),
		// Fetch button
		Button(
			css.Class("data-btn"),
			Type("button"),
			Attr("data-testid", "fx-ai-fetch"),
			Attr("aria-label", uistate.T("settings.fxAIFetch")),
			OnClick(onFetch),
			IfElse(loading.Get(), Text(uistate.T("settings.fxAIFetching")), Text(uistate.T("settings.fxAIFetch"))),
		),

		// Error display
		If(errMsg.Get() != "",
			P(css.Class(tw.TextXs), Style(map[string]string{"color": "var(--danger)", "margin-top": "0.4rem"}), errMsg.Get()),
		),

		// Review panel — only shown when we have proposed rates
		If(len(rows) > 0,
			Div(
				Attr("data-testid", "fx-ai-review"),
				css.Class(tw.Mt3),
				P(css.Class(tw.Text12, tw.TextFaint), uistate.T("settings.fxAIReviewTitle")),
				If(asOf.Get() != "", P(css.Class(tw.Text12, tw.TextFaint), uistate.T("settings.fxAIAsOf", asOf.Get()))),
				If(costStr.Get() != "", P(css.Class(tw.Text12, tw.TextFaint), costStr.Get())),
				Div(css.Class(tw.Mt2),
					Map(rows, func(r proposedRate) uic.Node {
						return uic.CreateElement(fxAIProposedRow, fxAIProposedRowProps{
							Code:     r.Code,
							Base:     props.Base,
							Current:  props.CurrentRates[r.Code],
							Proposed: r.Proposed,
							OnApply: func() {
								props.OnSet(r.Code, r.Proposed)
								// Remove the row from proposed after applying.
								next := make([]proposedRate, 0, len(proposed.Get()))
								for _, p2 := range proposed.Get() {
									if p2.Code != r.Code {
										next = append(next, p2)
									}
								}
								proposed.Set(next)
							},
						})
					}),
				),
				Button(
					css.Class("data-btn"),
					Type("button"),
					Attr("data-testid", "fx-ai-apply-all"),
					OnClick(onApplyAll),
					uistate.T("settings.fxAIApplyAll"),
				),
			),
		),
	)
}

// fxAIProposedRowProps carries the display data for one proposed-rate review row.
type fxAIProposedRowProps struct {
	Code     string
	Base     string
	Current  float64
	Proposed float64
	OnApply  func()
}

// fxAIProposedRow renders one proposed FX rate with the current rate for
// comparison and an individual Apply button. Its own component so the click
// hook sits at a stable position outside the Map loop.
func fxAIProposedRow(props fxAIProposedRowProps) uic.Node {
	onApply := uic.UseEvent(func() {
		if props.OnApply != nil {
			props.OnApply()
		}
	})
	curStr := "—"
	if props.Current > 0 {
		curStr = strconv.FormatFloat(props.Current, 'f', -1, 64)
	}
	propStr := strconv.FormatFloat(props.Proposed, 'f', -1, 64)
	return Div(css.Class("rate-row", tw.Mt1),
		Span(Style(map[string]string{"width": "40px"}), props.Code),
		Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.fxAICurrent")+": "+curStr),
		Span(css.Class(tw.Text12), " → "+propStr+" "+props.Base),
		Button(css.Class("data-btn"), Type("button"), OnClick(onApply), uistate.T("settings.fxAIApply")),
	)
}

// exportJSON downloads the full dataset as a JSON file (the portable
// export/import + sync payload), via the pure appstate export.
func exportJSON(notify func(string, bool)) {
	app := appstate.Default
	if app == nil {
		return
	}
	data, err := app.ExportJSONWithBlobs()
	if err != nil {
		notify(uistate.T("settings.exportDataErr", err.Error()), true)
		return
	}
	downloadBytes("cashflux.json", "application/json", data)
	recordBackupNow() // stamp the backup so the B28 reminder resets
	notify(uistate.T("settings.exportedData", "cashflux.json"), false)
}

// exportCSV downloads all transactions as a CSV file.
func exportCSV(notify func(string, bool)) {
	app := appstate.Default
	if app == nil {
		return
	}
	data, err := app.ExportCSV()
	if err != nil {
		notify(uistate.T("settings.exportTxnErr", err.Error()), true)
		return
	}
	downloadBytes("transactions.csv", "text/csv", data)
	notify(uistate.T("settings.exportedTxn", "transactions.csv"), false)
}

// exportLanguages downloads the whole language bundle (every supported language)
// as JSON — the file translators edit and re-import.
func exportLanguages(notify func(string, bool)) {
	data, err := uistate.ExportLanguages()
	if err != nil {
		notify(uistate.T("settings.exportLangsErr", err.Error()), true)
		return
	}
	downloadBytes("cashflux-languages.json", "application/json", data)
	notify(uistate.T("settings.exportedLangs"), false)
}

// importLanguages picks a language-bundle JSON file and merges it into the app,
// persisting it for next launch.
func importLanguages(notify func(string, bool)) {
	pickFile(".json", func(data []byte) {
		if err := uistate.ImportLanguages(data); err != nil {
			notify(uistate.T("settings.importLangsErr", err.Error()), true)
			return
		}
		notify(uistate.T("settings.importedLangs"), false)
	})
}

// importJSON picks a JSON dataset file and replaces all data with it, then
// bumps the data revision so screens refresh.  A confirmation gate (C295)
// is shown after the user has chosen a file but before the overwrite occurs,
// so they cannot lose their data without an explicit yes.
func importJSON(onChange func(), notify func(string, bool)) {
	pickFile(".json", func(data []byte) {
		app := appstate.Default
		if app == nil {
			return
		}
		// C295: confirm before overwriting all current data.
		uistate.ConfirmModalLabeled(
			uistate.T("settings.importConfirm"),
			uistate.T("settings.importConfirmBtn"),
			true,
			func(ok bool) {
				if !ok {
					return
				}
				// ImportJSONWithBlobs moves embedded artifact image bytes
				// into IndexedDB so the autosave doesn't write them back to
				// localStorage (C294 — blob-complete round-trip on import).
				if err := app.ImportJSONWithBlobs(data); err != nil {
					notify(uistate.T("settings.importErr", err.Error()), true)
					return
				}
				// A real import means the user is no longer on sample data (L6).
				uistate.SetSampleActive(false)
				onChange()
				notify(uistate.T("settings.importedData"), false)
			},
		)
	})
}

// loadSample replaces all data with the built-in sample dataset and refreshes.
func loadSample(onChange func(), notify func(string, bool)) {
	app := appstate.Default
	if app == nil {
		return
	}
	if err := app.LoadSample(); err != nil {
		notify(uistate.T("settings.loadSampleErr", err.Error()), true)
		return
	}
	onChange()
	uistate.SetSampleActive(true)
	uistate.RequestPersist() // C2: flush before a fast reload can race the autosave ticker
	notify(uistate.T("settings.loadedSample"), false)
}

// resetSampleData is the ONE-step demo reset: wipe + load-sample without the
// intermediate stop. It replaces everything with the fresh built-in sample
// (store.Load is a wholesale replace, so no separate wipe is needed), clears
// the two stores that describe the OLD data (the in-memory activity feed and
// the cached SMART content in the preserved settings KV), purges stray
// financial browser-store keys, persists the new snapshot, and reloads once
// the write commits — exactly what wipe-then-load did in two trips.
func resetSampleData(notify func(string, bool)) {
	uistate.ConfirmModalLabeled(uistate.T("settings.resetSampleConfirm"), uistate.T("settings.resetSampleConfirmBtn"), true, func(ok bool) {
		if !ok {
			return
		}
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.LoadSample(); err != nil {
			notify(uistate.T("settings.loadSampleErr", err.Error()), true)
			return
		}
		auditview.Feed.Clear()
		uistate.ClearSmartGenerated()
		uistate.SetSampleActive(true)
		// Persist the fresh sample and reload after the write commits;
		// wipeFinancialLocalState snapshots whatever the store now holds — the
		// sample — while sweeping the satellite keys derived from the old data.
		suspendAutosave = true
		wipeFinancialLocalState(reloadPage)
	})
}

// wipeData clears all data after a confirmation, then refreshes.
func wipeData(onChange func(), notify func(string, bool)) {
	// C298: name the destructive action on its own button ("Erase everything")
	// rather than a generic "Confirm".
	uistate.ConfirmModalLabeled(uistate.T("settings.wipeConfirm"), uistate.T("settings.wipeConfirmBtn"), true, func(ok bool) {
		if !ok {
			return
		}
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.Wipe(); err != nil {
			notify(uistate.T("settings.wipeErr", err.Error()), true)
			return
		}
		// The store wipe cleared the financial tables and the audit_log table, but two
		// things live outside that sweep and would otherwise survive: (1) the in-memory
		// activity feed (the Activity screen's preferred source, re-hydrated from the
		// table at boot), and (2) the data-derived SMART content — cached AI "messages",
		// dismissals, last-run stamps, and the digest log — which sit in the PRESERVED
		// settings KV. Clear both now, before the dataset export below, so a stale feed
		// or stale smart message can't reappear after the reload.
		auditview.Feed.Clear()
		uistate.ClearSmartGenerated()
		// A wipe means the user is starting fresh — hide the sample banner (L6).
		uistate.SetSampleActive(false)
		// Make the wipe authoritative: clear non-settings keys and persist the emptied
		// store, then reload (after the IndexedDB write commits) so all in-memory state
		// re-hydrates from the clean slate (nothing survives the wipe).
		suspendAutosave = true
		wipeFinancialLocalState(reloadPage)
	})
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
	cls := "data-btn"
	if props.Danger {
		cls += " data-btn-danger"
	}
	args := []any{css.Class(cls), Type("button")}
	onClick := props.OnClick
	args = append(args, OnClick(func() {
		if onClick != nil {
			onClick()
		}
	}), props.Label)
	return Button(args...)
}
