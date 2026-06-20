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
