// SPDX-License-Identifier: MIT

// Package cardgraph is the pure model + evaluator for the Widget Builder's visual
// programming system: a directed acyclic graph of typed nodes (sources, transforms,
// logic, visualizations) that evaluates to a renderable dashboard card. It has no
// platform dependencies (no syscall/js) and unit-tests on native Go — the wasm/UI
// layer draws the canvas and renders the result, but every bit of language behavior
// (type-checking, cycle detection, evaluation) lives here.
//
// See docs/WIDGET_BUILDER_DESIGN.md for the design. This file holds the core data
// model; node behavior is registered in nodes.go, validation in validate.go, and
// evaluation in eval.go.
package cardgraph

import "strings"

// PortType is the value type carried by a node port. Strong typing is the constraint
// that keeps a visually-wired card valid: an output only connects to a compatible
// input (with a small set of safe coercions, see CanFeed).
type PortType string

const (
	// TypeNumber is a numeric scalar (counts, money in major units, ratios).
	TypeNumber PortType = "number"
	// TypeText is a string scalar (labels, ids, formatted values).
	TypeText PortType = "text"
	// TypeBool is a boolean scalar.
	TypeBool PortType = "bool"
	// TypeColor is a CSS color string (e.g. "#3b82f6").
	TypeColor PortType = "color"
	// TypeCollection is a table of rows with named columns (a dataset/query result).
	TypeCollection PortType = "collection"
	// TypeSeries is an ordered list of (label → number) points (for charts).
	TypeSeries PortType = "series"
	// TypeViz is a renderable block (the output of a visualization node).
	TypeViz PortType = "viz"
)

// CanFeed reports whether a value of type out can be wired into an input of type in,
// allowing the safe coercions the language performs automatically: bool→number (1/0)
// and number/bool→text (for display). Everything else must match exactly.
func CanFeed(out, in PortType) bool {
	if out == in {
		return true
	}
	switch in {
	case TypeNumber:
		return out == TypeBool
	case TypeText:
		return out == TypeNumber || out == TypeBool
	}
	return false
}

// Value is a node's computed output: a tagged union over the scalar types, a data
// Collection, a Series, and a renderable Viz block. A tagged struct (rather than
// interface{}) keeps evaluation and tests explicit. The Type field says which member
// is meaningful. Color reuses Str.
type Value struct {
	Type   PortType
	Num    float64
	Str    string
	Bool   bool
	Coll   *Collection
	Series []SeriesPoint
	Viz    *VizBlock
}

// Row is one record in a Collection: column name → cell value (scalar).
type Row map[string]Value

// Collection is a table: an ordered column list plus the rows. Columns carry their
// type so transforms/displays know which are numeric.
type Collection struct {
	Cols []Column
	Rows []Row
}

// Column describes a Collection column.
type Column struct {
	Name string
	Type PortType // TypeNumber | TypeText | TypeBool
}

// SeriesPoint is one (label, value) datum in a Series.
type SeriesPoint struct {
	Label string
	Value float64
}

// Coll builds a collection value; Series and Color are the sibling constructors.
func Coll(c Collection) Value   { return Value{Type: TypeCollection, Coll: &c} }
func Ser(s []SeriesPoint) Value { return Value{Type: TypeSeries, Series: s} }
func Color(hex string) Value    { return Value{Type: TypeColor, Str: hex} }

// VizBlock is a renderable result the wasm layer turns into a dashboard tile body.
// Kind selects the renderer; the other fields are interpreted per Kind:
//   - "kpi":      Text (big figure), Tone
//   - "text":     Text (a paragraph/label)
//   - "progress": Text (formatted value), Sub (e.g. "of 1,000"), Pct (0..1 fill), Tone
//   - "badge":    Text (label), Tone (color)
//
// List/table/chart blocks extend this in later phases.
type VizBlock struct {
	Kind   string        // "kpi" | "text" | "progress" | "badge" | "chart" | "list" | "stat"
	Title  string        //
	Text   string        // the formatted value/label to display
	Sub    string        // secondary line (e.g. progress denominator, delta)
	Tone   string        // "" | "up" | "down" — display tone
	Accent string        // optional CSS color override (from a design node)
	Pct    float64       // 0..1 fill fraction, for "progress"
	Chart  string        // "line" | "area" | "bar" | "donut" — for "chart"
	Series []SeriesPoint // data for "chart"
	Cols   []Column      // columns for "list"
	Rows   []Row         // rows for "list"
	Hero   bool          // KPI hero variant (larger figure, matches kpiBodyHero)
	Action string        // workflow action kind, for "button"/"toggle"
	XLabel string        // x-axis label, for "chart"
	Blocks []VizBlock    // child blocks, for "stack" (composite multi-block tiles)
}

// Num builds a number value. Text, Bln, and Viz are the sibling constructors.
func Num(n float64) Value  { return Value{Type: TypeNumber, Num: n} }
func Text(s string) Value  { return Value{Type: TypeText, Str: s} }
func Bln(b bool) Value     { return Value{Type: TypeBool, Bool: b} }
func Viz(v VizBlock) Value { return Value{Type: TypeViz, Viz: &v} }

// AsNumber coerces a scalar value to a float64 (bool→1/0), or reports !ok for a
// non-numeric value.
func (v Value) AsNumber() (float64, bool) {
	switch v.Type {
	case TypeNumber:
		return v.Num, true
	case TypeBool:
		if v.Bool {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// NodeID identifies a node within a graph.
type NodeID string

// Point is a node's position on the canvas (UI-only; ignored by evaluation).
type Point struct{ X, Y float64 }

// Node is one step in the graph. Kind selects a registered behavior (see nodes.go);
// Props holds that node's configuration (a literal's value, a formula's expression,
// a scalar source's variable name, a KPI's title/format, …) as plain strings so the
// whole graph serializes losslessly to JSON.
type Node struct {
	ID   NodeID `json:"id"`
	Kind string `json:"kind"`
	// Var is an optional user-given variable name bound to this node's output. Once
	// named, any downstream node evaluated after it can reference the value by that
	// name — e.g. a formula node can write "income - rent" if upstream nodes are named
	// "income" and "rent" — without an explicit wire. Empty = unnamed. Must be a valid
	// identifier (letter/underscore then letters/digits/underscore) and unique.
	Var   string            `json:"var,omitempty"`
	Pos   Point             `json:"pos"`
	Props map[string]string `json:"props,omitempty"`
}

// ValidIdent reports whether s is a usable variable name (formula identifier rules).
func ValidIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		isLetter := c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		isDigit := c >= '0' && c <= '9'
		if i == 0 && !isLetter {
			return false
		}
		if i > 0 && !isLetter && !isDigit {
			return false
		}
	}
	return true
}

// Prop returns the named prop or "" if unset.
func (n Node) Prop(key string) string { return n.Props[key] }

// Edge wires one node's output port to another node's input port. Ports are named
// "nodeID:portName"; the single output port is conventionally named "out".
type Edge struct {
	From PortRef `json:"from"`
	To   PortRef `json:"to"`
}

// PortRef names a specific port on a specific node.
type PortRef struct {
	Node NodeID `json:"node"`
	Port string `json:"port"`
}

// String renders a port ref as "nodeID:portName".
func (p PortRef) String() string { return string(p.Node) + ":" + p.Port }

// ParsePortRef parses "nodeID:portName" into a PortRef. A missing colon yields the
// whole string as the node id with an empty port.
func ParsePortRef(s string) PortRef {
	if node, port, ok := strings.Cut(s, ":"); ok {
		return PortRef{Node: NodeID(node), Port: port}
	}
	return PortRef{Node: NodeID(s)}
}

// OutPort is the conventional name of a node's single output port.
const OutPort = "out"

// Graph is a complete card definition: nodes, the wires between them, and the id of
// the root output node whose Viz value is the rendered card.
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
	Root  NodeID `json:"root"`
}

// node returns the node with the given id and whether it was found.
func (g Graph) node(id NodeID) (Node, bool) {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}
