//go:build js && wasm

package app

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendauth"
	"github.com/monstercameron/CashFlux/internal/backup"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// settingsLeftProps carries all data and callbacks needed to render the left
// column of globalSettingsForm. No hooks are declared inside settingsLeftColumn.
type settingsLeftProps struct {
	MemberChips   []uic.Node
	OnBase        uic.Handler // UseEvent(func(e uic.Event){…})
	Base          string
	OnMethod      uic.Handler // UseEvent(func(e uic.Event){…})
	CurMethod     budgeting.Methodology
	FXRows        []uic.Node
	ScreenToggles []uic.Node
	FreshnessRows []uic.Node
}

// settingsLeftColumn renders the left column of the global settings panel
// (household members, base currency, budget method, FX rates, screen toggles,
// freshness windows, notifications, music). Pure rendering helper — no hooks.
func settingsLeftColumn(p settingsLeftProps) uic.Node {
	return Div(
		// Household members first — Renée reviews who is in the household before
		// adjusting anything else. Screens immediately after so she can hide modules
		// she doesn't use before diving into currency/budget config.
		H4(css.Class("set-label"), uistate.T("settings.householdMembers")),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1), p.MemberChips),
		H4(css.Class("set-label"), uistate.T("settings.screens")),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.screensHint")),
		Div(p.ScreenToggles),
		Hr(tw.BorderT, tw.BorderLine, Style(map[string]string{"border-bottom": "none", "margin": "1rem 0 0"})),
		H4(css.Class("set-label"), uistate.T("settings.baseCurrency")),
		Select(css.Class("set-input"), Attr("aria-label", uistate.T("settings.baseCurrency")), Title(uistate.T("settings.baseCurrency")), OnChange(p.OnBase), baseCurrencyOptions(p.Base)),
		H4(css.Class("set-label"), uistate.T("settings.budgetMethod")),
		Select(css.Class("set-input"), Attr("aria-label", uistate.T("settings.budgetMethod")), Title(uistate.T("settings.budgetMethod")), OnChange(p.OnMethod),
			Option(Value(string(budgeting.MethodSimple)), SelectedIf(p.CurMethod == budgeting.MethodSimple), uistate.T("settings.budgetMethodSimple")),
			Option(Value(string(budgeting.MethodZeroBased)), SelectedIf(p.CurMethod == budgeting.MethodZeroBased), uistate.T("settings.budgetMethodZero")),
			Option(Value(string(budgeting.MethodEnvelope)), SelectedIf(p.CurMethod == budgeting.MethodEnvelope), uistate.T("settings.budgetMethodEnvelope")),
		),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.budgetMethodNote")),
		H4(css.Class("set-label"), uistate.T("settings.exchangeRates")),
		If(len(p.FXRows) == 0, P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.noRates"))),
		Div(p.FXRows),
		Hr(tw.BorderT, tw.BorderLine, Style(map[string]string{"border-bottom": "none", "margin": "1rem 0 0"})),
		H4(css.Class("set-label"), uistate.T("settings.freshnessTitle")),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.freshnessHint")),
		Div(p.FreshnessRows),
		uic.CreateElement(notifySettings),
		uic.CreateElement(musicSettings),
	)
}

// settingsRightProps carries all data and callbacks needed to render the right
// column of globalSettingsForm. No hooks are declared inside settingsRightColumn.
//
// Fields typed uic.Handler are the direct result of uic.UseEvent(…) in the
// parent; the html/shorthand On*() prop helpers accept Handler transparently.
// Fields typed func(…) are plain closures that do not go through UseEvent.
type settingsRightProps struct {
	// Appearance
	Pr          prefs.Prefs
	OnTheme     func(string)
	OnAccent    func(string)
	OnMotion    func(string)
	OnDateStyle uic.Handler // UseEvent
	OnWeekStart func(string)
	// AI
	AiOn          bool
	OnAiToggle    func(bool)
	AiKey         string
	OnKey         uic.Handler // UseEvent
	OnRememberKey func(bool)
	OnModel       uic.Handler // UseEvent
	CurModel      string
	WsKey         string
	OnWsKey       uic.Handler // UseEvent
	// Cloud & server
	BackendOn         bool
	OnBackendToggle   func(bool)
	ServerMode        string
	OnServerMode      func(string)
	ServerURL         string
	OnServerURL       uic.Handler // UseEvent
	ServerToken       string
	OnServerToken     uic.Handler // UseEvent
	CloudSelected     bool
	AuthDiscovery     backendauth.Discovery
	ShowTokenAuth     bool
	ShowGoogleOAuth   bool
	ShowGitHubOAuth   bool
	OnSignInGoogle    uic.Handler // UseEvent
	OnSignInGitHub    uic.Handler // UseEvent
	OnSignOut         uic.Handler // UseEvent
	OnTestBackend     uic.Handler // UseEvent
	OnSyncNow         uic.Handler // UseEvent
	OnUploadKey       uic.Handler // UseEvent
	BillingInterval   string
	OnBillingInterval func(string)
	CloudPrice        string
	OnStartCheckout   uic.Handler // UseEvent
	OnOpenPortal      uic.Handler // UseEvent
	// Data
	OnExportJSON    func()
	OnExportCSV     func()
	OnImportJSON    func()
	OnLoadSample    func()
	OnWipe          func()
	OnBackupCadence uic.Handler // UseEvent
	// Advanced
	LangOptions   []uic.Node
	OnLang        uic.Handler // UseEvent
	OnExportLangs func()
	OnImportLangs func()
	Bump          func()
}

// settingsRightColumn renders the right column of the global settings panel
// (appearance, preferences, AI, cloud & server, data, advanced). Pure rendering
// helper — no hooks.
func settingsRightColumn(p settingsRightProps) uic.Node {
	return Div(
		// 1 · Appearance — most-used first so Renée can set her theme without scrolling.
		H4(css.Class("set-label"), uistate.T("settings.appearance")),
		ui.Segmented(ui.SegmentedProps{
			Options:  []ui.SegOption{{Value: string(prefs.ThemeDark), Label: uistate.T("settings.themeDark")}, {Value: string(prefs.ThemeLight), Label: uistate.T("settings.themeLight")}, {Value: string(prefs.ThemeSystem), Label: uistate.T("settings.themeSystem")}},
			Selected: string(p.Pr.Theme),
			OnSelect: p.OnTheme,
		}),
		Div(css.Class("toggle-row"),
			Span(uistate.T("settings.motion")),
			ui.Segmented(ui.SegmentedProps{
				Options:  []ui.SegOption{{Value: string(prefs.MotionFull), Label: uistate.T("settings.motionFull")}, {Value: string(prefs.MotionSubtle), Label: uistate.T("settings.motionSubtle")}, {Value: string(prefs.MotionOff), Label: uistate.T("settings.motionOff")}},
				Selected: string(p.Pr.Motion),
				OnSelect: p.OnMotion,
			}),
		),
		P(css.Class("muted", tw.TextXs), uistate.T("settings.motionHint")),
		Div(css.Class("toggle-row"),
			Span(uistate.T("settings.accent")),
			ui.SwatchPicker(ui.SwatchPickerProps{
				Colors:   []string{"#2e8b57", "#cfa14e", "#7c83ff", "#d8716f"},
				Selected: p.Pr.Accent,
				OnSelect: p.OnAccent,
			}),
		),
		accentContrastNote(p.Pr.Accent, p.Pr.Theme),
		// Density and display scale are in the theme editor (B20 unify).
		uic.CreateElement(themeEditor),

		// 2 · Preferences — date/week-start sit naturally after appearance.
		Hr(tw.BorderT, tw.BorderLine, Style(map[string]string{"border-bottom": "none", "margin": "1rem 0 0"})),
		H4(css.Class("set-label"), uistate.T("settings.preferences")),
		Div(css.Class("toggle-row"),
			Span(uistate.T("settings.weekStart")),
			ui.Segmented(ui.SegmentedProps{
				Options:  []ui.SegOption{{Value: string(prefs.WeekSunday), Label: uistate.T("settings.sunday")}, {Value: string(prefs.WeekMonday), Label: uistate.T("settings.monday")}},
				Selected: string(p.Pr.WeekStart),
				OnSelect: p.OnWeekStart,
			}),
		),
		Select(css.Class("set-input", tw.Mt045), Attr("aria-label", uistate.T("settings.dateFormat")), Title(uistate.T("settings.dateFormat")), OnChange(p.OnDateStyle),
			Option(Value(string(prefs.DateISO)), SelectedIf(p.Pr.DateStyle == prefs.DateISO), "2026-06-05  (ISO)"),
			Option(Value(string(prefs.DateUS)), SelectedIf(p.Pr.DateStyle == prefs.DateUS), "06/05/2026  (US)"),
			Option(Value(string(prefs.DateEU)), SelectedIf(p.Pr.DateStyle == prefs.DateEU), "05/06/2026  (European)"),
			Option(Value(string(prefs.DateLong)), SelectedIf(p.Pr.DateStyle == prefs.DateLong), "Jun 5, 2026  (Long)"),
		),

		// 3 · AI — setup-once; key + model select in one logical cluster.
		Hr(tw.BorderT, tw.BorderLine, Style(map[string]string{"border-bottom": "none", "margin": "1rem 0 0"})),
		H4(css.Class("set-label"), uistate.T("settings.aiTitle")),
		ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("settings.aiEnable"), On: p.AiOn, OnChange: p.OnAiToggle}),
		Input(css.Class("set-input", tw.Mt045), Type("password"), Attr("aria-label", uistate.T("settings.aiKeyPlaceholder")), Placeholder(uistate.T("settings.aiKeyPlaceholder")), Value(p.AiKey), OnInput(p.OnKey)),
		If(strings.TrimSpace(p.AiKey) == "", P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiNoKey"))),
		ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("settings.rememberKey"), On: p.Pr.RememberAIKey, OnChange: p.OnRememberKey}),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.rememberKeyNote")),
		Select(css.Class("set-input", tw.Mt045), Attr("aria-label", uistate.T("settings.aiModel")), Title(uistate.T("settings.aiModel")), OnChange(p.OnModel),
			Option(Value("gpt-4o-mini"), SelectedIf(p.CurModel == "gpt-4o-mini" || p.CurModel == ""), "GPT-4o mini"),
			Option(Value("gpt-4.1-nano"), SelectedIf(p.CurModel == "gpt-4.1-nano"), "GPT-4.1 nano"),
			Option(Value("gpt-4.1-mini"), SelectedIf(p.CurModel == "gpt-4.1-mini"), "GPT-4.1 mini"),
			Option(Value("gpt-4o"), SelectedIf(p.CurModel == "gpt-4o"), "GPT-4o"),
			Option(Value("gpt-4.1"), SelectedIf(p.CurModel == "gpt-4.1"), "GPT-4.1"),
			Option(Value("o4-mini"), SelectedIf(p.CurModel == "o4-mini"), "o4-mini (reasoning)"),
		),
		H4(css.Class("set-label"), uistate.T("settings.webSearchTitle")),
		Input(css.Class("set-input", tw.Mt045), Type("password"), Attr("aria-label", uistate.T("settings.webSearchKeyPlaceholder")), Placeholder(uistate.T("settings.webSearchKeyPlaceholder")), Value(p.WsKey), OnInput(p.OnWsKey)),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.webSearchHint")),

		// 4 · Cloud & server — power-user sync config after AI.
		Hr(tw.BorderT, tw.BorderLine, Style(map[string]string{"border-bottom": "none", "margin": "1rem 0 0"})),
		H4(css.Class("set-label"), uistate.T("settings.backendTitle")),
		// Clear on/off for all backend connections (sync + AI proxy). Off by intent
		// keeps the app fully local even with a server saved, so an unreachable
		// backend never throws websocket errors the user can't dismiss.
		ui.ToggleRow(ui.ToggleRowProps{Label: "Connect to a backend (sync + AI proxy)", On: p.BackendOn, OnChange: p.OnBackendToggle}),
		If(!p.BackendOn, P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), "Backend off — the app stays fully local; no sync or proxy connections are made.")),
		If(p.BackendOn, Fragment(
			ui.Segmented(ui.SegmentedProps{
				Label: uistate.T("settings.serverMode"),
				Options: []ui.SegOption{
					{Value: string(prefs.ServerCloud), Label: uistate.T("settings.serverModeCloud")},
					{Value: string(prefs.ServerSelfHosted), Label: uistate.T("settings.serverModeSelf")},
				},
				Selected: p.ServerMode,
				OnSelect: p.OnServerMode,
			}),
			Input(css.Class("set-input", tw.Mt045), Type("url"), Attr("aria-label", uistate.T("settings.backendURL")), Placeholder(defaultBackendURL), Value(p.ServerURL), OnInput(p.OnServerURL)),
			If(p.ShowTokenAuth, Input(css.Class("set-input", tw.Mt045), Type("password"), Attr("aria-label", uistate.T("settings.backendToken")), Placeholder(uistate.T("settings.backendToken")), Value(p.ServerToken), OnInput(p.OnServerToken))),
		)),
		If(p.CloudSelected, P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.backendNote"))),
		If(!p.CloudSelected, P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.selfHostedNote"))),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.authMode", p.AuthDiscovery.AuthMode)),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.syncStatus", syncStatusLabel())),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Mt045),
			If(p.ShowGoogleOAuth, Button(css.Class("btn"), Type("button"), OnClick(p.OnSignInGoogle), uistate.T("settings.signInGoogle"))),
			If(p.ShowGitHubOAuth, Button(css.Class("btn"), Type("button"), OnClick(p.OnSignInGitHub), uistate.T("settings.signInGitHub"))),
			If(strings.TrimSpace(p.ServerToken) != "", Button(css.Class("btn"), Type("button"), OnClick(p.OnSignOut), uistate.T("settings.signOut"))),
			Button(css.Class("btn"), Type("button"), OnClick(p.OnTestBackend), uistate.T("settings.testBackend")),
			Button(css.Class("btn"), Type("button"), OnClick(p.OnSyncNow), uistate.T("settings.syncNow")),
			Button(css.Class("btn"), Type("button"), OnClick(p.OnUploadKey), uistate.T("settings.uploadKey")),
			A(css.Class("btn"), Attr("href", "docs/SELF_HOSTING.md"), Attr("target", "_blank"), Attr("rel", "noreferrer"), uistate.T("settings.deploySelfHost")),
		),
		If(p.CloudSelected, Fragment(
			H4(css.Class("set-label"), uistate.T("settings.cloudPlanTitle")),
			P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudPlanNote")),
			Div(css.Class(tw.Text18, tw.FontSemibold, tw.Mt045), p.CloudPrice),
			P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudTrialNote")),
			ui.Segmented(ui.SegmentedProps{
				Label: uistate.T("settings.cloudPlanBilling"),
				Options: []ui.SegOption{
					{Value: "annual", Label: uistate.T("settings.cloudPlanAnnual")},
					{Value: "monthly", Label: uistate.T("settings.cloudPlanMonthly")},
				},
				Selected: p.BillingInterval,
				OnSelect: p.OnBillingInterval,
			}),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Mt045),
				Button(css.Class("btn btn-primary"), Type("button"), OnClick(p.OnStartCheckout), uistate.T("settings.cloudSubscribe")),
				Button(css.Class("btn"), Type("button"), OnClick(p.OnOpenPortal), uistate.T("settings.manageSub")),
			),
			P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudTrustLine")),
		)),

		// 5 · Data — export/import/wipe actions.
		Hr(tw.BorderT, tw.BorderLine, Style(map[string]string{"border-bottom": "none", "margin": "1rem 0 0"})),
		H4(css.Class("set-label"), uistate.T("settings.data")),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
			dataBtn(uistate.T("settings.exportJSON"), false, p.OnExportJSON),
			dataBtn(uistate.T("settings.exportCSV"), false, p.OnExportCSV),
			dataBtn(uistate.T("settings.importDataset"), false, p.OnImportJSON),
			dataBtn(uistate.T("settings.loadSample"), false, p.OnLoadSample),
			dataBtn(uistate.T("settings.wipe"), true, p.OnWipe),
		),
		H4(css.Class("set-label"), uistate.T("settings.backupCadence")),
		P(css.Class("muted", tw.TextXs), uistate.T("settings.backupCadenceHint")),
		Select(css.Class("set-input"), Attr("aria-label", uistate.T("settings.backupCadence")), Title(uistate.T("settings.backupCadence")), OnChange(p.OnBackupCadence),
			Option(Value("monthly"), SelectedIf(loadBackupCadence() == backup.Monthly), uistate.T("settings.cadenceMonthly")),
			Option(Value("weekly"), SelectedIf(loadBackupCadence() == backup.Weekly), uistate.T("settings.cadenceWeekly")),
			Option(Value("off"), SelectedIf(loadBackupCadence() == backup.Off), uistate.T("settings.cadenceOff")),
		),

		// 6 · Advanced — workspaces, app lock, languages; rarely needed at the bottom.
		Hr(tw.BorderT, tw.BorderLine, Style(map[string]string{"border-bottom": "none", "margin": "1rem 0 0"})),
		H4(css.Class("set-label"), uistate.T("ws.section")),
		P(css.Class("muted", tw.TextXs), uistate.T("ws.sectionHint")),
		workspacesSection(p.Bump),
		H4(css.Class("set-label"), uistate.T("applock.section")),
		P(css.Class("muted", tw.TextXs), uistate.T("applock.sectionHint")),
		appLockSection(p.Bump),
		H4(css.Class("set-label"), uistate.T("settings.languages")),
		Select(css.Class("set-input"), Attr("aria-label", uistate.T("settings.language")), Title(uistate.T("settings.language")), OnChange(p.OnLang), p.LangOptions),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
			dataBtn(uistate.T("settings.exportLangs"), false, p.OnExportLangs),
			dataBtn(uistate.T("settings.importLangs"), false, p.OnImportLangs),
		),
	)
}
