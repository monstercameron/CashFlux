// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/cardgraph"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// This file owns the Widget Builder — a free-form node-graph editor (the visual
// programming system; see docs/WIDGET_BUILDER_PLAN.md). The canvas is the primary
// surface: add nodes from the palette, name them (variables usable downstream), wire
// inputs in the inspector, pick the output node, and watch the live preview render the
// evaluated card. All symbols are vb-prefixed so they never collide with widgets.go
// (which a parallel effort edits). The route in screens.go points at VisualBuilder.

const vbCellPx, vbGapPx = 152, 10               // bento cell geometry (true tile proportions)
const vbNodeW, vbNodeH = 168.0, 64.0            // canvas node box size
const vbGraphKey = "cashflux:wb-graph"          // localStorage: the whole card graph
const vbCanvasPosKey = "cashflux:wb-canvas-pos" // localStorage: node positions (drag shim)

// vbDragShimJS is the canvas drag behavior, evaluated once. It delegates pointer events
// on the document so it survives Go re-rendering the canvas: mousedown on a .wb-node
// drags it (updating left/top + re-routing wires live), mouseup persists the position
// keyed by the node's data-step (= node id). Guarded against double-install.
const vbDragShimJS = `
(function(){
  if (window.__wbCanvasInit) return;
  window.__wbCanvasInit = true;
  var POS_KEY = "cashflux:wb-canvas-pos", VIEW_KEY = "cashflux:wb-canvas-view";
  function load(k){ try { return JSON.parse(localStorage.getItem(k) || "{}"); } catch(e){ return {}; } }
  function save(k,v){ try { localStorage.setItem(k, JSON.stringify(v)); } catch(e){} }
  function getView(){ var v = load(VIEW_KEY); return { tx: v.tx||0, ty: v.ty||0, s: v.s||1 }; }
  function applyView(world, v){ world.style.transformOrigin="0 0"; world.style.transform="translate("+v.tx+"px,"+v.ty+"px) scale("+v.s+")"; }
  function clampS(s){ return Math.max(0.3, Math.min(2.5, s)); }
  function esc(x){ try { return CSS.escape(x); } catch(e){ return x; } }
  function worldOf(world, cx, cy){ var r=world.getBoundingClientRect(); var s=getView().s; return { x:(cx-r.left)/s, y:(cy-r.top)/s }; }
  // World-space center of a port dot, found by querying the live element (robust to
  // any node layout + zoom level).
  function portCenter(world, nodeId, port, dir){
    var sel = '.wb-node[data-step="'+esc(nodeId)+'"] .wb-port-'+dir;
    if(dir==="in" && port) sel += '[data-port="'+esc(port)+'"]';
    var el = world.querySelector(sel); if(!el) return null;
    var r = el.getBoundingClientRect(), wr = world.getBoundingClientRect(), s = getView().s;
    return { x:(r.left+r.width/2-wr.left)/s, y:(r.top+r.height/2-wr.top)/s };
  }
  function bez(x1,y1,x2,y2){ var dx=(x2-x1)/2; if(dx<40) dx=40; return "M "+x1+" "+y1+" C "+(x1+dx)+" "+y1+", "+(x2-dx)+" "+y2+", "+x2+" "+y2; }
  function reroute(world){
    world.querySelectorAll("path.wb-wire").forEach(function(p){
      var f = portCenter(world, p.getAttribute("data-from"), "out", "out");
      var t = portCenter(world, p.getAttribute("data-to"), p.getAttribute("data-toport"), "in");
      if(f && t) p.setAttribute("d", bez(f.x,f.y,t.x,t.y));
    });
  }
  var node=null, pan=null, wire=null, moved=false;
  document.addEventListener("mousedown", function(e){
    var outPort = e.target.closest ? e.target.closest(".wb-port-out") : null;
    if(outPort){  // start a connection drag from an output port
      var world = outPort.closest(".wb-canvas"); if(!world) return;
      var svg = world.querySelector("svg.wb-wires");
      var temp = document.createElementNS("http://www.w3.org/2000/svg","path");
      temp.setAttribute("class","wb-wire-temp"); temp.setAttribute("fill","none");
      temp.setAttribute("stroke","var(--accent,#3b82f6)"); temp.setAttribute("stroke-width","2.5");
      temp.style.pointerEvents="none";
      if(svg) svg.appendChild(temp);
      wire = { from: outPort.getAttribute("data-node"), world: world, temp: temp };
      moved=false; e.preventDefault(); e.stopPropagation(); return;
    }
    if(e.target.closest && e.target.closest(".wb-port")){ e.preventDefault(); e.stopPropagation(); return; }
    var nodeEl = e.target.closest ? e.target.closest(".wb-node") : null;
    if(nodeEl){
      var w = nodeEl.closest(".wb-canvas"); if(!w) return; var v = getView();
      node = { id:nodeEl.getAttribute("data-step"), el:nodeEl, world:w,
        startL:parseFloat(nodeEl.style.left)||0, startT:parseFloat(nodeEl.style.top)||0,
        mx:e.clientX, my:e.clientY, s:v.s };
      moved=false; e.preventDefault(); return;
    }
    var bg = e.target.closest ? e.target.closest(".wb-canvas") : null;
    if(bg){ var vv = getView(); pan = { world:bg, startTX:vv.tx, startTY:vv.ty, mx:e.clientX, my:e.clientY };
      var vp = bg.parentElement; if(vp) vp.style.cursor="grabbing"; moved=false; e.preventDefault(); }
  });
  document.addEventListener("mousemove", function(e){
    if(wire){ var f = portCenter(wire.world, wire.from, "out", "out"); var c = worldOf(wire.world, e.clientX, e.clientY);
      if(f) wire.temp.setAttribute("d", bez(f.x,f.y,c.x,c.y)); moved=true; return; }
    if(node){ var nx = node.startL + (e.clientX-node.mx)/node.s, ny = node.startT + (e.clientY-node.my)/node.s;
      if(nx<0) nx=0; if(ny<0) ny=0; node.el.style.left=nx+"px"; node.el.style.top=ny+"px"; moved=true; reroute(node.world); return; }
    if(pan){ var v = getView(); v.tx = pan.startTX + (e.clientX-pan.mx); v.ty = pan.startTY + (e.clientY-pan.my);
      applyView(pan.world, v); save(VIEW_KEY, v); moved=true; }
  });
  document.addEventListener("mouseup", function(e){
    if(wire){
      var tgt = document.elementFromPoint(e.clientX, e.clientY);
      var toNode=null, toPort=null;
      var inPort = tgt && tgt.closest ? tgt.closest(".wb-port-in") : null;
      if(inPort){ toNode=inPort.getAttribute("data-node"); toPort=inPort.getAttribute("data-port"); }
      else { var nd = tgt && tgt.closest ? tgt.closest(".wb-node") : null;
        if(nd){ var fp = nd.querySelector(".wb-port-in"); if(fp){ toNode=fp.getAttribute("data-node"); toPort=fp.getAttribute("data-port"); } } }
      if(wire.temp && wire.temp.parentNode) wire.temp.parentNode.removeChild(wire.temp);
      if(toNode && toPort && toNode!==wire.from && window.__wbConnect){ window.__wbConnect(wire.from, toNode, toPort); }
      wire=null; return;
    }
    if(node){ if(moved){ var p = load(POS_KEY); p[node.id] = { x:parseFloat(node.el.style.left)||0, y:parseFloat(node.el.style.top)||0 }; save(POS_KEY, p); } node=null; }
    if(pan){ var vp = pan.world.parentElement; if(vp) vp.style.cursor=""; pan=null; }
  });
  document.addEventListener("wheel", function(e){
    var vp = e.target.closest ? e.target.closest(".vb-canvas-scroll") : null; if(!vp) return;
    var world = vp.querySelector(".wb-canvas"); if(!world) return;
    e.preventDefault();
    var v = getView(); var r = vp.getBoundingClientRect();
    var mx = e.clientX-r.left, my = e.clientY-r.top;
    var wx = (mx-v.tx)/v.s, wy = (my-v.ty)/v.s;
    var s2 = clampS(v.s * (e.deltaY<0 ? 1.1 : 0.9));
    v.tx = mx - wx*s2; v.ty = my - wy*s2; v.s = s2;
    applyView(world, v); save(VIEW_KEY, v);
  }, {passive:false});
  document.addEventListener("click", function(e){
    if(moved){ moved=false; return; }  // a drag ended, not a click
    var w = e.target.closest ? e.target.closest("path.wb-wire") : null;
    if(w && window.__wbDisconnect){ window.__wbDisconnect(w.getAttribute("data-to"), w.getAttribute("data-toport")); return; }
    var btn = e.target.closest ? e.target.closest("[data-zoom]") : null; if(!btn) return;
    var vp = btn.closest(".vb-canvas-scroll"); if(!vp) return;
    var world = vp.querySelector(".wb-canvas"); if(!world) return;
    var v = getView(); var r = vp.getBoundingClientRect();
    var dir = btn.getAttribute("data-zoom");
    if(dir==="reset"){ v = {tx:0,ty:0,s:1}; }
    else { var cx=r.width/2, cy=r.height/2, wx=(cx-v.tx)/v.s, wy=(cy-v.ty)/v.s;
      var s2 = clampS(v.s * (dir==="in" ? 1.2 : 1/1.2)); v.tx = cx-wx*s2; v.ty = cy-wy*s2; v.s = s2; }
    applyView(world, v); save(VIEW_KEY, v);
  });
  // Snap wires to their actual ports after Go re-renders the canvas (nodes/edges
  // change). Debounced via rAF; ignores 'd' changes so it doesn't self-trigger.
  var ro=null;
  function observe(){
    var world = document.querySelector(".wb-canvas");
    if(!world){ setTimeout(observe, 200); return; }
    reroute(world);
    if(ro) ro.disconnect();
    ro = new MutationObserver(function(){ requestAnimationFrame(function(){ var w=document.querySelector(".wb-canvas"); if(w) reroute(w); }); });
    ro.observe(world, { childList:true, subtree:true, attributes:true, attributeFilter:["style","data-from","data-to","data-toport"] });
  }
  observe();
})();
`

// vbStyleCSS is the builder's layout stylesheet, injected once from Go so it survives
// even if index.html is reverted by a parallel effort. (Node boxes + wires also carry
// inline styles; this covers the surrounding panes.)
const vbStyleCSS = `
.vb{display:flex;flex-direction:column;gap:.75rem;height:calc(100vh - 120px);min-height:560px}
.vb-toolbar{display:flex;align-items:center;gap:.6rem;flex-wrap:wrap}
.vb-tool-label{font-weight:600;font-size:14px}
.vb-sep{flex:1}
.vb-main{display:flex;gap:.6rem;flex:1;min-height:0}
.vb-palette{width:170px;flex:0 0 170px;overflow:auto;display:flex;flex-direction:column;gap:.25rem;padding:.5rem;border:1px solid var(--line,#2a2a2d);border-radius:10px;background:var(--bg-elev,#161618)}
.vb-pane-title{font-size:11px;text-transform:uppercase;letter-spacing:.06em;color:var(--faint,#9ca3af);margin-bottom:.25rem}
.vb-pal-group{font-size:10px;text-transform:uppercase;letter-spacing:.05em;color:var(--faint,#9ca3af);margin-top:.5rem}
.vb-pal-btn{text-align:left;padding:.3rem .5rem;border-radius:7px;border:1px solid var(--line,#2a2a2d);background:var(--bg,#0e0e10);color:inherit;cursor:pointer;font-size:12px}
.vb-pal-btn:hover{border-color:var(--accent,#3b82f6)}
.vb-canvas-scroll{flex:1;min-width:0;position:relative;overflow:hidden;border-radius:10px;border:1px solid var(--line,#2a2a2d);background:var(--bg,#0e0e10);cursor:grab}
.vb-canvas-scroll:active{cursor:grabbing}
.vb-canvas-scroll .wb-canvas{background-image:radial-gradient(circle, color-mix(in srgb, var(--dim,#6b7280) 22%, transparent) 1px, transparent 1px);background-size:16px 16px;will-change:transform}
.wb-zoom{position:absolute;right:10px;bottom:10px;display:flex;gap:5px;z-index:5}
.wb-zoom-btn{width:30px;height:30px;border-radius:8px;border:1px solid var(--line,#2a2a2d);background:var(--bg-elev,#1a1a1d);color:inherit;cursor:pointer;font-size:16px;line-height:1;display:inline-flex;align-items:center;justify-content:center}
.wb-zoom-btn:hover{border-color:var(--accent,#3b82f6)}
.wb-port{transition:transform .1s ease, border-color .1s ease, box-shadow .1s ease; z-index:2}
.wb-port-out{cursor:crosshair}
.wb-port:hover{border-color:var(--accent,#3b82f6)!important; box-shadow:0 0 0 4px color-mix(in srgb, var(--accent,#3b82f6) 22%, transparent)}
.wb-node:hover{border-color:color-mix(in srgb, var(--accent,#3b82f6) 45%, var(--line,#3a3a3d))}
.wb-wire{transition:stroke .1s ease}
.wb-wire:hover{stroke:var(--accent,#3b82f6)!important; stroke-width:3.5!important}
.wb-port-row:hover{color:var(--fg,#e5e7eb)!important}
.vb-inspector{width:250px;flex:0 0 250px;overflow:auto;display:flex;flex-direction:column;gap:.5rem;padding:.6rem;border:1px solid var(--line,#2a2a2d);border-radius:10px;background:var(--bg-elev,#161618)}
.vb-insp-section{font-size:11px;text-transform:uppercase;letter-spacing:.05em;color:var(--faint,#9ca3af);margin-top:.4rem}
.vb-insp-actions{display:flex;gap:.4rem;margin-top:.5rem}
.vb-previewpane{display:flex;flex-direction:column;gap:.35rem}
.vb .wb-field{display:flex;flex-direction:column;gap:.2rem}
.vb .wb-field-label{font-size:12px;color:var(--dim,#9ca3af)}
.vb .wb-stage{display:flex;align-items:center;justify-content:center;padding:1rem;border-radius:10px;background:var(--bg,#0e0e10)}
.vb .wtitle{font-family:'Fraunces',serif;font-weight:600}
`

// VisualBuilder is the node-graph widget editor.
func VisualBuilder() ui.Node {
	ui.UseEffect(func() func() { js.Global().Call("eval", vbDragShimJS); return nil }, "vb-drag-shim")
	ui.UseEffect(func() func() {
		doc := js.Global().Get("document")
		if doc.Call("getElementById", "vb-style").Type() == js.TypeNull {
			st := doc.Call("createElement", "style")
			st.Set("id", "vb-style")
			st.Set("textContent", vbStyleCSS)
			doc.Get("head").Call("appendChild", st)
		}
		return nil
	}, "vb-style")

	col := ui.UseState(2)
	row := ui.UseState(2)
	graph := ui.UseState(vbLoadGraph())
	selected := ui.UseState(cardgraph.NodeID(""))

	g := graph.Get()
	setGraph := func(ng cardgraph.Graph) { vbSaveGraph(ng); graph.Set(ng) }

	// Drag-to-wire bridge: the canvas shim drags from an output port to an input port
	// and calls window.__wbConnect(from, to, port); clicking a wire calls
	// window.__wbDisconnect(to, port). These mutate the graph via graph.Update (which
	// reads the live value, so it's safe from this once-installed callback).
	ui.UseEffect(func() func() {
		connect := js.FuncOf(func(_ js.Value, a []js.Value) any {
			if len(a) >= 3 {
				from, to, port := a[0].String(), a[1].String(), a[2].String()
				graph.Update(func(old cardgraph.Graph) cardgraph.Graph {
					ng := vbWireEdge(old, from, to, port)
					vbSaveGraph(ng)
					return ng
				})
			}
			return nil
		})
		disconnect := js.FuncOf(func(_ js.Value, a []js.Value) any {
			if len(a) >= 2 {
				to, port := a[0].String(), a[1].String()
				graph.Update(func(old cardgraph.Graph) cardgraph.Graph {
					ng := vbUnwire(old, to, port)
					vbSaveGraph(ng)
					return ng
				})
			}
			return nil
		})
		js.Global().Set("__wbConnect", connect)
		js.Global().Set("__wbDisconnect", disconnect)
		return func() { connect.Release(); disconnect.Release() }
	}, "vb-connect")

	// Mutations.
	addNode := func(kind string) {
		ng := vbCloneGraph(g)
		id := vbFreshID(ng)
		n := cardgraph.Node{ID: id, Kind: kind, Props: vbDefaultProps(kind), Pos: vbCascadePos(len(ng.Nodes))}
		ng.Nodes = append(ng.Nodes, n)
		if ng.Root == "" && vbOutType(kind) == cardgraph.TypeViz {
			ng.Root = id
		}
		setGraph(ng)
		selected.Set(id)
	}
	deleteNode := func(id cardgraph.NodeID) {
		ng := vbCloneGraph(g)
		kept := ng.Nodes[:0:0]
		for _, n := range ng.Nodes {
			if n.ID != id {
				kept = append(kept, n)
			}
		}
		ng.Nodes = kept
		edges := ng.Edges[:0:0]
		for _, e := range ng.Edges {
			if e.From.Node != id && e.To.Node != id {
				edges = append(edges, e)
			}
		}
		ng.Edges = edges
		if ng.Root == id {
			ng.Root = ""
		}
		setGraph(ng)
		selected.Set("")
	}
	setProp := func(id cardgraph.NodeID, key, val string) {
		ng := vbCloneGraph(g)
		for i := range ng.Nodes {
			if ng.Nodes[i].ID == id {
				if ng.Nodes[i].Props == nil {
					ng.Nodes[i].Props = map[string]string{}
				}
				ng.Nodes[i].Props[key] = val
			}
		}
		setGraph(ng)
	}
	setVar := func(id cardgraph.NodeID, v string) {
		ng := vbCloneGraph(g)
		for i := range ng.Nodes {
			if ng.Nodes[i].ID == id {
				ng.Nodes[i].Var = strings.TrimSpace(v)
			}
		}
		setGraph(ng)
	}
	setRoot := func(id cardgraph.NodeID) {
		ng := vbCloneGraph(g)
		ng.Root = id
		setGraph(ng)
	}
	wireInput := func(to cardgraph.NodeID, port, fromID string) {
		ng := vbCloneGraph(g)
		edges := ng.Edges[:0:0]
		for _, e := range ng.Edges { // drop any existing wire into this input
			if !(e.To.Node == to && e.To.Port == port) {
				edges = append(edges, e)
			}
		}
		if fromID != "" {
			edges = append(edges, cardgraph.Edge{
				From: cardgraph.PortRef{Node: cardgraph.NodeID(fromID), Port: cardgraph.OutPort},
				To:   cardgraph.PortRef{Node: to, Port: port},
			})
		}
		ng.Edges = edges
		setGraph(ng)
	}
	loadPreset := ui.UseEvent(func(e ui.Event) {
		if p, ok := vbPresets()[e.GetValue()]; ok {
			setGraph(p)
			selected.Set("")
		}
	})
	clearGraph := ui.UseEvent(func() { setGraph(vbStarterGraph()); selected.Set("") })

	// Saved-cards library: name + Save persists the current graph under that name to a
	// local library; "My cards" loads one back; Delete removes it. rev forces a
	// re-render so the library dropdown refreshes after a save/delete.
	cardName := ui.UseState("")
	rev := ui.UseState(0)
	_ = rev.Get()
	published := ui.UseState("")
	layoutAtom := uistate.UseLayoutItems()
	onCardName := ui.UseEvent(func(v string) { cardName.Set(v) })
	publish := ui.UseEvent(func() {
		name := strings.TrimSpace(cardName.Get())
		if name == "" {
			published.Set("Name the card first, then Publish.")
			return
		}
		lib := vbLoadCards()
		lib[name] = g
		vbSaveCards(lib)
		id := vbCardPrefix + name
		// Build a brand-new slice (never append into the atom's backing array) so the
		// atom sees a distinct value and notifies subscribers — an in-place append can
		// leave the dashboard reading a stale layout until a reload.
		next := append([]dashlayout.Item(nil), layoutAtom.Get()...)
		exists := false
		for _, it := range next {
			if it.ID == id {
				exists = true
			}
		}
		if !exists {
			next = append(next, dashlayout.Item{ID: id, ColSpan: col.Get(), RowSpan: row.Get()})
		}
		layoutAtom.Set(next)
		uistate.PersistItems(next)
		published.Set("Published “" + name + "” to your dashboard.")
	})
	saveCard := ui.UseEvent(func() {
		name := strings.TrimSpace(cardName.Get())
		if name == "" {
			return
		}
		lib := vbLoadCards()
		lib[name] = g
		vbSaveCards(lib)
		rev.Set(rev.Get() + 1)
	})
	loadCard := ui.UseEvent(func(e ui.Event) {
		if c, ok := vbLoadCards()[e.GetValue()]; ok {
			setGraph(c)
			cardName.Set(e.GetValue())
			selected.Set("")
		}
	})
	deleteCard := ui.UseEvent(func() {
		name := strings.TrimSpace(cardName.Get())
		lib := vbLoadCards()
		if _, ok := lib[name]; ok {
			delete(lib, name)
			vbSaveCards(lib)
			rev.Set(rev.Get() + 1)
		}
	})

	// Evaluate against live data.
	res := cardgraph.Eval(g, cardgraph.Context{Vars: vbVariableSurface(), Datasets: vbDatasets()})
	issues := cardgraph.Validate(g)

	// Apply dragged positions onto the nodes for rendering.
	pos := vbLoadPositions()
	for i := range g.Nodes {
		if p, ok := pos[string(g.Nodes[i].ID)]; ok {
			g.Nodes[i].Pos = p
		}
	}

	span := func(n int) string { return strconv.Itoa(n*vbCellPx+(n-1)*vbGapPx) + "px" }

	return Div(css.Class("vb"),
		// Toolbar
		Div(css.Class("vb-toolbar"),
			Span(css.Class("vb-tool-label"), "Widget builder"),
			vbSelectRaw("Preset", "", append([][2]string{{"", "Load a preset…"}}, vbPresetOptions()...), loadPreset),
			vbSelectRaw("My cards", "", append([][2]string{{"", "My cards…"}}, vbCardOptions()...), loadCard),
			Input(css.Class("set-input"), Type("text"), Value(cardName.Get()), Attr("placeholder", "Card name"),
				Attr("aria-label", "Card name"), Style(map[string]string{"width": "9rem"}), OnInput(onCardName)),
			Button(css.Class("data-btn"), Type("button"), Attr("data-testid", "vb-save"), OnClick(saveCard), "Save"),
			Button(css.Class("data-btn"), Type("button"), Attr("data-testid", "vb-publish"), OnClick(publish), "Publish ▸ dashboard"),
			Button(css.Class("data-btn"), Type("button"), OnClick(deleteCard), "Delete"),
			Button(css.Class("data-btn"), Type("button"), OnClick(clearGraph), "New / clear"),
			If(published.Get() != "", Span(css.Class("t-caption"), Style(map[string]string{"color": "var(--up,#16a34a)"}), published.Get())),
			Span(css.Class("vb-sep")),
			wmStepper("W", col.Get(), "Narrower", "Wider", func() { col.Set(clampSpan(col.Get()-1, 4)) }, func() { col.Set(clampSpan(col.Get()+1, 4)) }),
			wmStepper("H", row.Get(), "Shorter", "Taller", func() { row.Set(clampSpan(row.Get()-1, 3)) }, func() { row.Set(clampSpan(row.Get()+1, 3)) }),
		),
		// Three-pane: palette | canvas | inspector
		Div(css.Class("vb-main"),
			vbPalette(addNode),
			vbCanvas(g, selected.Get(), func(id cardgraph.NodeID) { selected.Set(id) }),
			vbInspector(g, selected.Get(), issues, setProp, setVar, setRoot, deleteNode, wireInput),
		),
		// Live preview
		Div(css.Class("vb-previewpane"),
			Span(css.Class("vb-tool-label"), "Live preview"),
			Div(css.Class("wb-stage"),
				Div(css.Class("w wb-tile"), Style(map[string]string{"width": span(col.Get()), "height": span(row.Get())}),
					vbRenderTile(res, g)),
			),
		),
	)
}

// ---- data + eval context ------------------------------------------------------

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
		Accounts: app.Accounts(), Transactions: app.Transactions(), Members: app.Members(),
		Budgets: app.Budgets(), Goals: app.Goals(), Tasks: app.Tasks(),
		Rates: currency.Rates{Base: base, Rates: app.Settings().FXRates}, Now: time.Now(),
	})
}

// vbDatasets builds the app collections a source.dataset node can read.
func vbDatasets() map[string]cardgraph.Collection {
	out := map[string]cardgraph.Collection{}
	app := appstate.Default
	if app == nil {
		return out
	}
	catName := map[string]string{}
	for _, c := range app.Categories() {
		catName[c.ID] = c.Name
	}
	major := func(m money.Money) float64 {
		div := 1.0
		for i := 0; i < currency.Decimals(m.Currency); i++ {
			div *= 10
		}
		return math.Abs(float64(m.Amount) / div)
	}
	txCols := []cardgraph.Column{
		{Name: "category", Type: cardgraph.TypeText}, {Name: "payee", Type: cardgraph.TypeText},
		{Name: "amount", Type: cardgraph.TypeNumber}, {Name: "type", Type: cardgraph.TypeText},
		{Name: "month", Type: cardgraph.TypeText},
	}
	var txRows []cardgraph.Row
	for _, t := range app.Transactions() {
		kind := "transfer"
		switch {
		case t.IsIncome():
			kind = "income"
		case t.IsExpense():
			kind = "expense"
		}
		cat := catName[t.CategoryID]
		if strings.TrimSpace(cat) == "" {
			cat = "Uncategorized"
		}
		txRows = append(txRows, cardgraph.Row{
			"category": cardgraph.Text(cat), "payee": cardgraph.Text(t.Payee),
			"amount": cardgraph.Num(major(t.Amount)), "type": cardgraph.Text(kind),
			"month": cardgraph.Text(t.Date.Format("2006-01")),
		})
	}
	out["transactions"] = cardgraph.Collection{Cols: txCols, Rows: txRows}

	acctCols := []cardgraph.Column{{Name: "name", Type: cardgraph.TypeText}, {Name: "type", Type: cardgraph.TypeText}}
	var acctRows []cardgraph.Row
	for _, a := range app.Accounts() {
		if a.Archived {
			continue
		}
		acctRows = append(acctRows, cardgraph.Row{"name": cardgraph.Text(a.Name), "type": cardgraph.Text(string(a.Type))})
	}
	out["accounts"] = cardgraph.Collection{Cols: acctCols, Rows: acctRows}
	return out
}

// ---- graph persistence + helpers ----------------------------------------------

func vbLoadGraph() cardgraph.Graph {
	v := js.Global().Get("localStorage").Call("getItem", vbGraphKey)
	if v.Type() == js.TypeString && v.String() != "" {
		var g cardgraph.Graph
		if err := json.Unmarshal([]byte(v.String()), &g); err == nil && len(g.Nodes) > 0 {
			return g
		}
	}
	return vbStarterGraph()
}

func vbSaveGraph(g cardgraph.Graph) {
	if b, err := json.Marshal(g); err == nil {
		js.Global().Get("localStorage").Call("setItem", vbGraphKey, string(b))
	}
}

// vbCardsKey holds the saved-cards library (name → graph) in localStorage.
const vbCardsKey = "cashflux:wb-cards"

// vbCardPrefix namespaces a published builder card's dashboard-layout Item.ID so
// the dashboard render loop can tell user-built tiles apart from the built-in
// widgets and route them through vbPublishedWidget.
const vbCardPrefix = "wb:"

// vbPublishedWidget renders a published builder card (by saved name) as a
// dashboard tile. Returns nil if the named card no longer exists in the library
// (e.g. the user deleted it after publishing) so the tile silently drops out.
func vbPublishedWidget(name string) ui.Node {
	g, ok := vbLoadCards()[name]
	if !ok {
		return nil
	}
	res := cardgraph.Eval(g, cardgraph.Context{Vars: vbVariableSurface(), Datasets: vbDatasets()})
	return uiw.Widget(uiw.WidgetProps{
		ID: vbCardPrefix + name, Title: name, Draggable: true, Resizable: true,
		Body: vbRenderTile(res, g),
	})
}

func vbLoadCards() map[string]cardgraph.Graph {
	out := map[string]cardgraph.Graph{}
	v := js.Global().Get("localStorage").Call("getItem", vbCardsKey)
	if v.Type() == js.TypeString && v.String() != "" {
		_ = json.Unmarshal([]byte(v.String()), &out)
	}
	return out
}

func vbSaveCards(lib map[string]cardgraph.Graph) {
	if b, err := json.Marshal(lib); err == nil {
		js.Global().Get("localStorage").Call("setItem", vbCardsKey, string(b))
	}
}

// vbCardOptions lists saved card names (sorted) for the "My cards" dropdown.
func vbCardOptions() [][2]string {
	lib := vbLoadCards()
	names := make([]string, 0, len(lib))
	for n := range lib {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([][2]string, 0, len(names))
	for _, n := range names {
		out = append(out, [2]string{n, n})
	}
	return out
}

func vbCloneGraph(g cardgraph.Graph) cardgraph.Graph {
	ng := cardgraph.Graph{Root: g.Root}
	for _, n := range g.Nodes {
		props := map[string]string{}
		for k, val := range n.Props {
			props[k] = val
		}
		ng.Nodes = append(ng.Nodes, cardgraph.Node{ID: n.ID, Kind: n.Kind, Var: n.Var, Pos: n.Pos, Props: props})
	}
	ng.Edges = append(ng.Edges, g.Edges...)
	return ng
}

func vbFreshID(g cardgraph.Graph) cardgraph.NodeID {
	used := map[cardgraph.NodeID]bool{}
	for _, n := range g.Nodes {
		used[n.ID] = true
	}
	for i := 1; ; i++ {
		id := cardgraph.NodeID("n" + strconv.Itoa(i))
		if !used[id] {
			return id
		}
	}
}

func vbCascadePos(n int) cardgraph.Point {
	return cardgraph.Point{X: float64(30 + (n%4)*185), Y: float64(20 + (n/4)*92)}
}

func vbLoadPositions() map[string]cardgraph.Point {
	out := map[string]cardgraph.Point{}
	v := js.Global().Get("localStorage").Call("getItem", vbCanvasPosKey)
	if v.Type() != js.TypeString {
		return out
	}
	var saved map[string]struct{ X, Y float64 }
	if err := json.Unmarshal([]byte(v.String()), &saved); err != nil {
		return out
	}
	for k, p := range saved {
		out[k] = cardgraph.Point{X: p.X, Y: p.Y}
	}
	return out
}

// vbOutType returns a node kind's output type (via the cardgraph registry).
func vbOutType(kind string) cardgraph.PortType {
	if s, ok := cardgraph.Lookup(kind); ok {
		return s.Out
	}
	return ""
}

// ---- node catalog (palette) ----------------------------------------------------

type vbCatItem struct{ Kind, Label, Group string }

func vbCatalog() []vbCatItem {
	return []vbCatItem{
		{cardgraph.KindSourceScalar, "Figure", "Data"},
		{cardgraph.KindSourceDataset, "Dataset", "Data"},
		{cardgraph.KindLiteralNumber, "Number", "Data"},
		{cardgraph.KindLiteralText, "Text", "Data"},
		{cardgraph.KindLiteralBool, "Yes / No", "Data"},
		{cardgraph.KindFilter, "Filter", "Transform"},
		{cardgraph.KindGroupBy, "Group by", "Transform"},
		{cardgraph.KindAggregate, "Aggregate", "Transform"},
		{cardgraph.KindFormula, "Formula", "Transform"},
		{cardgraph.KindCompare, "Compare", "Logic"},
		{cardgraph.KindBranchNumber, "Branch", "Logic"},
		{cardgraph.KindVizKPI, "KPI", "Display"},
		{cardgraph.KindVizStat, "Stat + Δ", "Display"},
		{cardgraph.KindVizChart, "Chart", "Display"},
		{cardgraph.KindVizList, "List / table", "Display"},
		{cardgraph.KindVizProgress, "Progress", "Display"},
		{cardgraph.KindVizBadge, "Badge", "Display"},
		{cardgraph.KindVizText, "Text", "Display"},
	}
}

func vbKindLabel(kind string) string {
	for _, c := range vbCatalog() {
		if c.Kind == kind {
			return c.Label
		}
	}
	return kind
}

func vbDefaultProps(kind string) map[string]string {
	switch kind {
	case cardgraph.KindSourceScalar:
		return map[string]string{"name": "net_worth"}
	case cardgraph.KindSourceDataset:
		return map[string]string{"which": "transactions"}
	case cardgraph.KindLiteralNumber:
		return map[string]string{"value": "0"}
	case cardgraph.KindLiteralBool:
		return map[string]string{"value": "true"}
	case cardgraph.KindFilter:
		return map[string]string{"col": "type", "op": "==", "value": "expense"}
	case cardgraph.KindGroupBy:
		return map[string]string{"group": "category", "value": "amount", "fn": "sum"}
	case cardgraph.KindAggregate:
		return map[string]string{"col": "amount", "fn": "sum"}
	case cardgraph.KindFormula:
		return map[string]string{"expr": "a"}
	case cardgraph.KindCompare:
		return map[string]string{"op": ">"}
	case cardgraph.KindVizKPI:
		return map[string]string{"title": "KPI", "format": "number", "tone": "auto"}
	case cardgraph.KindVizStat:
		return map[string]string{"title": "Stat", "format": "currency"}
	case cardgraph.KindVizChart:
		return map[string]string{"title": "Chart", "chart": "bar"}
	case cardgraph.KindVizList:
		return map[string]string{"title": "List", "limit": "6"}
	case cardgraph.KindVizProgress:
		return map[string]string{"title": "Progress", "format": "number"}
	case cardgraph.KindVizBadge:
		return map[string]string{"title": "Badge", "tone": "auto"}
	case cardgraph.KindVizText:
		return map[string]string{"title": "Text"}
	}
	return map[string]string{}
}

// ---- param schema (drives the inspector) ---------------------------------------

type vbParam struct {
	Key, Label, Kind string // Kind: text | number | select
	Opts             [][2]string
}

func vbFormatOpts() [][2]string {
	return [][2]string{{"number", "Number"}, {"percent", "Percent"}, {"currency", "Currency"}}
}
func vbFnOpts() [][2]string {
	return [][2]string{{"sum", "Sum"}, {"avg", "Average"}, {"count", "Count"}, {"min", "Min"}, {"max", "Max"}}
}
func vbOpOpts() [][2]string {
	return [][2]string{{"==", "="}, {"!=", "≠"}, {"contains", "contains"}, {">", ">"}, {"<", "<"}, {">=", "≥"}, {"<=", "≤"}}
}

func vbParamSchema(kind string) []vbParam {
	switch kind {
	case cardgraph.KindSourceScalar:
		opts := [][2]string{}
		for _, n := range engineenv.SortedNames() {
			opts = append(opts, [2]string{n, strings.ReplaceAll(n, "_", " ")})
		}
		return []vbParam{{"name", "Figure", "select", opts}}
	case cardgraph.KindSourceDataset:
		return []vbParam{{"which", "Dataset", "select", [][2]string{{"transactions", "Transactions"}, {"accounts", "Accounts"}}}}
	case cardgraph.KindLiteralNumber:
		return []vbParam{{"value", "Value", "number", nil}}
	case cardgraph.KindLiteralText:
		return []vbParam{{"value", "Value", "text", nil}}
	case cardgraph.KindLiteralBool:
		return []vbParam{{"value", "Value", "select", [][2]string{{"true", "Yes"}, {"false", "No"}}}}
	case cardgraph.KindFilter:
		return []vbParam{{"col", "Column", "text", nil}, {"op", "Operator", "select", vbOpOpts()}, {"value", "Value", "text", nil}}
	case cardgraph.KindGroupBy:
		return []vbParam{{"group", "Group by column", "text", nil}, {"value", "Value column", "text", nil}, {"fn", "Function", "select", vbFnOpts()}}
	case cardgraph.KindAggregate:
		return []vbParam{{"col", "Column", "text", nil}, {"fn", "Function", "select", vbFnOpts()}}
	case cardgraph.KindFormula:
		return []vbParam{{"expr", "Expression", "text", nil}}
	case cardgraph.KindCompare:
		return []vbParam{{"op", "Operator", "select", vbOpOpts()}}
	case cardgraph.KindVizKPI:
		return []vbParam{{"title", "Title", "text", nil}, {"format", "Format", "select", vbFormatOpts()}, {"tone", "Tone", "select", [][2]string{{"auto", "Auto (±)"}, {"", "None"}}}}
	case cardgraph.KindVizStat:
		return []vbParam{{"title", "Title", "text", nil}, {"format", "Format", "select", vbFormatOpts()}}
	case cardgraph.KindVizChart:
		return []vbParam{{"title", "Title", "text", nil}, {"chart", "Chart", "select", [][2]string{{"bar", "Bar"}, {"line", "Line"}, {"donut", "Donut"}}}}
	case cardgraph.KindVizList:
		return []vbParam{{"title", "Title", "text", nil}, {"limit", "Max rows", "number", nil}}
	case cardgraph.KindVizProgress:
		return []vbParam{{"title", "Title", "text", nil}, {"format", "Format", "select", vbFormatOpts()}}
	case cardgraph.KindVizBadge:
		return []vbParam{{"title", "Title", "text", nil}, {"tone", "Tone", "select", [][2]string{{"auto", "Auto (±)"}, {"up", "Good"}, {"down", "Bad"}, {"", "Neutral"}}}}
	case cardgraph.KindVizText:
		return []vbParam{{"title", "Title", "text", nil}}
	}
	return nil
}

// ---- starter graph + presets ---------------------------------------------------

func vbStarterGraph() cardgraph.Graph {
	return cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceScalar, Var: "net_worth", Props: map[string]string{"name": "net_worth"}, Pos: cardgraph.Point{X: 40, Y: 40}},
			{ID: "n2", Kind: cardgraph.KindVizKPI, Props: map[string]string{"title": "Net worth", "format": "currency", "tone": "auto"}, Pos: cardgraph.Point{X: 340, Y: 40}},
		},
		Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "value"}}},
		Root:  "n2",
	}
}

// vbPresets reproduces several current dashboard widgets as graphs, so they're
// reachable from the builder (proving the existing tiles are expressible this way).
func vbPresets() map[string]cardgraph.Graph {
	p := map[string]cardgraph.Graph{}
	p["networth"] = vbStarterGraph()

	p["spend-by-cat"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceDataset, Props: map[string]string{"which": "transactions"}, Pos: cardgraph.Point{X: 30, Y: 30}},
			{ID: "n2", Kind: cardgraph.KindFilter, Props: map[string]string{"col": "type", "op": "==", "value": "expense"}, Pos: cardgraph.Point{X: 230, Y: 30}},
			{ID: "n3", Kind: cardgraph.KindGroupBy, Props: map[string]string{"group": "category", "value": "amount", "fn": "sum"}, Pos: cardgraph.Point{X: 430, Y: 30}},
			{ID: "n4", Kind: cardgraph.KindVizChart, Props: map[string]string{"title": "Spending by category", "chart": "bar"}, Pos: cardgraph.Point{X: 630, Y: 30}},
		},
		Edges: []cardgraph.Edge{
			{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "in"}},
			{From: cardgraph.PortRef{Node: "n2", Port: "out"}, To: cardgraph.PortRef{Node: "n3", Port: "in"}},
			{From: cardgraph.PortRef{Node: "n3", Port: "out"}, To: cardgraph.PortRef{Node: "n4", Port: "series"}},
		},
		Root: "n4",
	}

	p["recent"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceDataset, Props: map[string]string{"which": "transactions"}, Pos: cardgraph.Point{X: 40, Y: 40}},
			{ID: "n2", Kind: cardgraph.KindVizList, Props: map[string]string{"title": "Recent transactions", "limit": "6"}, Pos: cardgraph.Point{X: 340, Y: 40}},
		},
		Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "in"}}},
		Root:  "n2",
	}

	// figureCard builds a one-figure KPI/stat card from an engine figure.
	figureCard := func(figure, title, vizKind, format string) cardgraph.Graph {
		return cardgraph.Graph{
			Nodes: []cardgraph.Node{
				{ID: "n1", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": figure}, Pos: cardgraph.Point{X: 40, Y: 40}},
				{ID: "n2", Kind: vizKind, Props: map[string]string{"title": title, "format": format, "tone": "auto"}, Pos: cardgraph.Point{X: 340, Y: 40}},
			},
			Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "value"}}},
			Root:  "n2",
		}
	}
	p["income-stat"] = figureCard("income", "Income", cardgraph.KindVizStat, "currency")
	p["spending"] = figureCard("expense", "Spending", cardgraph.KindVizStat, "currency")
	p["liabilities"] = figureCard("liabilities", "Liabilities", cardgraph.KindVizKPI, "currency")
	p["assets"] = figureCard("assets", "Assets", cardgraph.KindVizKPI, "currency")
	p["accounts-count"] = figureCard("accounts", "Accounts", cardgraph.KindVizKPI, "number")

	// Spending breakdown as a donut (same pipeline as the bar, different chart).
	donut := vbCloneGraph(p["spend-by-cat"])
	for i := range donut.Nodes {
		if donut.Nodes[i].ID == "n4" {
			donut.Nodes[i].Props["chart"] = "donut"
			donut.Nodes[i].Props["title"] = "Spending breakdown"
		}
	}
	p["spend-donut"] = donut

	// Spending trend over time: transactions → expense filter → group by month
	// (chronological) → line chart — the time-series shape the dashboard trend/cash-flow
	// tiles use.
	p["spend-trend"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceDataset, Props: map[string]string{"which": "transactions"}, Pos: cardgraph.Point{X: 30, Y: 30}},
			{ID: "n2", Kind: cardgraph.KindFilter, Props: map[string]string{"col": "type", "op": "==", "value": "expense"}, Pos: cardgraph.Point{X: 230, Y: 30}},
			{ID: "n3", Kind: cardgraph.KindGroupBy, Props: map[string]string{"group": "month", "value": "amount", "fn": "sum", "sort": "label"}, Pos: cardgraph.Point{X: 430, Y: 30}},
			{ID: "n4", Kind: cardgraph.KindVizChart, Props: map[string]string{"title": "Spending trend", "chart": "line"}, Pos: cardgraph.Point{X: 630, Y: 30}},
		},
		Edges: []cardgraph.Edge{
			{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "in"}},
			{From: cardgraph.PortRef{Node: "n2", Port: "out"}, To: cardgraph.PortRef{Node: "n3", Port: "in"}},
			{From: cardgraph.PortRef{Node: "n3", Port: "out"}, To: cardgraph.PortRef{Node: "n4", Port: "series"}},
		},
		Root: "n4",
	}

	// Accounts list.
	p["accounts"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceDataset, Props: map[string]string{"which": "accounts"}, Pos: cardgraph.Point{X: 40, Y: 40}},
			{ID: "n2", Kind: cardgraph.KindVizList, Props: map[string]string{"title": "Accounts", "limit": "8"}, Pos: cardgraph.Point{X: 340, Y: 40}},
		},
		Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "in"}}},
		Root:  "n2",
	}
	return p
}

func vbPresetOptions() [][2]string {
	return [][2]string{
		{"networth", "Net worth (KPI)"},
		{"income-stat", "Income (stat)"},
		{"spending", "Spending (stat)"},
		{"assets", "Assets (KPI)"},
		{"liabilities", "Liabilities (KPI)"},
		{"accounts-count", "Account count (KPI)"},
		{"spend-by-cat", "Spending by category (bar)"},
		{"spend-trend", "Spending trend (line)"},
		{"spend-donut", "Spending breakdown (donut)"},
		{"recent", "Recent transactions (list)"},
		{"accounts", "Accounts (list)"},
	}
}

// ---- panes ---------------------------------------------------------------------

func vbPalette(onAdd func(string)) ui.Node {
	groups := []string{"Data", "Transform", "Logic", "Display"}
	children := []ui.Node{Span(css.Class("vb-pane-title"), "Nodes")}
	for _, grp := range groups {
		children = append(children, Span(css.Class("vb-pal-group"), grp))
		for _, c := range vbCatalog() {
			if c.Group != grp {
				continue
			}
			children = append(children, ui.CreateElement(vbPaletteBtn, vbPalBtnProps{Kind: c.Kind, Label: c.Label, OnAdd: onAdd}))
		}
	}
	return Div(css.Class("vb-palette"), children)
}

type vbPalBtnProps struct {
	Kind, Label string
	OnAdd       func(string)
}

func vbPaletteBtn(p vbPalBtnProps) ui.Node {
	kind := p.Kind
	on := ui.UseEvent(func() {
		if p.OnAdd != nil {
			p.OnAdd(kind)
		}
	})
	return Button(css.Class("vb-pal-btn"), Type("button"), Attr("data-kind", p.Kind), OnClick(on), "+ "+p.Label)
}

func vbCanvas(g cardgraph.Graph, selected cardgraph.NodeID, onSelect func(cardgraph.NodeID)) ui.Node {
	posOf := func(id cardgraph.NodeID) cardgraph.Point {
		for _, n := range g.Nodes {
			if n.ID == id {
				return n.Pos
			}
		}
		return cardgraph.Point{}
	}
	var wires []ui.Node
	for _, e := range g.Edges {
		a, b := posOf(e.From.Node), posOf(e.To.Node)
		x1, y1 := a.X+vbNodeW, a.Y+vbNodeH/2
		x2, y2 := b.X, b.Y+vbNodeH/2
		dx := (x2 - x1) / 2
		if dx < 40 {
			dx = 40
		}
		d := fmt.Sprintf("M %.1f %.1f C %.1f %.1f, %.1f %.1f, %.1f %.1f", x1, y1, x1+dx, y1, x2-dx, y2, x2, y2)
		wires = append(wires, Path(css.Class("wb-wire"), Attr("d", d), Attr("fill", "none"),
			Attr("stroke", "var(--dim,#6b7280)"), Attr("stroke-width", "2.5"), Attr("stroke-linecap", "round"),
			Attr("data-from", string(e.From.Node)), Attr("data-to", string(e.To.Node)), Attr("data-toport", e.To.Port),
			Style(map[string]string{"pointer-events": "stroke", "cursor": "pointer"})))
	}
	children := []ui.Node{
		Svg(css.Class("wb-wires"), Style(map[string]string{"position": "absolute", "left": "0", "top": "0", "overflow": "visible", "pointer-events": "none"}),
			Attr("width", "2600"), Attr("height", "1600"), wires),
	}
	for _, n := range g.Nodes {
		var inPorts []string
		hasOut := false
		if spec, ok := cardgraph.Lookup(n.Kind); ok {
			for _, p := range spec.Inputs {
				inPorts = append(inPorts, p.Name)
			}
			hasOut = spec.Out != ""
		}
		children = append(children, ui.CreateElement(vbNodeBox, vbNodeBoxProps{
			ID: n.ID, Kind: n.Kind, Var: n.Var, X: n.Pos.X, Y: n.Pos.Y,
			InPorts: inPorts, HasOut: hasOut,
			Selected: n.ID == selected, IsRoot: n.ID == g.Root, OnSelect: onSelect,
		}))
	}
	// The world layer is large and absolutely positioned; pan/zoom is a CSS transform
	// (translate + scale) applied here from the persisted view and updated live by the
	// drag shim. transform-origin 0 0 keeps the math simple (screen = world*scale + t).
	view := vbLoadView()
	worldStyle := map[string]string{
		"position": "absolute", "left": "0", "top": "0", "width": "2600px", "height": "1600px",
		"transform-origin": "0 0",
		"transform":        fmt.Sprintf("translate(%.2fpx, %.2fpx) scale(%.3f)", view.TX, view.TY, view.S),
	}
	zoomBtn := func(dir, label string) ui.Node {
		return Button(ClassStr("wb-zoom-btn"), Type("button"), Attr("data-zoom", dir), Attr("aria-label", "zoom "+dir), label)
	}
	return Div(css.Class("vb-canvas-scroll"),
		Div(css.Class("wb-canvas"), Attr("role", "list"), Style(worldStyle), children),
		Div(css.Class("wb-zoom"),
			zoomBtn("out", "−"), zoomBtn("reset", "⤢"), zoomBtn("in", "+")),
	)
}

// vbWireEdge returns g with a single edge from fromID's output into (toID, port),
// replacing any existing wire into that input (one source per input port).
func vbWireEdge(g cardgraph.Graph, fromID, toID, port string) cardgraph.Graph {
	ng := vbCloneGraph(g)
	edges := ng.Edges[:0:0]
	for _, e := range ng.Edges {
		if !(e.To.Node == cardgraph.NodeID(toID) && e.To.Port == port) {
			edges = append(edges, e)
		}
	}
	if fromID != "" && fromID != toID {
		edges = append(edges, cardgraph.Edge{
			From: cardgraph.PortRef{Node: cardgraph.NodeID(fromID), Port: cardgraph.OutPort},
			To:   cardgraph.PortRef{Node: cardgraph.NodeID(toID), Port: port},
		})
	}
	ng.Edges = edges
	return ng
}

// vbUnwire returns g with any edge into (toID, port) removed.
func vbUnwire(g cardgraph.Graph, toID, port string) cardgraph.Graph {
	ng := vbCloneGraph(g)
	edges := ng.Edges[:0:0]
	for _, e := range ng.Edges {
		if !(e.To.Node == cardgraph.NodeID(toID) && e.To.Port == port) {
			edges = append(edges, e)
		}
	}
	ng.Edges = edges
	return ng
}

// vbView is the canvas pan/zoom state: a translate (tx,ty in screen px) and a scale.
type vbView struct {
	TX, TY, S float64
}

const vbViewKey = "cashflux:wb-canvas-view"

// vbLoadView reads the persisted pan/zoom (written by the drag shim), defaulting to
// the identity view (no pan, 100% zoom).
func vbLoadView() vbView {
	out := vbView{S: 1}
	v := js.Global().Get("localStorage").Call("getItem", vbViewKey)
	if v.Type() == js.TypeString {
		var saved struct{ TX, TY, S float64 }
		if err := json.Unmarshal([]byte(v.String()), &saved); err == nil {
			out.TX, out.TY = saved.TX, saved.TY
			if saved.S > 0 {
				out.S = saved.S
			}
		}
	}
	return out
}

type vbNodeBoxProps struct {
	ID               cardgraph.NodeID
	Kind, Var        string
	X, Y             float64
	InPorts          []string // input port names (one draggable target each)
	HasOut           bool     // whether this kind has an output port
	Selected, IsRoot bool
	OnSelect         func(cardgraph.NodeID)
}

func vbNodeBox(p vbNodeBoxProps) ui.Node {
	id := p.ID
	on := ui.UseEvent(func() {
		if p.OnSelect != nil {
			p.OnSelect(id)
		}
	})
	style := map[string]string{
		"left": strconv.FormatFloat(p.X, 'f', 0, 64) + "px", "top": strconv.FormatFloat(p.Y, 'f', 0, 64) + "px",
		"position": "absolute", "width": "176px", "box-sizing": "border-box",
		"border-radius": "10px", "cursor": "grab", "background": "var(--bg-elev,#1a1a1d)",
		"border": "1.5px solid var(--line,#3a3a3d)",
	}
	if p.Selected {
		style["border-color"] = "var(--accent,#3b82f6)"
		style["box-shadow"] = "0 0 0 3px color-mix(in srgb, var(--accent,#3b82f6) 22%, transparent)"
	}
	head := vbKindLabel(p.Kind)
	if p.IsRoot {
		head = "★ " + head
	}
	sub := p.Var
	if sub == "" {
		sub = "—"
	}

	// Header: kind label + variable name.
	header := Div(css.Class("wb-node-head"), Style(map[string]string{"padding": "0.4rem 0.6rem", "border-bottom": "1px solid var(--line,#2a2a2d)"}),
		Span(css.Class("wb-node-kind"), Style(map[string]string{"font-size": "10px", "text-transform": "uppercase", "letter-spacing": "0.05em", "color": "var(--faint,#9ca3af)", "display": "block"}), head),
		Span(css.Class("wb-node-val"), Style(map[string]string{"font-size": "13px", "font-weight": "600", "white-space": "nowrap", "overflow": "hidden", "text-overflow": "ellipsis", "display": "block"}), sub),
	)

	// One row per input port: a port dot on the left edge + its label. Each dot is a
	// drag target identified by data-node/data-port (the shim wires on drop).
	rows := []any{header}
	for _, pn := range p.InPorts {
		dot := Span(css.Class("wb-port wb-port-in"), Attr("data-node", string(p.ID)), Attr("data-port", pn), Attr("data-dir", "in"), Attr("aria-hidden", "true"),
			Style(map[string]string{"position": "absolute", "left": "-7px", "top": "50%", "transform": "translateY(-50%)",
				"width": "13px", "height": "13px", "border-radius": "999px", "background": "var(--bg,#0e0e10)", "border": "2px solid var(--dim,#6b7280)"}))
		rows = append(rows, Div(css.Class("wb-port-row"), Style(map[string]string{"position": "relative", "padding": "0.25rem 0.6rem", "font-size": "11px", "color": "var(--dim,#9ca3af)"}),
			dot, Span(pn)))
	}
	// Output port: a dot on the right edge, vertically centered. It is the wire SOURCE
	// (the shim starts a connection drag from here).
	args := []any{ClassStr("wb-node"), Style(style), Attr("data-step", string(p.ID)), Attr("data-kind", p.Kind), Attr("role", "listitem"), OnClick(on)}
	for _, r := range rows {
		args = append(args, r)
	}
	if p.HasOut {
		args = append(args, Span(css.Class("wb-port wb-port-out"), Attr("data-node", string(p.ID)), Attr("data-port", "out"), Attr("data-dir", "out"), Attr("aria-hidden", "true"),
			Style(map[string]string{"position": "absolute", "right": "-7px", "top": "50%", "transform": "translateY(-50%)",
				"width": "13px", "height": "13px", "border-radius": "999px", "background": "var(--accent,#3b82f6)", "border": "2px solid var(--accent,#3b82f6)", "cursor": "crosshair"})))
	}
	return Div(args...)
}

func vbInspector(g cardgraph.Graph, selected cardgraph.NodeID, issues []cardgraph.Issue,
	setProp func(cardgraph.NodeID, string, string), setVar func(cardgraph.NodeID, string),
	setRoot func(cardgraph.NodeID), deleteNode func(cardgraph.NodeID),
	wireInput func(cardgraph.NodeID, string, string)) ui.Node {

	if selected == "" {
		return Div(css.Class("vb-inspector"),
			Span(css.Class("vb-pane-title"), "Inspector"),
			P(css.Class("t-caption", tw.TextDim), "Select a node to configure it, or add one from the palette."))
	}
	var node cardgraph.Node
	found := false
	for _, n := range g.Nodes {
		if n.ID == selected {
			node, found = n, true
		}
	}
	if !found {
		return Div(css.Class("vb-inspector"), Span(css.Class("vb-pane-title"), "Inspector"))
	}

	children := []ui.Node{
		Span(css.Class("vb-pane-title"), vbKindLabel(node.Kind)),
		// Variable name
		ui.CreateElement(vbTextField, vbTextFieldProps{Label: "Name (variable)", Value: node.Var, Placeholder: "e.g. income",
			OnSet: func(v string) { setVar(node.ID, v) }}),
	}
	// Params
	for _, pm := range vbParamSchema(node.Kind) {
		pm := pm
		if pm.Kind == "select" {
			children = append(children, ui.CreateElement(vbSelectField, vbSelectFieldProps{Label: pm.Label, Value: node.Props[pm.Key], Opts: pm.Opts,
				OnSet: func(v string) { setProp(node.ID, pm.Key, v) }}))
		} else {
			children = append(children, ui.CreateElement(vbTextField, vbTextFieldProps{Label: pm.Label, Value: node.Props[pm.Key], Numeric: pm.Kind == "number",
				OnSet: func(v string) { setProp(node.ID, pm.Key, v) }}))
		}
	}
	// Input wiring: one select per input port, listing compatible upstream nodes.
	if spec, ok := cardgraph.Lookup(node.Kind); ok && len(spec.Inputs) > 0 {
		children = append(children, Span(css.Class("vb-insp-section"), "Inputs"))
		for _, port := range spec.Inputs {
			port := port
			opts := [][2]string{{"", "— none —"}}
			for _, other := range g.Nodes {
				if other.ID == node.ID {
					continue
				}
				if cardgraph.CanFeed(vbOutType(other.Kind), port.Type) {
					label := vbKindLabel(other.Kind)
					if other.Var != "" {
						label += " (" + other.Var + ")"
					}
					opts = append(opts, [2]string{string(other.ID), label})
				}
			}
			cur := ""
			for _, e := range g.Edges {
				if e.To.Node == node.ID && e.To.Port == port.Name {
					cur = string(e.From.Node)
				}
			}
			pname := port.Name
			children = append(children, ui.CreateElement(vbSelectField, vbSelectFieldProps{
				Label: pname + " ←", Value: cur, Opts: opts,
				OnSet: func(v string) { wireInput(node.ID, pname, v) }}))
		}
	}
	// Output / delete actions
	rootBtn := Button(css.Class("data-btn", tw.Mt3), Type("button"), OnClick(ui.UseEvent(func() { setRoot(node.ID) })), "Set as output ★")
	delBtn := Button(css.Class("data-btn"), Type("button"), OnClick(ui.UseEvent(func() { deleteNode(node.ID) })), "Delete node")
	children = append(children, Div(css.Class("vb-insp-actions"), rootBtn, delBtn))

	// This node's issues (if any)
	for _, is := range issues {
		if is.Node == node.ID && is.Message != "" {
			children = append(children, P(css.Class("t-caption"), Style(map[string]string{"color": "var(--down,#dc2626)"}), is.Message))
		}
	}
	return Div(css.Class("vb-inspector"), children)
}

type vbTextFieldProps struct {
	Label, Value, Placeholder string
	Numeric                   bool
	OnSet                     func(string)
}

func vbTextField(p vbTextFieldProps) ui.Node {
	on := ui.UseEvent(func(v string) {
		if p.OnSet != nil {
			p.OnSet(v)
		}
	})
	typ := "text"
	if p.Numeric {
		typ = "number"
	}
	return Div(css.Class("wb-field"),
		Span(css.Class("wb-field-label"), p.Label),
		Input(css.Class("set-input"), Type(typ), Value(p.Value), Attr("placeholder", p.Placeholder), Attr("aria-label", p.Label), OnInput(on)),
	)
}

type vbSelectFieldProps struct {
	Label, Value string
	Opts         [][2]string
	OnSet        func(string)
}

func vbSelectField(p vbSelectFieldProps) ui.Node {
	on := ui.UseEvent(func(e ui.Event) {
		if p.OnSet != nil {
			p.OnSet(e.GetValue())
		}
	})
	nodes := make([]ui.Node, 0, len(p.Opts))
	for _, o := range p.Opts {
		nodes = append(nodes, Option(Value(o[0]), SelectedIf(o[0] == p.Value), o[1]))
	}
	return Div(css.Class("wb-field"),
		Span(css.Class("wb-field-label"), p.Label),
		Select(css.Class("set-input"), Attr("aria-label", p.Label), OnChange(on), nodes),
	)
}

// vbSelectRaw is a label-less select for the toolbar (preset picker).
func vbSelectRaw(aria, value string, opts [][2]string, on ui.Handler) ui.Node {
	nodes := make([]ui.Node, 0, len(opts))
	for _, o := range opts {
		nodes = append(nodes, Option(Value(o[0]), SelectedIf(o[0] == value), o[1]))
	}
	return Select(css.Class("set-input"), Attr("aria-label", aria), OnChange(on), nodes)
}

// ---- preview rendering ---------------------------------------------------------

func vbBaseCurrency() string {
	app := appstate.Default
	if app == nil || app.Settings().BaseCurrency == "" {
		return "USD"
	}
	return app.Settings().BaseCurrency
}

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

func vbToneClass(tone string) string {
	switch tone {
	case "up":
		return " text-up"
	case "down":
		return " text-down"
	}
	return ""
}

// vbRootFormat finds the root node's "format" prop (for currency formatting at edge).
func vbRootFormat(g cardgraph.Graph) string {
	for _, n := range g.Nodes {
		if n.ID == g.Root {
			return n.Props["format"]
		}
	}
	return ""
}

func vbRenderTile(res cardgraph.Result, g cardgraph.Graph) ui.Node {
	if res.Render == nil {
		msg := "This card isn't finished — wire a value into the output node."
		for _, is := range res.Issues {
			if is.Message != "" {
				msg = is.Message
				break
			}
		}
		return Div(
			Div(css.Class("wh"), Span(css.Class("wtitle"), "Preview")),
			Div(css.Class("wbody"), P(css.Class("t-caption", tw.TextDim), msg)),
		)
	}
	v := res.Render
	format := vbRootFormat(g)
	return Div(
		Div(css.Class("wh"), Span(css.Class("wtitle"), v.Title)),
		Div(css.Class("wbody"), vbRenderViz(v, format)),
	)
}

func vbRenderViz(v *cardgraph.VizBlock, format string) ui.Node {
	switch v.Kind {
	case "text":
		return P(css.Class("t-body"), v.Text)
	case "badge":
		st := map[string]string{"display": "inline-block", "padding": "0.25rem 0.7rem", "border-radius": "999px", "font-weight": "600",
			"background": "color-mix(in srgb, var(--accent,#3b82f6) 18%, transparent)", "color": "var(--accent,#3b82f6)"}
		if v.Tone == "up" {
			st["background"], st["color"] = "color-mix(in srgb, var(--up,#16a34a) 18%, transparent)", "var(--up,#16a34a)"
		} else if v.Tone == "down" {
			st["background"], st["color"] = "color-mix(in srgb, var(--down,#dc2626) 18%, transparent)", "var(--down,#dc2626)"
		}
		return Span(Style(st), v.Text)
	case "progress":
		fillW := strconv.FormatFloat(v.Pct*100, 'f', 1, 64) + "%"
		track := map[string]string{"width": "100%", "height": "10px", "border-radius": "999px", "background": "color-mix(in srgb, var(--dim,#6b7280) 25%, transparent)", "overflow": "hidden", "margin-top": "0.4rem"}
		fill := map[string]string{"width": fillW, "height": "100%", "background": "var(--accent,#3b82f6)"}
		if v.Tone == "up" {
			fill["background"] = "var(--up,#16a34a)"
		}
		return Div(
			Div(css.Class("fig t-figure"+vbToneClass(v.Tone), tw.FontDisplay), vbMoneyFmt(v.Text, format)),
			P(css.Class("t-caption", tw.TextDim), vbMoneyFmt(v.Sub, format)),
			Div(css.Class("wb-bar"), Style(track), Div(css.Class("wb-bar-fill"), Style(fill))),
		)
	case "stat":
		return Div(
			Div(css.Class("fig t-figure"+vbToneClass(v.Tone), tw.FontDisplay), vbMoneyFmt(v.Text, format)),
			P(css.Class("t-caption"+vbToneClass(v.Tone)), v.Sub),
		)
	case "chart":
		return vbChart(v)
	case "list":
		return vbList(v)
	default: // kpi
		return Div(
			Div(css.Class("fig t-figure"+vbToneClass(v.Tone), tw.FontDisplay), vbMoneyFmt(v.Text, format)),
		)
	}
}

var vbChartColors = []string{"#3b82f6", "#16a34a", "#f59e0b", "#ef4444", "#8b5cf6", "#06b6d4", "#ec4899", "#84cc16"}

func vbChart(v *cardgraph.VizBlock) ui.Node {
	if len(v.Series) == 0 {
		return P(css.Class("t-caption", tw.TextDim), "No data to chart.")
	}
	switch v.Chart {
	case "donut":
		return vbDonut(v.Series)
	case "line":
		return vbLine(v.Series)
	default:
		return vbBars(v.Series)
	}
}

func vbSeriesMax(s []cardgraph.SeriesPoint) float64 {
	m := 0.0
	for _, p := range s {
		if p.Value > m {
			m = p.Value
		}
	}
	if m == 0 {
		m = 1
	}
	return m
}

func vbBars(s []cardgraph.SeriesPoint) ui.Node {
	max := vbSeriesMax(s)
	cols := make([]ui.Node, 0, len(s))
	for i, p := range s {
		h := strconv.FormatFloat(p.Value/max*100, 'f', 1, 64) + "%"
		bar := Div(Style(map[string]string{"width": "100%", "height": h, "min-height": "2px", "border-radius": "4px 4px 0 0", "background": vbChartColors[i%len(vbChartColors)]}))
		col := Div(css.Class("vb-bar-col"), Style(map[string]string{"flex": "1", "display": "flex", "flex-direction": "column", "justify-content": "flex-end", "align-items": "center", "gap": "0.2rem", "min-width": "0", "height": "100%"}),
			Div(Style(map[string]string{"width": "70%", "display": "flex", "align-items": "flex-end", "height": "100%"}), bar),
			Span(Style(map[string]string{"font-size": "10px", "color": "var(--faint,#9ca3af)", "white-space": "nowrap", "overflow": "hidden", "text-overflow": "ellipsis", "max-width": "100%"}), p.Label),
		)
		cols = append(cols, col)
	}
	return Div(css.Class("vb-chart"), Style(map[string]string{"display": "flex", "align-items": "stretch", "gap": "0.4rem", "height": "120px", "padding-top": "0.3rem"}), cols)
}

func vbLine(s []cardgraph.SeriesPoint) ui.Node {
	max := vbSeriesMax(s)
	w, h := 280.0, 110.0
	pts := ""
	var dots []ui.Node
	n := len(s)
	for i, p := range s {
		x := 0.0
		if n > 1 {
			x = float64(i) / float64(n-1) * w
		}
		y := h - (p.Value/max)*h
		pts += fmt.Sprintf("%.1f,%.1f ", x, y)
		dots = append(dots, Circle(Attr("cx", fmt.Sprintf("%.1f", x)), Attr("cy", fmt.Sprintf("%.1f", y)), Attr("r", "2.5"), Attr("fill", vbChartColors[0])))
	}
	children := []ui.Node{Polyline(Attr("points", strings.TrimSpace(pts)), Attr("fill", "none"), Attr("stroke", vbChartColors[0]), Attr("stroke-width", "2"))}
	children = append(children, dots...)
	return Svg(css.Class("vb-chart"), Attr("width", "100%"), Attr("viewBox", fmt.Sprintf("0 0 %.0f %.0f", w, h)), Attr("preserveAspectRatio", "none"), Style(map[string]string{"height": "120px"}), children)
}

func vbDonut(s []cardgraph.SeriesPoint) ui.Node {
	total := 0.0
	for _, p := range s {
		total += p.Value
	}
	if total == 0 {
		total = 1
	}
	stops, acc := "", 0.0
	legend := []ui.Node{}
	for i, p := range s {
		start := acc / total * 100
		acc += p.Value
		end := acc / total * 100
		c := vbChartColors[i%len(vbChartColors)]
		if stops != "" {
			stops += ", "
		}
		stops += fmt.Sprintf("%s %.2f%% %.2f%%", c, start, end)
		legend = append(legend, Div(Style(map[string]string{"display": "flex", "align-items": "center", "gap": "0.3rem", "font-size": "11px"}),
			Span(Style(map[string]string{"width": "9px", "height": "9px", "border-radius": "2px", "background": c})),
			Span(Style(map[string]string{"color": "var(--dim,#9ca3af)"}), p.Label)))
	}
	ring := Div(Style(map[string]string{"width": "96px", "height": "96px", "border-radius": "999px",
		"background": "conic-gradient(" + stops + ")", "mask": "radial-gradient(circle 28px at center, transparent 98%, #000 100%)",
		"-webkit-mask": "radial-gradient(circle 28px at center, transparent 98%, #000 100%)"}))
	return Div(css.Class("vb-chart"), Style(map[string]string{"display": "flex", "align-items": "center", "gap": "1rem"}),
		ring, Div(Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.25rem"}), legend))
}

func vbList(v *cardgraph.VizBlock) ui.Node {
	if len(v.Rows) == 0 {
		return P(css.Class("t-caption", tw.TextDim), "No rows.")
	}
	header := make([]ui.Node, 0, len(v.Cols))
	for _, c := range v.Cols {
		header = append(header, Th(Style(map[string]string{"text-align": "left", "font-size": "11px", "color": "var(--faint,#9ca3af)", "padding": "0.2rem 0.4rem", "text-transform": "uppercase"}), c.Name))
	}
	rows := make([]ui.Node, 0, len(v.Rows))
	for _, r := range v.Rows {
		cells := make([]ui.Node, 0, len(v.Cols))
		for _, c := range v.Cols {
			cell := r[c.Name]
			txt := cell.Str
			if cell.Type == cardgraph.TypeNumber {
				txt = strconv.FormatFloat(cell.Num, 'f', -1, 64)
			}
			cells = append(cells, Td(Style(map[string]string{"padding": "0.2rem 0.4rem", "font-size": "13px", "white-space": "nowrap"}), txt))
		}
		rows = append(rows, Tr(cells))
	}
	return Table(css.Class("vb-table"), Style(map[string]string{"width": "100%", "border-collapse": "collapse"}),
		Thead(Tr(header)), Tbody(rows))
}
