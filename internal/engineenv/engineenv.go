// SPDX-License-Identifier: MIT

// Package engineenv builds the "app engine variable surface": the named numeric
// figures a sandboxed formula can reference. It is built compositionally from two
// layers so every figure is auditable:
//
//   - ATOMS are indivisible: a pure reduction over the fundamental data
//     (transactions, accounts, bills/recurring, goals) — e.g. "assets" is the sum
//     of FX-converted asset-account balances. An atom can't be expressed as a
//     formula over other variables; it's a leaf, computed in Go.
//   - MOLECULES are compound: defined as a FORMULA over atoms (and earlier
//     molecules) — e.g. net_worth = "assets - liabilities". Their derivation is
//     data, not code, so a figure can be traced down to its atoms (see Explain).
//     Built-ins are seeded by DefaultMolecules; overrides/additions are passed in
//     via Data.Molecules (persisted in the dataset).
//
// Pure Go, no syscall/js — unit-tested natively. The wasm layer gathers the Data
// (pre-scoped, with the active period window and the persisted molecule set).
package engineenv

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/safespend"
)

// Data is everything Vars needs. It is the raw dataset slices (pre-scoped by the
// caller) plus the FX rate table, the reference time, the active period window,
// the custom-field definitions, and the molecule set (empty → DefaultMolecules).
type Data struct {
	Accounts     []domain.Account
	Transactions []domain.Transaction
	Members      []domain.Member
	Budgets      []domain.Budget
	Goals        []domain.Goal
	Tasks        []domain.Task
	Recurring    []domain.Recurring
	Rates        currency.Rates
	Now          time.Time
	PeriodStart  time.Time
	PeriodEnd    time.Time
	CustomDefs   []customfields.Def
	Molecules    []domain.Molecule
}

// atomNames lists the indivisible variables computeAtoms produces, in a stable
// order. Each is a reduction over the named fundamental source.
var atomNames = []string{
	"assets",       // Σ FX-converted balances of non-archived asset accounts
	"liabilities",  // Σ magnitudes of non-archived liability-account balances (positive)
	"liquid_cash",  // Σ balances of non-archived cash-type accounts (checking/debit/savings/cash)
	"income",        // Σ positive non-transfer transactions in the period
	"expense",       // Σ |negative non-transfer transactions| in the period (positive)
	"income_count",  // count of income (positive non-transfer) transactions in the period
	"expense_count", // count of expense (negative non-transfer) transactions in the period
	"bills_due",     // Σ bills due before this calendar month-end
	"goal_needs",    // Σ prorated monthly goal contributions
	"accounts",           // count of non-archived accounts
	"asset_accounts",     // count of non-archived asset-class accounts
	"liability_accounts", // count of non-archived liability-class accounts
	"transactions",       // count of transactions
	"members",      // count of members
	"budgets",      // count of budgets
	"goals",        // count of goals
	"tasks",        // count of tasks
}

// DefaultMolecules are the built-in compound variables, defined as formulas over
// atoms. Editing or extending these (persisted in the dataset) reshapes the
// derived figures without code changes — and keeps them auditable.
func DefaultMolecules() []domain.Molecule {
	return []domain.Molecule{
		{Name: "net_worth", Formula: "assets - liabilities", Doc: "Everything you own minus everything you owe."},
		{Name: "cashflow_net", Formula: "income - expense", Doc: "Net cash flow for the period (income minus spending)."},
		{Name: "savings_rate", Formula: "clamp(safediv(income - expense, income, 0) * 100, -100, 100)", Doc: "Percent of income kept this period."},
		{Name: "safe_to_spend", Formula: "liquid_cash - max(bills_due, 0) - max(goal_needs, 0)", Doc: "Liquid cash after this month's bills and goal set-asides."},
	}
}

// Names lists the built-in surface: atoms followed by the default molecule names.
// The binding editor shows these (plus CustomFieldNames) so a user knows what they
// can reference.
var Names = func() []string {
	out := append([]string(nil), atomNames...)
	for _, m := range DefaultMolecules() {
		out = append(out, m.Name)
	}
	return out
}()

// Vars computes the full variable surface: the atoms, then each molecule formula
// evaluated over the running map (so a molecule may reference atoms and any
// molecule declared before it). Money figures are major units of the base
// currency. Deterministic for a given Data.
func Vars(d Data) map[string]float64 {
	out := computeAtoms(d)
	mols := d.Molecules
	if len(mols) == 0 {
		mols = DefaultMolecules()
	}
	for _, m := range mols {
		if v, err := formula.Eval(m.Formula, formula.Env{Vars: out}); err == nil {
			if f, ok := v.(float64); ok {
				out[m.Name] = f
			} else if b, ok := v.(bool); ok {
				if b {
					out[m.Name] = 1
				} else {
					out[m.Name] = 0
				}
			}
		}
	}
	return out
}

// computeAtoms reduces the fundamental data to the indivisible atoms.
func computeAtoms(d Data) map[string]float64 {
	base := d.Rates.Base
	if base == "" {
		base = "USD"
	}
	div := 1.0
	for i := 0; i < currency.Decimals(base); i++ {
		div *= 10
	}
	major := func(amount int64) float64 { return float64(amount) / div }

	// Assets/liabilities: explaining variant excludes missing-FX accounts gracefully.
	nw, _ := ledger.NetWorthExplained(d.Accounts, d.Transactions, d.Rates)

	// Income/expense over the active period (falls back to the calendar month).
	start, end := d.PeriodStart, d.PeriodEnd
	if start.IsZero() || end.IsZero() {
		start, end = dateutil.MonthRange(d.Now)
	}
	income, expense, _ := ledger.PeriodTotals(d.Transactions, start, end, d.Rates)

	// Period transaction counts (by sign), for "N deposits / N transactions" labels.
	incCount, expCount := 0, 0
	for _, t := range d.Transactions {
		if !dateutil.InRange(t.Date, start, end) {
			continue
		}
		switch {
		case t.IsIncome():
			incCount++
		case t.IsExpense():
			expCount++
		}
	}

	// Safe-to-spend atoms — fundamental, FX-converted to base. Bills/goals are a
	// this-calendar-month commitment, independent of the dashboard's period.
	liquid, _ := ledger.LiquidBalance(d.Accounts, d.Transactions, d.Rates)
	_, monthEnd := dateutil.MonthRange(d.Now)
	toBase := safespend.ToBaseFunc(d.Rates)
	billsDue := safespend.BillsDueBefore(d.Accounts, d.Recurring, d.Now, monthEnd, toBase)
	goalNeeds := safespend.GoalContributionsProrated(d.Goals, d.Now, toBase)

	active, assetAccts, liabAccts := 0, 0, 0
	for _, a := range d.Accounts {
		if a.Archived {
			continue
		}
		active++
		if a.Class == domain.ClassLiability {
			liabAccts++
		} else {
			assetAccts++
		}
	}

	out := map[string]float64{
		"assets":       major(nw.Assets.Amount),
		"liabilities":  major(nw.Liabilities.Amount),
		"liquid_cash":  major(liquid.Amount),
		"income":        major(income.Amount),
		"expense":       major(expense.Amount),
		"income_count":  float64(incCount),
		"expense_count": float64(expCount),
		"bills_due":     major(billsDue),
		"goal_needs":    major(goalNeeds),
		"accounts":           float64(active),
		"asset_accounts":     float64(assetAccts),
		"liability_accounts": float64(liabAccts),
		"transactions":       float64(len(d.Transactions)),
		"members":      float64(len(d.Members)),
		"budgets":      float64(len(d.Budgets)),
		"goals":        float64(len(d.Goals)),
		"tasks":        float64(len(d.Tasks)),
	}
	addCustomFieldVars(out, d, start, end)
	return out
}

// Derivation explains how one variable is produced — the audit record. For a
// molecule it carries the formula and the value of each variable the formula
// references (the atoms it's built from); for an atom it carries a source note.
type Derivation struct {
	Name    string
	Kind    string             // "atom" | "molecule" | "custom"
	Formula string             // molecule only
	Source  string             // atom/custom only
	Value   float64            // the resolved value
	Inputs  map[string]float64 // molecule only: referenced var → its value
}

// atomSources documents each atom's fundamental reduction (for the audit).
var atomSources = map[string]string{
	"assets":       "Σ FX-converted balances of non-archived asset accounts",
	"liabilities":  "Σ magnitudes of non-archived liability-account balances",
	"liquid_cash":  "Σ balances of non-archived cash-type accounts",
	"income":        "Σ positive non-transfer transactions in the period",
	"expense":       "Σ |negative non-transfer transactions| in the period",
	"income_count":  "count of income transactions in the period",
	"expense_count": "count of expense transactions in the period",
	"bills_due":     "Σ bills due before this calendar month-end",
	"goal_needs":    "Σ prorated monthly goal contributions",
	"accounts":           "count of non-archived accounts",
	"asset_accounts":     "count of non-archived asset-class accounts",
	"liability_accounts": "count of non-archived liability-class accounts",
	"transactions":       "count of transactions",
	"members":      "count of members",
	"budgets":      "count of budgets",
	"goals":        "count of goals",
	"tasks":        "count of tasks",
}

// Explain returns how name is derived, given the computed vars and the molecule
// set (empty → defaults). It traces a molecule to the atoms it reads, so any
// figure on screen can be audited down to its indivisible inputs.
func Explain(name string, vars map[string]float64, molecules []domain.Molecule) (Derivation, bool) {
	if len(molecules) == 0 {
		molecules = DefaultMolecules()
	}
	for _, m := range molecules {
		if m.Name != name {
			continue
		}
		d := Derivation{Name: name, Kind: "molecule", Formula: m.Formula, Source: m.Doc, Value: vars[name], Inputs: map[string]float64{}}
		if refs, err := formula.References(m.Formula); err == nil {
			for _, r := range refs {
				if v, ok := vars[r]; ok {
					d.Inputs[r] = v
				}
			}
		}
		return d, true
	}
	if src, ok := atomSources[name]; ok {
		return Derivation{Name: name, Kind: "atom", Source: src, Value: vars[name]}, true
	}
	if _, ok := vars[name]; ok {
		return Derivation{Name: name, Kind: "custom", Source: "custom field sum", Value: vars[name]}, true
	}
	return Derivation{}, false
}

// addCustomFieldVars folds each NUMBER custom field into the surface as
// cf_<entity>_<key>, summed over that entity's collection (NOT scaled — values are
// taken as-is). Transaction fields are period-scoped; account/goal fields exclude
// archived.
func addCustomFieldVars(out map[string]float64, d Data, start, end time.Time) {
	for _, def := range d.CustomDefs {
		if def.Type != customfields.TypeNumber {
			continue
		}
		short := entityTypeShort(def.EntityType)
		if short == "" {
			continue
		}
		name := "cf_" + short + "_" + def.Key
		var total float64
		switch def.EntityType {
		case "transaction":
			for _, t := range d.Transactions {
				if !dateutil.InRange(t.Date, start, end) {
					continue
				}
				if v, ok := numFrom(t.Custom, def.Key); ok {
					total += v
				}
			}
		case "account":
			for _, a := range d.Accounts {
				if a.Archived {
					continue
				}
				if v, ok := numFrom(a.Custom, def.Key); ok {
					total += v
				}
			}
		case "budget":
			for _, b := range d.Budgets {
				if v, ok := numFrom(b.Custom, def.Key); ok {
					total += v
				}
			}
		case "goal":
			for _, g := range d.Goals {
				if g.Archived {
					continue
				}
				if v, ok := numFrom(g.Custom, def.Key); ok {
					total += v
				}
			}
		case "member":
			for _, m := range d.Members {
				if v, ok := numFrom(m.Custom, def.Key); ok {
					total += v
				}
			}
		case "task":
			for _, tk := range d.Tasks {
				if v, ok := numFrom(tk.Custom, def.Key); ok {
					total += v
				}
			}
		default:
			continue
		}
		out[name] = total
	}
}

// BudgetVars returns the per-budget variable overrides for a formula evaluated in a
// single budget's CONTEXT (a cover amount is evaluated in the destination's context; a
// source weight in that source's). It exposes the budget's own spent / limit /
// remaining / overspend / percent (major units), plus each of its NUMBER custom fields
// as cf_budget_<key> bound to THIS budget's value (rather than the household sum that
// Vars folds in). Layer these on top of Vars() with Merge so a cover formula can
// reference both household aggregates and this budget's own specifics.
//
//   - remaining = limit − spent (negative when over)
//   - overspend = max(0, spent − limit)  (0 when on/under budget)
//   - percent   = spent / limit × 100     (0 when limit is 0)
func BudgetVars(b domain.Budget, spentMajor, limitMajor float64, defs []customfields.Def) map[string]float64 {
	remaining := limitMajor - spentMajor
	overspend := 0.0
	if remaining < 0 {
		overspend = -remaining
	}
	percent := 0.0
	if limitMajor != 0 {
		percent = spentMajor / limitMajor * 100
	}
	out := map[string]float64{
		"spent":     spentMajor,
		"limit":     limitMajor,
		"remaining": remaining,
		"overspend": overspend,
		"percent":   percent,
	}
	for _, def := range defs {
		if def.EntityType != "budget" || def.Type != customfields.TypeNumber {
			continue
		}
		if v, ok := numFrom(b.Custom, def.Key); ok {
			out["cf_budget_"+def.Key] = v
		}
	}
	return out
}

// Merge returns a new map with over's entries layered on top of base (base is copied
// first, so neither input is mutated). Used to overlay per-budget context vars on the
// global surface for a contextual formula evaluation.
func Merge(base, over map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}

// entityTypeShort maps a custom-field EntityType to the cf_ namespace segment.
func entityTypeShort(entityType string) string {
	switch entityType {
	case "transaction":
		return "txn"
	case "account":
		return "acct"
	case "budget":
		return "budget"
	case "goal":
		return "goal"
	case "member":
		return "member"
	case "task":
		return "task"
	}
	return ""
}

// numFrom reads a numeric value for key from a custom map. Store-loaded JSON
// numbers arrive as float64; ints are accepted too. Non-numeric values are skipped.
func numFrom(custom map[string]any, key string) (float64, bool) {
	v, ok := custom[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// CustomFieldVar returns the engine variable name a numeric custom-field definition
// produces (cf_<entity>_<key>), or "" if the def is not a numeric field the surface
// folds in. Lets callers map a Def back to its metric without re-deriving the scheme.
func CustomFieldVar(def customfields.Def) string {
	if def.Type != customfields.TypeNumber {
		return ""
	}
	short := entityTypeShort(def.EntityType)
	if short == "" {
		return ""
	}
	return "cf_" + short + "_" + def.Key
}

// CustomFieldNames returns the cf_* variable names the given defs produce, in
// definition order — for the binding editor's reference list.
func CustomFieldNames(defs []customfields.Def) []string {
	var out []string
	for _, def := range defs {
		if def.Type != customfields.TypeNumber {
			continue
		}
		if short := entityTypeShort(def.EntityType); short != "" {
			out = append(out, "cf_"+short+"_"+def.Key)
		}
	}
	return out
}

// AtomNames returns the indivisible variable names (the leaves of the surface).
func AtomNames() []string { return append([]string(nil), atomNames...) }

// SortedNames returns the built-in variable names in alphabetical order.
func SortedNames() []string {
	out := append([]string(nil), Names...)
	sort.Strings(out)
	return out
}
