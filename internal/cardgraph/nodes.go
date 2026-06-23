package cardgraph

import (
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/formula"
)

// Port describes one input port of a node kind: a stable name and the type it
// accepts. Output is always a single port named OutPort with the kind's Out type.
type Port struct {
	Name string
	Type PortType
}

// Context is the evaluation environment a graph runs against: the named numeric and
// string figures a Source/Formula node can reference. The wasm layer builds this from
// engineenv.Vars(...) (the app variable surface) so the core stays decoupled from the
// dataset and unit-testable with a hand-built map.
type Context struct {
	Vars map[string]float64
	Strs map[string]string
}

// Spec is a node kind's contract: its input ports, its output type, and a pure
// evaluator. inputs holds the resolved value for each connected input port (keyed by
// port name); a missing input means that port is unwired. props is the node's config.
type Spec struct {
	Kind   string
	Inputs []Port
	Out    PortType
	Eval   func(inputs map[string]Value, props map[string]string, ctx Context) (Value, error)
}

// registry maps a node kind to its spec. Adding a node kind = registering a Spec here;
// the palette, type-checker, and evaluator are all derived from this single source.
var registry = map[string]Spec{}

func register(s Spec) { registry[s.Kind] = s }

// Lookup returns the spec for a node kind and whether it is registered.
func Lookup(kind string) (Spec, bool) { s, ok := registry[kind]; return s, ok }

// Kinds returns the registered node kinds in sorted order (stable palette listing).
func Kinds() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Node kind identifiers. v1 ships a minimal but real set proving the full pipeline:
// a literal and a live figure as sources, a formula as the transform/logic layer, and
// a KPI as the visualization. Later phases add dataset/filter/aggregate/list/chart.
const (
	KindLiteralNumber = "literal.number"
	KindLiteralText   = "literal.text"
	KindLiteralBool   = "literal.bool"
	KindSourceScalar  = "source.scalar"
	KindFormula       = "formula"
	KindCompare       = "logic.compare"
	KindBranchNumber  = "logic.branch.number"
	KindVizKPI        = "viz.kpi"
	KindVizText       = "viz.text"
	KindVizProgress   = "viz.progress"
	KindVizBadge      = "viz.badge"
)

func init() {
	// literal.number — emits a fixed number from props["value"].
	register(Spec{
		Kind: KindLiteralNumber, Out: TypeNumber,
		Eval: func(_ map[string]Value, props map[string]string, _ Context) (Value, error) {
			n, err := strconv.ParseFloat(strings.TrimSpace(props["value"]), 64)
			if err != nil {
				return Value{}, fmt.Errorf("literal.number: %q is not a number", props["value"])
			}
			return Num(n), nil
		},
	})

	// literal.text — emits a fixed string from props["value"].
	register(Spec{
		Kind: KindLiteralText, Out: TypeText,
		Eval: func(_ map[string]Value, props map[string]string, _ Context) (Value, error) {
			return Text(props["value"]), nil
		},
	})

	// literal.bool — emits a fixed boolean from props["value"] ("true" = true).
	register(Spec{
		Kind: KindLiteralBool, Out: TypeBool,
		Eval: func(_ map[string]Value, props map[string]string, _ Context) (Value, error) {
			return Bln(strings.EqualFold(strings.TrimSpace(props["value"]), "true")), nil
		},
	})

	// logic.compare — compares its two numeric inputs with props["op"] (one of
	// == != < <= > >=) and outputs a bool. The visual, n8n-style alternative to
	// writing a comparison in a formula; its bool output can drive a branch.
	register(Spec{
		Kind: KindCompare, Out: TypeBool,
		Inputs: []Port{{Name: "a", Type: TypeNumber}, {Name: "b", Type: TypeNumber}},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			a, aok := inputs["a"]
			b, bok := inputs["b"]
			if !aok || !bok {
				return Value{}, fmt.Errorf("logic.compare: connect both inputs")
			}
			an, _ := a.AsNumber()
			bn, _ := b.AsNumber()
			switch op := strings.TrimSpace(props["op"]); op {
			case "==":
				return Bln(an == bn), nil
			case "!=":
				return Bln(an != bn), nil
			case "<":
				return Bln(an < bn), nil
			case "<=":
				return Bln(an <= bn), nil
			case ">":
				return Bln(an > bn), nil
			case ">=":
				return Bln(an >= bn), nil
			default:
				return Value{}, fmt.Errorf("logic.compare: unknown operator %q", op)
			}
		},
	})

	// logic.branch.number — picks between two numbers by a boolean condition: when
	// "cond" is true it outputs "whenTrue", else "whenFalse". The conditional output
	// behind cards that adapt to state (e.g. an "over budget" figure). v1 branches on
	// numbers; text/viz branches follow the same shape in later phases.
	register(Spec{
		Kind: KindBranchNumber, Out: TypeNumber,
		Inputs: []Port{
			{Name: "cond", Type: TypeBool},
			{Name: "whenTrue", Type: TypeNumber},
			{Name: "whenFalse", Type: TypeNumber},
		},
		Eval: func(inputs map[string]Value, _ map[string]string, _ Context) (Value, error) {
			cond, ok := inputs["cond"]
			if !ok {
				return Value{}, fmt.Errorf("logic.branch: connect a condition")
			}
			pick := "whenFalse"
			if cond.Bool {
				pick = "whenTrue"
			}
			v, ok := inputs[pick]
			if !ok {
				return Value{}, fmt.Errorf("logic.branch: connect the %q value", pick)
			}
			return v, nil
		},
	})

	// source.scalar — reads a live figure from the app variable surface by name
	// (props["name"], e.g. "net_worth", "income"). Numeric vars win over strings.
	register(Spec{
		Kind: KindSourceScalar, Out: TypeNumber,
		Eval: func(_ map[string]Value, props map[string]string, ctx Context) (Value, error) {
			name := strings.TrimSpace(props["name"])
			if name == "" {
				return Value{}, fmt.Errorf("source.scalar: no figure chosen")
			}
			if v, ok := ctx.Vars[name]; ok {
				return Num(v), nil
			}
			if s, ok := ctx.Strs[name]; ok {
				return Text(s), nil
			}
			return Value{}, fmt.Errorf("source.scalar: unknown figure %q", name)
		},
	})

	// formula — evaluates props["expr"] with its two numeric inputs exposed as the
	// variables "a" and "b", plus the whole app variable surface from ctx. Reuses the
	// sandboxed expression engine verbatim; the result may be a number or bool.
	register(Spec{
		Kind: KindFormula, Out: TypeNumber,
		Inputs: []Port{{Name: "a", Type: TypeNumber}, {Name: "b", Type: TypeNumber}},
		Eval: func(inputs map[string]Value, props map[string]string, ctx Context) (Value, error) {
			expr := strings.TrimSpace(props["expr"])
			if expr == "" {
				return Value{}, fmt.Errorf("formula: no expression set")
			}
			vars := map[string]float64{}
			maps.Copy(vars, ctx.Vars)
			for _, name := range []string{"a", "b"} {
				if in, ok := inputs[name]; ok {
					if n, ok := in.AsNumber(); ok {
						vars[name] = n
					}
				}
			}
			res, err := formula.Eval(expr, formula.Env{Vars: vars, Strs: ctx.Strs})
			if err != nil {
				return Value{}, err
			}
			switch n := res.(type) {
			case float64:
				return Num(n), nil
			case bool:
				return Bln(n), nil
			default:
				return Value{}, fmt.Errorf("formula: expected a number, got %T", res)
			}
		},
	})

	// viz.kpi — renders its single input as a KPI block. props["title"] labels it;
	// props["format"] (number|percent|currency) controls display; props["tone"] sets
	// a fixed tone, or "auto" derives up/down from the sign of a numeric value.
	register(Spec{
		Kind: KindVizKPI, Out: TypeViz,
		Inputs: []Port{{Name: "value", Type: TypeNumber}},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			in, ok := inputs["value"]
			if !ok {
				return Value{}, fmt.Errorf("viz.kpi: connect a value")
			}
			text := in.Str
			tone := props["tone"]
			if n, ok := in.AsNumber(); ok {
				text = formatNumber(n, props["format"])
				if tone == "auto" {
					switch {
					case n > 0:
						tone = "up"
					case n < 0:
						tone = "down"
					default:
						tone = ""
					}
				}
			}
			if tone == "auto" {
				tone = ""
			}
			return Viz(VizBlock{Kind: "kpi", Title: props["title"], Text: text, Tone: tone}), nil
		},
	})

	// viz.text — renders its input as a text/label block (number coerced to text).
	// props["title"] labels it. Good for showing a name, category, or status string.
	register(Spec{
		Kind: KindVizText, Out: TypeViz,
		Inputs: []Port{{Name: "value", Type: TypeText}},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			in, ok := inputs["value"]
			if !ok {
				return Value{}, fmt.Errorf("viz.text: connect a value")
			}
			text := in.Str
			if n, ok := in.AsNumber(); ok && in.Type != TypeText {
				text = formatNumber(n, "")
			}
			return Viz(VizBlock{Kind: "text", Title: props["title"], Text: text}), nil
		},
	})

	// viz.progress — a progress bar: "value" of "max". props["title"] labels it,
	// props["format"] formats the value. Pct is value/max clamped to 0..1; the bar is
	// toned up when full. A missing/zero max degrades to an empty bar (no divide-by-0).
	register(Spec{
		Kind: KindVizProgress, Out: TypeViz,
		Inputs: []Port{{Name: "value", Type: TypeNumber}, {Name: "max", Type: TypeNumber}},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			vv, ok := inputs["value"]
			if !ok {
				return Value{}, fmt.Errorf("viz.progress: connect a value")
			}
			val, _ := vv.AsNumber()
			max := 0.0
			if mv, ok := inputs["max"]; ok {
				max, _ = mv.AsNumber()
			}
			pct := 0.0
			if max > 0 {
				pct = val / max
				if pct < 0 {
					pct = 0
				}
				if pct > 1 {
					pct = 1
				}
			}
			tone := ""
			if max > 0 && val >= max {
				tone = "up"
			}
			return Viz(VizBlock{
				Kind: "progress", Title: props["title"],
				Text: formatNumber(val, props["format"]),
				Sub:  "of " + formatNumber(max, props["format"]),
				Tone: tone, Pct: pct,
			}), nil
		},
	})

	// viz.badge — a small toned label. props["title"] labels it; the input becomes the
	// badge text; props["tone"] sets the color, or "auto" derives up/down from a number.
	register(Spec{
		Kind: KindVizBadge, Out: TypeViz,
		Inputs: []Port{{Name: "value", Type: TypeText}},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			in, ok := inputs["value"]
			if !ok {
				return Value{}, fmt.Errorf("viz.badge: connect a value")
			}
			text := in.Str
			tone := props["tone"]
			if n, ok := in.AsNumber(); ok && in.Type != TypeText {
				text = formatNumber(n, "")
				if tone == "auto" {
					switch {
					case n > 0:
						tone = "up"
					case n < 0:
						tone = "down"
					default:
						tone = ""
					}
				}
			}
			if tone == "auto" {
				tone = ""
			}
			return Viz(VizBlock{Kind: "badge", Title: props["title"], Text: text, Tone: tone}), nil
		},
	})
}

// formatNumber renders a KPI figure per format. Currency is left to the caller (it
// needs the base currency), so it falls back to a plain number here; percent appends
// "%"; number trims trailing zeros. Mirrors widgetspec.Format so display agrees.
func formatNumber(v float64, format string) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if format == "percent" {
		return s + "%"
	}
	return s
}
