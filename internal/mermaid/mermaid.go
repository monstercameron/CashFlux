// Package mermaid turns tested domain models into Mermaid diagram source text. It
// is pure Go (no syscall/js): the wasm layer's ui.Mermaid component renders the
// returned source to SVG. Generating diagrams from the model (not free text) keeps
// the determinism/explainability rule, and Escape sanitizes any label so
// user/AI/imported text can't break the syntax or inject markup (the renderer's
// strict mode is the second layer — C45).
package mermaid

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/split"
	"github.com/monstercameron/CashFlux/internal/workflow"
)

// Escape sanitizes a string for use as a Mermaid node/edge label: control
// characters and newlines collapse to spaces, embedded double quotes become single
// quotes (a bare " ends a quoted label), and angle brackets are dropped so no raw
// HTML reaches the renderer. The result is the inner label text — callers wrap it
// in the shape delimiters and quotes.
func Escape(label string) string {
	var b strings.Builder
	b.Grow(len(label))
	prevSpace := false
	for _, r := range label {
		switch {
		case r == '"':
			b.WriteByte('\'')
			prevSpace = false
		case r == '<':
			// entity-escape rather than drop, so comparison operators in condition
			// formulas survive ("amount > 100") while no raw tag can form (XSS)
			b.WriteString("&lt;")
			prevSpace = false
		case r == '>':
			b.WriteString("&gt;")
			prevSpace = false
		case r == ' ' || r == '\n' || r == '\r' || r == '\t' || r < 0x20:
			// collapse any run of whitespace (incl. control chars) to one space
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		default:
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

// Shape is a flowchart node shape.
type Shape int

const (
	ShapeBox     Shape = iota // ["label"] — a process/action
	ShapeRound                // ("label") — a start/terminal
	ShapeDiamond              // {"label"} — a decision/condition
)

// Flowchart accumulates nodes and edges and renders Mermaid `flowchart` source.
type Flowchart struct {
	dir   string
	nodes []string
	edges []string
}

// NewFlowchart starts a flowchart in the given direction ("TD", "LR", …);
// it defaults to "TD" when direction is empty.
func NewFlowchart(direction string) *Flowchart {
	if strings.TrimSpace(direction) == "" {
		direction = "TD"
	}
	return &Flowchart{dir: direction}
}

// Node adds a node with the given id (assumed Mermaid-safe — callers generate ids),
// label (escaped here), and shape.
func (f *Flowchart) Node(id, label string, shape Shape) *Flowchart {
	safe := Escape(label)
	var decl string
	switch shape {
	case ShapeRound:
		decl = id + "(\"" + safe + "\")"
	case ShapeDiamond:
		decl = id + "{\"" + safe + "\"}"
	default:
		decl = id + "[\"" + safe + "\"]"
	}
	f.nodes = append(f.nodes, decl)
	return f
}

// Edge connects from→to, with an optional (escaped) label.
func (f *Flowchart) Edge(from, to, label string) *Flowchart {
	if l := Escape(label); l != "" {
		f.edges = append(f.edges, from+" -->|\""+l+"\"| "+to)
	} else {
		f.edges = append(f.edges, from+" --> "+to)
	}
	return f
}

// String renders the full `flowchart` source.
func (f *Flowchart) String() string {
	var b strings.Builder
	b.WriteString("flowchart " + f.dir + "\n")
	for _, n := range f.nodes {
		b.WriteString("  " + n + "\n")
	}
	for _, e := range f.edges {
		b.WriteString("  " + e + "\n")
	}
	return b.String()
}

// FromWorkflow renders a workflow as a flowchart: a trigger terminal → an optional
// condition diamond → the actions in order. The edge out of a condition is labelled
// "yes" (the path taken when the condition holds — the dry-run path, C65).
func FromWorkflow(w workflow.Workflow) string {
	f := NewFlowchart("TD")
	f.Node("trig", triggerLabel(w.Trigger.Kind), ShapeRound)
	prev := "trig"
	if strings.TrimSpace(w.Condition) != "" {
		f.Node("cond", "if "+w.Condition, ShapeDiamond)
		f.Edge(prev, "cond", "")
		prev = "cond"
	}
	for i, a := range w.Actions {
		id := "a" + strconv.Itoa(i)
		f.Node(id, actionLabel(a), ShapeBox)
		edge := ""
		if prev == "cond" {
			edge = "yes"
		}
		f.Edge(prev, id, edge)
		prev = id
	}
	return f.String()
}

// FromCategories renders a category hierarchy as a left-to-right graph: each
// category is a node, and every category with a known parent gets a parent→child
// edge (so the tree's nesting is visible). Node ids are generated (c0, c1, …) so a
// category ID containing Mermaid-unsafe characters can never break the syntax;
// labels are escaped. Orphan parent references (parent not in the set) render the
// child as a root rather than a dangling edge.
func FromCategories(cats []domain.Category) string {
	f := NewFlowchart("LR")
	idToNode := make(map[string]string, len(cats))
	for i, c := range cats {
		idToNode[c.ID] = "c" + strconv.Itoa(i)
	}
	for i, c := range cats {
		f.Node("c"+strconv.Itoa(i), c.Name, ShapeBox)
	}
	for i, c := range cats {
		if c.ParentID == "" {
			continue
		}
		if parent, ok := idToNode[c.ParentID]; ok {
			f.Edge(parent, "c"+strconv.Itoa(i), "")
		}
	}
	return f.String()
}

// FromSettleUp renders a split settle-up plan as a who-owes-whom digraph: each
// person is a node and each transfer is a debtor→creditor edge labelled with the
// amount. name resolves a member id to a display name; amount formats the minor-unit
// amount (callers pass their money formatter, so this package stays currency-free).
// Node ids are generated per person so member ids never break the syntax.
func FromSettleUp(transfers []split.Transfer, name func(string) string, amount func(int64) string) string {
	f := NewFlowchart("LR")
	nodeOf := make(map[string]string)
	next := 0
	node := func(id string) string {
		if n, ok := nodeOf[id]; ok {
			return n
		}
		n := "m" + strconv.Itoa(next)
		next++
		nodeOf[id] = n
		label := id
		if name != nil {
			if nm := name(id); nm != "" {
				label = nm
			}
		}
		f.Node(n, label, ShapeRound)
		return n
	}
	for _, tr := range transfers {
		lbl := ""
		if amount != nil {
			lbl = amount(tr.Amount)
		}
		f.Edge(node(tr.From), node(tr.To), lbl)
	}
	return f.String()
}

// SankeyFlow is one weighted link in a Sankey diagram: Value units flow From → To.
type SankeyFlow struct {
	From  string
	To    string
	Value int64
}

// Sankey renders flows as Mermaid `sankey-beta` source (e.g. income → categories →
// savings/debt money-flow). The format is CSV (source,target,value), so labels are
// CSV-quoted here rather than via Escape. Flows with a non-positive value are
// skipped (Sankey weights must be positive).
func Sankey(flows []SankeyFlow) string {
	var b strings.Builder
	b.WriteString("sankey-beta\n\n")
	for _, fl := range flows {
		if fl.Value <= 0 {
			continue
		}
		b.WriteString(csvField(fl.From) + "," + csvField(fl.To) + "," + strconv.FormatInt(fl.Value, 10) + "\n")
	}
	return b.String()
}

// csvField CSV-quotes a Sankey label when it contains a comma, quote, or newline
// (doubling embedded quotes), so a label can't shift the row's columns.
func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func triggerLabel(k workflow.TriggerKind) string {
	switch k {
	case workflow.TriggerTxnAdded:
		return "When a transaction is added"
	default:
		return "Run manually"
	}
}

func actionLabel(a workflow.Action) string {
	switch a.Kind {
	case workflow.ActionCreateTask:
		return "Create task: " + a.Title
	case workflow.ActionApplyRules:
		return "Apply rules"
	case workflow.ActionNotify:
		return "Notify: " + a.Message
	case workflow.ActionSetCategory:
		return "Set category"
	case workflow.ActionAddTag:
		return "Add tag: " + a.Tag
	case workflow.ActionFlagReview:
		return "Flag for review"
	default:
		return string(a.Kind)
	}
}
