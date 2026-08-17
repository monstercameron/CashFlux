// SPDX-License-Identifier: MIT

// Package widgetcatalog is the data-driven catalog of building blocks a widget
// designer offers: the named METRICS a formula can reference (engine atoms +
// molecules + the household's custom fields), plus the option sets for formats,
// pipeline sources/transforms, content-block kinds and template verbs. Every picker
// in the Studio designer is populated from here, so nothing is hardcoded in the UI
// and a new engine variable or custom field shows up automatically. Pure Go, no
// syscall/js — unit-tested. See docs/UNIFIED_WIDGET_API.md.
package widgetcatalog

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/copytext"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
)

// Group buckets a metric for display in the picker.
type Group string

const (
	GroupCore      Group = "Core money"
	GroupActivity  Group = "Activity"
	GroupCounts    Group = "Counts"
	GroupCustom    Group = "Custom fields"
	GroupBudgets   Group = "Budgets"
	GroupAccounts  Group = "Accounts"
	GroupGoals     Group = "Goals"
	GroupDebt      Group = "Debts"
	GroupPools     Group = "Pools"
	GroupGroups    Group = "Account groups"
	GroupAllocate  Group = "Allocate"
	GroupPlanning  Group = "Planning"
	GroupRecurring Group = "Recurring"
	GroupEvents    Group = "Events"
)

// billsSmartMeta labels the smart-bill-schedule variables for the picker.
var billsSmartMeta = []struct{ Name, Label, Doc string }{
	{"bills_low_raw", "Bills: projected low (raw)", "The lowest projected liquid balance over the next 60 days under the raw due dates."},
	{"bills_check_load_raw", "Bills: heaviest paycheck (raw)", "The most any single pay period owes under the raw due dates."},
	{"bills_check_load_smart", "Bills: heaviest paycheck (smart)", "The most any single pay period owes under the pay-ahead smart schedule."},
	{"bills_even_gain", "Bills: paycheck-evening gain", "How much lighter the heaviest paycheck gets under the smart schedule."},
	{"bills_paid_ahead", "Bills: paid ahead", "How many bills the smart schedule pays ahead of their due date."},
	{"bills_suggest_gain", "Bills: best due-date shift gain", "The low-point improvement from the best single biller-side due-date change."},
}

// BillsSmartMetrics exposes the smart-bill-schedule variables (addBillsSmartVars)
// in the formula picker under the Recurring group.
func BillsSmartMetrics() []Metric {
	out := make([]Metric, 0, len(billsSmartMeta))
	for _, m := range billsSmartMeta {
		out = append(out, Metric{Name: m.Name, Label: m.Label, Doc: m.Doc, Group: GroupRecurring})
	}
	return out
}

// recurringFixedMeta labels the fixed recurring-schedule aggregates for the picker.
var recurringFixedMeta = []struct{ Name, Label, Doc string }{
	{"recurring_monthly_in", "Recurring money in / mo", "What your scheduled income flows add up to per month."},
	{"recurring_monthly_out", "Recurring money out / mo", "What your scheduled bills and subscriptions add up to per month."},
	{"recurring_monthly_net", "Recurring net / mo", "Scheduled money in minus money out, per month."},
	{"recurring_count", "Scheduled flows", "How many recurring flows are on the schedule."},
}

// recurringFieldMeta labels each per-flow metric suffix.
var recurringFieldMeta = map[string]struct{ Label, Doc string }{
	"monthly": {"monthly equivalent", "What this flow adds up to per month, whatever its cadence (signed)."},
	"amount":  {"amount per occurrence", "The flow's signed amount each time it occurs."},
}

// RecurringMetrics exposes the recurring-schedule aggregates + each flow's identity
// variables (addRecurringVars) in the formula picker under the Recurring group.
func RecurringMetrics(recs []domain.Recurring) []Metric {
	out := make([]Metric, 0, len(recurringFixedMeta)+len(recs)*len(engineenv.RecurringVarFields))
	for _, m := range recurringFixedMeta {
		out = append(out, Metric{Name: m.Name, Label: m.Label, Doc: m.Doc, Group: GroupRecurring})
	}
	for _, base := range engineenv.RecurringVarBases(recs) {
		for _, field := range engineenv.RecurringVarFields {
			meta := recurringFieldMeta[field]
			out = append(out, Metric{Name: base.Prefix + field, Label: base.Recurring.Label + " — " + meta.Label, Doc: meta.Doc, Group: GroupRecurring})
		}
	}
	return out
}

// planningFixedMeta labels the fixed planning-policy variables for the picker.
var planningFixedMeta = []struct{ Name, Label, Doc string }{
	{"runway_buffer", "Runway buffer", "The liquidity floor the cash runway warns below."},
	{"runway_days", "Runway horizon (days)", "How many days ahead the cash runway projects."},
	{"forecast_horizon", "Forecast horizon (months)", "How many months ahead the net-worth forecast projects."},
}

// planFieldMeta labels each per-plan metric suffix.
var planFieldMeta = map[string]struct{ Label, Doc string }{
	"end":     {"projected end balance", "The plan's balance at the end of its horizon."},
	"monthly": {"monthly change", "The plan's net monthly change."},
	"runway":  {"runway (months)", "Months until the plan's balance depletes (0 if it never does)."},
}

// PlanningMetrics exposes the planning-policy variables + each saved what-if plan's projection
// (addPlanningVars) in the formula picker under the Planning group.
func PlanningMetrics(plans []domain.Plan) []Metric {
	out := make([]Metric, 0, len(planningFixedMeta)+len(plans)*len(engineenv.PlanVarFields))
	for _, m := range planningFixedMeta {
		out = append(out, Metric{Name: m.Name, Label: m.Label, Doc: m.Doc, Group: GroupPlanning})
	}
	for _, base := range engineenv.PlanVarBases(plans) {
		for _, field := range engineenv.PlanVarFields {
			meta := planFieldMeta[field]
			out = append(out, Metric{Name: base.Prefix + field, Label: base.Plan.Name + " — " + meta.Label, Doc: meta.Doc, Group: GroupPlanning})
		}
	}
	return out
}

// allocMetricMeta labels the fixed allocate-plan variables for the picker.
var allocMetricMeta = []struct{ Name, Label, Doc string }{
	{"alloc_amount", "Amount to allocate", "The total you're putting to work this round."},
	{"alloc_reserve", "Reserve kept back", "Amount held back from the split as an emergency buffer."},
	{"alloc_max_per", "Per-destination cap", "The most any single destination may receive (0 = uncapped)."},
	{"alloc_allocatable", "Allocatable", "Amount minus reserve — what actually gets split."},
	{"alloc_reserved_pct", "Reserved %", "Reserve as a percent of the amount to allocate."},
	{"alloc_destination_count", "Eligible destinations", "How many places qualify to receive money (accounts, debts, goals)."},
}

// AllocMetrics exposes the allocate-plan variables (addAllocVars) in the formula picker under
// the Allocate group, so a plan figure can be dropped into a formula or dashboard widget.
func AllocMetrics() []Metric {
	out := make([]Metric, 0, len(allocMetricMeta))
	for _, m := range allocMetricMeta {
		out = append(out, Metric{Name: m.Name, Label: m.Label, Doc: m.Doc, Group: GroupAllocate})
	}
	return out
}

// debtFieldMeta labels + documents each per-debt metric suffix for the picker.
var debtFieldMeta = map[string]struct{ Label, Doc string }{
	"balance":       {"owed", "The amount currently owed on this debt (base currency)."},
	"apr":           {"APR %", "The debt's annual interest rate, as entered."},
	"min_payment":   {"min payment", "The required minimum monthly payment."},
	"limit":         {"credit limit", "The credit limit (0 for installment loans)."},
	"available":     {"available", "Remaining credit = limit minus owed (0 when no limit)."},
	"utilization":   {"utilization %", "Owed as a percent of the credit limit (0 when no limit)."},
	"carry":         {"carrying cost / mo", "Monthly interest to hold this debt: owed × APR/100 ÷ 12."},
	"statement_day": {"statement day", "Day of the month the statement closes (0 when unknown)."},
	"due_day":       {"due day", "Day of the month the payment is due (0 when unknown)."},
}

// goalFieldMeta labels + documents each per-goal metric suffix for the picker.
var goalFieldMeta = map[string]struct{ Label, Doc string }{
	"target":      {"target", "The goal's target amount (savings goals)."},
	"saved":       {"saved", "Amount saved toward the goal so far (savings goals)."},
	"remaining":   {"left", "Target minus saved (0 once reached; savings goals)."},
	"percent":     {"% funded", "Saved as a percent of the target (savings goals)."},
	"progress":    {"progress %", "Percent complete, whatever the goal's kind (money, to-dos, milestone, or habit)."},
	"tasks_done":  {"to-dos done", "Number of the goal's linked to-dos that are done."},
	"tasks_total": {"to-dos total", "Number of to-dos linked to the goal."},
	"done":        {"done", "1 when the goal has reached its objective, else 0."},
	"streak":      {"streak", "Current habit check-in streak (0 for non-habit goals)."},
}

// accountFieldMeta labels + documents each per-account metric suffix for the picker.
var accountFieldMeta = map[string]struct{ Label, Doc string }{
	"balance": {"balance", "This account's current balance (base currency)."},
	"cleared": {"cleared", "Balance counting only cleared transactions."},
	"in":      {"money in", "Non-transfer money into this account this period (base currency)."},
	"out":     {"money out", "Non-transfer money out of this account this period (base currency)."},
}

// budgetFieldMeta labels + documents each per-budget metric suffix for the picker.
var budgetFieldMeta = map[string]struct{ Label, Doc string }{
	"limit":     {"limit", "This budget's limit for its period."},
	"spent":     {"spent", "Spent against this budget this period."},
	"remaining": {"left", "Limit minus spent (negative when overspent)."},
	"over":      {"overspend", "How far over the limit (0 when within budget)."},
	"percent":   {"used %", "Spent as a percent of the limit."},
}

// Metric is one named value a formula can reference, with a friendly label and a
// one-line description so a casual user can choose the figure they care about.
// Atoms are indivisible reductions over the data; molecules are compound figures
// defined as a formula over atoms (Formula is set, Molecule true) — e.g. net worth =
// "assets - liabilities" — so the picker can show that a figure is built from atoms.
type Metric struct {
	Name     string // engine variable name (e.g. "net_worth", "cf_txn_tip")
	Label    string // human label (e.g. "Net worth")
	Doc      string // one-line explanation
	Group    Group
	Molecule bool   // true if this is a compound figure (built from atoms via a formula)
	Formula  string // the molecule's definition over atoms (empty for atoms/custom fields)
}

// Option is a value/label pair for a select-style picker.
type Option struct {
	Value string
	Label string
	// LabelKey names Label's catalog entry (C362). This package is pure — it has
	// no language context and must not acquire one — so it carries the key and
	// the English side by side and the UI resolves at render time.
	LabelKey string
}

// Localized returns a copy of o with its label rendered through tr, falling back
// to the English the catalog does not know.
func (o Option) Localized(tr copytext.Translator) Option {
	o.Label = copytext.Text{Key: o.LabelKey, Fallback: o.Label}.Resolve(tr)
	return o
}

// metricMeta curates the label + group + one-line doc for each built-in engine
// variable. A variable not listed still appears (label derived from its name), so
// the catalog never silently drops a metric the engine exposes.
var metricMeta = map[string]struct {
	Label string
	Group Group
	Doc   string
}{
	"net_worth":          {"Net worth", GroupCore, "Everything you own minus everything you owe."},
	"assets":             {"Assets", GroupCore, "Total balance of your asset accounts."},
	"liabilities":        {"Liabilities", GroupCore, "Total balance of what you owe."},
	"liquid_cash":        {"Liquid cash", GroupCore, "Cash you can spend right now."},
	"safe_to_spend":      {"Safe to spend", GroupCore, "Liquid cash after this month's bills and goals."},
	"income":             {"Income", GroupActivity, "Money in over the chosen period."},
	"expense":            {"Spending", GroupActivity, "Money out over the chosen period."},
	"cashflow_net":       {"Net cash flow", GroupActivity, "Income minus spending for the period."},
	"savings_rate":       {"Savings rate", GroupActivity, "Percent of income you kept."},
	"cash_runway_days":   {"Cash runway (days)", GroupActivity, "How many days your liquid cash lasts at your average spending over the last three months. 0 means it does not apply — you are not spending down, or the balance is already negative."},
	"bills_due":          {"Bills due", GroupActivity, "Bills due before month-end."},
	"goal_needs":         {"Goal set-asides", GroupActivity, "What your goals need this month."},
	"income_count":       {"Number of deposits", GroupCounts, "How many deposits this period."},
	"expense_count":      {"Number of expenses", GroupCounts, "How many expenses this period."},
	"accounts":           {"Accounts", GroupCounts, "Count of active accounts."},
	"asset_accounts":     {"Asset accounts", GroupCounts, "Count of asset accounts."},
	"liability_accounts": {"Liability accounts", GroupCounts, "Count of liability accounts."},
	"transactions":       {"Transactions", GroupCounts, "Count of transactions."},
	"members":            {"Household members", GroupCounts, "Count of household members."},
	"budgets":            {"Budgets", GroupCounts, "Count of budgets."},
	"goals":              {"Goals", GroupCounts, "Count of goals."},
	"tasks":              {"To-dos", GroupCounts, "Count of to-dos."},
}

// Metrics returns every metric a formula can reference: the built-in engine
// variables (atoms then molecules) followed by the household's numeric custom
// fields, each labelled and described. A molecule carries its atom-built Formula
// (taken from molecules, falling back to the engine defaults) so the designer can
// show that a compound figure is composed from atoms. defs/molecules may be nil.
func Metrics(defs []customfields.Def, molecules []domain.Molecule) []Metric {
	if len(molecules) == 0 {
		molecules = engineenv.DefaultMolecules()
	}
	formulaOf := make(map[string]string, len(molecules))
	for _, m := range molecules {
		formulaOf[m.Name] = m.Formula
	}
	out := make([]Metric, 0, len(engineenv.Names)+len(defs))
	for _, name := range engineenv.Names {
		m := Metric{Name: name, Label: humanize(name), Group: GroupActivity}
		if meta, ok := metricMeta[name]; ok {
			m.Label, m.Group, m.Doc = meta.Label, meta.Group, meta.Doc
		}
		if f, ok := formulaOf[name]; ok {
			m.Molecule, m.Formula = true, f
		}
		out = append(out, m)
	}
	for _, name := range engineenv.CustomFieldNames(defs) {
		out = append(out, Metric{
			Name:  name,
			Label: customFieldLabel(name, defs),
			Doc:   "Sum of your custom field over its entity.",
			Group: GroupCustom,
		})
	}
	return out
}

// BudgetMetrics returns the per-budget metrics (limit/spent/left/overspend/used%) so a
// specific budget can be referenced in a formula or dashboard widget — e.g.
// budget_groceries_remaining. Built from engineenv's naming so the labels always match
// the variables the surface actually resolves. Returns nothing when there are no budgets.
func BudgetMetrics(budgets []domain.Budget) []Metric {
	bases := engineenv.BudgetVarBases(budgets)
	out := make([]Metric, 0, len(bases)*len(engineenv.BudgetVarFields))
	for _, base := range bases {
		for _, field := range engineenv.BudgetVarFields {
			meta := budgetFieldMeta[field]
			out = append(out, Metric{
				Name:  base.Prefix + field,
				Label: base.Budget.Name + " — " + meta.Label,
				Doc:   meta.Doc,
				Group: GroupBudgets,
			})
		}
	}
	return out
}

// AccountMetrics returns the per-account metrics (balance/cleared) so a specific account
// can be referenced in a formula or dashboard widget — e.g. account_checking_balance.
// Built from engineenv's naming so labels always match the variables the surface resolves.
func AccountMetrics(accounts []domain.Account) []Metric {
	bases := engineenv.AccountVarBases(accounts)
	out := make([]Metric, 0, len(bases)*len(engineenv.AccountVarFields))
	for _, base := range bases {
		for _, field := range engineenv.AccountVarFields {
			meta := accountFieldMeta[field]
			out = append(out, Metric{
				Name:  base.Prefix + field,
				Label: base.Account.Name + " — " + meta.Label,
				Doc:   meta.Doc,
				Group: GroupAccounts,
			})
		}
	}
	return out
}

// GoalMetrics returns the per-goal metrics (target/saved/left/%funded) so a specific
// savings goal can be referenced in a formula or dashboard widget — e.g.
// goal_emergency_remaining. Built from engineenv's naming so labels match the surface.
func GoalMetrics(goals []domain.Goal) []Metric {
	bases := engineenv.GoalVarBases(goals)
	out := make([]Metric, 0, len(bases)*len(engineenv.GoalVarFields))
	for _, base := range bases {
		for _, field := range engineenv.GoalVarFields {
			meta := goalFieldMeta[field]
			out = append(out, Metric{
				Name:  base.Prefix + field,
				Label: base.Goal.Name + " — " + meta.Label,
				Doc:   meta.Doc,
				Group: GroupGoals,
			})
		}
	}
	return out
}

// DebtMetrics returns the per-debt metrics (owed/APR/min payment/limit/available/
// utilization) so a specific liability can be referenced in a formula or dashboard widget
// — e.g. debt_visa_utilization. Built from engineenv's naming so labels always match the
// variables the surface resolves. Returns nothing when there are no debts.
func DebtMetrics(accounts []domain.Account) []Metric {
	bases := engineenv.DebtVarBases(accounts)
	out := make([]Metric, 0, len(bases)*len(engineenv.DebtVarFields))
	for _, base := range bases {
		for _, field := range engineenv.DebtVarFields {
			meta := debtFieldMeta[field]
			out = append(out, Metric{
				Name:  base.Prefix + field,
				Label: base.Account.Name + " — " + meta.Label,
				Doc:   meta.Doc,
				Group: GroupDebt,
			})
		}
	}
	return out
}

// PoolMetrics returns the per-pool combined-value metric (pool_<slug>_value) so a custom
// account group can be referenced by name in a formula or dashboard widget. Built from
// engineenv's naming so labels always match the variables the surface resolves.
func PoolMetrics(pools []engineenv.PoolDef) []Metric {
	bases := engineenv.PoolVarBases(pools)
	out := make([]Metric, 0, len(bases)*len(engineenv.PoolVarFields))
	for _, base := range bases {
		for _, field := range engineenv.PoolVarFields {
			out = append(out, Metric{
				Name:  base.Prefix + field,
				Label: base.Pool.Name + " — value",
				Doc:   "Combined current value of the accounts in this pool.",
				Group: GroupPools,
			})
		}
	}
	return out
}

// GroupMetrics returns the per-group net-subtotal metric (group_<slug>_total) so a
// user-defined /accounts grouping (AC1) can be referenced by name in a formula or
// dashboard widget. Built from engineenv's naming so labels match the surface.
func GroupMetrics(groups []engineenv.AccountGroupDef) []Metric {
	bases := engineenv.GroupVarBases(groups)
	out := make([]Metric, 0, len(bases)*len(engineenv.GroupVarFields))
	for _, base := range bases {
		for _, field := range engineenv.GroupVarFields {
			out = append(out, Metric{
				Name:  base.Prefix + field,
				Label: base.Group.Name + " — net total",
				Doc:   "Net subtotal of the accounts in this group (assets minus liabilities, base currency).",
				Group: GroupGroups,
			})
		}
	}
	return out
}

// eventFieldMeta labels the per-event metric suffixes for the picker.
var eventFieldMeta = map[string]struct{ Suffix, Doc string }{
	"total": {"total", "Net money (income minus spending) across the transactions mapped to this event."},
	"spend": {"spend", "How much this event cost — the magnitude of its net spending."},
	"count": {"count", "How many transactions are mapped to this event."},
}

// EventMetrics returns the per-event metrics (event_<slug>_{total,spend,count}) so a
// spending event can be referenced by name in a formula or dashboard widget (TX10).
// Built from engineenv's naming so labels always match the resolved variables.
func EventMetrics(evs []engineenv.EventDef) []Metric {
	bases := engineenv.EventVarBases(evs)
	out := make([]Metric, 0, len(bases)*len(engineenv.EventVarFields))
	for _, base := range bases {
		for _, field := range engineenv.EventVarFields {
			meta := eventFieldMeta[field]
			out = append(out, Metric{
				Name:  base.Prefix + field,
				Label: base.Event.Name + " — " + meta.Suffix,
				Doc:   meta.Doc,
				Group: GroupEvents,
			})
		}
	}
	return out
}

// MetricNames returns just the names (for quick validation / reference lists).
func MetricNames(defs []customfields.Def) []string {
	ms := Metrics(defs, nil)
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Name
	}
	return out
}

// translator resolves the catalog keys this package's labels carry. The UI
// installs it once at boot (SetTranslator) and it is read-only after, because a
// pure package cannot import the UI to ask.
//
// A package-level hook rather than a parameter on every accessor is a deliberate
// trade. This package has a dozen public accessors read from dozens of places,
// and threading a translator through all of them would put the burden of
// remembering on every future caller — the shape that left three bill surfaces
// un-deduped in C340. Unset, every label renders the producer's English, so the
// package stays fully usable and testable without it.
var translator copytext.Translator

// SetTranslator installs the catalog resolver for this package's labels (C362).
// Call once, at boot, before any UI reads the catalog.
func SetTranslator(tr copytext.Translator) { translator = tr }

// localized renders a slice of options through the installed translator. Every
// accessor already returns a fresh slice, so this never mutates a package table.
func localized(in []Option) []Option {
	out := make([]Option, len(in))
	for i, o := range in {
		out[i] = o.Localized(translator)
	}
	return out
}

// Formats are the KPI/figure display formats.
func Formats() []Option {
	return localized([]Option{
		{Value: "currency", Label: "Money", LabelKey: "wcat.fmtMoney"},
		{Value: "percent", Label: "Percent", LabelKey: "wcat.fmtPercent"},
		{Value: "number", Label: "Number", LabelKey: "wcat.fmtNumber"},
	})
}

// Kinds are the widget kinds the designer can produce.
func Kinds() []Option {
	return localized([]Option{
		{Value: "kpi", Label: "Single figure", LabelKey: "wcat.kindFigure"},
		{Value: "compound", Label: "Custom layout", LabelKey: "wcat.kindCompound"},
		{Value: "list", Label: "List", LabelKey: "wcat.kindList"},
		{Value: "chart", Label: "Chart", LabelKey: "wcat.kindChart"},
	})
}

// FigureFormats are the display formats for a single figure block (the standard
// formats plus a signed +/- money variant).
func FigureFormats() []Option {
	return localized(append(Formats(), Option{Value: "signed", Label: "Signed (+/−)", LabelKey: "wcat.fmtSigned"}))
}

// ListDisplays are the ways a list widget can present its rows: cap to N, scroll all
// within the tile, or page through them.
func ListDisplays() []Option {
	return localized([]Option{
		{Value: "cap", Label: "Show top rows", LabelKey: "wcat.listCap"},
		{Value: "scroll", Label: "Scroll all", LabelKey: "wcat.listScroll"},
		{Value: "page", Label: "Page through", LabelKey: "wcat.listPage"},
	})
}

// ChartSourceTypes are the two shapes a chart can take.
func ChartSourceTypes() []Option {
	return localized([]Option{
		{Value: "series", Label: "Trend over time", LabelKey: "wcat.srcSeries"},
		{Value: "collection", Label: "Breakdown", LabelKey: "wcat.srcCollection"},
	})
}

// Starter is a one-click preset that pre-fills the designer so a casual user never
// faces a blank canvas. Pure data — the UI maps a click onto its form fields. For a
// compound preset, Blocks carries the canonical layout so re-picking the preset always
// restores it (never leaves stale user-edited blocks).
type Starter struct {
	Label, Title, Kind, Formula, Format, Sub, Collection, Series string
	Blocks                                                       []domain.Block
	// LabelKey / TitleKey name the catalog entries for Label and Title (C362).
	LabelKey, TitleKey string
}

// Localized returns a copy of st with its label and title rendered through tr.
func (st Starter) Localized(tr copytext.Translator) Starter {
	st.Label = copytext.Text{Key: st.LabelKey, Fallback: st.Label}.Resolve(tr)
	st.Title = copytext.Text{Key: st.TitleKey, Fallback: st.Title}.Resolve(tr)
	return st
}

// RowCounts are the row-count choices for a list widget.
func RowCounts() []Option {
	return localized([]Option{{Value: "3", Label: "3"}, {Value: "5", Label: "5"}, {Value: "6", Label: "6"}, {Value: "10", Label: "10"}, {Value: "15", Label: "15"}})
}

// IncomeVsSpendingBlocks is the canonical "income vs spending" compound layout: a
// caption over two side-by-side money figures. Reused as the designer's default
// compound blocks and the "Income vs spending" starter so they never drift.
func IncomeVsSpendingBlocks() []domain.Block {
	return []domain.Block{
		{Kind: domain.BlockText, TextKey: "wcat.blockThisMonth", Text: "This month", Style: domain.Style{FontWeight: "600"}},
		{Kind: domain.BlockFigure, Bind: "income|currency", ColSpan: 2, Style: domain.Style{Text: "var(--up)"}},
		{Kind: domain.BlockFigure, Bind: "expense|currency", ColSpan: 2, Style: domain.Style{Text: "var(--down)"}},
	}
}

// Starters returns the built-in starter presets, ordered simplest-first.
func Starters() []Starter {
	return localizedStarters([]Starter{
		{Label: "Net worth", LabelKey: "wcat.startNetWorth", Title: "Net worth", TitleKey: "wcat.startNetWorth", Kind: "kpi", Formula: "net_worth", Format: "currency"},
		{Label: "Savings rate", LabelKey: "wcat.startSavingsRate", Title: "Savings rate", TitleKey: "wcat.startSavingsRate", Kind: "kpi", Formula: "savings_rate", Format: "percent"},
		{Label: "Income vs spending", LabelKey: "wcat.startIncomeVsSpending", Title: "This month", TitleKey: "wcat.blockThisMonth", Kind: "compound", Blocks: IncomeVsSpendingBlocks()},
		{Label: "Recent activity", LabelKey: "wcat.startRecentActivity", Title: "Recent activity", TitleKey: "wcat.startRecentActivity", Kind: "list", Collection: "transactions"},
		{Label: "Spending breakdown", LabelKey: "wcat.startSpendingBreakdown", Title: "Spending breakdown", TitleKey: "wcat.startSpendingBreakdown", Kind: "chart", Collection: "spending-breakdown"},
		{Label: "Net worth trend", LabelKey: "wcat.startNetWorthTrend", Title: "Net worth trend", TitleKey: "wcat.startNetWorthTrend", Kind: "chart", Series: "networth"},
	})
}

// localizedStarters renders a starter list through the installed translator.
func localizedStarters(in []Starter) []Starter {
	out := make([]Starter, len(in))
	for i, st := range in {
		out[i] = st.Localized(translator)
	}
	return out
}

// SortField is one column a list widget can be ordered by: the Frame column name the
// engine's sort transform targets, a friendly label, and whether the column is numeric
// (so the designer can offer "High → Low" vs "A → Z" direction labels). Defined per
// collection so the sort picker only ever offers columns that exist in that source.
type SortField struct {
	Column   string // Frame column name (must match the widgetsource resolver's output)
	Label    string // human label (e.g. "Amount", "Date")
	LabelKey string // Label's catalog entry (C362)
	Numeric  bool   // numeric/money/percent → hi/low direction; text → a/z
}

// Localized returns a copy of f with its label rendered through tr.
func (f SortField) Localized(tr copytext.Translator) SortField {
	f.Label = copytext.Text{Key: f.LabelKey, Fallback: f.Label}.Resolve(tr)
	return f
}

// Collection is a list/chart row source, defined in one place with its picker label,
// the full-data screen it links to (route + link label), AND the columns it can be
// sorted by. Everything about a source lives on its definition so there's a single
// source of truth — no parallel mapping.
type Collection struct {
	Value, Label     string
	Route, LinkLabel string      // full-data screen; empty Route = no "view all" target
	Sort             []SortField // columns this collection's rows can be ordered by
	// LabelKey / LinkLabelKey name the catalog entries for the two user-facing
	// strings above (C362).
	LabelKey, LinkLabelKey string
	// DefaultSort is the column a fresh list of this collection is ordered by out of
	// the box (must be one of Sort's columns; "" = the source's natural order).
	// DefaultDesc sets that default's direction (true = descending). The designer
	// pre-selects this so a new list looks intentional, and it stays fully overridable.
	DefaultSort string
	DefaultDesc bool
}

// collectionDefs is the canonical collection list. Add a collection here once and it
// flows to the picker (Collections), the "view all" link (CollectionRoute) and the
// sort control (SortFields/DefaultSort) alike. Sort columns must match the widgetsource Frame.
var collectionDefs = []Collection{
	{Value: "transactions", Label: "Recent transactions", LabelKey: "wcat.collRecentTransactions", Route: "/transactions", LinkLabel: "View all transactions", LinkLabelKey: "wcat.linkAllTransactions",
		Sort:        []SortField{{Column: "date", Label: "Date", LabelKey: "wcat.colDate", Numeric: true}, {Column: "amount", Label: "Amount", LabelKey: "wcat.colAmount", Numeric: true}, {Column: "desc", Label: "Description", LabelKey: "wcat.colDescription"}},
		DefaultSort: "date", DefaultDesc: true}, // newest first
	// The full ledger source the widgetized /transactions table renders from: the same
	// transactions with the richer columns (payee/account/category/cleared). No "view
	// all" route — this IS the full view.
	{Value: "transactions-full", Label: "All transactions", LabelKey: "wcat.collAllTransactions",
		Sort:        []SortField{{Column: "date", Label: "Date", LabelKey: "wcat.colDate", Numeric: true}, {Column: "amount", Label: "Amount", LabelKey: "wcat.colAmount", Numeric: true}, {Column: "payee", Label: "Payee", LabelKey: "wcat.colPayee"}, {Column: "account", Label: "Account", LabelKey: "wcat.colAccount"}, {Column: "category", Label: "Category", LabelKey: "wcat.colCategory"}, {Column: "source", Label: "Source", LabelKey: "wcat.colSource"}},
		DefaultSort: "date", DefaultDesc: true}, // newest first
	{Value: "accounts", Label: "Account balances", LabelKey: "wcat.collAccountBalances", Route: "/accounts", LinkLabel: "View all accounts", LinkLabelKey: "wcat.linkAllAccounts",
		Sort:        []SortField{{Column: "balance", Label: "Balance", LabelKey: "wcat.colBalance", Numeric: true}, {Column: "name", Label: "Name", LabelKey: "wcat.colName"}},
		DefaultSort: "balance", DefaultDesc: true}, // largest balance first
	{Value: "budgets", Label: "Budget status", LabelKey: "wcat.collBudgetStatus", Route: "/budgets", LinkLabel: "View all budgets", LinkLabelKey: "wcat.linkAllBudgets",
		Sort:        []SortField{{Column: "percent", Label: "Used %", LabelKey: "wcat.colUsedPct", Numeric: true}, {Column: "name", Label: "Name", LabelKey: "wcat.colName"}},
		DefaultSort: "percent", DefaultDesc: true}, // most-used / at-risk first
	{Value: "bills", Label: "Upcoming bills", LabelKey: "wcat.collUpcomingBills", Route: "/bills", LinkLabel: "View all bills", LinkLabelKey: "wcat.linkAllBills",
		Sort:        []SortField{{Column: "due", Label: "Due date", LabelKey: "wcat.colDueDate", Numeric: true}, {Column: "amount", Label: "Amount", LabelKey: "wcat.colAmount", Numeric: true}, {Column: "name", Label: "Name", LabelKey: "wcat.colName"}},
		DefaultSort: "due", DefaultDesc: false}, // soonest due first
	{Value: "spending-breakdown", Label: "Spending by category", LabelKey: "wcat.collSpendingByCategory", Route: "/reports", LinkLabel: "Open spending reports", LinkLabelKey: "wcat.linkSpendingReports",
		Sort:        []SortField{{Column: "amount", Label: "Amount", LabelKey: "wcat.colAmount", Numeric: true}, {Column: "percent", Label: "Share", LabelKey: "wcat.colShare", Numeric: true}, {Column: "name", Label: "Category", LabelKey: "wcat.colCategory"}},
		DefaultSort: "amount", DefaultDesc: true}, // biggest spend first
}

// Localized returns a copy of c with every user-facing string — its own label,
// its "view all" link, and each sort column's label — rendered through tr.
func (c Collection) Localized(tr copytext.Translator) Collection {
	c.Label = copytext.Text{Key: c.LabelKey, Fallback: c.Label}.Resolve(tr)
	c.LinkLabel = copytext.Text{Key: c.LinkLabelKey, Fallback: c.LinkLabel}.Resolve(tr)
	sorts := make([]SortField, len(c.Sort))
	for i, f := range c.Sort {
		sorts[i] = f.Localized(tr)
	}
	c.Sort = sorts
	return c
}

// CollectionDefs returns the full collection definitions (label + link target).
func CollectionDefs() []Collection {
	out := make([]Collection, len(collectionDefs))
	for i, c := range collectionDefs {
		out[i] = c.Localized(translator)
	}
	return out
}

// Collections are the row sources for a List/Chart pipeline (picker options derived
// from the canonical definitions).
func Collections() []Option {
	out := make([]Option, len(collectionDefs))
	for i, c := range collectionDefs {
		out[i] = Option{Value: c.Value, Label: c.Label, LabelKey: c.LabelKey}.Localized(translator)
	}
	return out
}

// CollectionRoute returns the full-data screen route + link label for a collection,
// looked up from its definition. ("","") when the collection has no dedicated screen.
func CollectionRoute(collection string) (path, label string) {
	for _, c := range collectionDefs {
		if c.Value == collection {
			return c.Route, c.LinkLabel
		}
	}
	return "", ""
}

// SortFields returns the columns a collection's list can be ordered by ("" / unknown
// collection → none, so the UI simply hides the sort control). Looked up from the
// canonical collection definition.
func SortFields(collection string) []SortField {
	for _, c := range collectionDefs {
		if c.Value == collection {
			out := make([]SortField, len(c.Sort))
			for i, f := range c.Sort {
				out[i] = f.Localized(translator)
			}
			return out
		}
	}
	return nil
}

// DefaultSort returns a collection's out-of-the-box ordering (column + descending),
// pre-selected by the designer. ("", false) for an unknown collection or one whose
// natural order is the intended default.
func DefaultSort(collection string) (column string, desc bool) {
	for _, c := range collectionDefs {
		if c.Value == collection {
			return c.DefaultSort, c.DefaultDesc
		}
	}
	return "", false
}

// SortDirections returns the two ordering choices with labels that read naturally for
// the column type: numeric columns get High↔Low, text columns get A↔Z. The stable
// values ("desc"/"asc") map to the engine's sort arg (a "-" prefix for descending).
func SortDirections(numeric bool) []Option {
	if numeric {
		return localized([]Option{{Value: "desc", Label: "High → Low", LabelKey: "wcat.sortHighLow"}, {Value: "asc", Label: "Low → High", LabelKey: "wcat.sortLowHigh"}})
	}
	return localized([]Option{{Value: "asc", Label: "A → Z", LabelKey: "wcat.sortAZ"}, {Value: "desc", Label: "Z → A", LabelKey: "wcat.sortZA"}})
}

// SeriesMetrics are the time-series sources for a Chart pipeline.
func SeriesMetrics() []Option {
	return localized([]Option{
		{Value: "networth", Label: "Net worth over time", LabelKey: "wcat.serNetWorth"},
		{Value: "cashflow", Label: "Cash flow by month", LabelKey: "wcat.serCashFlow"},
	})
}

// Transforms are the Frame→Frame pipeline steps the designer can add.
func Transforms() []Option {
	return localized([]Option{
		{Value: "limit", Label: "Limit rows", LabelKey: "wcat.trLimit"},
		{Value: "sort", Label: "Sort by column", LabelKey: "wcat.trSort"},
		{Value: "filter", Label: "Filter rows", LabelKey: "wcat.trFilter"},
	})
}

// BlockKinds are the content blocks a compound (custom-layout) widget can place.
func BlockKinds() []Option {
	return localized([]Option{
		{Value: "figure", Label: "Figure (a metric)", LabelKey: "wcat.oFigureAMetric"},
		{Value: "text", Label: "Text / caption", LabelKey: "wcat.oTextCaption"},
		{Value: "icon", Label: "Icon", LabelKey: "wcat.oIcon"},
		{Value: "divider", Label: "Divider", LabelKey: "wcat.oDivider"},
		{Value: "spacer", Label: "Spacer", LabelKey: "wcat.oSpacer"},
		{Value: "dataview", Label: "Embedded data", LabelKey: "wcat.oEmbeddedData"},
	})
}

// TemplateVerbs are the formatting verbs usable in a sub-label / text template
// token ("{{ metric | verb }}").
func TemplateVerbs() []Option {
	return []Option{
		{Value: "currency", Label: "Money", LabelKey: "wcat.fmtMoney"},
		{Value: "percent", Label: "Percent", LabelKey: "wcat.fmtPercent"},
		{Value: "number", Label: "Number", LabelKey: "wcat.fmtNumber"},
		{Value: "signed", Label: "Signed money (+/-)", LabelKey: "wcat.fmtSignedMoney"},
		{Value: "plural:item", Label: "Count + noun", LabelKey: "wcat.fmtCountNoun"},
		{Value: "arrow", Label: "Up/down arrow", LabelKey: "wcat.fmtArrow"},
	}
}

// humanize turns a snake_case variable name into a Title Case label as a fallback.
func humanize(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// customFieldLabel resolves a cf_<entity>_<key> variable back to the custom field's
// display name + entity, falling back to a humanized key.
func customFieldLabel(varName string, defs []customfields.Def) string {
	for _, d := range defs {
		if engineenv.CustomFieldVar(d) == varName {
			return d.Label + " (" + d.EntityType + ")"
		}
	}
	return humanize(strings.TrimPrefix(varName, "cf_"))
}
