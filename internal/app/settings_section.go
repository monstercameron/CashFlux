// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/aiprovider"
	"github.com/monstercameron/CashFlux/internal/backendauth"
	"github.com/monstercameron/CashFlux/internal/backup"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
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

// settingsHouseholdPane renders the Household tab of the global settings panel
// (members, screen toggles, base currency, budget method, FX rates). Pure
// rendering helper — no hooks.
func settingsHouseholdPane(p settingsLeftProps) uic.Node {
	return Div(
		// Household members first — Renée reviews who is in the household before
		// adjusting anything else. Screens immediately after so she can hide modules
		// she doesn't use before diving into currency/budget config.
		H4(css.Class("set-label"), uistate.T("settings.householdMembers")),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1), p.MemberChips),
		H4(css.Class("set-label"), uistate.T("settings.screens")),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.screensHint")),
		Div(css.Class("set-toggle-list"), p.ScreenToggles),
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
	)
}

// settingsAlertsPane renders the Alerts tab: freshness windows, notifications
// and per-alert toggles, the learning threshold, and music. Pure rendering
// helper — no hooks of its own (the embedded components own theirs).
func settingsAlertsPane(p settingsLeftProps) uic.Node {
	return Div(
		H4(css.Class("set-label"), uistate.T("settings.freshnessTitle")),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.freshnessHint")),
		Div(p.FreshnessRows),
		uic.CreateElement(notifySettings),
		uic.CreateElement(learnThresholdRow),
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
	OnAppearanceLink func()      // switches to the Appearance tab
	OnDateStyle      uic.Handler // UseEvent
	OnWeekStart      func(string)
	OnPayCycleAnchor func(string) // sets PayCycleAnchor ("YYYY-MM-DD" or "")
	OnMonthlyIncome  func(string) // sets MonthlyIncomeMinor from a major-unit string (empty = 0)
	// OnIdleCashBenchmarkAPR: AC15's idle-cash benchmark rate (percent, e.g. "4.5").
	OnIdleCashBenchmarkAPR uic.Handler // UseEvent
	// AI
	AiOn           bool
	OnAiToggle     func(bool)
	AiKey          string
	OnKey          uic.Handler // UseEvent
	KeySet         bool        // a cloud AI key is stored server-side (§7.11)
	OnRemoveKey    uic.Handler // UseEvent — clears the server-side key
	OnRememberKey  func(bool)
	OnModel        uic.Handler // UseEvent
	CurModel       string
	Models         []string    // model ids fetched live from OpenAI (empty = fall back to built-in defaults)
	OnReloadModels uic.Handler // UseEvent — refetch the model list
	ModelsLoading  bool
	ModelsErr      string
	WsKey          string
	OnWsKey        uic.Handler // UseEvent
	// AG18 — OpenAI-compatible base-URL override (Ollama/LM Studio/proxy).
	BaseURL   string
	OnBaseURL uic.Handler // UseEvent
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
	OnResetSample   func() // one-step wipe + reseed (the demo reset)
	OnWipe          func()
	OnBackupCadence uic.Handler // UseEvent
	OnExportPack    func()      // AC16: generate + download the estate emergency pack
	// Advanced
	LangOptions   []uic.Node
	LangCount     int         // number of installed languages; the picker hides at <2
	OnLang        uic.Handler // UseEvent
	OnExportLangs func()
	OnImportLangs func()
	Bump          func()
}

// aiModelDisplayName returns a short human-readable label for the given OpenAI
// model ID. Falls back to the raw ID for any future models not yet listed.
func aiModelDisplayName(model string) string {
	switch model {
	case "", "gpt-5.4-mini":
		// Covers "" (unset) and the default selection.
		return "GPT-5.4 mini"
	case "gpt-5.5":
		return "GPT-5.5"
	case "o4-mini":
		return "o4-mini (reasoning)"
	default:
		// A dynamically-fetched model with no friendly label — show its raw id
		// rather than mislabelling it as the default.
		return model
	}
}

// modelIDList returns the model ids to show in the picker: the live list fetched
// from OpenAI when available, else the built-in defaults. The current selection is
// always included so a custom or older model stays visible even when it isn't in
// the fetched list.
func modelIDList(models []string, cur string) []string {
	ids := models
	if len(ids) == 0 {
		ids = []string{"gpt-5.4-mini", "gpt-5.5", "o4-mini"}
	}
	cur = strings.TrimSpace(cur)
	if cur != "" {
		for _, m := range ids {
			if m == cur {
				return ids
			}
		}
		ids = append([]string{cur}, ids...)
	}
	return ids
}

// settingsPreferencesPane renders the Preferences tab: the appearance link,
// week start, date format, pay-cycle anchor, and monthly income. Pure
// rendering helper — no hooks.
func settingsPreferencesPane(p settingsRightProps) uic.Node {
	return Div(
		// Appearance — hands off to the Appearance tab; all theming controls
		// live there so Preferences stays focused.
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
			Option(Value(string(prefs.DateISO)), SelectedIf(p.Pr.DateStyle == prefs.DateISO), uistate.T("settings.dateOptISO")),
			Option(Value(string(prefs.DateUS)), SelectedIf(p.Pr.DateStyle == prefs.DateUS), uistate.T("settings.dateOptUS")),
			Option(Value(string(prefs.DateEU)), SelectedIf(p.Pr.DateStyle == prefs.DateEU), uistate.T("settings.dateOptEU")),
			Option(Value(string(prefs.DateLong)), SelectedIf(p.Pr.DateStyle == prefs.DateLong), uistate.T("settings.dateOptLong")),
		),
		// C128: pay-cycle anchor — a known payday date used to align every-2-weeks
		// budget periods to the user's actual pay cycle instead of the fixed epoch.
		H4(css.Class("set-label"), uistate.T("settings.payCycleAnchor")),
		Input(css.Class("set-input", tw.Mt045), Type("date"), Attr("aria-label", uistate.T("settings.payCycleAnchor")),
			Value(p.Pr.PayCycleAnchor),
			OnChange(func(e uic.Event) {
				if p.OnPayCycleAnchor != nil {
					p.OnPayCycleAnchor(e.GetValue())
				}
			}),
		),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.payCycleAnchorHint")),
		// C22: monthly income — the household's configured take-home pay. When
		// set (> 0), budgeting helpers (50/30/20 template, income banners,
		// safe-to-spend) prefer this figure over transaction-derived income,
		// giving stable budgets even in months with unusual deposits.
		H4(css.Class("set-label"), uistate.T("settings.monthlyIncome")),
		uic.CreateElement(monthlyIncomeInput, monthlyIncomeInputProps{
			ValueMinor: p.Pr.MonthlyIncomeMinor,
			OnChange:   p.OnMonthlyIncome,
		}),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.monthlyIncomeHint")),
		// AC15: the idle-cash benchmark rate — a stated assumption (never a live feed)
		// that the /accounts idle-cash line and the idle_cash_forgone_annual formula
		// variable compare against. Blank/zero hides the figure entirely.
		H4(css.Class("set-label"), uistate.T("settings.idleCashBenchmarkLabel")),
		Input(css.Class("set-input", tw.Mt045), Type("number"), Attr("min", "0"), Attr("step", "0.1"),
			Attr("data-testid", "idle-cash-benchmark"), Attr("aria-label", uistate.T("settings.idleCashBenchmarkLabel")),
			Value(idleCashBenchmarkDisplay(p.Pr.IdleCashBenchmarkAPR)), OnInput(p.OnIdleCashBenchmarkAPR)),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.idleCashBenchmarkHint")),
	)
}

// idleCashBenchmarkDisplay renders the idle-cash benchmark APR for the number input,
// blank when unset (0) so the field shows its placeholder rather than a bare "0".
func idleCashBenchmarkDisplay(apr float64) string {
	if apr <= 0 {
		return ""
	}
	return strconv.FormatFloat(apr, 'f', -1, 64)
}

// settingsAIPane renders the AI tab: the BYOK key, model choice, and the web
// search key. Pure rendering helper — no hooks.
func settingsAIPane(p settingsRightProps) uic.Node {
	return Div(
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
		// The model list is fetched live from OpenAI's /v1/models (when a key is set),
		// so new models appear without a code change; it falls back to the built-in
		// defaults offline / before it loads. Reload refetches on demand.
		Div(css.Class("set-model-row", tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt045),
			Select(css.Class("set-input"), Attr("aria-label", uistate.T("settings.aiModel")), Title(uistate.T("settings.aiModel")), OnChange(p.OnModel),
				MapKeyed(modelIDList(p.Models, p.CurModel),
					func(m string) any { return m },
					func(m string) uic.Node {
						return Option(Value(m), SelectedIf(p.CurModel == m || (p.CurModel == "" && m == "gpt-5.4-mini")), aiModelDisplayName(m))
					},
				),
			),
			Button(css.Class("btn btn-tool btn-sm"), Type("button"), Attr("aria-label", uistate.T("settings.aiModelReload")), Title(uistate.T("settings.aiModelReload")), OnClick(p.OnReloadModels),
				If(p.ModelsLoading, Span(uistate.T("settings.aiModelLoading"))),
				If(!p.ModelsLoading, Span(uistate.T("settings.aiModelReload"))),
			),
		),
		// C250: surface the active model and BYOK billing transparency so users know
		// which model is active and that they pay OpenAI directly per token used.
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiModelNote", aiModelDisplayName(p.CurModel))),
		If(strings.TrimSpace(p.ModelsErr) != "", P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiModelLoadFailed"))),
		If(len(p.Models) > 0, P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiModelLive", len(p.Models)))),
		// AG18: point the app at any OpenAI-compatible endpoint (a local model or a
		// proxy) — the honest "no key leaves the house" path. Blank = OpenAI direct.
		H4(css.Class("set-label"), uistate.T("settings.aiBaseUrlTitle")),
		Input(css.Class("set-input", tw.Mt045), Type("text"), Attr("spellcheck", "false"),
			Attr("aria-label", uistate.T("settings.aiBaseUrlTitle")), Attr("data-testid", "settings-ai-base-url"),
			Placeholder(uistate.T("settings.aiBaseUrlPlaceholder")), Value(p.BaseURL), OnInput(p.OnBaseURL)),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiBaseUrlHint")),
		If(aiprovider.IsLocalEndpoint(p.BaseURL), P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiBaseUrlLocal"))),
		// AG19: the assistant's transparent, editable memory.
		uic.CreateElement(agentMemoryForm),
		H4(css.Class("set-label"), uistate.T("settings.webSearchTitle")),
		Input(css.Class("set-input", tw.Mt045), Type("password"), Attr("aria-label", uistate.T("settings.webSearchKeyPlaceholder")), Placeholder(uistate.T("settings.webSearchKeyPlaceholder")), Value(p.WsKey), OnInput(p.OnWsKey)),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.webSearchHint")),
	)
}

// settingsCloudPane renders the Cloud tab: backend connection, auth, sync,
// conflict recovery, and the subscription surface. Pure rendering helper.
func settingsCloudPane(p settingsRightProps) uic.Node {
	return Div(
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
		ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("settings.backendToggle"), On: p.BackendOn, OnChange: p.OnBackendToggle}),
		If(!p.BackendOn, P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.backendOffHint"))),
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
			// Test/Sync/Upload each open a real connection to the saved server URL, so
			// they only appear while the backend is switched on — otherwise clicking
			// them fires a network request the "Backend off — fully local" copy just
			// promised wouldn't happen.
			If(p.BackendOn, Button(css.Class("btn"), Type("button"), OnClick(p.OnTestBackend), uistate.T("settings.testBackend"))),
			If(p.BackendOn, Button(css.Class("btn"), Type("button"), OnClick(p.OnSyncNow), uistate.T("settings.syncNow"))),
			If(p.BackendOn, Button(css.Class("btn"), Type("button"), OnClick(p.OnUploadKey), uistate.T("settings.uploadKey"))),
			A(css.Class("btn"), Attr("href", "https://github.com/monstercameron/CashFlux/blob/main/docs/SELF_HOSTING.md"), Attr("target", "_blank"), Attr("rel", "noreferrer"), uistate.T("settings.deploySelfHost")),
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
	)
}

// settingsDataPane renders the Data tab: export/import/backup actions, the
// backup cadence, and workspaces. Pure rendering helper — no hooks.
func settingsDataPane(p settingsRightProps) uic.Node {
	return Div(
		H4(css.Class("set-label"), uistate.T("settings.data")),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
			dataBtn(uistate.T("settings.exportJSON"), false, p.OnExportJSON),
			dataBtn(uistate.T("settings.exportCSV"), false, p.OnExportCSV),
			dataBtn(uistate.T("settings.backupAll"), false, p.OnBackupAll),
			dataBtn(uistate.T("settings.importDataset"), false, p.OnImportJSON),
			dataBtn(uistate.T("settings.loadSample"), false, p.OnLoadSample),
			dataBtn(uistate.T("settings.resetSample"), false, p.OnResetSample),
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

		ui.Divider(),
		// AC16: the estate emergency pack — a plain-language document, generated on
		// this device, for a spouse or executor who needs to step in. Framed with care
		// and an explicit no-passwords guarantee before the action, and confirmed
		// (p.OnExportPack) before anything is generated.
		H4(css.Class("set-label"), uistate.T("settings.emergencyPackTitle")),
		P(css.Class("muted", tw.TextXs), uistate.T("settings.emergencyPackHint")),
		Button(css.Class("btn", tw.Mt045), Type("button"), Attr("data-testid", "export-emergency-pack"),
			OnClick(p.OnExportPack), uistate.T("settings.emergencyPackBtn")),

		ui.Divider(),
		H4(css.Class("set-label"), uistate.T("ws.section")),
		P(css.Class("muted", tw.TextXs), uistate.T("ws.sectionHint")),
		workspacesSection(p.Bump),
	)
}

// settingsAdvancedPane renders the Advanced tab: app lock and languages. Pure
// rendering helper — no hooks (the debug log + about line are appended by the
// caller, which owns their state).
func settingsAdvancedPane(p settingsRightProps) uic.Node {
	return Div(
		H4(css.Class("set-label"), uistate.T("applock.section")),
		P(css.Class("muted", tw.TextXs), uistate.T("applock.sectionHint")),
		appLockSection(p.Bump),
		H4(css.Class("set-label"), uistate.T("settings.languages")),
		// The display-language picker only earns its place once a second language
		// is installed — with just English it's a one-option no-op. Import a
		// translation file (below) to unlock it.
		If(p.LangCount > 1,
			Select(css.Class("set-input"), Attr("aria-label", uistate.T("settings.language")), Title(uistate.T("settings.language")), OnChange(p.OnLang), p.LangOptions),
		),
		If(p.LangCount <= 1, P(css.Class("muted", tw.TextXs), uistate.T("settings.languageSingleHint"))),
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Py1),
			dataBtn(uistate.T("settings.exportLangs"), false, p.OnExportLangs),
			dataBtn(uistate.T("settings.importLangs"), false, p.OnImportLangs),
		),
	)
}

// monthlyIncomeInputProps configures the monthly-income text field. ValueMinor
// is the persisted value in minor units (0 = unset); OnChange receives the raw
// string the user typed, which the caller converts and persists.
type monthlyIncomeInputProps struct {
	ValueMinor int64
	OnChange   func(string)
}

// monthlyIncomeInput is the "Monthly income" entry field (C22). It is its own
// component so the OnInput hook sits at a stable position — not inside a loop.
// The field shows a major-unit decimal (e.g. "5000.00" for 500000 cents); the
// caller is responsible for parsing and persisting the value.
func monthlyIncomeInput(props monthlyIncomeInputProps) uic.Node {
	// Derive the display value: major units as a plain decimal string, or blank
	// when unset (0) so the placeholder shows instead of a distracting "0.00".
	displayVal := ""
	if props.ValueMinor > 0 {
		// USD-style: divide by 100. Use the generic 2-decimal representation for
		// all currencies since this is a display default; exact decimal places are
		// calibrated on the parse side where the base currency is available.
		displayVal = strconv.FormatFloat(float64(props.ValueMinor)/100, 'f', 2, 64)
	}
	on := uic.UseEvent(func(v string) {
		if props.OnChange != nil {
			props.OnChange(v)
		}
	})
	return Input(
		css.Class("set-input", tw.Mt045),
		Type("text"),
		Attr("inputmode", "decimal"),
		Attr("aria-label", uistate.T("settings.monthlyIncome")),
		Attr("placeholder", uistate.T("settings.monthlyIncomePlaceholder")),
		Value(displayVal),
		OnInput(on),
	)
}
