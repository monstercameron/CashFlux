// SPDX-License-Identifier: MIT

package cardgraph

import (
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/rules"

	"github.com/monstercameron/CashFlux/internal/formula"
)

// Port describes one input port of a node kind: a stable name and the type it
// accepts. Output is always a single port named OutPort with the kind's Out type.
type Port struct {
	Name string
	Type PortType
}

// Context is the evaluation environment a graph runs against: the named numeric and
// string figures a Source/Formula node references, plus the named datasets a
// source.dataset node reads (transactions, accounts, …). The wasm layer builds this
// from engineenv.Vars(...) and appstate so the core stays decoupled and unit-testable
// with hand-built maps.
type Context struct {
	Vars     map[string]float64
	Strs     map[string]string
	Datasets map[string]Collection
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
	KindVizChart      = "viz.chart"
	KindVizList       = "viz.list"
	KindVizStat       = "viz.stat"
	KindSourceDataset = "source.dataset"
	KindFilter        = "data.filter"
	KindGroupBy       = "data.groupby"
	KindAggregate     = "data.aggregate"
	KindRule          = "data.rule"
	KindLiteralColor  = "literal.color"
	KindVizStack      = "viz.stack"
	KindUIButton      = "ui.button"
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
		Inputs: []Port{{Name: "value", Type: TypeNumber}, {Name: "sub", Type: TypeText}},
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
			// Optional sub-line (e.g. a "▲ 4% this month" context line) + hero variant,
			// so a cloned KPI tile matches the dashboard's kpiBody/kpiBodyHero.
			sub := props["sub"]
			if sv, ok := inputs["sub"]; ok && sv.Str != "" {
				sub = sv.Str
			}
			return Viz(VizBlock{Kind: "kpi", Title: props["title"], Text: text, Tone: tone, Sub: sub, Hero: props["hero"] == "true"}), nil
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

	// source.dataset — emits a named app dataset as a Collection (props["which"] one of
	// transactions/accounts/budgets/goals/tasks/bills). The rows come from Context.
	register(Spec{
		Kind: KindSourceDataset, Out: TypeCollection,
		Eval: func(_ map[string]Value, props map[string]string, ctx Context) (Value, error) {
			which := strings.TrimSpace(props["which"])
			if which == "" {
				return Value{}, fmt.Errorf("source.dataset: pick a dataset")
			}
			c, ok := ctx.Datasets[which]
			if !ok {
				return Coll(Collection{}), nil // no data yet → empty collection, not an error
			}
			return Coll(c), nil
		},
	})

	// data.filter — keep rows where column props["col"] satisfies props["op"] vs
	// props["value"]. Numeric columns compare numerically; others compare as text
	// (== / != / contains, case-insensitive). A blank column passes everything through.
	register(Spec{
		Kind: KindFilter, Out: TypeCollection,
		Inputs: []Port{{Name: "in", Type: TypeCollection}},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			in, ok := inputs["in"]
			if !ok || in.Coll == nil {
				return Coll(Collection{}), nil
			}
			col := strings.TrimSpace(props["col"])
			if col == "" {
				return in, nil
			}
			op, want := props["op"], props["value"]
			out := Collection{Cols: in.Coll.Cols}
			for _, row := range in.Coll.Rows {
				if filterMatch(row[col], op, want) {
					out.Rows = append(out.Rows, row)
				}
			}
			return Coll(out), nil
		},
	})

	// data.groupby — group rows by the text column props["group"], aggregating the
	// numeric column props["value"] with props["fn"] (sum/avg/count/min/max). Outputs a
	// Series (one point per group), sorted by value descending — ready for a chart.
	register(Spec{
		Kind: KindGroupBy, Out: TypeSeries,
		Inputs: []Port{{Name: "in", Type: TypeCollection}},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			in, ok := inputs["in"]
			if !ok || in.Coll == nil {
				return Ser(nil), nil
			}
			group := strings.TrimSpace(props["group"])
			valCol := strings.TrimSpace(props["value"])
			fn := props["fn"]
			if group == "" {
				return Value{}, fmt.Errorf("data.groupby: pick a column to group by")
			}
			type acc struct {
				sum, min, max float64
				n             int
			}
			order := []string{}
			groups := map[string]*acc{}
			for _, row := range in.Coll.Rows {
				key := row[group].Str
				if key == "" {
					if n, ok := row[group].AsNumber(); ok {
						key = formatNumber(n, "")
					}
				}
				a := groups[key]
				if a == nil {
					a = &acc{}
					groups[key] = a
					order = append(order, key)
				}
				v, _ := row[valCol].AsNumber()
				if a.n == 0 || v < a.min {
					a.min = v
				}
				if a.n == 0 || v > a.max {
					a.max = v
				}
				a.sum += v
				a.n++
			}
			pts := make([]SeriesPoint, 0, len(order))
			for _, k := range order {
				a := groups[k]
				pts = append(pts, SeriesPoint{Label: k, Value: aggValue(fn, a.sum, a.min, a.max, a.n)})
			}
			sort.SliceStable(pts, func(i, j int) bool { return pts[i].Value > pts[j].Value })
			return Ser(pts), nil
		},
	})

	// data.aggregate — reduce a collection's numeric column props["col"] to a single
	// Number via props["fn"] (sum/avg/count/min/max). count ignores the column.
	register(Spec{
		Kind: KindAggregate, Out: TypeNumber,
		Inputs: []Port{{Name: "in", Type: TypeCollection}},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			in, ok := inputs["in"]
			if !ok || in.Coll == nil {
				return Num(0), nil
			}
			col, fn := strings.TrimSpace(props["col"]), props["fn"]
			var sum, min, max float64
			n := 0
			for _, row := range in.Coll.Rows {
				v, _ := row[col].AsNumber()
				if n == 0 || v < min {
					min = v
				}
				if n == 0 || v > max {
					max = v
				}
				sum += v
				n++
			}
			return Num(aggValue(fn, sum, min, max, n)), nil
		},
	})

	// viz.chart — renders a Series as a chart (props["chart"] = line|bar|donut).
	register(Spec{
		Kind: KindVizChart, Out: TypeViz,
		Inputs: []Port{{Name: "series", Type: TypeSeries}, {Name: "accent", Type: TypeColor}},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			s := inputs["series"].Series
			chart := props["chart"]
			if chart == "" {
				chart = "line"
			}
			return Viz(VizBlock{Kind: "chart", Title: props["title"], Chart: chart, Series: s, Accent: inputs["accent"].Str}), nil
		},
	})

	// viz.list — renders a Collection as a list/table, capped at props["limit"] rows.
	register(Spec{
		Kind: KindVizList, Out: TypeViz,
		Inputs: []Port{{Name: "in", Type: TypeCollection}},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			in, ok := inputs["in"]
			if !ok || in.Coll == nil {
				return Viz(VizBlock{Kind: "list", Title: props["title"]}), nil
			}
			rows := in.Coll.Rows
			if lim, err := strconv.Atoi(strings.TrimSpace(props["limit"])); err == nil && lim > 0 && lim < len(rows) {
				rows = rows[:lim]
			}
			return Viz(VizBlock{Kind: "list", Title: props["title"], Cols: in.Coll.Cols, Rows: rows}), nil
		},
	})

	// viz.stat — a KPI plus its change vs a previous value: inputs "value" and "prev"
	// produce a delta % with up/down tone. props["title"], props["format"].
	register(Spec{
		Kind: KindVizStat, Out: TypeViz,
		Inputs: []Port{{Name: "value", Type: TypeNumber}, {Name: "prev", Type: TypeNumber}, {Name: "accent", Type: TypeColor}},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			vv, ok := inputs["value"]
			if !ok {
				return Value{}, fmt.Errorf("viz.stat: connect a value")
			}
			val, _ := vv.AsNumber()
			sub, tone := "", ""
			if pv, ok := inputs["prev"]; ok {
				prev, _ := pv.AsNumber()
				if prev != 0 {
					d := (val - prev) / prev * 100
					switch {
					case d > 0:
						tone, sub = "up", "▲ "+formatNumber(d, "")+"%"
					case d < 0:
						tone, sub = "down", "▼ "+formatNumber(-d, "")+"%"
					default:
						sub = "no change"
					}
				}
			}
			return Viz(VizBlock{Kind: "stat", Title: props["title"], Text: formatNumber(val, props["format"]), Sub: sub, Tone: tone, Accent: inputs["accent"].Str}), nil
		},
	})

	// literal.color — a fixed CSS color, for wiring into a display node's accent port.
	register(Spec{
		Kind: KindLiteralColor, Out: TypeColor,
		Eval: func(_ map[string]Value, props map[string]string, _ Context) (Value, error) {
			c := strings.TrimSpace(props["value"])
			if c == "" {
				c = "#3b82f6"
			}
			return Color(c), nil
		},
	})

	// data.rule — applies the auto-categorization rules engine (internal/rules) as a
	// collection filter: keep rows whose Text column contains the keyword(s) and fall
	// within the amount range. This is the "rule node" — a saved rule embedded in the
	// graph. props: textcol, amountcol, any (comma keywords), min, max.
	register(Spec{
		Kind: KindRule, Out: TypeCollection,
		Inputs: []Port{{Name: "in", Type: TypeCollection}},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			in, ok := inputs["in"]
			if !ok || in.Coll == nil {
				return Coll(Collection{}), nil
			}
			textCol := strings.TrimSpace(props["textcol"])
			amountCol := strings.TrimSpace(props["amountcol"])
			var any []string
			for _, k := range strings.Split(props["any"], ",") {
				if s := strings.TrimSpace(k); s != "" {
					any = append(any, s)
				}
			}
			minA, _ := strconv.ParseFloat(strings.TrimSpace(props["min"]), 64)
			maxA, _ := strconv.ParseFloat(strings.TrimSpace(props["max"]), 64)
			cond := rules.Condition{AnyKeywords: any, MinAmount: int64(minA), MaxAmount: int64(maxA)}
			out := Collection{Cols: in.Coll.Cols}
			for _, row := range in.Coll.Rows {
				amt, _ := row[amountCol].AsNumber()
				tv := rules.TxnView{Text: row[textCol].Str, Amount: int64(amt)}
				if cond.Matches(tv) {
					out.Rows = append(out.Rows, row)
				}
			}
			return Coll(out), nil
		},
	})

	// ui.button — an interactive button that runs a workflow action when clicked (the
	// builder's basic interactivity, like the dashboard To-do tile's controls). props:
	// label, action (a workflow.Action kind such as postRecurring/applyRules).
	register(Spec{
		Kind: KindUIButton, Out: TypeViz,
		Eval: func(_ map[string]Value, props map[string]string, _ Context) (Value, error) {
			label := props["label"]
			if strings.TrimSpace(label) == "" {
				label = "Run"
			}
			return Viz(VizBlock{Kind: "button", Text: label, Action: props["action"]}), nil
		},
	})

	// viz.stack — composes up to four child visualizations into one card (header + chart
	// + list composites, like the real dashboard tiles). Each input accepts a Viz; the
	// non-empty ones render top-to-bottom.
	register(Spec{
		Kind: KindVizStack, Out: TypeViz,
		Inputs: []Port{
			{Name: "block1", Type: TypeViz}, {Name: "block2", Type: TypeViz},
			{Name: "block3", Type: TypeViz}, {Name: "block4", Type: TypeViz},
		},
		Eval: func(inputs map[string]Value, props map[string]string, _ Context) (Value, error) {
			var blocks []VizBlock
			for _, name := range []string{"block1", "block2", "block3", "block4"} {
				if v, ok := inputs[name]; ok && v.Type == TypeViz && v.Viz != nil {
					blocks = append(blocks, *v.Viz)
				}
			}
			if len(blocks) == 0 {
				return Value{}, fmt.Errorf("viz.stack: connect at least one block")
			}
			return Viz(VizBlock{Kind: "stack", Title: props["title"], Blocks: blocks}), nil
		},
	})
}

// filterMatch evaluates a single-column predicate for data.filter. Numeric comparisons
// when the cell is numeric; otherwise case-insensitive text equality/contains.
func filterMatch(cell Value, op, want string) bool {
	if n, ok := cell.AsNumber(); ok {
		w, err := strconv.ParseFloat(strings.TrimSpace(want), 64)
		if err == nil {
			switch op {
			case "==":
				return n == w
			case "!=":
				return n != w
			case "<":
				return n < w
			case "<=":
				return n <= w
			case ">":
				return n > w
			case ">=":
				return n >= w
			}
		}
	}
	s, w := strings.ToLower(cell.Str), strings.ToLower(strings.TrimSpace(want))
	switch op {
	case "!=":
		return s != w
	case "contains":
		return strings.Contains(s, w)
	default: // "==" and unknown ops
		return s == w
	}
}

// aggValue applies an aggregation function to pre-collected stats.
func aggValue(fn string, sum, min, max float64, n int) float64 {
	switch fn {
	case "count":
		return float64(n)
	case "avg":
		if n == 0 {
			return 0
		}
		return sum / float64(n)
	case "min":
		return min
	case "max":
		return max
	default: // sum
		return sum
	}
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
