// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcatalog"
)

const assistantFormulaLifetime = 5 * time.Minute

var assistantFormulaNow = time.Now
var assistantFormulaCache = newViewCache[*assistantFormulaSnapshot](2)

type assistantFormulaContext struct {
	ObservedAt   time.Time `json:"observed_at"`
	BaseCurrency string    `json:"base_currency"`
	Scope        string    `json:"scope"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodBefore time.Time `json:"period_before"`
	Period       string    `json:"period"`
	Units        string    `json:"units_note"`
}

type assistantFormulaMetric struct {
	Name      string   `json:"name"`
	Label     string   `json:"label"`
	Doc       string   `json:"doc"`
	Group     string   `json:"group"`
	Kind      string   `json:"kind"`
	Unit      string   `json:"unit"`
	Formula   string   `json:"formula,omitempty"`
	Available bool     `json:"available"`
	Value     *float64 `json:"value,omitempty"`
}

type assistantFormulaSnapshot struct {
	Context   assistantFormulaContext
	Identity  string
	Nonce     string
	Vars      map[string]float64
	Molecules []domain.Molecule
	Metrics   []assistantFormulaMetric
	Err       error
}

type assistantFormulaToken struct {
	Identity string    `json:"identity"`
	Observed time.Time `json:"observed"`
	Nonce    string    `json:"nonce"`
	Select   string    `json:"selection"`
}

type assistantFormulaSelection struct {
	Filter        string `json:"filter"`
	Group         string `json:"group"`
	MoleculesOnly bool   `json:"molecules_only"`
	IncludeValues bool   `json:"include_values"`
}

type assistantFormulaPageArgs struct {
	Filter        string `json:"filter"`
	Group         string `json:"group"`
	MoleculesOnly bool   `json:"molecules_only"`
	IncludeValues bool   `json:"include_values"`
	Offset        int    `json:"offset"`
	Limit         int    `json:"limit"`
	Snapshot      string `json:"snapshot"`
}

func assistantFormulaDigest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("formula context cannot be identified: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func assistantFormulaFailure(err error, restart bool) string {
	out, _ := json.Marshal(struct {
		Error     string `json:"error"`
		Available bool   `json:"available"`
		Restart   bool   `json:"restart_required"`
	}{Error: err.Error(), Restart: restart})
	return string(out)
}

func assistantFormulaEncodeToken(snapshot *assistantFormulaSnapshot, selection string) string {
	encoded, _ := json.Marshal(assistantFormulaToken{snapshot.Identity, snapshot.Context.ObservedAt, snapshot.Nonce, selection})
	return base64.RawURLEncoding.EncodeToString(encoded)
}

// assistantFormulaInputs reads configuration from the bound dataset, not the
// globally active App. Parsers/defaults are shared with the existing pages.
func assistantFormulaInputs(app *appstate.App, now time.Time) (liveEngineVarInputs, error) {
	s := app.Store()
	read := func(key string, settings bool) (string, error) {
		var raw string
		var err error
		if settings {
			raw, _, err = s.GetSettingKV(key)
		} else {
			raw, _, err = s.GetKV(key)
		}
		if err != nil {
			return "", fmt.Errorf("read formula configuration %s: %w", key, err)
		}
		return raw, nil
	}
	object := func(key string, settings bool) (string, error) {
		raw, err := read(key, settings)
		if err != nil {
			return "", err
		}
		if raw != "" && (strings.TrimSpace(raw) == "" || strings.TrimSpace(raw)[0] != '{') {
			return "", fmt.Errorf("formula configuration %s must be a JSON object", key)
		}
		return raw, nil
	}
	if _, err := object("cashflux:prefs", true); err != nil {
		return liveEngineVarInputs{}, err
	}
	prefs, _, err := assistantDataPreferences(s)
	if err != nil {
		return liveEngineVarInputs{}, err
	}
	allocRaw, err := object("cashflux:allocate:config", false)
	if err != nil {
		return liveEngineVarInputs{}, err
	}
	alloc, err := uistate.ParseAllocConfig(allocRaw)
	if err != nil {
		return liveEngineVarInputs{}, fmt.Errorf("formula allocation configuration: %w", err)
	}
	planningRaw, err := object("cashflux:planning:config", false)
	if err != nil {
		return liveEngineVarInputs{}, err
	}
	planning, err := uistate.ParsePlanningConfig(planningRaw)
	if err != nil {
		return liveEngineVarInputs{}, fmt.Errorf("formula planning configuration: %w", err)
	}
	billsRaw, err := object("cashflux:bills:smartconfig", false)
	if err != nil {
		return liveEngineVarInputs{}, err
	}
	bills, err := uistate.ParseBillsSmartConfig(billsRaw)
	if err != nil {
		return liveEngineVarInputs{}, fmt.Errorf("formula bill schedule configuration: %w", err)
	}
	poolsRaw, err := read("cashflux:invest:pools", false)
	if err != nil {
		return liveEngineVarInputs{}, err
	}
	var pools []engineenv.PoolDef
	if poolsRaw != "" {
		trimmed := strings.TrimSpace(poolsRaw)
		if trimmed == "" || trimmed[0] != '[' {
			return liveEngineVarInputs{}, fmt.Errorf("formula investment pools must be a JSON array")
		}
		var entries []json.RawMessage
		if err := json.Unmarshal([]byte(poolsRaw), &entries); err != nil {
			return liveEngineVarInputs{}, fmt.Errorf("formula investment pools: %w", err)
		}
		for _, entry := range entries {
			trimmed := strings.TrimSpace(string(entry))
			if trimmed == "" || trimmed[0] != '{' {
				return liveEngineVarInputs{}, fmt.Errorf("formula investment pool must be a JSON object")
			}
			var pool uistate.InvestPool
			if err := json.Unmarshal(entry, &pool); err != nil {
				return liveEngineVarInputs{}, fmt.Errorf("formula investment pool: %w", err)
			}
			pools = append(pools, engineenv.PoolDef{Name: pool.Name, AccountIDs: append([]string(nil), pool.AccountIDs...)})
		}
	}
	smartRaw, err := object("cashflux:smart-settings", true)
	if err != nil {
		return liveEngineVarInputs{}, err
	}
	smartSettings := smart.EnableFreeOnly(smart.Settings{})
	if smartRaw != "" {
		smartSettings = smart.Settings{}
		if err := json.Unmarshal([]byte(smartRaw), &smartSettings); err != nil {
			return liveEngineVarInputs{}, fmt.Errorf("formula Smart settings: %w", err)
		}
		smartSettings = smart.Migrate(smartSettings)
	}
	var counts engineenv.SmartCounts
	for _, code := range smartSettings.EnabledCodes() {
		switch {
		case smartengine.HasEngine(code):
			counts.FreeOn++
		case smartai.Implemented(code):
			counts.AIOn++
		}
	}
	return liveEngineVarInputs{Now: now, WeekStart: prefs.WeekStartWeekday(), PayCycleAnchor: prefs.PayCycleAnchor,
		IdleBenchmark: prefs.IdleCashBenchmarkAPR, Alloc: engineenv.AllocData{AmountMinor: alloc.AmountMinor, ReserveMinor: alloc.ReserveMinor, MaxPerMinor: alloc.MaxPerMinor},
		Planning:    engineenv.PlanningData{RunwayBufferMinor: planning.RunwayBufferMinor, RunwayDays: planning.RunwayDays, ForecastMonths: planning.ForecastMonths},
		BillsConfig: bills, Pools: pools, Smart: counts}, nil
}

// assistantFormulaMetrics uses the shared catalog builders with the same bound
// pool definitions the calculation receives. No active-App UI getters are used.
func assistantFormulaMetrics(app *appstate.App, inputs liveEngineVarInputs) []widgetcatalog.Metric {
	metrics := widgetcatalog.Metrics(app.CustomFieldDefs(), app.Molecules())
	metrics = append(metrics, widgetcatalog.BudgetMetrics(app.Budgets())...)
	metrics = append(metrics, widgetcatalog.AccountMetrics(app.Accounts())...)
	metrics = append(metrics, widgetcatalog.GoalMetrics(app.Goals())...)
	metrics = append(metrics, widgetcatalog.DebtMetrics(app.Accounts())...)
	metrics = append(metrics, widgetcatalog.PoolMetrics(inputs.Pools)...)
	metrics = append(metrics, widgetcatalog.GroupMetrics(liveGroupDefs(app))...)
	metrics = append(metrics, widgetcatalog.EventMetrics(liveEventDefs(app))...)
	metrics = append(metrics, widgetcatalog.AllocMetrics()...)
	metrics = append(metrics, widgetcatalog.PlanningMetrics(app.Plans())...)
	metrics = append(metrics, widgetcatalog.RecurringMetrics(app.Recurring())...)
	metrics = append(metrics, widgetcatalog.ReportsMetrics()...)
	metrics = append(metrics, widgetcatalog.NetWorthMetrics()...)
	metrics = append(metrics, widgetcatalog.HealthMetrics()...)
	metrics = append(metrics, widgetcatalog.CreditMetrics()...)
	metrics = append(metrics, widgetcatalog.BillsSmartMetrics()...)
	metrics = append(metrics, widgetcatalog.AssistantMetrics()...)
	metrics = append(metrics, widgetcatalog.SmartMetrics()...)
	return metrics
}

// assistantFormulaPreflight checks the same persisted collections consumed by
// the canonical live engine. App accessors log errors and return empty slices;
// a tool must not turn those errors into a zero-valued financial environment.
func assistantFormulaPreflight(app *appstate.App) error {
	s := app.Store()
	checks := []struct {
		name string
		read func() error
	}{
		{"accounts", func() error { _, e := s.ListAccounts(); return e }},
		{"transactions", func() error { _, e := s.ListTransactions(); return e }},
		{"transaction links", func() error { _, e := s.ListTxnLinks(); return e }},
		{"members", func() error { _, e := s.ListMembers(); return e }},
		{"categories", func() error { _, e := app.CategoriesChecked(); return e }},
		{"budgets", func() error { _, e := s.ListBudgets(); return e }},
		{"goals", func() error { _, e := s.ListGoals(); return e }},
		{"tasks", func() error { _, e := s.ListTasks(); return e }},
		{"recurring", func() error { _, e := s.ListRecurring(); return e }},
		{"plans", func() error { _, e := s.ListPlans(); return e }},
		{"account groups", func() error { _, e := s.ListAccountGroups(); return e }},
		{"events", func() error { _, e := s.ListEvents(); return e }},
	}
	for _, check := range checks {
		if err := check.read(); err != nil {
			return fmt.Errorf("read formula %s: %w", check.name, err)
		}
	}
	return nil
}

func assistantFormulaSnapshotFor(app *appstate.App, tokenText string) (*assistantFormulaSnapshot, assistantFormulaToken, error) {
	var token assistantFormulaToken
	now := assistantFormulaNow()
	asOf := now
	if tokenText != "" {
		if len(tokenText) > 2048 {
			return nil, token, fmt.Errorf("invalid formula snapshot; restart at offset 0 without snapshot")
		}
		encoded, err := base64.RawURLEncoding.DecodeString(tokenText)
		if err != nil || json.Unmarshal(encoded, &token) != nil || token.Identity == "" || token.Nonce == "" || token.Observed.IsZero() {
			return nil, token, fmt.Errorf("invalid formula snapshot; restart at offset 0 without snapshot")
		}
		asOf = token.Observed
		age := now.Sub(asOf)
		currentDate := now.In(asOf.Location())
		if age < 0 || age > assistantFormulaLifetime || currentDate.Year() != asOf.Year() || currentDate.YearDay() != asOf.YearDay() {
			return nil, token, fmt.Errorf("formula snapshot expired; restart at offset 0 without snapshot")
		}
	}
	if app == nil || app.Store() == nil {
		return nil, token, fmt.Errorf("formula dataset is unavailable")
	}
	// Always perform fresh fallible metadata reads, including on a warm cache.
	settings, err := app.Store().GetSettings()
	if err != nil {
		return nil, token, fmt.Errorf("read formula settings: %w", err)
	}
	if _, err := app.Store().ListMolecules(); err != nil {
		return nil, token, fmt.Errorf("read formula molecules: %w", err)
	}
	if _, err := app.Store().ListCustomFieldDefs(); err != nil {
		return nil, token, fmt.Errorf("read formula custom fields: %w", err)
	}
	inputs, err := assistantFormulaInputs(app, asOf)
	if err != nil {
		return nil, token, err
	}
	identityInputs := inputs
	identityInputs.Now = asOf.Truncate(time.Minute)
	revision := revKey(app)
	identity, err := assistantFormulaDigest(struct {
		Revision string
		Inputs   liveEngineVarInputs
	}{revision, identityInputs})
	if err != nil {
		return nil, token, err
	}
	if tokenText != "" && token.Identity != identity {
		return nil, token, fmt.Errorf("formula inputs changed; restart at offset 0 without snapshot")
	}
	key := revision + "|assistant-formulas|" + identity
	snapshot := assistantFormulaCache.GetOrCompute(key, func() *assistantFormulaSnapshot {
		result := &assistantFormulaSnapshot{Identity: identity, Nonce: id.New()}
		if err := assistantFormulaPreflight(app); err != nil {
			result.Err = err
			return result
		}
		base := settings.BaseCurrency
		if base == "" {
			base = "USD"
		}
		start, before := dateutil.MonthRange(asOf)
		result.Context = assistantFormulaContext{ObservedAt: asOf, BaseCurrency: base, Scope: "whole household",
			PeriodStart: start, PeriodBefore: before, Period: "current calendar month at observed_at",
			Units: "Monetary engine values use major units of base_currency. Counts, ratios, percentages and custom formulas retain their own units; unspecified units are not inferred."}
		result.Vars = make(map[string]float64)
		for name, value := range liveEngineVarsWithInputs(app, inputs) {
			result.Vars[name] = value
		}
		result.Molecules = append([]domain.Molecule(nil), app.Molecules()...)
		result.Metrics, result.Molecules = assistantFormulaCatalog(assistantFormulaMetrics(app, inputs), result.Molecules, result.Vars)
		if revKey(app) != revision {
			result.Err = fmt.Errorf("formula data changed while reading; restart at offset 0 without snapshot")
		}
		return result
	})
	if snapshot.Err != nil {
		assistantFormulaCache.Clear() // Do not retain a transient failed read.
		return nil, token, snapshot.Err
	}
	if tokenText != "" && snapshot.Nonce != token.Nonce {
		return nil, token, fmt.Errorf("formula snapshot was evicted; restart at offset 0 without snapshot")
	}
	return snapshot, token, nil
}

// assistantFormulaCatalog preserves catalog metadata, adds custom molecules and
// uncatalogued engine names, and resolves duplicate entries by stable name.
func assistantFormulaCatalog(metrics []widgetcatalog.Metric, molecules []domain.Molecule, vars map[string]float64) ([]assistantFormulaMetric, []domain.Molecule) {
	byName := map[string]widgetcatalog.Metric{}
	defaultMolecules := map[string]domain.Molecule{}
	for _, molecule := range engineenv.DefaultMolecules() {
		defaultMolecules[molecule.Name] = molecule
	}
	reserved := map[string]bool{}
	builtin := map[string]bool{}
	for _, name := range engineenv.Names {
		builtin[name] = true
		if _, molecule := defaultMolecules[name]; !molecule {
			reserved[name] = true
		}
	}
	for _, metric := range metrics {
		if _, defaultMolecule := defaultMolecules[metric.Name]; !metric.Molecule && !defaultMolecule {
			reserved[metric.Name] = true
		}
		byName[metric.Name] = metric
	}
	var effective []domain.Molecule
	for _, molecule := range molecules {
		if reserved[molecule.Name] {
			metric := byName[molecule.Name]
			metric.Molecule, metric.Formula = false, ""
			byName[molecule.Name] = metric
			continue
		}
		effective = append(effective, molecule)
		metric, exists := byName[molecule.Name]
		if !exists {
			metric = widgetcatalog.Metric{Name: molecule.Name, Label: strings.ReplaceAll(molecule.Name, "_", " "), Group: widgetcatalog.GroupCustom}
		}
		metric.Molecule, metric.Formula = true, molecule.Formula
		if molecule.Doc != "" {
			metric.Doc = molecule.Doc
		}
		byName[molecule.Name] = metric
	}
	for name := range vars {
		if _, exists := byName[name]; !exists {
			byName[name] = widgetcatalog.Metric{Name: name, Label: strings.ReplaceAll(name, "_", " "), Group: widgetcatalog.GroupActivity}
		}
	}
	rows := make([]assistantFormulaMetric, 0, len(byName))
	for _, metric := range byName {
		kind := "atom"
		if metric.Molecule {
			kind = "molecule"
		} else if strings.HasPrefix(metric.Name, "cf_") {
			kind = "custom_field"
		} else if !builtin[metric.Name] && (strings.HasPrefix(metric.Name, "account_") || strings.HasPrefix(metric.Name, "budget_") || strings.HasPrefix(metric.Name, "goal_") ||
			strings.HasPrefix(metric.Name, "debt_") || strings.HasPrefix(metric.Name, "pool_") || strings.HasPrefix(metric.Name, "group_") ||
			strings.HasPrefix(metric.Name, "event_") || strings.HasPrefix(metric.Name, "plan_") || strings.HasPrefix(metric.Name, "recurring_")) {
			kind = "entity"
		}
		if metric.Doc == "" {
			metric.Doc = "Canonical engine variable; units and availability are reported separately."
		}
		row := assistantFormulaMetric{Name: metric.Name, Label: metric.Label, Doc: metric.Doc, Group: string(metric.Group), Kind: kind,
			Unit: assistantFormulaUnit(metric, defaultMolecules), Formula: metric.Formula}
		if value, exists := vars[metric.Name]; exists && !math.IsNaN(value) && !math.IsInf(value, 0) {
			row.Available = true
			copy := value
			row.Value = &copy
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows, effective
}

func assistantFormulaUnit(metric widgetcatalog.Metric, defaults map[string]domain.Molecule) string {
	if metric.Molecule {
		original, known := defaults[metric.Name]
		if !known || original.Formula != metric.Formula {
			return "unspecified"
		}
	}
	if metric.Group == widgetcatalog.GroupCounts || strings.HasSuffix(metric.Name, "_count") {
		return "count"
	}
	if strings.HasPrefix(metric.Name, "cf_") {
		return "custom_number"
	}
	switch metric.Name {
	case "assets", "liabilities", "liquid_cash", "income", "expense", "bills_due", "goal_needs", "earmarked_total", "net_worth", "cashflow_net",
		"safe_to_spend", "unreserved_cash", "daily_allowance", "remaining_discretionary", "revolving_balance", "credit_limit_total", "min_payments_total", "interest_drag_monthly":
		return "base_currency_major"
	case "savings_rate", "credit_utilization", "debt_to_asset_pct", "idle_cash_benchmark":
		return "percent"
	case "days_left", "cash_runway_days", "runway_days":
		return "days"
	}
	if strings.HasPrefix(metric.Name, "budget_") {
		if strings.HasSuffix(metric.Name, "_percent") {
			return "percent"
		}
		return "base_currency_major"
	}
	if strings.HasPrefix(metric.Name, "account_") || strings.HasPrefix(metric.Name, "pool_") || strings.HasPrefix(metric.Name, "group_") {
		return "base_currency_major"
	}
	return "unspecified"
}

// agToolsFormula exposes one coherent canonical engine observation as paginated
// metadata and exact evaluations, independent of helper reconstruction per turn.
func agToolsFormula(app *appstate.App, _ string, _ currency.Rates) []chatTool {
	return []chatTool{
		{spec: ai.FunctionTool("list_formula_metrics",
			"Discover ALL canonical formula metric names, labels, groups, docs, units, formulas and availability. Metadata only by default; set include_values for observed numeric values. Whole household, current calendar month at fixed observed_at, base-currency major units for money. Stable name-order pagination: limit defaults50/max100; continue with snapshot and next_offset using identical filters. Changed, expired or evicted snapshots require restarting offset0 without snapshot. Unknown/unavailable metrics never imply zero.",
			json.RawMessage(`{"type":"object","properties":{"filter":{"type":"string"},"group":{"type":"string"},"molecules_only":{"type":"boolean"},"include_values":{"type":"boolean"},"offset":{"type":"integer","minimum":0},"limit":{"type":"integer","minimum":0,"maximum":100},"snapshot":{"type":"string"}},"additionalProperties":false}`)),
			run: func(raw json.RawMessage) string {
				var args assistantFormulaPageArgs
				if err := assistantObjectArgs(raw, &args); err != nil {
					return assistantFormulaFailure(err, false)
				}
				if args.Offset < 0 || args.Limit < 0 || args.Limit > 100 {
					return assistantFormulaFailure(fmt.Errorf("offset must be nonnegative; limit must be 0 through 100"), false)
				}
				if args.Offset > 0 && args.Snapshot == "" {
					return assistantFormulaFailure(fmt.Errorf("continuation requires snapshot; restart at offset 0"), true)
				}
				if args.Limit == 0 {
					args.Limit = 50
				}
				selection := assistantFormulaSelection{strings.ToLower(strings.TrimSpace(args.Filter)), strings.ToLower(strings.TrimSpace(args.Group)), args.MoleculesOnly, args.IncludeValues}
				selectionID, _ := assistantFormulaDigest(selection)
				snapshot, token, err := assistantFormulaSnapshotFor(app, args.Snapshot)
				if err != nil {
					return assistantFormulaFailure(err, args.Snapshot != "")
				}
				if args.Snapshot != "" && token.Select != selectionID {
					return assistantFormulaFailure(fmt.Errorf("formula selection changed; restart at offset 0 without snapshot"), true)
				}
				rows := []assistantFormulaMetric{}
				for _, row := range snapshot.Metrics {
					if args.MoleculesOnly && row.Kind != "molecule" || selection.Group != "" && strings.ToLower(row.Group) != selection.Group ||
						selection.Filter != "" && !strings.Contains(strings.ToLower(row.Name+" "+row.Label+" "+row.Doc+" "+row.Formula), selection.Filter) {
						continue
					}
					if !args.IncludeValues {
						row.Value = nil
					}
					rows = append(rows, row)
				}
				if args.Offset > len(rows) {
					return assistantFormulaFailure(fmt.Errorf("offset exceeds filtered_total"), false)
				}
				end := min(args.Offset+args.Limit, len(rows))
				var next *int
				if end < len(rows) {
					copy := end
					next = &copy
				}
				return assistantDataJSON(struct {
					assistantFormulaContext
					Snapshot      string                   `json:"snapshot"`
					Total         int                      `json:"total"`
					FilteredTotal int                      `json:"filtered_total"`
					Offset        int                      `json:"offset"`
					NextOffset    *int                     `json:"next_offset"`
					Metrics       []assistantFormulaMetric `json:"metrics"`
				}{snapshot.Context, assistantFormulaEncodeToken(snapshot, selectionID), len(snapshot.Metrics), len(rows), args.Offset, next, rows[args.Offset:end]}, nil)
			}},
		{spec: ai.FunctionTool("evaluate_formula", evalFormulaToolDesc()+" Whole-household current calendar month; pass an optional catalog snapshot to use that same fixed observation. Missing or nonfinite referenced values fail explicitly.",
			json.RawMessage(`{"type":"object","properties":{"expression":{"type":"string"},"snapshot":{"type":"string"}},"required":["expression"],"additionalProperties":false}`)),
			run: func(raw json.RawMessage) string {
				var args struct {
					Expression string `json:"expression"`
					Snapshot   string `json:"snapshot"`
				}
				if err := assistantObjectArgs(raw, &args); err != nil {
					return assistantFormulaFailure(err, false)
				}
				expr := strings.TrimSpace(args.Expression)
				if expr == "" {
					return assistantFormulaFailure(fmt.Errorf("provide an expression to evaluate"), false)
				}
				program, err := formula.Compile(expr)
				if err != nil {
					return assistantFormulaFailure(err, false)
				}
				snapshot, token, err := assistantFormulaSnapshotFor(app, args.Snapshot)
				if err != nil {
					return assistantFormulaFailure(err, args.Snapshot != "")
				}
				for _, name := range program.References() {
					value, exists := snapshot.Vars[name]
					if !exists || math.IsNaN(value) || math.IsInf(value, 0) {
						return assistantFormulaFailure(fmt.Errorf("formula variable %q is unavailable; no zero is implied", name), false)
					}
				}
				value, err := program.Eval(formula.Env{Vars: snapshot.Vars})
				if err != nil {
					return assistantFormulaFailure(err, false)
				}
				var derivation *engineenv.Derivation
				if _, exists := snapshot.Vars[expr]; exists {
					if explained, ok := engineenv.Explain(expr, snapshot.Vars, snapshot.Molecules); ok {
						derivation = &explained
					}
				}
				selection := token.Select
				if selection == "" {
					selection, _ = assistantFormulaDigest(assistantFormulaSelection{})
				}
				return assistantDataJSON(struct {
					assistantFormulaContext
					Snapshot   string                `json:"snapshot"`
					Expression string                `json:"expression"`
					Available  bool                  `json:"available"`
					Value      formula.Value         `json:"value"`
					Formatted  string                `json:"formatted_value"`
					Derivation *engineenv.Derivation `json:"derivation,omitempty"`
				}{snapshot.Context, assistantFormulaEncodeToken(snapshot, selection), expr, true, value, formatFormulaValue(value), derivation}, nil)
			}},
	}
}
