// SPDX-License-Identifier: MIT

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
	FXAIFetch     uic.Node // AI live-rate fetch panel (nil when no key configured)
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
		ui.Divider(),
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
		// C81: spell out the rate convention so users enter the right number — a rate is
		// "base currency per 1 unit of the listed currency" (e.g. 1.08 ⇒ 1 unit = 1.08 base).
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.fxConventionHint")),
		If(len(p.FXRows) == 0, P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.noRates"))),
		Div(p.FXRows),
		p.FXAIFetch,
		Hr(css.Class(tw.BorderT, tw.BorderLine), Style(map[string]string{"border-bottom": "none", "margin": "1rem 0 0"})),
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
	Pr               prefs.Prefs
	OnAppearanceLink func()      // closes panel + navigates to /appearance
	OnDateStyle      uic.Handler // UseEvent
	OnWeekStart      func(string)
	// AI
	AiOn          bool
	OnAiToggle    func(bool)
	AiKey         string
	OnKey         uic.Handler // UseEvent
	KeySet        bool        // a cloud AI key is stored server-side (§7.11)
	OnRemoveKey   uic.Handler // UseEvent — clears the server-side key
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
	HasConflictBackup bool        // C309: a local edit lost an LWW conflict and is recoverable
	OnRestoreConflict uic.Handler // UseEvent
	OnDiscardConflict uic.Handler // UseEvent
	OnUploadKey       uic.Handler // UseEvent
	BillingInterval   string
	OnBillingInterval func(string)
	CloudPrice        string
	OnStartCheckout   uic.Handler // UseEvent
	OnOpenPortal      uic.Handler // UseEvent
	// Data
	OnExportJSON    func()
	OnExportCSV     func()
	OnBackupAll     func() // C297: full multi-workspace backup, also in the palette
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

// aiModelDisplayName returns a short human-readable label for the given OpenAI
// model ID. Falls back to the raw ID for any future models not yet listed.
func aiModelDisplayName(model string) string {
	switch model {
	case "gpt-5.5":
		return "GPT-5.5"
	case "o4-mini":
		return "o4-mini (reasoning)"
	default:
		// Covers "" (unset) and "gpt-5.4-mini" — the default selection.
		return "GPT-5.4 mini"
	}
}

// settingsRightColumn renders the right column of the global settings panel
// (appearance, preferences, AI, cloud & server, data, advanced). Pure rendering
// helper — no hooks.
func settingsRightColumn(p settingsRightProps) uic.Node {
	return Div(
		// 1 · Appearance — link to the dedicated /appearance page (B34); all
		// theming controls live there so the Settings panel stays focused.
		H4(css.Class("set-label"), uistate.T("settings.appearance")),
		P(css.Class("muted", tw.TextXs), uistate.T("settings.appearanceHint")),
		If(p.OnAppearanceLink != nil, Button(css.Class("btn", tw.Mt2), Type("button"),
			OnClick(func() {
				if p.OnAppearanceLink != nil {
					p.OnAppearanceLink()
				}
			}),
			uistate.T("settings.appearanceLink"),
		)),

		// 2 · Preferences — date/week-start sit naturally after appearance.
		ui.Divider(),
		H4(css.Class("set-label"), uistate.T("settings.preferences")),
		Div(css.Class("toggle-row"),
			Span(uistate.T("settings.weekStart")),
			ui.Segmented(ui.SegmentedProps{
				Label: uistate.T("settings.weekStart"), // C318: name the radiogroup
				Options: []ui.SegOption{
					{Value: string(prefs.WeekSunday), Label: uistate.T("settings.sunday")},
					{Value: string(prefs.WeekMonday), Label: uistate.T("settings.monday")},
					{Value: string(prefs.WeekSaturday), Label: uistate.T("settings.saturday")},
				},
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
		ui.Divider(),
		H4(css.Class("set-label"), uistate.T("settings.aiTitle")),
		// AI is enabled by the presence of an API key (the no-key hint below is the
		// affordance). The former local-only "Enable AI" toggle gated nothing and reset
		// to off on every open, so it was removed to avoid a misleading dead control (§6.12).
		Input(css.Class("set-input", tw.Mt045), Type("password"), Attr("aria-label", uistate.T("settings.aiKeyPlaceholder")), Placeholder(uistate.T("settings.aiKeyPlaceholder")), Value(p.AiKey), OnInput(p.OnKey)),
		// C292: always disclose where the key goes (BYOK, device-local, direct to OpenAI),
		// not only the conditional no-key hint below — the trust statement shouldn't hide
		// the moment a key is entered.
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiKeyTrust")),
		// C100: explain what the key is for, that it's BYOK/pay-per-use, and where to get one.
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiKeyExplainer")),
		If(strings.TrimSpace(p.AiKey) == "", P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiNoKey"))),
		ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("settings.rememberKey"), On: p.Pr.RememberAIKey, OnChange: p.OnRememberKey}),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.rememberKeyNote")),
		Select(css.Class("set-input", tw.Mt045), Attr("aria-label", uistate.T("settings.aiModel")), Title(uistate.T("settings.aiModel")), OnChange(p.OnModel),
			Option(Value("gpt-5.4-mini"), SelectedIf(p.CurModel == "gpt-5.4-mini" || p.CurModel == ""), "GPT-5.4 mini"),
			Option(Value("gpt-5.5"), SelectedIf(p.CurModel == "gpt-5.5"), "GPT-5.5"),
			Option(Value("o4-mini"), SelectedIf(p.CurModel == "o4-mini"), "o4-mini (reasoning)"),
		),
		// C250: surface the active model and BYOK billing transparency so users know
		// which model is active and that they pay OpenAI directly per token used.
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiModelNote", aiModelDisplayName(p.CurModel))),
		H4(css.Class("set-label"), uistate.T("settings.webSearchTitle")),
		Input(css.Class("set-input", tw.Mt045), Type("password"), Attr("aria-label", uistate.T("settings.webSearchKeyPlaceholder")), Placeholder(uistate.T("settings.webSearchKeyPlaceholder")), Value(p.WsKey), OnInput(p.OnWsKey)),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.webSearchHint")),

		// 4 · Cloud & server — power-user sync config after AI.
		ui.Divider(),
		H4(css.Class("set-label"), uistate.T("settings.backendTitle")),
		// C304: framing line — communicates this section as the subscription/connection
		// surface (sync + backup + bundled AI, self-host option), not raw infrastructure.
		// Placed immediately after the heading so the user's first question ("what is this
		// for?") is answered before they see any controls.
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudSectionIntro")),
		// C291: always-visible data-disclosure — what leaves the device when sync is on
		// vs. off. Shown before the toggle so the user sees the trade-off before acting.
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudDataDisclosure")),
		// C300: persistent price teaser — shown when the user is not yet subscribed to
		// Cloud so the plan cost is discoverable in Settings without relying on the
		// one-shot UpgradeSheet. Omitted once authenticated (ServerToken set) to avoid
		// showing a subscribe pitch to an existing subscriber.
		If(p.CloudSelected && strings.TrimSpace(p.ServerToken) == "",
			P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1),
				uistate.T("settings.cloudPricingTeaser", p.CloudPrice),
			),
		),
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
		// C309: recoverable conflict backup — when a local edit lost an LWW conflict
		// (server had newer changes), offer to restore the saved local copy or discard
		// the backup, so the change is never silently lost.
		If(p.HasConflictBackup, Div(css.Class("conflict-restore", tw.Flex, tw.FlexCol, tw.Gap1, tw.Mt2, tw.Px3, tw.Py2, tw.Rounded4, tw.BorderL),
			Attr("role", "status"), Attr("data-testid", "sync-conflict-restore"),
			P(css.Class(tw.Text12, tw.TextDim), uistate.T("sync.restoreConflictHint")),
			Div(css.Class(tw.Flex, tw.Gap2, tw.Mt1),
				Button(css.Class("btn", "btn-sm", "btn-primary"), Type("button"), OnClick(p.OnRestoreConflict), uistate.T("sync.restoreConflict")),
				Button(css.Class("btn", "btn-sm"), Type("button"), OnClick(p.OnDiscardConflict), uistate.T("sync.discardConflict")),
			),
		)),
		// Cloud AI-key status: "Key set" + Remove, shown once a key has been uploaded (§7.11).
		If(p.KeySet, Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1),
			Span(css.Class(tw.Text12, tw.TextDim), uistate.T("settings.serverKeySet")),
			Button(css.Class("btn", "btn-sm", "btn-del"), Type("button"), OnClick(p.OnRemoveKey), uistate.T("settings.removeKey")),
		)),
		// Signed-in devices list + per-device revoke (§7.11) — shown once authenticated.
		If(strings.TrimSpace(p.ServerToken) != "", uic.CreateElement(DevicesList)),
		If(p.CloudSelected, Fragment(
			// C302: discoverable manage/cancel/downgrade surface — shown whenever the
			// user is authenticated with the cloud backend, so a subscriber can always
			// find the billing portal without hunting through the checkout UI.
			If(strings.TrimSpace(p.ServerToken) != "", Fragment(
				H4(css.Class("set-label"), uistate.T("settings.manageSubTitle")),
				P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.manageSubHint")),
				Button(css.Class("btn", tw.Mt045), Type("button"),
					Attr("data-testid", "manage-subscription"),
					OnClick(p.OnOpenPortal),
					uistate.T("settings.manageSub"),
				),
			)),
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
			),
			P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.cloudTrustLine")),
		)),

		// 5 · Data — export/import/wipe actions.
		ui.Divider(),
		H4(css.Class("set-label"), uistate.T("settings.data")),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
			dataBtn(uistate.T("settings.exportJSON"), false, p.OnExportJSON),
			dataBtn(uistate.T("settings.exportCSV"), false, p.OnExportCSV),
			dataBtn(uistate.T("settings.backupAll"), false, p.OnBackupAll),
			dataBtn(uistate.T("settings.importDataset"), false, p.OnImportJSON),
			dataBtn(uistate.T("settings.loadSample"), false, p.OnLoadSample),
			dataBtn(uistate.T("settings.wipe"), true, p.OnWipe),
		),
		P(css.Class("muted", tw.TextXs), uistate.T("settings.dataExportHint")),
		// C299: surface how recently the user backed up, so a stale backup is visible.
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), Attr("data-testid", "last-backup"), lastBackupSummary()),
		H4(css.Class("set-label"), uistate.T("settings.backupCadence")),
		P(css.Class("muted", tw.TextXs), uistate.T("settings.backupCadenceHint")),
		Select(css.Class("set-input"), Attr("aria-label", uistate.T("settings.backupCadence")), Title(uistate.T("settings.backupCadence")), OnChange(p.OnBackupCadence),
			Option(Value("monthly"), SelectedIf(loadBackupCadence() == backup.Monthly), uistate.T("settings.cadenceMonthly")),
			Option(Value("weekly"), SelectedIf(loadBackupCadence() == backup.Weekly), uistate.T("settings.cadenceWeekly")),
			Option(Value("off"), SelectedIf(loadBackupCadence() == backup.Off), uistate.T("settings.cadenceOff")),
		),

		// 6 · Advanced — workspaces, app lock, languages; rarely needed at the bottom.
		ui.Divider(),
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
