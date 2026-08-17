// SPDX-License-Identifier: MIT

package reports

// This file builds the money-flow diagram's DATA — which nodes exist, what each
// one is worth, and which categories sit behind it — separately from the
// geometry (sankey.go) and from the SVG the wasm layer draws. It lives in the
// pure package so the picture and the assistant read the same numbers: the
// Annual Review renders these edges, and the assistant's money-flow tool reports
// them and traces each ribbon back to the category ids that produced it. Before
// this, the node/edge construction lived in the view, so it could be looked at
// but never queried.

import "sort"

// FlowKind names what one money-flow node is, so a caller can style it, explain
// it, or trace it without re-deriving the meaning from the label text.
type FlowKind string

const (
	// FlowIncomeSource is one income category feeding the hub.
	FlowIncomeSource FlowKind = "income_source"
	// FlowOtherIncome is the pooled tail of income categories past the cap.
	FlowOtherIncome FlowKind = "other_income"
	// FlowIncomeHub is the middle column — everything that came in.
	FlowIncomeHub FlowKind = "income"
	// FlowSpendCategory is one spending category the hub feeds.
	FlowSpendCategory FlowKind = "spending_category"
	// FlowOtherSpending is the pooled tail of spending categories past the cap.
	FlowOtherSpending FlowKind = "other_spending"
	// FlowSavings is what was left over — income the period did not spend.
	FlowSavings FlowKind = "savings"
	// FlowFromSavings is the inflow that names an overspend: without it the hub
	// silently grows to the expense total and the gap is invisible.
	FlowFromSavings FlowKind = "from_savings"
)

// Pooled reports whether the node stands for several categories at once, so a
// caller can say "these five categories" instead of implying one.
func (k FlowKind) Pooled() bool { return k == FlowOtherIncome || k == FlowOtherSpending }

// MoneyFlowLabels are the display names for the diagram's synthetic nodes — the
// ones that are not a category. They are passed in (rather than hard-coded)
// because the UI translates them; empty fields fall back to plain English so a
// non-UI caller can build the diagram without an i18n table.
type MoneyFlowLabels struct {
	Income         string // the hub
	OtherIncome    string // pooled income tail
	EverythingElse string // pooled spending tail
	Savings        string // left over
	FromSavings    string // the overspend inflow
	// Disambiguate renames a CATEGORY whose own name collides with the hub's
	// label. Without it, a salary filed under a category literally called
	// "Income" forms a From==To self-loop that the layout drops — the money
	// vanishes from the picture while the headline total still counts it. Nil
	// falls back to appending " (category)".
	Disambiguate func(name string) string
}

func (l MoneyFlowLabels) withDefaults() MoneyFlowLabels {
	if l.Income == "" {
		l.Income = "Income"
	}
	if l.OtherIncome == "" {
		l.OtherIncome = "Other income"
	}
	if l.EverythingElse == "" {
		l.EverythingElse = "Everything else"
	}
	if l.Savings == "" {
		l.Savings = "Savings"
	}
	if l.FromSavings == "" {
		l.FromSavings = "Drawn from savings"
	}
	if l.Disambiguate == nil {
		l.Disambiguate = func(name string) string { return name + " (category)" }
	}
	return l
}

// MoneyFlowInputs is everything the diagram is built from. Income and Spending
// are the period's per-category totals, largest first (as SpendingByCategory and
// IncomeByCategory return them) — their order decides the diagram's stacking
// order and which rows survive the caps. Net is the period's income minus
// expense from the ledger, which is NOT re-derived here: it comes from the same
// figure the report's headline shows, so the picture and the caption agree.
type MoneyFlowInputs struct {
	Income   []CategorySpend
	Spending []CategorySpend
	Net      int64
	// Name resolves a category id to its display name. Nil yields the raw id.
	Name   func(categoryID string) string
	Labels MoneyFlowLabels
	// TopSources caps the named income sources (default 5); the rest pool into
	// one "Other income" node. TopSpending does the same for spending
	// categories (default 10). The caps exist because a diagram with forty
	// ribbons is a texture, not a picture.
	TopSources  int
	TopSpending int
}

// MoneyFlowNode is one node in the diagram: what it is called, what kind of
// thing it is, what it is worth, and — the part the assistant needs — exactly
// which categories it stands for, so a node can be traced back to transactions.
// CategoryIDs is empty for the hub and for the savings/overspend nodes, which
// are computed figures rather than groupings of records.
type MoneyFlowNode struct {
	Label       string
	Kind        FlowKind
	Value       int64
	CategoryIDs []string
}

// MoneyFlowEdge is one ribbon: the money that moved from one node to another,
// with the categories behind it. Kind is the kind of the node at the non-hub end
// — what this ribbon is about.
type MoneyFlowEdge struct {
	From, To    string
	Value       int64
	Kind        FlowKind
	CategoryIDs []string
}

// MoneyFlowDiagram is the assembled money-flow picture: the nodes and ribbons
// plus the three totals that caption it.
type MoneyFlowDiagram struct {
	Nodes []MoneyFlowNode
	Edges []MoneyFlowEdge
	// IncomeTotal and SpendingTotal are the sums the DIAGRAM draws (every row
	// passed in, pooled tail included), so they reconcile against the ribbons.
	IncomeTotal   int64
	SpendingTotal int64
	// Net is the ledger's figure, echoed from the inputs.
	Net int64
}

// Flows renders the diagram as the weighted links LayoutSankey consumes.
func (d MoneyFlowDiagram) Flows() []Flow {
	out := make([]Flow, 0, len(d.Edges))
	for _, e := range d.Edges {
		out = append(out, Flow{From: e.From, To: e.To, Value: e.Value})
	}
	return out
}

// Node returns the node with the given label.
func (d MoneyFlowDiagram) Node(label string) (MoneyFlowNode, bool) {
	for _, n := range d.Nodes {
		if n.Label == label {
			return n, true
		}
	}
	return MoneyFlowNode{}, false
}

// Edge returns the ribbon between two node labels.
func (d MoneyFlowDiagram) Edge(from, to string) (MoneyFlowEdge, bool) {
	for _, e := range d.Edges {
		if e.From == from && e.To == to {
			return e, true
		}
	}
	return MoneyFlowEdge{}, false
}

// BuildMoneyFlow assembles the three-column money-flow diagram: income
// categories → the income hub → spending categories, plus whatever was saved (or
// an explicit "drawn from savings" inflow when the period overspent).
//
// Rows past the caps pool into one node each rather than being dropped, so the
// ribbons still sum to the totals; zero and negative rows are skipped, since a
// ribbon has no direction to draw for them.
func BuildMoneyFlow(in MoneyFlowInputs) MoneyFlowDiagram {
	labels := in.Labels.withDefaults()
	name := in.Name
	if name == nil {
		name = func(id string) string { return id }
	}
	topSources := in.TopSources
	if topSources <= 0 {
		topSources = 5
	}
	topSpending := in.TopSpending
	if topSpending <= 0 {
		topSpending = 10
	}
	// A category whose name equals the hub's would collapse into the hub.
	catLabel := func(id string) string {
		n := name(id)
		if n == labels.Income {
			return labels.Disambiguate(n)
		}
		return n
	}

	d := MoneyFlowDiagram{Net: in.Net}
	hub := MoneyFlowNode{Label: labels.Income, Kind: FlowIncomeHub}
	var sources, sinks []MoneyFlowNode

	// Income sources → the hub.
	var pooledIncome int64
	var pooledIncomeIDs []string
	shown := 0
	for _, r := range in.Income {
		if r.Amount <= 0 {
			continue
		}
		d.IncomeTotal += r.Amount
		if shown < topSources {
			label := catLabel(r.CategoryID)
			sources = append(sources, MoneyFlowNode{Label: label, Kind: FlowIncomeSource, Value: r.Amount, CategoryIDs: []string{r.CategoryID}})
			d.Edges = append(d.Edges, MoneyFlowEdge{From: label, To: hub.Label, Value: r.Amount, Kind: FlowIncomeSource, CategoryIDs: []string{r.CategoryID}})
			shown++
			continue
		}
		pooledIncome += r.Amount
		pooledIncomeIDs = append(pooledIncomeIDs, r.CategoryID)
	}
	if pooledIncome > 0 {
		sources = append(sources, MoneyFlowNode{Label: labels.OtherIncome, Kind: FlowOtherIncome, Value: pooledIncome, CategoryIDs: pooledIncomeIDs})
		d.Edges = append(d.Edges, MoneyFlowEdge{From: labels.OtherIncome, To: hub.Label, Value: pooledIncome, Kind: FlowOtherIncome, CategoryIDs: pooledIncomeIDs})
	}

	// The hub → spending categories.
	var pooledSpend int64
	var pooledSpendIDs []string
	shown = 0
	for _, r := range in.Spending {
		v := absMinorUnits(r.Amount)
		if v == 0 {
			continue
		}
		d.SpendingTotal += v
		if shown < topSpending {
			label := catLabel(r.CategoryID)
			sinks = append(sinks, MoneyFlowNode{Label: label, Kind: FlowSpendCategory, Value: v, CategoryIDs: []string{r.CategoryID}})
			d.Edges = append(d.Edges, MoneyFlowEdge{From: hub.Label, To: label, Value: v, Kind: FlowSpendCategory, CategoryIDs: []string{r.CategoryID}})
			shown++
			continue
		}
		pooledSpend += v
		pooledSpendIDs = append(pooledSpendIDs, r.CategoryID)
	}
	if pooledSpend > 0 {
		sinks = append(sinks, MoneyFlowNode{Label: labels.EverythingElse, Kind: FlowOtherSpending, Value: pooledSpend, CategoryIDs: pooledSpendIDs})
		d.Edges = append(d.Edges, MoneyFlowEdge{From: hub.Label, To: labels.EverythingElse, Value: pooledSpend, Kind: FlowOtherSpending, CategoryIDs: pooledSpendIDs})
	}

	// What was kept — or, when the period overspent, where the gap came from.
	// The deficit ribbon is appended last on purpose: the layout stacks a column
	// in first-appearance order, so this puts the inflow under the real sources.
	var tail []MoneyFlowNode
	if sav := in.Net; sav > 0 {
		tail = append(tail, MoneyFlowNode{Label: labels.Savings, Kind: FlowSavings, Value: sav})
		d.Edges = append(d.Edges, MoneyFlowEdge{From: hub.Label, To: labels.Savings, Value: sav, Kind: FlowSavings})
	} else if def := -sav; def > 0 {
		tail = append(tail, MoneyFlowNode{Label: labels.FromSavings, Kind: FlowFromSavings, Value: def})
		d.Edges = append(d.Edges, MoneyFlowEdge{From: labels.FromSavings, To: hub.Label, Value: def, Kind: FlowFromSavings})
	}

	if len(d.Edges) == 0 {
		return MoneyFlowDiagram{Net: in.Net}
	}
	hub.Value = hubValue(d.Edges, hub.Label)
	d.Nodes = append(d.Nodes, hub)
	d.Nodes = append(d.Nodes, sources...)
	d.Nodes = append(d.Nodes, sinks...)
	d.Nodes = append(d.Nodes, tail...)
	return d
}

// hubValue is what the middle bar is worth: the larger of what reaches it and
// what leaves it, matching how the layout sizes a node that both sends and
// receives.
func hubValue(edges []MoneyFlowEdge, hub string) int64 {
	var in, out int64
	for _, e := range edges {
		switch hub {
		case e.To:
			in += e.Value
		case e.From:
			out += e.Value
		}
	}
	if in > out {
		return in
	}
	return out
}

// absMinorUnits is the magnitude of a signed minor-unit amount. Spending rows
// are documented as absolute, but the diagram must not invert a ribbon if one
// ever arrives signed.
func absMinorUnits(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// FlowCategorySet returns the category ids behind a set of nodes or edges,
// deduplicated and ordered, ready to filter transactions with. Uncategorized
// spend carries an empty id, which is a real selector ("no category") rather
// than a missing value, so it is preserved.
func FlowCategorySet(ids ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, group := range ids {
		for _, id := range group {
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}
