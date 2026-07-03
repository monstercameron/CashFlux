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

	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
)

// Group buckets a metric for display in the picker.
type Group string

const (
	GroupCore     Group = "Core money"
	GroupActivity Group = "Activity"
	GroupCounts   Group = "Counts"
	GroupCustom   Group = "Custom fields"
	GroupBudgets  Group = "Budgets"
	GroupAccounts Group = "Accounts"
	GroupGoals    Group = "Goals"
	GroupDebt     Group = "Debts"
	GroupPools    Group = "Pools"
)

// debtFieldMeta labels + documents each per-debt metric suffix for the picker.
var debtFieldMeta = map[string]struct{ Label, Doc string }{
	"balance":     {"owed", "The amount currently owed on this debt (base currency)."},
	"apr":         {"APR %", "The debt's annual interest rate, as entered."},
	"min_payment": {"min payment", "The required minimum monthly payment."},
	"limit":       {"credit limit", "The credit limit (0 for installment loans)."},
	"available":   {"available", "Remaining credit = limit minus owed (0 when no limit)."},
	"utilization": {"utilization %", "Owed as a percent of the credit limit (0 when no limit)."},
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

// MetricNames returns just the names (for quick validation / reference lists).
func MetricNames(defs []customfields.Def) []string {
	ms := Metrics(defs, nil)
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Name
	}
	return out
}

// Formats are the KPI/figure display formats.
func Formats() []Option {
	return []Option{
		{"currency", "Money"},
		{"percent", "Percent"},
		{"number", "Number"},
	}
}

// Kinds are the widget kinds the designer can produce.
func Kinds() []Option {
	return []Option{
		{"kpi", "Single figure"},
		{"compound", "Custom layout"},
		{"list", "List"},
		{"chart", "Chart"},
	}
}

// FigureFormats are the display formats for a single figure block (the standard
// formats plus a signed +/- money variant).
func FigureFormats() []Option {
	return append(Formats(), Option{Value: "signed", Label: "Signed (+/−)"})
}

// ListDisplays are the ways a list widget can present its rows: cap to N, scroll all
// within the tile, or page through them.
func ListDisplays() []Option {
	return []Option{
		{Value: "cap", Label: "Show top rows"},
		{Value: "scroll", Label: "Scroll all"},
		{Value: "page", Label: "Page through"},
	}
}

// ChartSourceTypes are the two shapes a chart can take.
func ChartSourceTypes() []Option {
	return []Option{
		{"series", "Trend over time"},
		{"collection", "Breakdown"},
	}
}

// Starter is a one-click preset that pre-fills the designer so a casual user never
// faces a blank canvas. Pure data — the UI maps a click onto its form fields. For a
// compound preset, Blocks carries the canonical layout so re-picking the preset always
// restores it (never leaves stale user-edited blocks).
type Starter struct {
	Label, Title, Kind, Formula, Format, Sub, Collection, Series string
	Blocks                                                       []domain.Block
}

// RowCounts are the row-count choices for a list widget.
func RowCounts() []Option {
	return []Option{{Value: "3", Label: "3"}, {Value: "5", Label: "5"}, {Value: "6", Label: "6"}, {Value: "10", Label: "10"}, {Value: "15", Label: "15"}}
}

// IncomeVsSpendingBlocks is the canonical "income vs spending" compound layout: a
// caption over two side-by-side money figures. Reused as the designer's default
// compound blocks and the "Income vs spending" starter so they never drift.
func IncomeVsSpendingBlocks() []domain.Block {
	return []domain.Block{
		{Kind: domain.BlockText, Text: "This month", Style: domain.Style{FontWeight: "600"}},
		{Kind: domain.BlockFigure, Bind: "income|currency", ColSpan: 2, Style: domain.Style{Text: "var(--up)"}},
		{Kind: domain.BlockFigure, Bind: "expense|currency", ColSpan: 2, Style: domain.Style{Text: "var(--down)"}},
	}
}

// Starters returns the built-in starter presets, ordered simplest-first.
func Starters() []Starter {
	return []Starter{
		{Label: "Net worth", Title: "Net worth", Kind: "kpi", Formula: "net_worth", Format: "currency"},
		{Label: "Savings rate", Title: "Savings rate", Kind: "kpi", Formula: "savings_rate", Format: "percent"},
		{Label: "Income vs spending", Title: "This month", Kind: "compound", Blocks: IncomeVsSpendingBlocks()},
		{Label: "Recent activity", Title: "Recent activity", Kind: "list", Collection: "transactions"},
		{Label: "Spending breakdown", Title: "Spending breakdown", Kind: "chart", Collection: "spending-breakdown"},
		{Label: "Net worth trend", Title: "Net worth trend", Kind: "chart", Series: "networth"},
	}
}

// SortField is one column a list widget can be ordered by: the Frame column name the
// engine's sort transform targets, a friendly label, and whether the column is numeric
// (so the designer can offer "High → Low" vs "A → Z" direction labels). Defined per
// collection so the sort picker only ever offers columns that exist in that source.
type SortField struct {
	Column  string // Frame column name (must match the widgetsource resolver's output)
	Label   string // human label (e.g. "Amount", "Date")
	Numeric bool   // numeric/money/percent → hi/low direction; text → a/z
}

// Collection is a list/chart row source, defined in one place with its picker label,
// the full-data screen it links to (route + link label), AND the columns it can be
// sorted by. Everything about a source lives on its definition so there's a single
// source of truth — no parallel mapping.
type Collection struct {
	Value, Label     string
	Route, LinkLabel string      // full-data screen; empty Route = no "view all" target
	Sort             []SortField // columns this collection's rows can be ordered by
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
	{Value: "transactions", Label: "Recent transactions", Route: "/transactions", LinkLabel: "View all transactions",
		Sort:        []SortField{{Column: "date", Label: "Date", Numeric: true}, {Column: "amount", Label: "Amount", Numeric: true}, {Column: "desc", Label: "Description"}},
		DefaultSort: "date", DefaultDesc: true}, // newest first
	// The full ledger source the widgetized /transactions table renders from: the same
	// transactions with the richer columns (payee/account/category/cleared). No "view
	// all" route — this IS the full view.
	{Value: "transactions-full", Label: "All transactions",
		Sort:        []SortField{{Column: "date", Label: "Date", Numeric: true}, {Column: "amount", Label: "Amount", Numeric: true}, {Column: "payee", Label: "Payee"}, {Column: "account", Label: "Account"}, {Column: "category", Label: "Category"}, {Column: "source", Label: "Source"}},
		DefaultSort: "date", DefaultDesc: true}, // newest first
	{Value: "accounts", Label: "Account balances", Route: "/accounts", LinkLabel: "View all accounts",
		Sort:        []SortField{{Column: "balance", Label: "Balance", Numeric: true}, {Column: "name", Label: "Name"}},
		DefaultSort: "balance", DefaultDesc: true}, // largest balance first
	{Value: "budgets", Label: "Budget status", Route: "/budgets", LinkLabel: "View all budgets",
		Sort:        []SortField{{Column: "percent", Label: "Used %", Numeric: true}, {Column: "name", Label: "Name"}},
		DefaultSort: "percent", DefaultDesc: true}, // most-used / at-risk first
	{Value: "bills", Label: "Upcoming bills", Route: "/bills", LinkLabel: "View all bills",
		Sort:        []SortField{{Column: "due", Label: "Due date", Numeric: true}, {Column: "amount", Label: "Amount", Numeric: true}, {Column: "name", Label: "Name"}},
		DefaultSort: "due", DefaultDesc: false}, // soonest due first
	{Value: "spending-breakdown", Label: "Spending by category", Route: "/reports", LinkLabel: "Open spending reports",
		Sort:        []SortField{{Column: "amount", Label: "Amount", Numeric: true}, {Column: "percent", Label: "Share", Numeric: true}, {Column: "name", Label: "Category"}},
		DefaultSort: "amount", DefaultDesc: true}, // biggest spend first
}

// CollectionDefs returns the full collection definitions (label + link target).
func CollectionDefs() []Collection { return append([]Collection(nil), collectionDefs...) }

// Collections are the row sources for a List/Chart pipeline (picker options derived
// from the canonical definitions).
func Collections() []Option {
	out := make([]Option, len(collectionDefs))
	for i, c := range collectionDefs {
		out[i] = Option{Value: c.Value, Label: c.Label}
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
			return append([]SortField(nil), c.Sort...)
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
		return []Option{{"desc", "High → Low"}, {"asc", "Low → High"}}
	}
	return []Option{{"asc", "A → Z"}, {"desc", "Z → A"}}
}

// SeriesMetrics are the time-series sources for a Chart pipeline.
func SeriesMetrics() []Option {
	return []Option{
		{"networth", "Net worth over time"},
		{"cashflow", "Cash flow by month"},
	}
}

// Transforms are the Frame→Frame pipeline steps the designer can add.
func Transforms() []Option {
	return []Option{
		{"limit", "Limit rows"},
		{"sort", "Sort by column"},
		{"filter", "Filter rows"},
	}
}

// BlockKinds are the content blocks a compound (custom-layout) widget can place.
func BlockKinds() []Option {
	return []Option{
		{"figure", "Figure (a metric)"},
		{"text", "Text / caption"},
		{"icon", "Icon"},
		{"divider", "Divider"},
		{"spacer", "Spacer"},
		{"dataview", "Embedded data"},
	}
}

// TemplateVerbs are the formatting verbs usable in a sub-label / text template
// token ("{{ metric | verb }}").
func TemplateVerbs() []Option {
	return []Option{
		{"currency", "Money"},
		{"percent", "Percent"},
		{"number", "Number"},
		{"signed", "Signed money (+/-)"},
		{"plural:item", "Count + noun"},
		{"arrow", "Up/down arrow"},
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
