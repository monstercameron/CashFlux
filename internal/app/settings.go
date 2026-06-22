//go:build js && wasm

package app

import (
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backendauth"
	"github.com/monstercameron/CashFlux/internal/backup"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/contrast"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/i18n"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/version"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/GoWebComponents/css"
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
// notifySettings is the Settings-modal toggle for OS/browser notifications (C75):
// flipping it on records the opt-in and requests permission; the catch-up runner
// then posts browser notifications in addition to the in-app feed.
func notifySettings() uic.Node {
	on := uic.UseState(uistate.BrowserNotifyEnabled())
	return Div(
		Div(css.Class("set-label"), uistate.T("settings.notifyTitle")),
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
		Div(css.Class("set-label"), uistate.T("settings.music")),
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
			Div(css.Class("set-label"), props.Title),
			colorRow,
			importance,
		)
	}
	rows := make([]any, 0, len(schema.Fields)+3)
	rows = append(rows, Div(css.Class("set-label"), schema.Title))
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
		Span("Tile color"),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Input(Type("color"), Style(map[string]string{"width": "2rem", "height": "1.6rem", "padding": "0", "border": "none", "background": "none"}), Attr("aria-label", "Tile color"), Value(val), OnChange(on)),
			If(props.Current != "", Button(css.Class("btn"), Type("button"), OnClick(func() {
				if props.OnSet != nil {
					props.OnSet("")
				}
			}), "Clear")),
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
	opts := []any{css.Class("set-input"), Title(uistate.T("widget.importance")), OnChange(on)}
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
			Input(css.Class("rate-in"), Type("number"), Value(strconv.Itoa(f.Int(props.Cfg))), OnInput(on)),
		)
	case widgetcfg.Select:
		on := uic.UseEvent(func(e uic.Event) { props.OnSet(f.Key, e.GetValue()) })
		cur := f.Str(props.Cfg)
		opts := make([]any, 0, len(f.Options)+2)
		opts = append(opts, css.Class("set-input"), OnChange(on))
		for _, o := range f.Options {
			opts = append(opts, Option(Value(o.Value), SelectedIf(cur == o.Value), o.Label))
		}
		return Div(css.Class("toggle-row"), Span(f.Label), Select(opts...))
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
	return Div(css.Class("rate-row"),
		Span(Style(map[string]string{"width": "110px"}), props.Label),
		Input(css.Class("rate-in"), Type("number"), Value(strconv.Itoa(props.Days)), OnInput(on)),
		Span(css.Class(tw.TextFaint), uistate.T("settings.freshNever")),
	)
}

// hideableScreens lists the screens a user can show or hide from the sidebar.
// The dashboard is intentionally omitted — it is locked visible in
// internal/modules. Covers every routed main-line screen (primary nav, the Tools
// group, and the System group).
var hideableScreens = []struct{ Label, Path string }{
	{"Accounts", "/accounts"},
	{"Transactions", "/transactions"},
	{"Budgets", "/budgets"},
	{"Goals", "/goals"},
	{"To-do", "/todo"},
	{"Planning", "/planning"},
	{"Allocate", "/allocate"},
	{"Insights", "/insights"},
	{"Documents", "/documents"},
	{"Customize", "/customize"},
	{"Members", "/members"},
	{"Categories", "/categories"},
	{"Rules", "/rules"},
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
		// Keep the persisted copy in step when the user has opted in (C27).
		if prefsAtom.Get().RememberAIKey {
			uistate.PersistAIKey(v)
		}
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
	if app := appstate.Default; app != nil {
		members = app.Members()
		s := app.Settings()
		if s.BaseCurrency != "" {
			base = s.BaseCurrency
		}
		for _, code := range currency.Codes() {
			if code == base {
				continue
			}
			stale := false
			if s.FXUpdatedAt != nil {
				stale = currency.RateStale(s.FXUpdatedAt[code], time.Now(), currency.DefaultRateMaxAge)
			}
			fxRows = append(fxRows, uic.CreateElement(fxRateRow, fxRateRowProps{
				Code: code, Base: base, Rate: s.FXRates[code], OnSet: setRate, Stale: stale,
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
	onServerURL := uic.UseEvent(func(v string) {
		serverURL.Set(v)
		p := prefsAtom.Get()
		p.ServerURL = strings.TrimSpace(v)
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
			notify(uistate.T("settings.serverKeyStored"), false)
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
		Div(css.Class("set-label"), uistate.T("settings.householdMembers")),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1), memberChips),
		Div(css.Class("set-label"), uistate.T("settings.baseCurrency")),
		Select(css.Class("set-input"), Attr("aria-label", uistate.T("settings.baseCurrency")), Title(uistate.T("settings.baseCurrency")), OnChange(onBase), baseCurrencyOptions(base)),
		Div(css.Class("set-label"), uistate.T("settings.budgetMethod")),
		Select(css.Class("set-input"), Attr("aria-label", uistate.T("settings.budgetMethod")), Title(uistate.T("settings.budgetMethod")), OnChange(onMethod),
			Option(Value(string(budgeting.MethodSimple)), SelectedIf(curMethod == budgeting.MethodSimple), uistate.T("settings.budgetMethodSimple")),
			Option(Value(string(budgeting.MethodZeroBased)), SelectedIf(curMethod == budgeting.MethodZeroBased), uistate.T("settings.budgetMethodZero")),
			Option(Value(string(budgeting.MethodEnvelope)), SelectedIf(curMethod == budgeting.MethodEnvelope), uistate.T("settings.budgetMethodEnvelope")),
		),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.budgetMethodNote")),
		Div(css.Class("set-label"), uistate.T("settings.exchangeRates")),
		If(len(fxRows) == 0, P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.noRates"))),
		Div(fxRows),
		Div(css.Class("set-label"), uistate.T("settings.screens")),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.screensHint")),
		Div(screenToggles),
		Div(css.Class("set-label"), uistate.T("settings.freshnessTitle")),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.freshnessHint")),
		Div(freshnessRows),
		uic.CreateElement(notifySettings),
		uic.CreateElement(musicSettings),
	)

	activeLang := uistate.ActiveLanguage()
	langOptions := make([]uic.Node, 0)
	for _, l := range uistate.Languages() {
		langOptions = append(langOptions, Option(Value(string(l)), SelectedIf(activeLang == l), langDisplay(l)))
	}

	right := Div(
		Div(css.Class("set-label"), uistate.T("settings.aiTitle")),
		ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("settings.aiEnable"), On: aiOn.Get(), OnChange: func(v bool) { aiOn.Set(v) }}),
		Input(css.Class("set-input", tw.Mt045), Type("password"), Placeholder(uistate.T("settings.aiKeyPlaceholder")), Value(aiKey.Get()), OnInput(onKey)),
		If(strings.TrimSpace(aiKey.Get()) == "", P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiNoKey"))),
		ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("settings.rememberKey"), On: pr.RememberAIKey, OnChange: func(v bool) {
			p := prefsAtom.Get()
			p.RememberAIKey = v
			savePrefs(p)
			if v {
				uistate.PersistAIKey(aiKey.Get())
			} else {
				uistate.ClearAIKey()
			}
		}}),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.rememberKeyNote")),
		Div(css.Class("set-label"), uistate.T("settings.webSearchTitle")),
		Input(css.Class("set-input", tw.Mt045), Type("password"), Attr("aria-label", uistate.T("settings.webSearchTitle")), Placeholder(uistate.T("settings.webSearchKeyPlaceholder")), Value(wsKey.Get()), OnInput(onWsKey)),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.webSearchHint")),
		Div(css.Class("set-label"), uistate.T("settings.backendTitle")),
		// Clear on/off for all backend connections (sync + AI proxy). Off by intent
		// keeps the app fully local even with a server saved, so an unreachable
		// backend never throws websocket errors the user can't dismiss.
		ui.ToggleRow(ui.ToggleRowProps{Label: "Connect to a backend (sync + AI proxy)", On: backendOn.Get(), OnChange: onBackendToggle}),
		If(!backendOn.Get(), P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), "Backend off — the app stays fully local; no sync or proxy connections are made.")),
		If(backendOn.Get(), Fragment(
			ui.Segmented(ui.SegmentedProps{
				Label: uistate.T("settings.serverMode"),
				Options: []ui.SegOption{
					{Value: string(prefs.ServerCloud), Label: uistate.T("settings.serverModeCloud")},
					{Value: string(prefs.ServerSelfHosted), Label: uistate.T("settings.serverModeSelf")},
				},
				Selected: serverMode.Get(),
				OnSelect: onServerMode,
			}),
			Input(css.Class("set-input", tw.Mt045), Type("url"), Attr("aria-label", uistate.T("settings.backendURL")), Placeholder(defaultBackendURL), Value(serverURL.Get()), OnInput(onServerURL)),
			If(showTokenAuth, Input(css.Class("set-input", tw.Mt045), Type("password"), Attr("aria-label", uistate.T("settings.backendToken")), Placeholder(uistate.T("settings.backendToken")), Value(serverToken.Get()), OnInput(onServerToken))),
		)),
		If(cloudSelected, P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.backendNote"))),
		If(!cloudSelected, P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.selfHostedNote"))),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.authMode", authDiscovery.AuthMode)),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.syncStatus", syncStatusLabel())),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Mt045),
			If(showGoogleOAuth, Button(css.Class("btn"), Type("button"), OnClick(signInGoogle), uistate.T("settings.signInGoogle"))),
			If(showGitHubOAuth, Button(css.Class("btn"), Type("button"), OnClick(signInGitHub), uistate.T("settings.signInGitHub"))),
			If(strings.TrimSpace(serverToken.Get()) != "", Button(css.Class("btn"), Type("button"), OnClick(signOut), uistate.T("settings.signOut"))),
			Button(css.Class("btn"), Type("button"), OnClick(testBackend), uistate.T("settings.testBackend")),
			Button(css.Class("btn"), Type("button"), OnClick(syncNow), uistate.T("settings.syncNow")),
			Button(css.Class("btn"), Type("button"), OnClick(uploadKey), uistate.T("settings.uploadKey")),
			A(css.Class("btn"), Attr("href", "docs/SELF_HOSTING.md"), Attr("target", "_blank"), Attr("rel", "noreferrer"), uistate.T("settings.deploySelfHost")),
		),
		If(cloudSelected, Fragment(
			Div(css.Class("set-label"), uistate.T("settings.cloudPlanTitle")),
			P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudPlanNote")),
			Div(css.Class(tw.Text18, tw.FontSemibold, tw.Mt045), cloudPrice),
			P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudTrialNote")),
			ui.Segmented(ui.SegmentedProps{
				Label: uistate.T("settings.cloudPlanBilling"),
				Options: []ui.SegOption{
					{Value: "annual", Label: uistate.T("settings.cloudPlanAnnual")},
					{Value: "monthly", Label: uistate.T("settings.cloudPlanMonthly")},
				},
				Selected: billingInterval.Get(),
				OnSelect: func(v string) { billingInterval.Set(v) },
			}),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Mt045),
				Button(css.Class("btn btn-primary"), Type("button"), OnClick(startCheckout), uistate.T("settings.cloudSubscribe")),
				Button(css.Class("btn"), Type("button"), OnClick(openPortal), uistate.T("settings.manageSub")),
			),
			P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudTrustLine")),
		)),
		Select(css.Class("set-input", tw.Mt045), Attr("aria-label", uistate.T("settings.aiModel")), Title(uistate.T("settings.aiModel")), OnChange(onModel),
			Option(Value("gpt-4o-mini"), SelectedIf(curModel == "gpt-4o-mini" || curModel == ""), "GPT-4o mini"),
			Option(Value("gpt-4.1-nano"), SelectedIf(curModel == "gpt-4.1-nano"), "GPT-4.1 nano"),
			Option(Value("gpt-4.1-mini"), SelectedIf(curModel == "gpt-4.1-mini"), "GPT-4.1 mini"),
			Option(Value("gpt-4o"), SelectedIf(curModel == "gpt-4o"), "GPT-4o"),
			Option(Value("gpt-4.1"), SelectedIf(curModel == "gpt-4.1"), "GPT-4.1"),
			Option(Value("o4-mini"), SelectedIf(curModel == "o4-mini"), "o4-mini (reasoning)"),
		),
		Div(css.Class("set-label"), uistate.T("settings.appearance")),
		ui.Segmented(ui.SegmentedProps{
			Options:  []ui.SegOption{{Value: string(prefs.ThemeDark), Label: uistate.T("settings.themeDark")}, {Value: string(prefs.ThemeLight), Label: uistate.T("settings.themeLight")}, {Value: string(prefs.ThemeSystem), Label: uistate.T("settings.themeSystem")}},
			Selected: string(pr.Theme),
			OnSelect: func(v string) {
				p := prefsAtom.Get()
				p.Theme = prefs.Theme(v)
				savePrefs(p)
			},
		}),
		Div(css.Class("toggle-row"),
			Span(uistate.T("settings.accent")),
			ui.SwatchPicker(ui.SwatchPickerProps{
				Colors:   []string{"#2e8b57", "#cfa14e", "#7c83ff", "#d8716f"},
				Selected: pr.Accent,
				OnSelect: func(c string) {
					p := prefsAtom.Get()
					p.Accent = c
					savePrefs(p)
				},
			}),
		),
		accentContrastNote(pr.Accent, pr.Theme),
		// Density and display scale moved into the theme editor below — the theme
		// engine is now the single source of truth for both (B20 unify).
		uic.CreateElement(themeEditor),
		Div(css.Class("set-label"), uistate.T("settings.preferences")),
		Div(css.Class("toggle-row"),
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
		Select(css.Class("set-input", tw.Mt045), Attr("aria-label", uistate.T("settings.dateFormat")), Title(uistate.T("settings.dateFormat")), OnChange(onDateStyle),
			Option(Value(string(prefs.DateISO)), SelectedIf(pr.DateStyle == prefs.DateISO), "2026-06-05  (ISO)"),
			Option(Value(string(prefs.DateUS)), SelectedIf(pr.DateStyle == prefs.DateUS), "06/05/2026  (US)"),
			Option(Value(string(prefs.DateEU)), SelectedIf(pr.DateStyle == prefs.DateEU), "05/06/2026  (European)"),
			Option(Value(string(prefs.DateLong)), SelectedIf(pr.DateStyle == prefs.DateLong), "Jun 5, 2026  (Long)"),
		),
		Div(css.Class("set-label"), uistate.T("settings.data")),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
			dataBtn(uistate.T("settings.exportJSON"), false, func() { exportJSON(notify) }),
			dataBtn(uistate.T("settings.exportCSV"), false, func() { exportCSV(notify) }),
			dataBtn(uistate.T("settings.import"), false, func() { importJSON(bump, notify) }),
			dataBtn(uistate.T("settings.loadSample"), false, func() { loadSample(bump, notify) }),
			dataBtn(uistate.T("settings.wipe"), true, func() { wipeData(bump, notify) }),
		),
		Div(css.Class("set-label"), uistate.T("settings.backupCadence")),
		P(css.Class("muted", tw.TextXs), uistate.T("settings.backupCadenceHint")),
		Select(css.Class("set-input"), Attr("aria-label", uistate.T("settings.backupCadence")), Title(uistate.T("settings.backupCadence")), OnChange(onBackupCadence),
			Option(Value("monthly"), SelectedIf(loadBackupCadence() == backup.Monthly), uistate.T("settings.cadenceMonthly")),
			Option(Value("weekly"), SelectedIf(loadBackupCadence() == backup.Weekly), uistate.T("settings.cadenceWeekly")),
			Option(Value("off"), SelectedIf(loadBackupCadence() == backup.Off), uistate.T("settings.cadenceOff")),
		),
		Div(css.Class("set-label"), uistate.T("ws.section")),
		P(css.Class("muted", tw.TextXs), uistate.T("ws.sectionHint")),
		workspacesSection(bump),
		Div(css.Class("set-label"), uistate.T("applock.section")),
		P(css.Class("muted", tw.TextXs), uistate.T("applock.sectionHint")),
		appLockSection(bump),
		Div(css.Class("set-label"), uistate.T("settings.languages")),
		Select(css.Class("set-input"), Attr("aria-label", uistate.T("settings.language")), Title(uistate.T("settings.language")), OnChange(onLang), langOptions),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
			dataBtn(uistate.T("settings.exportLangs"), false, func() { exportLanguages(notify) }),
			dataBtn(uistate.T("settings.importLangs"), false, func() { importLanguages(notify) }),
		),
	)

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
	debugLog := Div(css.Class(tw.Mt5),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween),
			Div(css.Class("set-label"), uistate.T("settings.debugLog")),
			dataBtn(uistate.T("settings.refresh"), false, refreshLog),
		),
		logBody,
	)

	about := Div(css.Class("set-about", tw.Mt5, tw.Pt3, tw.BorderT, tw.BorderLine, tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.TextFaint, tw.Text12),
		Span("CashFlux "+version.Label()),
		A(Attr("href", "https://github.com/monstercameron/CashFlux/blob/main/CHANGELOG.md"), Attr("target", "_blank"),
			Attr("rel", "noopener noreferrer"), css.Class(tw.HoverTextFg, tw.Underline), uistate.T("settings.changelog")),
	)

	return Div(
		Div(css.Class(tw.Grid, tw.GridCols2, tw.GapX7, tw.ContentStart), left, right),
		debugLog,
		about,
	)
}

// accentSurfaceHexes returns the elevated surface color(s) the accent is judged
// against for the given theme — both when the theme follows the system, since we
// can't know which is active at render time.
func accentSurfaceHexes(theme prefs.Theme) []string {
	const darkElev, lightElev = "#1a1a1d", "#efede8"
	switch theme {
	case prefs.ThemeLight:
		return []string{lightElev}
	case prefs.ThemeDark:
		return []string{darkElev}
	default: // system: judged against both
		return []string{darkElev, lightElev}
	}
}

// accentContrastNote renders a small line stating the selected accent's contrast
// ratio against the theme surface and whether it clears WCAG AA for UI/large
// elements (3:1), using the pure internal/contrast helpers. Accent is used for
// fills, active states, and the focus ring, so the large/UI threshold applies.
func accentContrastNote(accent string, theme prefs.Theme) uic.Node {
	if accent == "" {
		accent = "#2e8b57" // the default swatch
	}
	worst := 21.0
	for _, surf := range accentSurfaceHexes(theme) {
		if r, err := contrast.Ratio(accent, surf); err == nil && r < worst {
			worst = r
		}
	}
	if contrast.PassesAA(worst, true) {
		return Span(css.Class("muted", tw.TextXs), uistate.T("settings.accentContrastOk", worst))
	}
	return Span(css.Class(tw.TextXs), Style(map[string]string{"color": "var(--danger)"}),
		uistate.T("settings.accentContrastLow", worst))
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
	Stale      bool // the rate hasn't been refreshed in a while (L4)
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
	return Div(css.Class("rate-row"),
		Span(Style(map[string]string{"width": "40px"}), props.Code),
		Span(css.Class(tw.TextFaint), uistate.T("settings.fxRateLabel", props.Code)),
		Input(css.Class("rate-in"), Type("number"), Attr("step", "any"), Attr("min", "0"), Attr("placeholder", "—"), Value(val), OnChange(on)),
		Span(css.Class(tw.TextFaint), props.Base),
		If(props.Stale, Span(css.Class(tw.TextXs), Attr("data-testid", "fx-stale"), Attr("title", uistate.T("settings.fxStaleTitle")), Style(map[string]string{"color": "#cfa14e", "margin-left": "0.5rem"}), uistate.T("settings.fxStale"))),
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
// bumps the data revision so screens refresh.
func importJSON(onChange func(), notify func(string, bool)) {
	pickFile(".json", func(data []byte) {
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.ImportJSON(data); err != nil {
			notify(uistate.T("settings.importErr", err.Error()), true)
			return
		}
		// A real import means the user is no longer on sample data (L6).
		uistate.SetSampleActive(false)
		onChange()
		notify(uistate.T("settings.importedData"), false)
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
	notify(uistate.T("settings.loadedSample"), false)
}

// wipeData clears all data after a confirmation, then refreshes.
func wipeData(onChange func(), notify func(string, bool)) {
	confirmModal(uistate.T("settings.wipeConfirm"), true, func(ok bool) {
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
		// A wipe means the user is starting fresh — hide the sample banner (L6).
		uistate.SetSampleActive(false)
		onChange()
		notify(uistate.T("settings.wiped"), false)
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
	args := []any{css.Class("data-btn"), Type("button")}
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
