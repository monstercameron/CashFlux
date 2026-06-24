// SPDX-License-Identifier: MIT

// Package widgetspec is the pure model for custom-page widgets: the catalog of
// widget types a user can place, the data sources a list/table widget can read,
// and the deterministic evaluation + formatting a KPI widget needs. Rendering
// lives in the wasm UI layer; everything here is platform-independent so it
// unit-tests on native Go (no syscall/js).
package widgetspec

import (
	"fmt"
	"strconv"

	"github.com/monstercameron/CashFlux/internal/formula"
)

// Widget type identifiers, stored in domain.PageWidget.Type. KPI/List/Chart/Text
// are the config-driven Phase-B templates; Table/Image bind to artifacts (Phase C).
const (
	TypeKPI   = "kpi"
	TypeList  = "list"
	TypeChart = "chart"
	TypeText  = "text"
	TypeTable = "table"
	TypeImage = "image"
)

// Def describes a selectable option (a widget type or a list source) for the
// add/configure UI: a stable value plus a human label and one-line description.
type Def struct {
	Type  string
	Label string
	Desc  string
}

// catalog is the config-driven widget palette offered in Phase B. Table/Image are
// added with the artifacts work and intentionally excluded here.
var catalog = []Def{
	{TypeKPI, "KPI", "A single number from a formula over your figures."},
	{TypeList, "List", "Rows from your data (transactions, accounts, …)."},
	{TypeChart, "Chart", "A trend line of a figure over time."},
	{TypeText, "Text", "A note you write — Markdown supported (headings, lists, links)."},
	{TypeTable, "Table", "A table from an imported dataset artifact."},
	{TypeImage, "Image", "An uploaded image artifact."},
}

// Catalog returns the placeable widget types, in display order.
func Catalog() []Def {
	out := make([]Def, len(catalog))
	copy(out, catalog)
	return out
}

// List data sources a List widget can read.
const (
	SourceTransactions = "transactions"
	SourceAccounts     = "accounts"
	SourceBudgets      = "budgets"
	SourceGoals        = "goals"
	SourceTasks        = "tasks"
	SourceBills        = "bills" // upcoming recurring bills (L63 GAP-A)
)

var listSources = []Def{
	{SourceTransactions, "Transactions", "Income, expenses, and transfers."},
	{SourceAccounts, "Accounts", "Everything you own and owe."},
	{SourceBudgets, "Budgets", "Spending limits and their status."},
	{SourceGoals, "Goals", "Savings targets and progress."},
	{SourceTasks, "Tasks", "Your to-do items."},
	{SourceBills, "Bills", "Upcoming recurring bills and subscriptions."},
}

// ListSources returns the data sources a List widget can bind to, in display order.
func ListSources() []Def { out := make([]Def, len(listSources)); copy(out, listSources); return out }

// Known reports whether t is a recognized widget type.
func Known(t string) bool {
	switch t {
	case TypeKPI, TypeList, TypeChart, TypeText, TypeTable, TypeImage:
		return true
	}
	return false
}

// KPI value formats.
const (
	FormatNumber   = "number"
	FormatPercent  = "percent"
	FormatCurrency = "currency"
)

// EvalKPI evaluates a KPI widget's formula against the variable surface and
// returns a number. Booleans coerce to 1/0; a string result is an error (a KPI
// must be numeric). Empty expressions are a friendly error rather than a parse
// failure. Deterministic: it's a thin wrapper over the pure formula engine.
func EvalKPI(expr string, vars map[string]float64) (float64, error) {
	if expr == "" {
		return 0, fmt.Errorf("widgetspec: no formula set")
	}
	val, err := formula.Eval(expr, formula.Env{Vars: vars})
	if err != nil {
		return 0, err
	}
	switch n := val.(type) {
	case float64:
		return n, nil
	case bool:
		if n {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("widgetspec: formula must produce a number, got %T", val)
	}
}

// Format renders a KPI value for display per the chosen format. Currency is
// formatted by the caller (it needs the base currency), so here it falls back to
// a plain number; number trims trailing zeros, percent appends "%".
func Format(value float64, format string) string {
	switch format {
	case FormatPercent:
		return trim(value) + "%"
	default: // FormatNumber, FormatCurrency (caller may override), or unset
		return trim(value)
	}
}

// trim formats a float without a trailing ".000…" while keeping real decimals.
func trim(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
