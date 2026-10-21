// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aiprovider"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/backup"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
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
		uic.CreateElement(settingsDefaultAccountControl, struct{}{}),
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

func settingsDefaultAccountControl(_ struct{}) uic.Node {
	app := appstate.Default
	if app == nil {
		return uic.Fragment()
	}
	selected := app.DefaultAccountID()
	options := []any{Option(Value(""), SelectedIf(selected == ""), uistate.T("defaultAccount.none"))}
	for _, account := range app.Accounts() {
		if account.Archived && account.ID != selected {
			continue
		}
		label := account.Name
		if account.Archived {
			label += " — " + uistate.T("accounts.archived")
		}
		options = append(options, Option(Value(account.ID), SelectedIf(selected == account.ID), label))
	}
	onChange := uic.UseEvent(func(id string) {
		var err error
		if strings.TrimSpace(id) == "" {
			err = app.ClearDefaultAccountID()
		} else {
			err = app.SetDefaultAccountID(id)
		}
		if err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		if strings.TrimSpace(id) == "" {
			uistate.PostNotice(uistate.T("defaultAccount.householdCleared"), false)
		} else {
			uistate.PostNotice(uistate.T("defaultAccount.householdSet"), false)
		}
	})
	return Div(css.Class("set-default-account"),
		H4(css.Class("set-label"), uistate.T("defaultAccount.householdLabel")),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("defaultAccount.householdHint")),
		Select(css.Class("set-input"), Attr("data-testid", "settings-default-account"), Attr("aria-label", uistate.T("defaultAccount.householdLabel")), OnChange(onChange), options),
	)
}

// settingsAlertsPane renders the Alerts tab: freshness windows plus the
// notifications and per-alert controls. The auto-category suggestion threshold
// moved to Preferences (it's a transactions-behavior control) and background
// music moved to Appearance (an experience setting) so Alerts stops being a
// catch-all bucket (#3). Pure rendering helper — no hooks of its own (the
// embedded components own theirs).
func settingsAlertsPane(p settingsLeftProps) uic.Node {
	return Div(
		H4(css.Class("set-label"), uistate.T("settings.freshnessTitle")),
		P(css.Class(tw.TextFaint, tw.Text12), uistate.T("settings.freshnessHint")),
		Div(p.FreshnessRows),
		uic.CreateElement(notifySettings),
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
	AiOn             bool
	OnAiToggle       func(bool)
	AiKey            string
	OnKey            func(string) // the field owns the hook (see uiw.fieldCore)
	OnRememberKey    func(bool)
	OnServerAIRoute  func(bool)
	OnModel          uic.Handler // UseEvent
	CurModel         string
	Models           []string // model ids from the selected AI route (empty = built-in defaults)
	ModelsLoaded     bool
	ModelQuery       string
	OnModelQuery     func(string)
	OnUseCustomModel uic.Handler
	OnReloadModels   uic.Handler // UseEvent — refetch the model list
	ModelsLoading    bool
	ModelsErr        string
	WsKey            string
	OnWsKey          func(string)
	// AG18 — OpenAI-compatible base-URL override (Ollama/LM Studio/proxy).
	BaseURL   string
	OnBaseURL func(string)
	// Cloud & server: owned entirely by CloudConnectionPane (cloudtab.go) now —
	// see settingsCloudPane's doc comment.
	// Data
	OnExportJSON func()
	OnExportCSV  func()
	// OnExportSQLite/OnImportSQLite move the whole household as a real SQLite
	// database file — the export for when the destination is not this app.
	OnExportSQLite  func()
	OnImportSQLite  func()
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
				return ai.SortModelIDs(ids, cur)
			}
		}
		ids = append([]string{cur}, ids...)
	}
	return ai.SortModelIDs(ids, cur)
}

func filterModelIDs(models []string, query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return models
	}
	filtered := make([]string, 0, len(models))
	for _, model := range models {
		if strings.Contains(strings.ToLower(model), query) {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func modelGroupLabel(model string) string {
	switch ai.GroupModelID(model) {
	case ai.ModelGroupReasoning:
		return uistate.T("settings.modelGroupReasoning")
	case ai.ModelGroupChat:
		return uistate.T("settings.modelGroupChat")
	default:
		return uistate.T("settings.modelGroupOther")
	}
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
			Attr("data-testid", "idle-cash-benchmark"), Attr("aria-label", uistate.T("settings.idleCashBenchmarkLabel")), OnInput(p.OnIdleCashBenchmarkAPR), ui.FieldValue(idleCashBenchmarkDisplay(p.Pr.IdleCashBenchmarkAPR))),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.idleCashBenchmarkHint")),
		// Transactions behavior: how many payee→category corrections must accumulate
		// before Quick-Add offers a suggestion. Moved here from Alerts (#3) — it's a
		// transactions preference, not an alert.
		ui.Divider(),
		uic.CreateElement(learnThresholdRow),
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
	serverAI := uistate.BackendAIActive(p.Pr)
	serverAvailable := uistate.BackendConnectionActive(p.Pr)
	routeText := func(serverKey, directKey string, args ...any) string {
		if serverAI {
			return uistate.T(serverKey, args...)
		}
		return uistate.T(directKey, args...)
	}
	return Div(
		If(serverAvailable, Fragment(
			ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("settings.aiUseServer"), On: serverAI, OnChange: p.OnServerAIRoute}),
			P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiRouteHint")),
		)),
		H4(css.Class("set-label"), routeText("settings.aiServerTitle", "settings.aiTitle")),
		IfElse(serverAI,
			Fragment(
				P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiServerExplainer")),
				P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiServerDestination", backendHost(p.Pr.ServerURL))),
				uic.CreateElement(serverAIKeySettings),
			),
			Fragment(
				ui.TextInput(ui.TextFieldProps{Class: "set-input", Value: p.AiKey, OnInput: p.OnKey,
					AriaLabel: uistate.T("settings.aiKeyPlaceholder"), Placeholder: uistate.T("settings.aiKeyPlaceholder"),
					Rules: []any{tw.Mt045}, Extra: []any{Type("password"), Attr("data-testid", "settings-ai-device-key")}}),
				P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiKeyTrust")),
				P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiKeyExplainer")),
				If(strings.TrimSpace(p.AiKey) == "", P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiNoKey"))),
				ui.ToggleRow(ui.ToggleRowProps{Label: uistate.T("settings.rememberKey"), On: p.Pr.RememberAIKey, OnChange: p.OnRememberKey}),
				P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.rememberKeyNote")),
			),
		),
		// The list comes from the selected route. Search only filters what is
		// shown, and a typed id can still be selected as a custom model.
		ui.TextInput(ui.TextFieldProps{Class: "set-input", Value: p.ModelQuery, OnInput: p.OnModelQuery,
			AriaLabel: uistate.T("settings.aiModelSearch"), Placeholder: uistate.T("settings.aiModelSearch"),
			Extra: []any{Attr("data-testid", "settings-ai-model-search"), Attr("spellcheck", "false")}}),
		Div(css.Class("set-model-row", tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt045),
			Select(css.Class("set-input"), Attr("aria-label", uistate.T("settings.aiModel")), Attr("data-testid", "settings-ai-model"), Title(uistate.T("settings.aiModel")), OnChange(p.OnModel),
				MapKeyed(filterModelIDs(modelIDList(p.Models, p.CurModel), p.ModelQuery),
					func(m string) any { return m },
					func(m string) uic.Node {
						label := modelGroupLabel(m) + " · " + aiModelDisplayName(m)
						if p.ModelsLoaded && !containsModelID(p.Models, m) {
							label += " (" + uistate.T("settings.aiModelNotLatest") + ")"
						}
						return Option(Value(m), SelectedIf(p.CurModel == m || (p.CurModel == "" && m == "gpt-5.4-mini")), label)
					},
				),
			),
			Button(css.Class("btn btn-tool btn-sm"), Type("button"), Attr("data-testid", "settings-ai-model-reload"), Attr("aria-label", uistate.T("settings.aiModelReload")), Title(uistate.T("settings.aiModelReload")), OnClick(p.OnReloadModels),
				If(p.ModelsLoading, Span(uistate.T("settings.aiModelLoading"))),
				If(!p.ModelsLoading, Span(uistate.T("settings.aiModelReload"))),
			),
		),
		If(strings.TrimSpace(p.ModelQuery) != "", Button(css.Class("btn btn-tool btn-sm", tw.Mt1), Type("button"), Attr("data-testid", "settings-ai-model-use-custom"), OnClick(p.OnUseCustomModel), uistate.T("settings.aiModelUseCustom", strings.TrimSpace(p.ModelQuery)))),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiModelKindsHint")),
		// Describe the active model and which route will handle its requests.
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), routeText("settings.aiServerModelNote", "settings.aiModelNote", aiModelDisplayName(p.CurModel))),
		If(strings.TrimSpace(p.ModelsErr) != "", P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), routeText("settings.aiServerModelLoadFailed", "settings.aiModelLoadFailed"))),
		If(len(p.Models) > 0, P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), routeText("settings.aiServerModelLive", "settings.aiModelLive", len(p.Models)))),
		// AG18: point the app at any OpenAI-compatible endpoint (a local model or a
		// proxy) — the honest "no key leaves the house" path. Blank = OpenAI direct.
		If(!serverAI, Fragment(
			H4(css.Class("set-label"), uistate.T("settings.aiBaseUrlTitle")),
			ui.TextInput(ui.TextFieldProps{Class: "set-input", Rules: []any{tw.Mt045}, Value: p.BaseURL, OnInput: p.OnBaseURL,
				AriaLabel: uistate.T("settings.aiBaseUrlTitle"), Placeholder: uistate.T("settings.aiBaseUrlPlaceholder"),
				Extra: []any{Attr("spellcheck", "false"), Attr("data-testid", "settings-ai-base-url")}}),
			P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiBaseUrlHint")),
			If(aiprovider.IsLocalEndpoint(p.BaseURL), P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.aiBaseUrlLocal"))),
		)),
		// AG19: the assistant's transparent, editable memory.
		uic.CreateElement(agentMemoryForm),
		H4(css.Class("set-label"), uistate.T("settings.webSearchTitle")),
		ui.TextInput(ui.TextFieldProps{Class: "set-input", Value: p.WsKey, OnInput: p.OnWsKey,
			AriaLabel: uistate.T("settings.webSearchKeyPlaceholder"), Placeholder: uistate.T("settings.webSearchKeyPlaceholder"),
			Rules: []any{tw.Mt045}, Extra: []any{Type("password")}}),
		P(css.Class(tw.TextFaint, tw.Text12, tw.Mt1), uistate.T("settings.webSearchHint")),
	)
}

func containsModelID(models []string, id string) bool {
	for _, model := range models {
		if model == id {
			return true
		}
	}
	return false
}

// settingsCloudPane renders the Cloud tab. It is a thin wrapper around
// CloudConnectionPane (cloudtab.go) — a self-contained component like
// PasswordAuthCard/DeviceLinkCard, not a props-driven pane — since the old
// /sync page and this tab used to be two independently-drifted
// implementations of the same "connect a backend" feature (2026-07-24
// unification). Takes no props: every field here is the pane's own concern.
func settingsCloudPane() uic.Node {
	return uic.CreateElement(CloudConnectionPane)
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
			dataBtn(uistate.T("settings.exportSQLite"), false, p.OnExportSQLite),
			dataBtn(uistate.T("settings.importSQLite"), false, p.OnImportSQLite),
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
		// #55: the pre-operation safety-checkpoint ring — snapshots taken
		// automatically before imports and bulk actions, restorable in one click.
		checkpointsSection(),

		ui.Divider(),
		// #53: the local integrity check — cross-checks the dataset against
		// itself and lists any inconsistencies with drill-throughs.
		dataHealthSection(),

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
		OnInput(on), ui.FieldValue(displayVal))
}
