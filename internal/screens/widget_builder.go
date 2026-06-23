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

// This file owns the Widget Builder screen — the visual programming system (see
// docs/WIDGET_BUILDER_DESIGN.md). It is deliberately self-contained with vb-prefixed
// symbols so it never collides with anything in widgets.go (which a parallel effort
// edits). The route in screens.go points at VisualBuilder.

// Bento cell geometry, mirrored from the dashboard grid (.bento --cell + gap) so the
// builder stage previews a tile at its true on-dashboard proportions.
const vbCellPx, vbGapPx = 152, 10

// Pipeline step ids, in left-to-right flow order: a data source → an optional
// transform → a visualization. Each step's output feeds the next.
const (
	vbStepSource    = "source"
	vbStepTransform = "transform"
	vbStepVisualize = "visualize"
)

// Canvas geometry: node box size and the fixed drawing surface the wires are
// computed against (the surface scrolls within its wrapper).
const (
	vbNodeW      = 156.0
	vbNodeH      = 66.0
	vbCanvasW    = 760.0
	vbCanvasH    = 340.0
	vbCanvasWStr = "760"
	vbCanvasHStr = "340"
)

// vbCanvasPosKey is the localStorage key the drag shim writes node positions to; the
// builder reads it so dragged layouts persist across re-renders.
const vbCanvasPosKey = "cashflux:wb-canvas-pos"

// vbDragShimJS is the canvas drag behavior, evaluated once from VisualBuilder. It
// delegates pointer events on the document so it survives the Go virtual-DOM
// re-rendering the canvas: mousedown on a .wb-node starts a drag, mousemove updates
// the node's left/top and re-routes the connected SVG wires live, and mouseup
// persists the position to localStorage. Guarded so repeated eval is a no-op. Kept in
// sync with web/wb-canvas.js. Uses string concatenation (no backticks) so it can live
// in a Go raw string.
const vbDragShimJS = `
(function(){
  if (window.__wbCanvasInit) return;
  window.__wbCanvasInit = true;
  var POS_KEY = "cashflux:wb-canvas-pos";
  var NODE_W = 156, NODE_H = 66;
  var drag = null;
  function load(){ try { return JSON.parse(localStorage.getItem(POS_KEY) || "{}"); } catch(e){ return {}; } }
  function save(p){ try { localStorage.setItem(POS_KEY, JSON.stringify(p)); } catch(e){} }
  function reroute(canvas){
    var ports = {};
    canvas.querySelectorAll(".wb-node").forEach(function(n){
      var step = n.getAttribute("data-step");
      var x = parseFloat(n.style.left) || 0, y = parseFloat(n.style.top) || 0;
      ports[step] = { inX:x, inY:y+NODE_H/2, outX:x+NODE_W, outY:y+NODE_H/2 };
    });
    canvas.querySelectorAll("path.wb-wire").forEach(function(p){
      var f = ports[p.getAttribute("data-from")], t = ports[p.getAttribute("data-to")];
      if(!f || !t) return;
      var x1=f.outX,y1=f.outY,x2=t.inX,y2=t.inY,dx=(x2-x1)/2; if(dx<50) dx=50;
      p.setAttribute("d","M "+x1+" "+y1+" C "+(x1+dx)+" "+y1+", "+(x2-dx)+" "+y2+", "+x2+" "+y2);
    });
  }
  document.addEventListener("mousedown", function(e){
    var node = e.target.closest ? e.target.closest(".wb-node") : null;
    if(!node) return;
    var canvas = node.closest(".wb-canvas");
    if(!canvas) return;
    var rect = node.getBoundingClientRect();
    drag = { step:node.getAttribute("data-step"), el:node, canvas:canvas, offX:e.clientX-rect.left, offY:e.clientY-rect.top, moved:false };
    e.preventDefault();
  });
  document.addEventListener("mousemove", function(e){
    if(!drag) return;
    var c = drag.canvas.getBoundingClientRect();
    var nx = e.clientX - c.left - drag.offX, ny = e.clientY - c.top - drag.offY;
    if(nx<0) nx=0; if(ny<0) ny=0;
    drag.el.style.left = nx+"px"; drag.el.style.top = ny+"px";
    drag.moved = true;
    reroute(drag.canvas);
  });
  document.addEventListener("mouseup", function(){
    if(!drag) return;
    if(drag.moved){
      var p = load();
      p[drag.step] = { x: parseFloat(drag.el.style.left)||0, y: parseFloat(drag.el.style.top)||0 };
      save(p);
      window.dispatchEvent(new CustomEvent("cashflux-wb-moved"));
    }
    drag = null;
  });
})();
`

// VisualBuilder is the widget-creation screen and front end of the visual programming
// system. Top-to-bottom: a stage rendering a LIVE preview tile — the card is built as
// an internal/cardgraph graph (source → optional transform → visualization) and
// evaluated against the real app figures, so the figure shown is your actual data; an
// n8n-style node canvas with draggable boxes and bezier wires; a per-step config
// panel; and a size control (width 1–4, height 1–3). The graph is the single source
// of truth — the canvas and the stage both read it.
func VisualBuilder() ui.Node {
	col := ui.UseState(1)
	row := ui.UseState(1)
	c, r := col.Get(), row.Get()

	active := ui.UseState(vbStepSource)
	// Install the canvas drag behavior (pointer drag + live wire re-routing) by
	// evaluating the shim once. It's injected from here rather than relying on a
	// <script> tag so it survives even if index.html is unavailable; the shim guards
	// against double-install. (web/wb-canvas.js holds the same source for reference.)
	ui.UseEffect(func() func() {
		js.Global().Call("eval", vbDragShimJS)
		return nil
	}, "vb-drag-shim")
	// Node positions come from localStorage (written by the drag shim), merged over
	// the default layout, so dragged positions survive re-renders.
	positions := vbLoadPositions()

	// Source: pick the kind of primitive (a live figure or a literal number/text/bool)
	// and its value. Transform: an optional formula. Visualize: pick the display widget
	// (KPI / Text / Progress / Badge) and its options. All held as small atoms; the
	// handlers are created once here (stable hook order) and threaded into the panel.
	sourceKind := ui.UseState("figure")
	sourceFigure := ui.UseState("net_worth")
	sourceNumber := ui.UseState("0")
	sourceText := ui.UseState("")
	sourceBool := ui.UseState("false")
	transformExpr := ui.UseState("")
	vizKind := ui.UseState("kpi")
	vizTitle := ui.UseState("")
	vizFormat := ui.UseState("number")
	vizMax := ui.UseState("100")

	h := vbHandlers{
		SourceKind:   ui.UseEvent(func(e ui.Event) { sourceKind.Set(e.GetValue()) }),
		SourceFigure: ui.UseEvent(func(e ui.Event) { sourceFigure.Set(e.GetValue()) }),
		SourceNumber: ui.UseEvent(func(v string) { sourceNumber.Set(v) }),
		SourceText:   ui.UseEvent(func(v string) { sourceText.Set(v) }),
		SourceBool:   ui.UseEvent(func(e ui.Event) { sourceBool.Set(e.GetValue()) }),
		Transform:    ui.UseEvent(func(v string) { transformExpr.Set(v) }),
		VizKind:      ui.UseEvent(func(e ui.Event) { vizKind.Set(e.GetValue()) }),
		Title:        ui.UseEvent(func(v string) { vizTitle.Set(v) }),
		Format:       ui.UseEvent(func(e ui.Event) { vizFormat.Set(e.GetValue()) }),
		Max:          ui.UseEvent(func(v string) { vizMax.Set(v) }),
	}

	cfg := vbConfig{
		SourceKind: sourceKind.Get(), Figure: sourceFigure.Get(), Number: sourceNumber.Get(),
		TextVal: sourceText.Get(), BoolVal: sourceBool.Get(), TransformExpr: transformExpr.Get(),
		VizKind: vizKind.Get(), Title: vizTitle.Get(), Format: vizFormat.Get(), Max: vizMax.Get(),
	}
	g := vbBuildGraph(cfg)
	res := cardgraph.Eval(g, cardgraph.Context{Vars: vbVariableSurface()})

	span := func(n int) string {
		return strconv.Itoa(n*vbCellPx+(n-1)*vbGapPx) + "px"
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
						H3(vbCardTitle(res, vbDefaultTitle(cfg))),
					),
					Div(ClassStr("wbody"), vbStageBody(res, vizFormat.Get())),
				),
			),
		),
		Section(css.Class("card"),
			H3(css.Class("card-title"), uistate.T("widgetBuilder.pipelineTitle")),
			P(css.Class("t-body", tw.TextDim, tw.Mb3), uistate.T("widgetBuilder.pipelineHint")),
			vbCanvas(active.Get(), vbStepValues(cfg), positions, func(step string) { active.Set(step) }),
		),
		Section(css.Class("card"),
			H3(css.Class("card-title"), uistate.T("widgetBuilder.configTitle")),
			vbConfigPanel(active.Get(), cfg, h),
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

// vbConfig is the full builder state, flattened — the single description the graph,
// the canvas summaries, and the config panel are all derived from.
type vbConfig struct {
	SourceKind    string // "figure" | "number" | "text" | "bool"
	Figure        string // engineenv variable name (figure source)
	Number        string // literal number value
	TextVal       string // literal text value
	BoolVal       string // "true" | "false"
	TransformExpr string // optional formula over the source value (var "a")
	VizKind       string // "kpi" | "text" | "progress" | "badge"
	Title         string // override title ("" = derived)
	Format        string // "number" | "percent" | "currency"
	Max           string // progress denominator
}

// vbHandlers bundles the event handlers the config panel needs, created once in
// VisualBuilder so hook order stays stable regardless of which step is shown.
type vbHandlers struct {
	SourceKind, SourceFigure, SourceNumber, SourceText, SourceBool ui.Handler
	Transform, VizKind, Title, Format, Max                         ui.Handler
}

// vbNumericSource reports whether the chosen source produces a number (so a transform
// formula and numeric viz make sense); text sources don't.
func (c vbConfig) vbNumericSource() bool {
	return c.SourceKind != "text"
}

// vbDefaultTitle derives a card title when the user hasn't typed one.
func vbDefaultTitle(c vbConfig) string {
	if t := strings.TrimSpace(c.Title); t != "" {
		return t
	}
	if c.SourceKind == "figure" {
		return vbPretty(c.Figure)
	}
	return uistate.T("widgetBuilder.sampleTitle")
}

// vbBuildGraph assembles the card's node graph from the config: a source primitive →
// an optional formula transform (numeric sources only) → the chosen visualization. A
// progress viz also gets a literal "max" node wired into its second input.
func vbBuildGraph(c vbConfig) cardgraph.Graph {
	var src cardgraph.Node
	switch c.SourceKind {
	case "number":
		src = cardgraph.Node{ID: "src", Kind: cardgraph.KindLiteralNumber, Props: map[string]string{"value": c.Number}}
	case "text":
		src = cardgraph.Node{ID: "src", Kind: cardgraph.KindLiteralText, Props: map[string]string{"value": c.TextVal}}
	case "bool":
		src = cardgraph.Node{ID: "src", Kind: cardgraph.KindLiteralBool, Props: map[string]string{"value": c.BoolVal}}
	default:
		src = cardgraph.Node{ID: "src", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": c.Figure}}
	}

	title := vbDefaultTitle(c)
	var viz cardgraph.Node
	switch c.VizKind {
	case "text":
		viz = cardgraph.Node{ID: "viz", Kind: cardgraph.KindVizText, Props: map[string]string{"title": title}}
	case "progress":
		viz = cardgraph.Node{ID: "viz", Kind: cardgraph.KindVizProgress, Props: map[string]string{"title": title, "format": c.Format}}
	case "badge":
		viz = cardgraph.Node{ID: "viz", Kind: cardgraph.KindVizBadge, Props: map[string]string{"title": title, "tone": "auto"}}
	default:
		viz = cardgraph.Node{ID: "viz", Kind: cardgraph.KindVizKPI, Props: map[string]string{"title": title, "format": c.Format, "tone": "auto"}}
	}

	nodes := []cardgraph.Node{src}
	var edges []cardgraph.Edge
	last := cardgraph.PortRef{Node: "src", Port: cardgraph.OutPort}

	// Transform: only for numeric sources, and only when an expression is set.
	if c.vbNumericSource() && strings.TrimSpace(c.TransformExpr) != "" {
		nodes = append(nodes, cardgraph.Node{ID: "xf", Kind: cardgraph.KindFormula, Props: map[string]string{"expr": c.TransformExpr}})
		edges = append(edges, cardgraph.Edge{From: last, To: cardgraph.PortRef{Node: "xf", Port: "a"}})
		last = cardgraph.PortRef{Node: "xf", Port: cardgraph.OutPort}
	}

	nodes = append(nodes, viz)
	edges = append(edges, cardgraph.Edge{From: last, To: cardgraph.PortRef{Node: "viz", Port: "value"}})

	if c.VizKind == "progress" {
		nodes = append(nodes, cardgraph.Node{ID: "max", Kind: cardgraph.KindLiteralNumber, Props: map[string]string{"value": c.Max}})
		edges = append(edges, cardgraph.Edge{From: cardgraph.PortRef{Node: "max", Port: cardgraph.OutPort}, To: cardgraph.PortRef{Node: "viz", Port: "max"}})
	}

	return cardgraph.Graph{Nodes: nodes, Edges: edges, Root: "viz"}
}

// vbVariableSurface returns the live engine variable surface (net_worth, income,
// counts, …) the builder evaluates cards against, or an empty map when app state isn't
// hydrated yet.
func vbVariableSurface() map[string]float64 {
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

// vbCardTitle is the preview tile's header: the evaluated card's title, falling back
// to the in-progress title while the graph can't render.
func vbCardTitle(res cardgraph.Result, fallback string) string {
	if res.Render != nil && res.Render.Title != "" {
		return res.Render.Title
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return uistate.T("widgetBuilder.sampleTitle")
}

// toneClass maps a viz tone to the figure color class.
func toneClass(tone string) string {
	switch tone {
	case "up":
		return " text-up"
	case "down":
		return " text-down"
	}
	return ""
}

// vbMoneyFmt formats a plain numeric string as money when format=="currency"; other
// formats pass through (the pure core already appended "%" for percent).
func vbMoneyFmt(text, format string) string {
	if format != "currency" {
		return text
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return text
	}
	base := vbBaseCurrency()
	pow := 1.0
	for i := 0; i < currency.Decimals(base); i++ {
		pow *= 10
	}
	return fmtMoney(money.Money{Amount: int64(math.Round(f * pow)), Currency: base})
}

// vbStageBody renders the evaluated card into the preview tile body by visualization
// kind (kpi / text / progress / badge), or a friendly "unfinished" note carrying the
// first issue's message when the graph can't resolve. Currency is formatted at the
// edge (the pure core leaves it a plain number, since formatting needs the base
// currency).
func vbStageBody(res cardgraph.Result, format string) ui.Node {
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
	v := res.Render
	switch v.Kind {
	case "text":
		return Div(P(css.Class("t-body"), v.Text))
	case "badge":
		badgeStyle := map[string]string{
			"display": "inline-block", "padding": "0.25rem 0.7rem", "border-radius": "999px",
			"font-weight": "600", "font-size": "0.95rem",
			"background": "color-mix(in srgb, var(--accent,#3b82f6) 18%, transparent)",
			"color":      "var(--accent,#3b82f6)",
		}
		if v.Tone == "up" {
			badgeStyle["background"] = "color-mix(in srgb, var(--up,#16a34a) 18%, transparent)"
			badgeStyle["color"] = "var(--up,#16a34a)"
		} else if v.Tone == "down" {
			badgeStyle["background"] = "color-mix(in srgb, var(--down,#dc2626) 18%, transparent)"
			badgeStyle["color"] = "var(--down,#dc2626)"
		}
		return Div(Span(Style(badgeStyle), v.Text))
	case "progress":
		fillW := strconv.FormatFloat(v.Pct*100, 'f', 1, 64) + "%"
		track := map[string]string{"width": "100%", "height": "10px", "border-radius": "999px",
			"background": "color-mix(in srgb, var(--dim,#6b7280) 25%, transparent)", "overflow": "hidden", "margin-top": "0.5rem"}
		fill := map[string]string{"width": fillW, "height": "100%", "border-radius": "999px",
			"background": "var(--accent,#3b82f6)"}
		if v.Tone == "up" {
			fill["background"] = "var(--up,#16a34a)"
		}
		return Div(
			Div(css.Class("fig t-figure"+toneClass(v.Tone), tw.FontDisplay), vbMoneyFmt(v.Text, format)),
			P(css.Class("t-caption", tw.TextDim), vbMoneyFmt(v.Sub, format)),
			Div(css.Class("wb-bar"), Style(track), Div(css.Class("wb-bar-fill"), Style(fill))),
		)
	default: // kpi
		return Div(
			Div(css.Class("fig t-figure"+toneClass(v.Tone), tw.FontDisplay), vbMoneyFmt(v.Text, format)),
			P(css.Class("t-caption", tw.TextDim, tw.Mt1), uistate.T("widgetBuilder.liveSub")),
		)
	}
}

// vbBaseCurrency is the user's base currency for formatting money figures, or USD when
// app state isn't ready.
func vbBaseCurrency() string {
	app := appstate.Default
	if app == nil || app.Settings().BaseCurrency == "" {
		return "USD"
	}
	return app.Settings().BaseCurrency
}

// vbPretty turns a variable name ("net_worth") into a label ("net worth").
func vbPretty(name string) string { return strings.ReplaceAll(name, "_", " ") }

// vbStepValues resolves each node's summary line from the card config, so the nodes
// mirror the real graph (chosen figure, transform expression, viz type). Keyed by step.
func vbStepValues(c vbConfig) map[string]string {
	var srcVal string
	switch c.SourceKind {
	case "number":
		srcVal = "# " + c.Number
	case "text":
		srcVal = "\"" + c.TextVal + "\""
	case "bool":
		srcVal = "bool " + c.BoolVal
	default:
		srcVal = vbPretty(c.Figure)
	}
	xf := uistate.T("widgetBuilder.nodeTransformVal")
	if e := strings.TrimSpace(c.TransformExpr); e != "" && c.vbNumericSource() {
		xf = "a → " + e
	}
	viz := map[string]string{"kpi": "KPI", "text": "Text", "progress": "Progress", "badge": "Badge"}[c.VizKind]
	if viz == "" {
		viz = "KPI"
	}
	return map[string]string{
		vbStepSource:    srcVal,
		vbStepTransform: xf,
		vbStepVisualize: viz,
	}
}

// vbPoint is a node's top-left position on the canvas.
type vbPoint struct{ X, Y float64 }

// vbInitialPositions lays the three steps out left-to-right with the transform node
// dropped a little lower, giving the graph an n8n-like shape from the start.
func vbInitialPositions() map[string]vbPoint {
	return map[string]vbPoint{
		vbStepSource:    {X: 40, Y: 70},
		vbStepTransform: {X: 300, Y: 190},
		vbStepVisualize: {X: 560, Y: 70},
	}
}

// vbLoadPositions returns the node positions saved by the drag shim, merged over the
// default layout (so an undragged node keeps its default spot).
func vbLoadPositions() map[string]vbPoint {
	out := vbInitialPositions()
	v := js.Global().Get("localStorage").Call("getItem", vbCanvasPosKey)
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
		out[k] = vbPoint{X: p.X, Y: p.Y}
	}
	return out
}

// vbCanvas renders the n8n-style node canvas: draggable node boxes positioned on a 2D
// surface, joined by curved bezier wires drawn from each node's output port to the
// next node's input port. Clicking a node selects it (opens its config panel);
// dragging is handled by web/wb-canvas.js. All three steps are always shown (each is
// clickable to configure); the transform is a pass-through until a formula is set.
func vbCanvas(active string, values map[string]string, pos map[string]vbPoint, onSelect func(string)) ui.Node {
	wire := func(from, to string) ui.Node {
		a, b := pos[from], pos[to]
		x1, y1 := a.X+vbNodeW, a.Y+vbNodeH/2
		x2, y2 := b.X, b.Y+vbNodeH/2
		dx := (x2 - x1) / 2
		if dx < 50 {
			dx = 50
		}
		d := fmt.Sprintf("M %.1f %.1f C %.1f %.1f, %.1f %.1f, %.1f %.1f", x1, y1, x1+dx, y1, x2-dx, y2, x2, y2)
		// Stroke is set inline too so wires are visible without the stylesheet.
		return Path(css.Class("wb-wire"), Attr("d", d), Attr("fill", "none"),
			Attr("stroke", "var(--dim,#6b7280)"), Attr("stroke-width", "2"), Attr("stroke-linecap", "round"),
			Attr("data-from", from), Attr("data-to", to))
	}
	// The SVG wire layer fills the surface; structural styles are inline so the canvas
	// lays out correctly even if the stylesheet is unavailable.
	svgStyle := map[string]string{"position": "absolute", "left": "0", "top": "0", "overflow": "visible", "pointer-events": "none"}
	children := []ui.Node{
		Svg(css.Class("wb-wires"), Style(svgStyle), Attr("width", vbCanvasWStr), Attr("height", vbCanvasHStr),
			Attr("viewBox", "0 0 "+vbCanvasWStr+" "+vbCanvasHStr),
			wire(vbStepSource, vbStepTransform), wire(vbStepTransform, vbStepVisualize)),
	}
	add := func(step, ttl string) {
		p := pos[step]
		children = append(children, ui.CreateElement(vbCanvasNode, vbNodeProps{
			Step: step, Title: ttl, Value: values[step], X: p.X, Y: p.Y,
			Active: step == active, OnSelect: onSelect,
		}))
	}
	add(vbStepSource, uistate.T("widgetBuilder.nodeSource"))
	add(vbStepTransform, uistate.T("widgetBuilder.nodeTransform"))
	add(vbStepVisualize, uistate.T("widgetBuilder.nodeVisualize"))

	scrollStyle := map[string]string{
		"position": "relative", "overflow": "auto", "border-radius": "12px",
		"border":           "1px solid var(--line,#e5e7eb)",
		"background-image": "radial-gradient(circle, color-mix(in srgb, var(--dim,#6b7280) 22%, transparent) 1px, transparent 1px)",
		"background-size":  "16px 16px",
	}
	canvasStyle := map[string]string{"position": "relative", "width": vbPx(vbCanvasW), "height": vbPx(vbCanvasH)}
	return Div(css.Class("wb-canvas-scroll"), Style(scrollStyle),
		Div(css.Class("wb-canvas"), Attr("role", "list"), Style(canvasStyle), children),
	)
}

// vbPx renders a pixel length for an inline style.
func vbPx(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) + "px" }

type vbNodeProps struct {
	Step     string
	Title    string
	Value    string
	X, Y     float64
	Active   bool
	OnSelect func(string)
}

// vbCanvasNode is one node box on the canvas, with an inbound and outbound port.
// Clicking selects it (for the config panel); the data-step attribute lets the
// wb-canvas.js shim handle pointer dragging and live wire re-routing. Its own
// component so the click hook stays at a stable position (the On*-hooks rule).
func vbCanvasNode(p vbNodeProps) ui.Node {
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
	// Structural + cosmetic styles are inline so a node renders as a positioned box on
	// the canvas even without the stylesheet (which a parallel effort sometimes reverts).
	style := map[string]string{
		"left": vbPx(p.X), "top": vbPx(p.Y), "position": "absolute",
		"width": "156px", "box-sizing": "border-box", "display": "flex", "align-items": "center",
		"gap": "0.5rem", "padding": "0.6rem 0.7rem", "border-radius": "10px", "cursor": "grab",
		"background": "var(--bg-elev,#1a1a1d)", "border": "1.5px solid var(--line,#3a3a3d)",
	}
	if p.Active {
		style["border-color"] = "var(--accent,#3b82f6)"
		style["box-shadow"] = "0 0 0 3px color-mix(in srgb, var(--accent,#3b82f6) 22%, transparent)"
	}
	portStyle := func(side string) map[string]string {
		s := map[string]string{
			"position": "absolute", "top": "50%", "width": "11px", "height": "11px", "border-radius": "999px",
			"background": "var(--bg,#0e0e10)", "border": "2px solid var(--dim,#6b7280)", "transform": "translateY(-50%)",
		}
		s[side] = "-6px"
		return s
	}
	return Div(ClassStr(cls), Style(style),
		Attr("data-step", p.Step), Attr("role", "listitem"), Attr("tabindex", "0"),
		Attr("aria-pressed", vbBoolAttr(p.Active)),
		OnClick(onSelect),
		Span(css.Class("wb-port wb-port-in"), Style(portStyle("left")), Attr("aria-hidden", "true")),
		Div(css.Class("wb-node-body"), Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.15rem", "min-width": "0"}),
			Span(css.Class("wb-node-kind"), Style(map[string]string{"font-size": "11px", "text-transform": "uppercase", "letter-spacing": "0.06em", "color": "var(--faint,#9ca3af)"}), p.Title),
			Span(css.Class("wb-node-val"), Style(map[string]string{"font-size": "13px", "font-weight": "600", "white-space": "nowrap", "overflow": "hidden", "text-overflow": "ellipsis", "max-width": "118px"}), p.Value),
		),
		Span(css.Class("wb-port wb-port-out"), Style(portStyle("right")), Attr("aria-hidden", "true")),
	)
}

func vbBoolAttr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// vbField wraps a labeled control.
func vbField(label string, control ui.Node) ui.Node {
	return Div(css.Class("wb-field"),
		Span(css.Class("wb-field-label"), label),
		control,
	)
}

// vbSelect builds a labeled <select> from {value,label} options with the current
// value pre-selected.
func vbSelect(label string, current string, opts [][2]string, on ui.Handler) ui.Node {
	nodes := make([]ui.Node, 0, len(opts))
	for _, o := range opts {
		nodes = append(nodes, Option(Value(o[0]), SelectedIf(o[0] == current), o[1]))
	}
	return vbField(label, Select(css.Class("set-input"), Attr("aria-label", label), OnChange(on), nodes))
}

// vbConfigPanel renders the controls for the selected step, branching on the chosen
// kind so each primitive/visualization gets the right fields. Handlers are created
// once at the VisualBuilder level (stable hook positions) and passed in via h.
func vbConfigPanel(active string, c vbConfig, h vbHandlers) ui.Node {
	switch active {
	case vbStepSource:
		kindSel := vbSelect(uistate.T("widgetBuilder.cfgSourceKind"), c.SourceKind, [][2]string{
			{"figure", uistate.T("widgetBuilder.srcFigure")},
			{"number", uistate.T("widgetBuilder.srcNumber")},
			{"text", uistate.T("widgetBuilder.srcText")},
			{"bool", uistate.T("widgetBuilder.srcBool")},
		}, h.SourceKind)

		var detail ui.Node
		switch c.SourceKind {
		case "number":
			detail = vbField(uistate.T("widgetBuilder.cfgValue"),
				Input(css.Class("set-input"), Type("number"), Value(c.Number), Attr("aria-label", uistate.T("widgetBuilder.cfgValue")), OnInput(h.SourceNumber)))
		case "text":
			detail = vbField(uistate.T("widgetBuilder.cfgValue"),
				Input(css.Class("set-input"), Type("text"), Value(c.TextVal), Attr("aria-label", uistate.T("widgetBuilder.cfgValue")), OnInput(h.SourceText)))
		case "bool":
			detail = vbSelect(uistate.T("widgetBuilder.cfgValue"), c.BoolVal, [][2]string{
				{"true", uistate.T("widgetBuilder.boolTrue")}, {"false", uistate.T("widgetBuilder.boolFalse")},
			}, h.SourceBool)
		default: // figure
			fopts := make([][2]string, 0, len(engineenv.Names))
			for _, name := range engineenv.SortedNames() {
				fopts = append(fopts, [2]string{name, vbPretty(name)})
			}
			detail = vbSelect(uistate.T("widgetBuilder.cfgSource"), c.Figure, fopts, h.SourceFigure)
		}
		return Div(css.Class("wb-config"), kindSel, detail,
			P(css.Class("t-caption", tw.TextDim, tw.Mt2), uistate.T("widgetBuilder.cfgSourceHint")))

	case vbStepVisualize:
		kindSel := vbSelect(uistate.T("widgetBuilder.cfgVizKind"), c.VizKind, [][2]string{
			{"kpi", uistate.T("widgetBuilder.vizKpi")},
			{"text", uistate.T("widgetBuilder.vizText")},
			{"progress", uistate.T("widgetBuilder.vizProgress")},
			{"badge", uistate.T("widgetBuilder.vizBadge")},
		}, h.VizKind)
		fields := []ui.Node{kindSel,
			vbField(uistate.T("widgetBuilder.cfgTitle"),
				Input(css.Class("set-input"), Type("text"), Value(c.Title),
					Attr("placeholder", uistate.T("widgetBuilder.cfgTitlePlaceholder")),
					Attr("aria-label", uistate.T("widgetBuilder.cfgTitle")), OnInput(h.Title))),
		}
		// Number format applies to the figure-bearing widgets (KPI, Progress).
		if c.VizKind == "kpi" || c.VizKind == "progress" {
			fields = append(fields, vbSelect(uistate.T("widgetBuilder.cfgFormat"), c.Format, [][2]string{
				{"number", uistate.T("widgetBuilder.fmtNumber")},
				{"percent", uistate.T("widgetBuilder.fmtPercent")},
				{"currency", uistate.T("widgetBuilder.fmtCurrency")},
			}, h.Format))
		}
		if c.VizKind == "progress" {
			fields = append(fields, vbField(uistate.T("widgetBuilder.cfgMax"),
				Input(css.Class("set-input"), Type("number"), Value(c.Max), Attr("aria-label", uistate.T("widgetBuilder.cfgMax")), OnInput(h.Max))))
		}
		return Div(css.Class("wb-config"), fields)

	default: // transform
		return Div(css.Class("wb-config"),
			vbField(uistate.T("widgetBuilder.cfgTransform"),
				Input(css.Class("set-input"), Type("text"), Value(c.TransformExpr),
					Attr("placeholder", uistate.T("widgetBuilder.cfgTransformPlaceholder")),
					Attr("aria-label", uistate.T("widgetBuilder.cfgTransform")), OnInput(h.Transform))),
			P(css.Class("t-caption", tw.TextDim, tw.Mt2), uistate.T("widgetBuilder.cfgTransformHint")),
		)
	}
}
