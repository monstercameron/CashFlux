//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/cardgraph"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Bento cell geometry, mirrored from the dashboard grid (.bento --cell + gap) so
// the builder stage previews a tile at its true on-dashboard proportions.
const dashCellPx, dashGapPx = 152, 10

// Pipeline step ids, in left-to-right flow order. Each step's output feeds the
// next: a data source → an optional transform → a visualization.
const (
	wbStepSource    = "source"
	wbStepTransform = "transform"
	wbStepVisualize = "visualize"
)

// Canvas geometry: node box size and the fixed drawing surface the wires are
// computed against (the surface scrolls within its wrapper).
const (
	wbNodeW      = 156.0
	wbNodeH      = 66.0
	wbCanvasW    = 760.0
	wbCanvasH    = 340.0
	wbCanvasWStr = "760"
	wbCanvasHStr = "340"
)

// WidgetBuilder is the widget-creation screen and the front end of the visual
// programming system (see docs/WIDGET_BUILDER_DESIGN.md). Top-to-bottom: a stage
// rendering a LIVE preview tile — the card is built as an internal/cardgraph graph
// (source → optional transform → visualization) and evaluated against the real app
// figures, so the figure shown is your actual data; an n8n-style node canvas with
// draggable boxes and bezier wires; a per-step configuration panel; and a size
// control (width 1–4, height 1–3). The graph is the single source of truth — the
// canvas and the stage both read it.
func WidgetBuilder() ui.Node {
	col := ui.UseState(1)
	row := ui.UseState(1)
	c, r := col.Get(), row.Get()

	active := ui.UseState(wbStepSource)
	// Node positions come from localStorage (written by the wb-canvas.js drag shim),
	// merged over the default layout, so dragged positions survive re-renders.
	positions := wbLoadPositions()

	sourceVar := ui.UseState("net_worth")
	transformExpr := ui.UseState("")
	vizTitle := ui.UseState("")
	vizFormat := ui.UseState("number")

	onSource := ui.UseEvent(func(e ui.Event) { sourceVar.Set(e.GetValue()) })
	onTransform := ui.UseEvent(func(v string) { transformExpr.Set(v) })
	onTitle := ui.UseEvent(func(v string) { vizTitle.Set(v) })
	onFormat := ui.UseEvent(func(e ui.Event) { vizFormat.Set(e.GetValue()) })

	title := vizTitle.Get()
	if strings.TrimSpace(title) == "" {
		title = wbPretty(sourceVar.Get())
	}
	g := wbBuildGraph(sourceVar.Get(), transformExpr.Get(), title, vizFormat.Get())
	res := cardgraph.Eval(g, cardgraph.Context{Vars: wbVariableSurface()})

	span := func(n int) string {
		return strconv.Itoa(n*dashCellPx+(n-1)*dashGapPx) + "px"
	}
	setCol := func(n int) { col.Set(clampSpan(n, dashMaxColSpan)) }
	setRow := func(n int) { row.Set(clampSpan(n, dashMaxRowSpan)) }

	return Div(css.Class("wb"),
		Section(css.Class("card"),
			H3(css.Class("card-title"), uistate.T("widgetBuilder.stageTitle")),
			Div(css.Class("wb-stage"),
				Div(css.Class("w wb-tile"), Style(map[string]string{"width": span(c), "height": span(r)}),
					Div(css.Class("wh"),
						Span(css.Class("grip"), Attr("aria-hidden", "true"), "⠿"),
						H3(wbCardTitle(res, title)),
					),
					Div(ClassStr("wbody"), wbStageBody(res, vizFormat.Get())),
				),
			),
		),
		Section(css.Class("card"),
			H3(css.Class("card-title"), uistate.T("widgetBuilder.pipelineTitle")),
			P(css.Class("t-body", tw.TextDim, tw.Mb3), uistate.T("widgetBuilder.pipelineHint")),
			wbCanvas(active.Get(), wbStepValues(sourceVar.Get(), transformExpr.Get(), vizFormat.Get()),
				positions, func(step string) { active.Set(step) }),
		),
		Section(css.Class("card"),
			H3(css.Class("card-title"), uistate.T("widgetBuilder.configTitle")),
			wbConfigPanel(active.Get(), sourceVar.Get(), transformExpr.Get(), vizTitle.Get(), vizFormat.Get(), onSource, onTransform, onTitle, onFormat),
		),
		Section(css.Class("card"),
			H3(css.Class("card-title"), uistate.T("widgetBuilder.sizeTitle")),
			P(css.Class("t-body", tw.TextDim, tw.Mb3), uistate.T("widgetBuilder.sizeHint")),
			Div(css.Class("wb-size"),
				wmStepper("W", c, uistate.T("widget.narrower"), uistate.T("widget.wider"),
					func() { setCol(c - 1) }, func() { setCol(c + 1) }),
				wmStepper("H", r, uistate.T("widget.shorter"), uistate.T("widget.taller"),
					func() { setRow(r - 1) }, func() { setRow(r + 1) }),
			),
		),
	)
}

// wbBuildGraph assembles the card's node graph from the configured steps. The shape
// is source.scalar → [formula] → viz.kpi: the transform formula node is inserted only
// when an expression is given (it exposes the source value as the variable "a"),
// otherwise the source feeds the visualization directly.
func wbBuildGraph(sourceVar, transformExpr, title, vizFormat string) cardgraph.Graph {
	src := cardgraph.Node{ID: "src", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": sourceVar}}
	viz := cardgraph.Node{ID: "viz", Kind: cardgraph.KindVizKPI, Props: map[string]string{"title": title, "format": vizFormat, "tone": "auto"}}
	out := cardgraph.PortRef{Node: "viz", Port: "value"}

	if strings.TrimSpace(transformExpr) == "" {
		return cardgraph.Graph{
			Nodes: []cardgraph.Node{src, viz},
			Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "src", Port: cardgraph.OutPort}, To: out}},
			Root:  "viz",
		}
	}
	f := cardgraph.Node{ID: "xf", Kind: cardgraph.KindFormula, Props: map[string]string{"expr": transformExpr}}
	return cardgraph.Graph{
		Nodes: []cardgraph.Node{src, f, viz},
		Edges: []cardgraph.Edge{
			{From: cardgraph.PortRef{Node: "src", Port: cardgraph.OutPort}, To: cardgraph.PortRef{Node: "xf", Port: "a"}},
			{From: cardgraph.PortRef{Node: "xf", Port: cardgraph.OutPort}, To: out},
		},
		Root: "viz",
	}
}

// wbVariableSurface returns the live engine variable surface (net_worth, income,
// counts, …) the builder evaluates cards against, or an empty map when app state
// isn't hydrated yet.
func wbVariableSurface() map[string]float64 {
	app := appstate.Default
	if app == nil {
		return map[string]float64{}
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	return engineenv.Vars(engineenv.Data{
		Accounts:     app.Accounts(),
		Transactions: app.Transactions(),
		Members:      app.Members(),
		Budgets:      app.Budgets(),
		Goals:        app.Goals(),
		Tasks:        app.Tasks(),
		Rates:        currency.Rates{Base: base, Rates: app.Settings().FXRates},
		Now:          time.Now(),
	})
}

// wbCardTitle is the preview tile's header: the evaluated card's title, falling
// back to the in-progress title while the graph can't render.
func wbCardTitle(res cardgraph.Result, fallback string) string {
	if res.Render != nil && res.Render.Title != "" {
		return res.Render.Title
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return uistate.T("widgetBuilder.sampleTitle")
}

// wbStageBody renders the evaluated card into the preview tile body: the big figure
// with its tone when the graph resolves, or a friendly "unfinished" note carrying
// the first issue's message when it doesn't. Currency format is applied here at the
// edge (the pure core leaves it a plain number, since formatting needs the base
// currency) so money figures read as "$12,480".
func wbStageBody(res cardgraph.Result, format string) ui.Node {
	if res.Render == nil {
		msg := uistate.T("widgetBuilder.unfinished")
		for _, is := range res.Issues {
			if is.Message != "" {
				msg = is.Message
				break
			}
		}
		return P(css.Class("t-caption", tw.TextDim), msg)
	}
	text := res.Render.Text
	if format == "currency" {
		if f, err := strconv.ParseFloat(text, 64); err == nil {
			base := wbBaseCurrency()
			pow := 1.0
			for i := 0; i < currency.Decimals(base); i++ {
				pow *= 10
			}
			text = fmtMoney(money.Money{Amount: int64(math.Round(f * pow)), Currency: base})
		}
	}
	figClass := "fig t-figure"
	switch res.Render.Tone {
	case "up":
		figClass += " text-up"
	case "down":
		figClass += " text-down"
	}
	return Div(
		Div(css.Class(figClass, tw.FontDisplay), text),
		P(css.Class("t-caption", tw.TextDim, tw.Mt1), uistate.T("widgetBuilder.liveSub")),
	)
}

// wbBaseCurrency is the user's base currency for formatting money figures, or USD
// when app state isn't ready.
func wbBaseCurrency() string {
	app := appstate.Default
	if app == nil || app.Settings().BaseCurrency == "" {
		return "USD"
	}
	return app.Settings().BaseCurrency
}

// wbPretty turns a variable name ("net_worth") into a label ("net worth").
func wbPretty(name string) string { return strings.ReplaceAll(name, "_", " ") }

// wbStepValues resolves each pipeline node's summary line from the card config, so
// the nodes mirror the real graph (the chosen figure, the transform expression, the
// viz type) rather than static placeholders. Keyed by step id.
func wbStepValues(sourceVar, transformExpr, vizFormat string) map[string]string {
	xf := uistate.T("widgetBuilder.nodeTransformVal")
	if e := strings.TrimSpace(transformExpr); e != "" {
		xf = "a → " + e
	}
	return map[string]string{
		wbStepSource:    wbPretty(sourceVar),
		wbStepTransform: xf,
		wbStepVisualize: "KPI · " + vizFormat,
	}
}

// wbPoint is a node's top-left position on the canvas.
type wbPoint struct{ X, Y float64 }

// wbInitialPositions lays the three steps out left-to-right with the transform node
// dropped a little lower, giving the graph an n8n-like shape from the start.
func wbInitialPositions() map[string]wbPoint {
	return map[string]wbPoint{
		wbStepSource:    {X: 40, Y: 70},
		wbStepTransform: {X: 300, Y: 190},
		wbStepVisualize: {X: 560, Y: 70},
	}
}

// wbCanvasPosKey is the localStorage key the drag shim (web/wb-canvas.js) writes
// node positions to; the builder reads it so dragged layouts persist.
const wbCanvasPosKey = "cashflux:wb-canvas-pos"

// wbLoadPositions returns the node positions saved by the drag shim, merged over the
// default layout (so an undragged node keeps its default spot).
func wbLoadPositions() map[string]wbPoint {
	out := wbInitialPositions()
	v := js.Global().Get("localStorage").Call("getItem", wbCanvasPosKey)
	if v.Type() != js.TypeString {
		return out
	}
	var saved map[string]struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if err := json.Unmarshal([]byte(v.String()), &saved); err != nil {
		return out
	}
	for k, p := range saved {
		out[k] = wbPoint{X: p.X, Y: p.Y}
	}
	return out
}

// wbCanvas renders the n8n-style node canvas: draggable node boxes positioned on a
// 2D surface, joined by curved bezier wires drawn from each node's output port to the
// next node's input port. Clicking a node selects it (opens its config panel);
// dragging a node moves it (onMove persists the new position). The wired shape is
// source → [transform] → visualize, mirroring the live card graph.
func wbCanvas(active string, values map[string]string, pos map[string]wbPoint, onSelect func(string)) ui.Node {
	wire := func(from, to string) ui.Node {
		a, b := pos[from], pos[to]
		x1, y1 := a.X+wbNodeW, a.Y+wbNodeH/2
		x2, y2 := b.X, b.Y+wbNodeH/2
		dx := (x2 - x1) / 2
		if dx < 50 {
			dx = 50
		}
		d := fmt.Sprintf("M %.1f %.1f C %.1f %.1f, %.1f %.1f, %.1f %.1f", x1, y1, x1+dx, y1, x2-dx, y2, x2, y2)
		// data-from / data-to let the drag shim re-route this wire live.
		return Path(css.Class("wb-wire"), Attr("d", d), Attr("fill", "none"),
			Attr("data-from", from), Attr("data-to", to))
	}
	// All three steps are always on the canvas (so each is clickable to configure);
	// the transform is a pass-through until a formula is set. Wires flow
	// source → transform → visualize.
	children := []ui.Node{
		Svg(css.Class("wb-wires"), Attr("width", wbCanvasWStr), Attr("height", wbCanvasHStr),
			Attr("viewBox", "0 0 "+wbCanvasWStr+" "+wbCanvasHStr),
			wire(wbStepSource, wbStepTransform), wire(wbStepTransform, wbStepVisualize)),
	}
	add := func(step, ttl string) {
		p := pos[step]
		children = append(children, ui.CreateElement(wbCanvasNode, wbNodeProps{
			Step: step, Title: ttl, Value: values[step], X: p.X, Y: p.Y,
			Active: step == active, OnSelect: onSelect,
		}))
	}
	add(wbStepSource, uistate.T("widgetBuilder.nodeSource"))
	add(wbStepTransform, uistate.T("widgetBuilder.nodeTransform"))
	add(wbStepVisualize, uistate.T("widgetBuilder.nodeVisualize"))

	return Div(css.Class("wb-canvas-scroll"),
		Div(css.Class("wb-canvas"), Attr("role", "list"),
			Style(map[string]string{"width": wbPx(wbCanvasW), "height": wbPx(wbCanvasH)}),
			children),
	)
}

// wbPx renders a pixel length for an inline style.
func wbPx(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) + "px" }

type wbNodeProps struct {
	Step     string
	Title    string
	Value    string
	X, Y     float64
	Active   bool
	OnSelect func(string)
}

// wbCanvasNode is one node box on the canvas, with an inbound and outbound port.
// Clicking selects it (for the config panel); the data-step attribute lets the
// wb-canvas.js shim handle pointer dragging and live wire re-routing. Its own
// component so the click hook stays at a stable position (the On*-hooks rule).
func wbCanvasNode(p wbNodeProps) ui.Node {
	cls := "wb-node"
	if p.Active {
		cls += " is-active"
	}
	step := p.Step
	onSelect := ui.UseEvent(func() {
		if p.OnSelect != nil {
			p.OnSelect(step)
		}
	})
	return Div(ClassStr(cls),
		Style(map[string]string{"left": wbPx(p.X), "top": wbPx(p.Y)}),
		Attr("data-step", p.Step), Attr("role", "listitem"), Attr("tabindex", "0"),
		Attr("aria-pressed", boolAttr(p.Active)),
		OnClick(onSelect),
		Span(css.Class("wb-port wb-port-in"), Attr("aria-hidden", "true")),
		Div(css.Class("wb-node-body"),
			Span(css.Class("wb-node-kind"), p.Title),
			Span(css.Class("wb-node-val"), p.Value),
		),
		Span(css.Class("wb-port wb-port-out"), Attr("aria-hidden", "true")),
	)
}

func boolAttr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// wbConfigPanel renders the configuration controls for the selected pipeline step.
// Source: pick which live figure feeds the card. Transform: an optional formula over
// the source value (exposed as "a"). Visualize: title + number format. Handlers are
// created once at the WidgetBuilder level (stable hook positions) and passed in, so
// conditionally showing one branch here can't shift hook order.
func wbConfigPanel(active, sourceVar, transformExpr, vizTitle, vizFormat string, onSource, onTransform, onTitle, onFormat ui.Handler) ui.Node {
	switch active {
	case wbStepSource:
		opts := make([]ui.Node, 0, len(engineenv.Names))
		for _, name := range engineenv.SortedNames() {
			opts = append(opts, Option(Value(name), SelectedIf(name == sourceVar), wbPretty(name)))
		}
		return Div(css.Class("wb-config"),
			Div(css.Class("wb-field"),
				Span(css.Class("wb-field-label"), uistate.T("widgetBuilder.cfgSource")),
				Select(css.Class("set-input"), Attr("aria-label", uistate.T("widgetBuilder.cfgSource")), OnChange(onSource), opts),
			),
			P(css.Class("t-caption", tw.TextDim, tw.Mt2), uistate.T("widgetBuilder.cfgSourceHint")),
		)
	case wbStepVisualize:
		formats := []struct{ v, k string }{
			{"number", "widgetBuilder.fmtNumber"},
			{"percent", "widgetBuilder.fmtPercent"},
			{"currency", "widgetBuilder.fmtCurrency"},
		}
		fopts := make([]ui.Node, 0, len(formats))
		for _, f := range formats {
			fopts = append(fopts, Option(Value(f.v), SelectedIf(f.v == vizFormat), uistate.T(f.k)))
		}
		return Div(css.Class("wb-config"),
			Div(css.Class("wb-field"),
				Span(css.Class("wb-field-label"), uistate.T("widgetBuilder.cfgTitle")),
				Input(css.Class("set-input"), Type("text"), Value(vizTitle),
					Attr("placeholder", uistate.T("widgetBuilder.cfgTitlePlaceholder")),
					Attr("aria-label", uistate.T("widgetBuilder.cfgTitle")), OnInput(onTitle)),
			),
			Div(css.Class("wb-field"),
				Span(css.Class("wb-field-label"), uistate.T("widgetBuilder.cfgFormat")),
				Select(css.Class("set-input"), Attr("aria-label", uistate.T("widgetBuilder.cfgFormat")), OnChange(onFormat), fopts),
			),
		)
	default: // transform
		return Div(css.Class("wb-config"),
			Div(css.Class("wb-field"),
				Span(css.Class("wb-field-label"), uistate.T("widgetBuilder.cfgTransform")),
				Input(css.Class("set-input"), Type("text"), Value(transformExpr),
					Attr("placeholder", uistate.T("widgetBuilder.cfgTransformPlaceholder")),
					Attr("aria-label", uistate.T("widgetBuilder.cfgTransform")), OnInput(onTransform)),
			),
			P(css.Class("t-caption", tw.TextDim, tw.Mt2), uistate.T("widgetBuilder.cfgTransformHint")),
		)
	}
}
