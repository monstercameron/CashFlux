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
	"github.com/monstercameron/CashFlux/internal/chartspec"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// This file owns the Widget Builder — a free-form node-graph editor (the visual
// programming system; see docs/WIDGET_BUILDER_PLAN.md). The canvas is the primary
// surface: add nodes from the palette, name them (variables usable downstream), wire
// inputs in the inspector, pick the output node, and watch the live preview render the
// evaluated card. All symbols are vb-prefixed so they never collide with widgets.go
// (which a parallel effort edits). The route in screens.go points at VisualBuilder.

const vbCellPx, vbGapPx = 152, 10               // bento cell geometry (true tile proportions)
const vbNodeW, vbNodeH = 168.0, 64.0            // canvas node box size
const vbGraphKey = "cashflux:wb-graph"          // dataset app KV: the whole card graph
const vbCanvasPosKey = "cashflux:wb-canvas-pos" // dataset app KV: node positions (drag shim, via cashfluxData* bridge)

// vbDragShimJS is the canvas drag behavior, evaluated once. It delegates pointer events
// on the document so it survives Go re-rendering the canvas: mousedown on a .wb-node
// drags it (updating left/top + re-routing wires live), mouseup persists the position
// keyed by the node's data-step (= node id). Guarded against double-install.
const vbDragShimJS = `
(function(){
  if (window.__wbCanvasInit) return;
  window.__wbCanvasInit = true;
  var POS_KEY = "cashflux:wb-canvas-pos", VIEW_KEY = "cashflux:wb-canvas-view";
  function load(k){ try { var r = (typeof window.cashfluxDataGet==="function") ? window.cashfluxDataGet(k) : ((typeof window.cashfluxStoreGet==="function") ? window.cashfluxStoreGet(k) : localStorage.getItem(k)); return JSON.parse(r || "{}"); } catch(e){ return {}; } }
  function save(k,v){ try { var s = JSON.stringify(v); if (typeof window.cashfluxDataSet==="function") window.cashfluxDataSet(k,s); else if (typeof window.cashfluxStoreSet==="function") window.cashfluxStoreSet(k,s); else localStorage.setItem(k,s); } catch(e){} }
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
    else if(dir==="fit"){
      // Frame all nodes in the viewport (with padding).
      var nodes = world.querySelectorAll(".wb-node"); if(nodes.length===0){ v={tx:0,ty:0,s:1}; }
      else {
        var minX=1e9,minY=1e9,maxX=-1e9,maxY=-1e9;
        nodes.forEach(function(n){ var x=parseFloat(n.style.left)||0, y=parseFloat(n.style.top)||0;
          if(x<minX)minX=x; if(y<minY)minY=y; if(x+176>maxX)maxX=x+176; if(y+n.offsetHeight>maxY)maxY=y+n.offsetHeight; });
        var pad=40, bw=(maxX-minX)+pad*2, bh=(maxY-minY)+pad*2;
        var s2=clampS(Math.min(r.width/bw, r.height/bh));
        v.s=s2; v.tx = r.width/2 - ((minX+maxX)/2)*s2; v.ty = r.height/2 - ((minY+maxY)/2)*s2;
      }
    }
    else { var cx=r.width/2, cy=r.height/2, wx=(cx-v.tx)/v.s, wy=(cy-v.ty)/v.s;
      var s2b = clampS(v.s * (dir==="in" ? 1.2 : 1/1.2)); v.tx = cx-wx*s2b; v.ty = cy-wy*s2b; v.s = s2b; }
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
.vb{display:flex;flex-direction:column;gap:.8rem;height:calc(100vh - 165px);min-height:620px;margin-top:1rem}
.vb-head{display:flex;flex-direction:column;gap:.15rem}
.vb-title{font-family:var(--font-display,'Fraunces',serif);font-size:1.6rem;font-weight:600;letter-spacing:-.01em;margin:0}
.vb-sub{font-size:.85rem;opacity:.65}
.vb-toolbar{display:flex;align-items:center;gap:1.5rem;row-gap:.6rem;flex-wrap:wrap;padding-bottom:.85rem;border-bottom:1px solid var(--border,#2a2a2d)}
.vb-group{display:flex;align-items:center;gap:.45rem;flex-wrap:wrap}
.vb-group-label{font-size:.62rem;font-weight:600;letter-spacing:.12em;text-transform:uppercase;opacity:.5;margin-right:.2rem;white-space:nowrap}
.vb-toolbar .set-input{width:auto;min-width:9.5rem;max-width:12rem}
.vb-sep{flex:1}
.vb-publish{padding:.4rem .9rem;font-size:.82rem}
.vb-danger{color:var(--danger,#dc2626);border-color:color-mix(in srgb, var(--danger,#dc2626) 45%, var(--border,#2a2a2d))}
.vb-danger:hover{background:color-mix(in srgb, var(--danger,#dc2626) 10%, transparent)}
.vb-status{font-size:.8rem;color:var(--accent,#3b82f6)}
.vb-main{display:flex;gap:.8rem;flex:1;min-height:0}
.vb-palette{width:180px;flex:0 0 180px;overflow:auto;display:flex;flex-direction:column;gap:.3rem;padding:.7rem;border:1px solid var(--border,#2a2a2d);border-radius:14px;background:color-mix(in srgb, var(--text,#e5e7eb) 2.5%, transparent)}
.vb-pane-title{font-size:.66rem;font-weight:600;text-transform:uppercase;letter-spacing:.14em;color:var(--accent,#3b82f6);margin-bottom:.25rem}
.vb-pal-group{font-size:.6rem;font-weight:600;text-transform:uppercase;letter-spacing:.1em;opacity:.5;margin-top:.6rem}
.vb-pal-btn{text-align:left;padding:.32rem .55rem;border-radius:7px;border:1px solid var(--border,#2a2a2d);background:none;color:inherit;cursor:pointer;font-size:12px}
.vb-pal-btn:hover{border-color:var(--accent,#3b82f6);color:var(--accent,#3b82f6)}
.vb-canvas-scroll{flex:1;min-width:0;position:relative;overflow:hidden;border-radius:14px;border:1px solid var(--border,#2a2a2d);background:var(--bg,#0e0e10);cursor:grab}
.vb-canvas-scroll:active{cursor:grabbing}
.vb-canvas-scroll .wb-canvas{background-image:radial-gradient(circle, color-mix(in srgb, var(--text-dim,#6b7280) 22%, transparent) 1px, transparent 1px);background-size:16px 16px;will-change:transform}
.wb-zoom{position:absolute;right:10px;bottom:10px;display:flex;gap:5px;z-index:5}
.wb-zoom-btn{width:30px;height:30px;border-radius:8px;border:1px solid var(--border,#2a2a2d);background:var(--bg-elev,#1a1a1d);color:inherit;cursor:pointer;font-size:16px;line-height:1;display:inline-flex;align-items:center;justify-content:center}
.wb-zoom-btn:hover{border-color:var(--accent,#3b82f6)}
.wb-port{transition:transform .1s ease, border-color .1s ease, box-shadow .1s ease; z-index:2}
.wb-port-out{cursor:crosshair}
.wb-port:hover{border-color:var(--accent,#3b82f6)!important; box-shadow:0 0 0 4px color-mix(in srgb, var(--accent,#3b82f6) 22%, transparent)}
.wb-node:hover{border-color:color-mix(in srgb, var(--accent,#3b82f6) 45%, var(--border,#3a3a3d))}
.wb-wire{transition:stroke .1s ease}
.wb-wire:hover{stroke:var(--accent,#3b82f6)!important; stroke-width:3.5!important}
.wb-port-row:hover{color:var(--text,#e5e7eb)!important}
.vb-dock{width:352px;flex:0 0 352px;display:flex;flex-direction:column;gap:.8rem;min-height:0}
.vb-inspector{flex:1 1 auto;min-height:130px;overflow:auto;display:flex;flex-direction:column;gap:.5rem;padding:.7rem;border:1px solid var(--border,#2a2a2d);border-radius:14px;background:color-mix(in srgb, var(--text,#e5e7eb) 2.5%, transparent)}
.vb-insp-section{font-size:.6rem;font-weight:600;text-transform:uppercase;letter-spacing:.1em;opacity:.55;margin-top:.5rem}
.vb-insp-actions{display:flex;gap:.4rem;margin-top:.5rem}
.vb-previewpane{display:flex;flex-direction:column;gap:.45rem;flex:0 0 auto}
.vb-preview-head{display:flex;align-items:baseline;gap:.7rem}
.vb-preview-title{font-family:var(--font-display,'Fraunces',serif);font-size:1.05rem;font-weight:600;border-left:3px solid var(--accent,#3b82f6);padding-left:.55rem}
.vb-preview-hint{font-size:.72rem;opacity:.5}
.vb-metric-doc{font-size:.72rem;opacity:.65;line-height:1.4}
.vb-metric-formula{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.68rem;color:var(--accent,#3b82f6);background:color-mix(in srgb, var(--accent,#3b82f6) 8%, transparent);border-radius:6px;padding:.3rem .45rem;line-height:1.4;word-break:break-word}
.vb .wb-field{display:flex;flex-direction:column;gap:.2rem}
.vb .wb-field-label{font-size:12px;color:var(--text-dim,#9ca3af)}
.vb .wb-stage{display:flex;align-items:flex-start;justify-content:flex-start;padding:.55rem;border-radius:14px;background:color-mix(in srgb, var(--text,#e5e7eb) 2.5%, transparent);max-height:330px;overflow:auto}
.vb .wtitle{font-family:var(--font-display,'Fraunces',serif);font-weight:600}
.vb .wb-tile .wbody{flex:1;min-height:0;display:flex;flex-direction:column;overflow:hidden}
/* D3 reads the container's measured height; guarantee one so charts never collapse to
   0px in an auto-height tile body (preview pane or a freshly-published card). */
/* Charts fill the tile body (flex:1) with a small floor so they stay legible without
   overflowing a 1-row tile; taller tiles give the chart more height automatically. */
.vb-chart{width:100%;min-height:90px;flex:1}
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

	loaded := vbLoadGraph()
	col := ui.UseState(vbDim(loaded.Cols, 4))
	row := ui.UseState(vbDim(loaded.Rows, 3))
	graph := ui.UseState(loaded)
	selected := ui.UseState(cardgraph.NodeID(""))
	undoStack := ui.UseState([]cardgraph.Graph{})
	redoStack := ui.UseState([]cardgraph.Graph{})

	g := graph.Get()
	// sized stamps the card's current width/height onto a graph so the size travels with
	// it everywhere it is persisted (working draft, library, published tile).
	sized := func(gr cardgraph.Graph) cardgraph.Graph {
		gr.Cols, gr.Rows = col.Get(), row.Get()
		return gr
	}
	// setCol/setRow change the card size and immediately persist it onto the working
	// draft (no undo entry — resizing is a view choice, not a structural edit) so the
	// chosen size survives a reload of the in-progress card.
	setCol := func(n int) {
		col.Set(n)
		ng := g
		ng.Cols, ng.Rows = n, row.Get()
		vbSaveGraph(ng)
	}
	setRow := func(n int) {
		row.Set(n)
		ng := g
		ng.Cols, ng.Rows = col.Get(), n
		vbSaveGraph(ng)
	}
	// setGraph records the prior graph for undo (and clears the redo stack), so every
	// structural edit is reversible.
	setGraph := func(ng cardgraph.Graph) {
		undoStack.Set(append(append([]cardgraph.Graph{}, undoStack.Get()...), g))
		redoStack.Set(nil)
		ng = sized(ng)
		vbSaveGraph(ng)
		graph.Set(ng)
	}
	undo := ui.UseEvent(func() {
		h := undoStack.Get()
		if len(h) == 0 {
			return
		}
		prev := h[len(h)-1]
		undoStack.Set(h[:len(h)-1])
		redoStack.Set(append(append([]cardgraph.Graph{}, redoStack.Get()...), g))
		vbSaveGraph(prev)
		graph.Set(prev)
		selected.Set("")
	})
	redo := ui.UseEvent(func() {
		f := redoStack.Get()
		if len(f) == 0 {
			return
		}
		nx := f[len(f)-1]
		redoStack.Set(f[:len(f)-1])
		undoStack.Set(append(append([]cardgraph.Graph{}, undoStack.Get()...), g))
		vbSaveGraph(nx)
		graph.Set(nx)
		selected.Set("")
	})

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
		// On unmount, null the globals so the shim's guard skips them. We intentionally
		// do NOT Release here: the shim could still hold a reference and calling a
		// released FuncOf panics ("call to released function"); a single leaked callback
		// per mount is the safe trade.
		return func() { js.Global().Set("__wbConnect", js.Null()); js.Global().Set("__wbDisconnect", js.Null()) }
	}, "vb-connect")

	// Mutations.
	addNode := func(kind string) {
		ng := vbCloneGraph(g)
		id := vbFreshID(ng)
		n := cardgraph.Node{ID: id, Kind: kind, Props: vbDefaultProps(kind), Pos: vbNextPos(ng)}
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
			// Restore the preset's intended size (if it carries one) before setGraph, so
			// the W/H steppers and the stamped graph agree (setGraph reads col/row).
			if p.Cols > 0 {
				col.Set(vbDim(p.Cols, 4))
			}
			if p.Rows > 0 {
				row.Set(vbDim(p.Rows, 3))
			}
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
		lib[name] = sized(g)
		vbSaveCards(lib)
		id := vbCardPrefix + name
		// Build a brand-new slice (never append into the atom's backing array) so the
		// atom sees a distinct value and notifies subscribers — an in-place append can
		// leave the dashboard reading a stale layout until a reload.
		next := append([]dashlayout.Item(nil), layoutAtom.Get()...)
		exists := false
		for i, it := range next {
			if it.ID == id {
				exists = true
				// Re-publishing after a resize updates the tile's span in place.
				next[i].ColSpan, next[i].RowSpan = col.Get(), row.Get()
			}
		}
		if !exists {
			next = append(next, dashlayout.Item{ID: id, ColSpan: col.Get(), RowSpan: row.Get()})
		}
		layoutAtom.Set(next)
		uistate.PersistItems(next)
		// Publishing is a deliberate, durable act that is often followed straight
		// away by a navigation or reload — flush the dataset now instead of
		// letting the 4s autosave tick race the unload (the same C2 hatch the
		// sample loader uses).
		uistate.RequestPersist()
		published.Set("Published “" + name + "” to your dashboard.")
	})
	saveCard := ui.UseEvent(func() {
		name := strings.TrimSpace(cardName.Get())
		if name == "" {
			return
		}
		lib := vbLoadCards()
		lib[name] = sized(g)
		vbSaveCards(lib)
		uistate.RequestPersist()
		rev.Set(rev.Get() + 1)
	})
	loadCard := ui.UseEvent(func(e ui.Event) {
		if c, ok := vbLoadCards()[e.GetValue()]; ok {
			// Restore the saved card's size onto the W/H steppers before loading it.
			col.Set(vbDim(c.Cols, 4))
			row.Set(vbDim(c.Rows, 3))
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
			// A published card's layout item goes with it — otherwise the
			// dashboard silently drops the tile but the Manage ledger and board
			// map keep listing a phantom widget.
			id := vbCardPrefix + name
			cur := layoutAtom.Get()
			next := make([]dashlayout.Item, 0, len(cur))
			for _, it := range cur {
				if it.ID != id {
					next = append(next, it)
				}
			}
			if len(next) != len(cur) {
				layoutAtom.Set(next)
				uistate.PersistItems(next)
			}
			uistate.RequestPersist()
			rev.Set(rev.Get() + 1)
		}
	})

	// Evaluate against live data.
	res := cardgraph.Eval(g, cardgraph.Context{Vars: vbVariableSurface(), Strs: vbStringSurface(), Datasets: vbDatasets()})
	issues := cardgraph.Validate(g)

	// Animate the live-preview KPI figure exactly like the dashboard does: countup.js
	// scans the whole document for [data-countup] and tweens to the new value. Key the
	// effect on the rendered figure+subline so it fires on mount and on value changes,
	// not on every unrelated re-render.
	figSig := ""
	if res.Render != nil {
		figSig = res.Render.Text + "|" + res.Render.Sub
	}
	ui.UseEffect(func() func() {
		if fn := js.Global().Get("cashfluxCountUpScan"); fn.Type() == js.TypeFunction {
			fn.Invoke()
		}
		return nil
	}, figSig)

	// Apply dragged positions onto the nodes for rendering.
	pos := vbLoadPositions()
	for i := range g.Nodes {
		if p, ok := pos[string(g.Nodes[i].ID)]; ok {
			g.Nodes[i].Pos = p
		}
	}

	span := func(n int) string { return strconv.Itoa(n*vbCellPx+(n-1)*vbGapPx) + "px" }

	return Div(css.Class("vb"),
		// Masthead: the studio eyebrow + serif title language the sibling tabs use.
		Div(css.Class("vb-head"),
			Span(css.Class("studio-eyebrow"), uistate.T("wman.eyebrow")),
			H2(css.Class("vb-title"), uistate.T("vbld.title")),
			Span(css.Class("vb-sub"), uistate.T("vbld.lede")),
		),
		// Command bar: intent-grouped — load something, work this card, drive the
		// canvas — with Publish as the one primary action.
		Div(css.Class("vb-toolbar"),
			Div(css.Class("vb-group"),
				Span(css.Class("vb-group-label"), uistate.T("vbld.startFrom")),
				vbSelectRaw("Preset", "", append([][2]string{{"", "Load a preset…"}}, vbPresetOptions()...), loadPreset),
				vbSelectRaw("My cards", "", append([][2]string{{"", "My cards…"}}, vbCardOptions()...), loadCard),
			),
			Div(css.Class("vb-group"),
				Span(css.Class("vb-group-label"), uistate.T("vbld.thisCard")),
				Input(css.Class("set-input"), Type("text"), Value(cardName.Get()), Attr("placeholder", "Card name"),
					Attr("aria-label", "Card name"), Style(map[string]string{"width": "9rem"}), OnInput(onCardName)),
				Button(css.Class("data-btn"), Type("button"), Attr("data-testid", "vb-save"), OnClick(saveCard), "Save"),
				Button(css.Class("btn btn-primary vb-publish"), Type("button"), Attr("data-testid", "vb-publish"), OnClick(publish), "Publish → dashboard"),
				Button(css.Class("data-btn vb-danger"), Type("button"), Attr("title", uistate.T("vbld.deleteTitle")), Attr("aria-label", uistate.T("vbld.deleteTitle")), OnClick(deleteCard), "Delete"),
			),
			Span(css.Class("vb-sep")),
			Div(css.Class("vb-group"),
				Span(css.Class("vb-group-label"), uistate.T("vbld.canvas")),
				Button(css.Class("data-btn"), Type("button"), Attr("data-testid", "vb-undo"), OnClick(undo), "↶ Undo"),
				Button(css.Class("data-btn"), Type("button"), Attr("data-testid", "vb-redo"), OnClick(redo), "↷ Redo"),
				Button(css.Class("data-btn"), Type("button"), OnClick(clearGraph), "New / clear"),
				wmStepper("W", col.Get(), "Narrower", "Wider", func() { setCol(clampSpan(col.Get()-1, 4)) }, func() { setCol(clampSpan(col.Get()+1, 4)) }),
				wmStepper("H", row.Get(), "Shorter", "Taller", func() { setRow(clampSpan(row.Get()-1, 3)) }, func() { setRow(clampSpan(row.Get()+1, 3)) }),
			),
		),
		If(published.Get() != "", Div(css.Class("vb-status"), Attr("role", "status"), published.Get())),
		// Workspace: palette | canvas | right dock (inspector over the live
		// preview) — the canvas keeps the full workspace height and the preview
		// stays in view while wiring.
		Div(css.Class("vb-main"),
			vbPalette(addNode),
			vbCanvas(g, selected.Get(), func(id cardgraph.NodeID) { selected.Set(id) }),
			Div(css.Class("vb-dock"),
				vbInspector(g, selected.Get(), issues, setProp, setVar, setRoot, deleteNode, wireInput),
				Div(css.Class("vb-previewpane"),
					Div(css.Class("vb-preview-head"),
						Span(css.Class("vb-preview-title"), "Live preview"),
						Span(css.Class("vb-preview-hint"), uistate.T("vbld.previewHint")),
					),
					Div(css.Class("wb-stage"),
						Div(css.Class("w wb-tile"), Style(map[string]string{"width": span(col.Get()), "height": span(row.Get())}),
							vbRenderTile(res, g, row.Get())),
					),
				),
			),
		),
	)
}

// ---- data + eval context ------------------------------------------------------

// vbVarsMemo caches the engine variable surface briefly: computing it walks
// pools, plans, and the smart passes over the whole ledger, and the builder
// re-renders on every keystroke and stepper click — recomputing per render
// made the whole tab lag. Data edits still show up within half a second.
var vbVarsMemo struct {
	at   time.Time
	vars map[string]float64
}

// vbVariableSurface is the FULL engine variable surface (atoms + molecules +
// custom fields + per-budget/goal/smart figures) over the current month — the
// same surface the dashboard engine and the Design tab use, so a Figure or
// Formula node here can reference every variable a published widget can,
// including cf_* custom-field values and user-edited molecules.
func vbVariableSurface() map[string]float64 {
	app := appstate.Default
	if app == nil {
		return map[string]float64{}
	}
	if vbVarsMemo.vars != nil && time.Since(vbVarsMemo.at) < 500*time.Millisecond {
		return vbVarsMemo.vars
	}
	vbVarsMemo.vars = liveEngineVars(app)
	vbVarsMemo.at = time.Now()
	return vbVarsMemo.vars
}

// vbStringSurface exposes named string values a source.scalar (text) node can bind —
// currently the net-worth month-over-month subline, computed exactly the way the
// dashboard's assets KPI does so a clone reads identically.
func vbStringSurface() map[string]string {
	out := map[string]string{}
	app := appstate.Default
	if app == nil {
		return out
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	accounts, txns := app.Accounts(), app.Transactions()
	net, _, _, err := ledger.NetWorth(accounts, txns, rates)
	if err != nil {
		return out
	}
	sub := "No change this month"
	if prev, _ := ledger.NetWorthSeries(accounts, txns, []time.Time{dateutil.MonthStart(time.Now())}, rates); len(prev) == 1 {
		if d, ok := ledger.PercentChange(net.Amount, prev[0].Amount); ok {
			delta := money.New(net.Amount-prev[0].Amount, net.Currency)
			switch {
			case d < 0:
				sub = fmt.Sprintf("▼ %d%% (%s) this month", -d, fmtMoney(delta))
			case d > 0:
				sub = fmt.Sprintf("▲ %d%% (+%s) this month", d, fmtMoney(delta))
			case delta.Amount != 0:
				sub = fmt.Sprintf("%s this month", fmtMoney(delta))
			}
		}
	}
	out["net_worth_sub"] = sub
	return out
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
		{Name: "month", Type: cardgraph.TypeText}, {Name: "date", Type: cardgraph.TypeText},
		{Name: "desc", Type: cardgraph.TypeText}, {Name: "signed", Type: cardgraph.TypeNumber},
	}
	// signedMajor preserves the sign (negative for expenses) so a recent-transactions
	// list can tone outflows red the way the dashboard ledger does.
	signedMajor := func(m money.Money) float64 {
		div := 1.0
		for i := 0; i < currency.Decimals(m.Currency); i++ {
			div *= 10
		}
		return float64(m.Amount) / div
	}
	var txRows []cardgraph.Row
	for _, t := range app.Transactions() {
		kind := "transfer"
		signed := signedMajor(t.Amount)
		switch {
		case t.IsIncome():
			kind = "income"
		case t.IsExpense():
			kind = "expense"
			signed = -math.Abs(signed)
		}
		cat := catName[t.CategoryID]
		if strings.TrimSpace(cat) == "" {
			cat = "Uncategorized"
		}
		desc := strings.TrimSpace(t.Payee)
		if desc == "" {
			desc = cat
		}
		txRows = append(txRows, cardgraph.Row{
			"category": cardgraph.Text(cat), "payee": cardgraph.Text(t.Payee),
			"amount": cardgraph.Num(major(t.Amount)), "type": cardgraph.Text(kind),
			"month": cardgraph.Text(t.Date.Format("2006-01")),
			"date":  cardgraph.Text(t.Date.Format("Jan 2")), "desc": cardgraph.Text(desc),
			"signed": cardgraph.Num(signed),
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

	// Budgets: name + limit (major units).
	budCols := []cardgraph.Column{{Name: "name", Type: cardgraph.TypeText}, {Name: "limit", Type: cardgraph.TypeNumber}}
	var budRows []cardgraph.Row
	for _, b := range app.Budgets() {
		budRows = append(budRows, cardgraph.Row{"name": cardgraph.Text(b.Name), "limit": cardgraph.Num(major(b.Limit))})
	}
	out["budgets"] = cardgraph.Collection{Cols: budCols, Rows: budRows}

	// Goals: name, target, saved (major units).
	goalCols := []cardgraph.Column{{Name: "name", Type: cardgraph.TypeText}, {Name: "target", Type: cardgraph.TypeNumber}, {Name: "saved", Type: cardgraph.TypeNumber}}
	var goalRows []cardgraph.Row
	for _, gl := range app.Goals() {
		goalRows = append(goalRows, cardgraph.Row{"name": cardgraph.Text(gl.Name), "target": cardgraph.Num(major(gl.TargetAmount)), "saved": cardgraph.Num(major(gl.CurrentAmount))})
	}
	out["goals"] = cardgraph.Collection{Cols: goalCols, Rows: goalRows}

	// Tasks: title + done flag (as text "done"/"open").
	taskCols := []cardgraph.Column{{Name: "title", Type: cardgraph.TypeText}, {Name: "status", Type: cardgraph.TypeText}}
	var taskRows []cardgraph.Row
	for _, t := range app.Tasks() {
		taskRows = append(taskRows, cardgraph.Row{"title": cardgraph.Text(t.Title), "status": cardgraph.Text(string(t.Status))})
	}
	out["tasks"] = cardgraph.Collection{Cols: taskCols, Rows: taskRows}

	// Bills: upcoming recurring charges — reuse the transactions surface labeled as a
	// separate dataset for discoverability (expenses only).
	billCols := []cardgraph.Column{{Name: "payee", Type: cardgraph.TypeText}, {Name: "amount", Type: cardgraph.TypeNumber}}
	var billRows []cardgraph.Row
	for _, t := range app.Transactions() {
		if t.IsExpense() {
			billRows = append(billRows, cardgraph.Row{"payee": cardgraph.Text(t.Payee), "amount": cardgraph.Num(major(t.Amount))})
		}
	}
	out["bills"] = cardgraph.Collection{Cols: billCols, Rows: billRows}

	// net_worth_series: end-of-month net worth for the last six months — the time series
	// behind the dashboard's net-worth trend tile, so an area/line chart clones it. Built
	// from the exact ledger.NetWorthSeries the dashboard uses.
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	monthStart := dateutil.MonthStart(time.Now())
	cutoffs := make([]time.Time, 0, 6)
	labels := make([]string, 0, 6)
	for i := 5; i >= 0; i-- {
		end := monthStart.AddDate(0, -i, 0)
		cutoffs = append(cutoffs, end)
		labels = append(labels, end.AddDate(0, 0, -1).Format("Jan"))
	}
	nwCols := []cardgraph.Column{{Name: "month", Type: cardgraph.TypeText}, {Name: "value", Type: cardgraph.TypeNumber}}
	var nwRows []cardgraph.Row
	if series, err := ledger.NetWorthSeries(app.Accounts(), app.Transactions(), cutoffs, rates); err == nil {
		for i, m := range series {
			nwRows = append(nwRows, cardgraph.Row{"month": cardgraph.Text(labels[i]), "value": cardgraph.Num(signedMajor(m))})
		}
	}
	out["net_worth_series"] = cardgraph.Collection{Cols: nwCols, Rows: nwRows}
	return out
}

// ---- graph persistence + helpers ----------------------------------------------

// vbDim returns a stored card dimension clamped to [1,max], treating 0/unset as the
// 2×2 default (so an older card saved before sizes were persisted opens at 2).
func vbDim(v, max int) int {
	if v < 1 {
		return 2
	}
	return clampSpan(v, max)
}

func vbLoadGraph() cardgraph.Graph {
	if raw := uistate.KVGet(vbGraphKey); raw != "" {
		var g cardgraph.Graph
		if err := json.Unmarshal([]byte(raw), &g); err == nil && len(g.Nodes) > 0 {
			return g
		}
	}
	return vbStarterGraph()
}

func vbSaveGraph(g cardgraph.Graph) {
	if b, err := json.Marshal(g); err == nil {
		uistate.KVSet(vbGraphKey, string(b))
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
// vbPublishedWidget renders a published builder card at the given dashboard-grid span
// (colSpan×rowSpan from the layout item). A zero span falls back to the size saved with
// the card, then to 2×2 — so a custom card honors its chosen width/height like the
// built-in tiles, instead of collapsing to a single cell.
func vbPublishedWidget(name string, colSpan, rowSpan int) ui.Node {
	g, ok := vbLoadCards()[name]
	if !ok {
		return nil
	}
	if colSpan < 1 {
		colSpan = vbDim(g.Cols, 4)
	}
	if rowSpan < 1 {
		rowSpan = vbDim(g.Rows, 3)
	}
	res := cardgraph.Eval(g, cardgraph.Context{Vars: vbVariableSurface(), Strs: vbStringSurface(), Datasets: vbDatasets()})
	return uiw.Widget(uiw.WidgetProps{
		ID: vbCardPrefix + name, Title: name, Draggable: true, Resizable: true,
		GridColumn: "span " + strconv.Itoa(clampSpan(colSpan, 4)),
		GridRow:    "span " + strconv.Itoa(clampSpan(rowSpan, 3)),
		Body:       vbRenderTile(res, g, clampSpan(rowSpan, 3)),
	})
}

func vbLoadCards() map[string]cardgraph.Graph {
	out := map[string]cardgraph.Graph{}
	if raw := uistate.KVGet(vbCardsKey); raw != "" {
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out
}

func vbSaveCards(lib map[string]cardgraph.Graph) {
	if b, err := json.Marshal(lib); err == nil {
		uistate.KVSet(vbCardsKey, string(b))
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

// vbNextPos picks a non-overlapping spot for a new node: below the lowest existing
// node (using each node's dragged position when present, else its stored Pos), so
// adding nodes after rearranging the canvas never stacks them on top of others.
func vbNextPos(g cardgraph.Graph) cardgraph.Point {
	dragged := vbLoadPositions()
	maxBottom, minLeft := 0.0, 1e9
	for _, n := range g.Nodes {
		p := n.Pos
		if dp, ok := dragged[string(n.ID)]; ok {
			p = dp
		}
		if p.Y+vbNodeH > maxBottom {
			maxBottom = p.Y + vbNodeH
		}
		if p.X < minLeft {
			minLeft = p.X
		}
	}
	if len(g.Nodes) == 0 || minLeft > 1e8 {
		minLeft = 40
	}
	return cardgraph.Point{X: minLeft, Y: maxBottom + 28}
}

func vbLoadPositions() map[string]cardgraph.Point {
	out := map[string]cardgraph.Point{}
	raw := uistate.KVGet(vbCanvasPosKey) // dataset app KV (single source of truth); migrates legacy browser-store
	if raw == "" {
		return out
	}
	var saved map[string]struct{ X, Y float64 }
	if err := json.Unmarshal([]byte(raw), &saved); err != nil {
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
		{cardgraph.KindRule, "Rule", "Transform"},
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
		{cardgraph.KindLiteralColor, "Color", "Style"},
		{cardgraph.KindStyleAccent, "Accent color", "Style"},
		{cardgraph.KindStyleTone, "Tone (▲▼)", "Style"},
		{cardgraph.KindVizStack, "Stack (compose)", "Layout"},
		{cardgraph.KindUIButton, "Button", "Interact"},
		{cardgraph.KindUIToggle, "Toggle", "Interact"},
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
		return map[string]string{"title": "KPI", "format": "number", "tone": "auto", "hero": "false"}
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
	case cardgraph.KindLiteralColor:
		return map[string]string{"value": "#3b82f6"}
	case cardgraph.KindRule:
		return map[string]string{"textcol": "payee", "amountcol": "amount", "any": "", "min": "0", "max": "0"}
	case cardgraph.KindVizStack:
		return map[string]string{"title": "Card", "dir": "column"}
	case cardgraph.KindStyleAccent:
		return map[string]string{"color": "#3b82f6"}
	case cardgraph.KindStyleTone:
		return map[string]string{"tone": "up"}
	case cardgraph.KindUIButton:
		return map[string]string{"label": "Apply rules", "action": "applyRules"}
	case cardgraph.KindUIToggle:
		return map[string]string{"label": "Mark all updated", "action": "postRecurring"}
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
		// Rendered by vbMetricField: the grouped, documented, filterable metric
		// picker — the same catalog the Design tab's picker draws from, plus the
		// live surface's raw extras (cf_* custom fields, smart_*, per-plan …).
		return []vbParam{{"name", "Figure", "metric", nil}}
	case cardgraph.KindSourceDataset:
		return []vbParam{{"which", "Dataset", "select", [][2]string{{"transactions", "Transactions"}, {"accounts", "Accounts"}, {"budgets", "Budgets"}, {"goals", "Goals"}, {"tasks", "Tasks"}, {"bills", "Bills"}, {"net_worth_series", "Net worth (6-mo series)"}}}}
	case cardgraph.KindLiteralColor:
		return []vbParam{{"value", "Color", "color", nil}}
	case cardgraph.KindRule:
		return []vbParam{{"textcol", "Text column", "text", nil}, {"any", "Keywords (any)", "text", nil}, {"amountcol", "Amount column", "text", nil}, {"min", "Min amount", "number", nil}, {"max", "Max amount", "number", nil}}
	case cardgraph.KindVizStack:
		return []vbParam{{"title", "Title", "text", nil}, {"dir", "Direction", "select", [][2]string{{"column", "Stacked (top→bottom)"}, {"row", "Side by side"}}}}
	case cardgraph.KindStyleAccent:
		return []vbParam{{"color", "Accent color", "color", nil}}
	case cardgraph.KindStyleTone:
		return []vbParam{{"tone", "Tone", "select", [][2]string{{"up", "Positive (green)"}, {"down", "Negative (red)"}, {"", "Neutral"}}}}
	case cardgraph.KindUIButton, cardgraph.KindUIToggle:
		return []vbParam{{"label", "Label", "text", nil}, {"action", "Action", "select", [][2]string{{"applyRules", "Apply rules"}, {"postRecurring", "Post recurring"}, {"addTask", "Add task"}}}}
	case cardgraph.KindLiteralNumber:
		return []vbParam{{"value", "Value", "number", nil}}
	case cardgraph.KindLiteralText:
		return []vbParam{{"value", "Value", "text", nil}}
	case cardgraph.KindLiteralBool:
		return []vbParam{{"value", "Value", "select", [][2]string{{"true", "Yes"}, {"false", "No"}}}}
	case cardgraph.KindFilter:
		return []vbParam{{"col", "Column", "text", nil}, {"op", "Operator", "select", vbOpOpts()}, {"value", "Value", "text", nil}}
	case cardgraph.KindGroupBy:
		return []vbParam{{"group", "Group by column", "text", nil}, {"value", "Value column", "text", nil}, {"fn", "Function", "select", vbFnOpts()}, {"sort", "Order", "select", [][2]string{{"value", "By value (high→low)"}, {"label", "By label (A→Z / time)"}, {"none", "Keep input order"}}}}
	case cardgraph.KindAggregate:
		return []vbParam{{"col", "Column", "text", nil}, {"fn", "Function", "select", vbFnOpts()}}
	case cardgraph.KindFormula:
		return []vbParam{{"expr", "Expression", "text", nil}}
	case cardgraph.KindCompare:
		return []vbParam{{"op", "Operator", "select", vbOpOpts()}}
	case cardgraph.KindVizKPI:
		return []vbParam{{"title", "Title", "text", nil}, {"format", "Format", "select", vbFormatOpts()}, {"tone", "Tone", "select", [][2]string{{"auto", "Auto (±)"}, {"", "None"}}}, {"hero", "Hero (large)", "select", [][2]string{{"false", "No"}, {"true", "Yes"}}}}
	case cardgraph.KindVizStat:
		return []vbParam{{"title", "Title", "text", nil}, {"format", "Format", "select", vbFormatOpts()}}
	case cardgraph.KindVizChart:
		return []vbParam{{"title", "Title", "text", nil}, {"chart", "Chart", "select", [][2]string{{"bar", "Bar"}, {"line", "Line"}, {"area", "Area"}, {"donut", "Donut"}, {"segbar", "Segmented bar"}}}, {"xlabel", "X-axis label", "text", nil}}
	case cardgraph.KindVizList:
		return []vbParam{{"title", "Title", "text", nil}, {"limit", "Max rows", "number", nil}, {"cols", "Columns (comma-sep)", "text", nil}}
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
			// limit is the hard ceiling; the tile height trims it to what fits (≈3/row), so
			// a 2-tall tile shows ~6 like the dashboard while a 3-tall tile shows more.
			{ID: "n2", Kind: cardgraph.KindVizList, Props: map[string]string{"title": "Recent transactions", "limit": "12", "cols": "date,desc,signed"}, Pos: cardgraph.Point{X: 340, Y: 40}},
		},
		Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "in"}}},
		Root:  "n2",
		Cols:  2, Rows: 2,
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

	// Net-worth trend: the 6-month end-of-month series → area chart — clones the
	// dashboard's net-worth trend tile through the same chart pipeline.
	p["networth-trend"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceDataset, Props: map[string]string{"which": "net_worth_series"}, Pos: cardgraph.Point{X: 40, Y: 40}},
			{ID: "n2", Kind: cardgraph.KindGroupBy, Props: map[string]string{"group": "month", "value": "value", "fn": "sum", "sort": "none"}, Pos: cardgraph.Point{X: 300, Y: 40}},
			{ID: "n3", Kind: cardgraph.KindVizChart, Props: map[string]string{"title": "Net worth trend", "chart": "area"}, Pos: cardgraph.Point{X: 560, Y: 40}},
		},
		Edges: []cardgraph.Edge{
			{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "in"}},
			{From: cardgraph.PortRef{Node: "n2", Port: "out"}, To: cardgraph.PortRef{Node: "n3", Port: "series"}},
		},
		Root: "n3",
	}

	// Cash flow: income − spending → stat (the surplus/deficit figure the dashboard
	// cash-flow tile shows), wired through a formula node.
	p["cashflow"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceScalar, Var: "income", Props: map[string]string{"name": "income"}, Pos: cardgraph.Point{X: 40, Y: 30}},
			{ID: "n2", Kind: cardgraph.KindSourceScalar, Var: "expense", Props: map[string]string{"name": "expense"}, Pos: cardgraph.Point{X: 40, Y: 150}},
			{ID: "n3", Kind: cardgraph.KindFormula, Props: map[string]string{"expr": "a - b"}, Pos: cardgraph.Point{X: 320, Y: 90}},
			{ID: "n4", Kind: cardgraph.KindVizStat, Props: map[string]string{"title": "Cash flow", "format": "currency", "tone": "auto"}, Pos: cardgraph.Point{X: 600, Y: 90}},
		},
		Edges: []cardgraph.Edge{
			{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n3", Port: "a"}},
			{From: cardgraph.PortRef{Node: "n2", Port: "out"}, To: cardgraph.PortRef{Node: "n3", Port: "b"}},
			{From: cardgraph.PortRef{Node: "n3", Port: "out"}, To: cardgraph.PortRef{Node: "n4", Port: "value"}},
		},
		Root: "n4",
	}

	// Assets KPI with the month-over-month subline — clones the dashboard's assets tile
	// 1:1, including the "▲ x% (+$…) this month" subtext from the string surface.
	p["assets-card"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": "assets"}, Pos: cardgraph.Point{X: 40, Y: 30}},
			{ID: "n2", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": "net_worth_sub"}, Pos: cardgraph.Point{X: 40, Y: 150}},
			{ID: "n3", Kind: cardgraph.KindVizKPI, Props: map[string]string{"title": "Assets", "format": "currency", "tone": "auto", "hero": "false"}, Pos: cardgraph.Point{X: 340, Y: 90}},
		},
		Edges: []cardgraph.Edge{
			{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n3", Port: "value"}},
			{From: cardgraph.PortRef{Node: "n2", Port: "out"}, To: cardgraph.PortRef{Node: "n3", Port: "sub"}},
		},
		Root: "n3",
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

	// Styled KPI: net worth → KPI → style.accent (custom color) → root. Demonstrates the
	// styling tools — the figure renders in the chosen accent instead of the default tone.
	p["styled-kpi"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": "net_worth"}, Pos: cardgraph.Point{X: 40, Y: 40}},
			{ID: "n2", Kind: cardgraph.KindVizKPI, Props: map[string]string{"title": "Net worth", "format": "currency", "tone": "", "hero": "true"}, Pos: cardgraph.Point{X: 320, Y: 40}},
			{ID: "n3", Kind: cardgraph.KindLiteralColor, Props: map[string]string{"value": "#8b5cf6"}, Pos: cardgraph.Point{X: 320, Y: 200}},
			{ID: "n4", Kind: cardgraph.KindStyleAccent, Props: map[string]string{"color": "#8b5cf6"}, Pos: cardgraph.Point{X: 620, Y: 90}},
		},
		Edges: []cardgraph.Edge{
			{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "value"}},
			{From: cardgraph.PortRef{Node: "n2", Port: "out"}, To: cardgraph.PortRef{Node: "n4", Port: "in"}},
			{From: cardgraph.PortRef{Node: "n3", Port: "out"}, To: cardgraph.PortRef{Node: "n4", Port: "color"}},
		},
		Root: "n4",
		Cols: 2, Rows: 1,
	}

	// Side-by-side layout: two KPIs composed in a row via the layout stack.
	p["dual-kpi"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "a1", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": "income"}, Pos: cardgraph.Point{X: 40, Y: 30}},
			{ID: "a2", Kind: cardgraph.KindVizKPI, Props: map[string]string{"title": "Income", "format": "currency", "tone": "auto"}, Pos: cardgraph.Point{X: 320, Y: 30}},
			{ID: "b1", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": "expense"}, Pos: cardgraph.Point{X: 40, Y: 200}},
			{ID: "b2", Kind: cardgraph.KindVizKPI, Props: map[string]string{"title": "Spending", "format": "currency", "tone": "auto"}, Pos: cardgraph.Point{X: 320, Y: 200}},
			{ID: "st", Kind: cardgraph.KindVizStack, Props: map[string]string{"title": "This month", "dir": "row"}, Pos: cardgraph.Point{X: 620, Y: 110}},
		},
		Edges: []cardgraph.Edge{
			{From: cardgraph.PortRef{Node: "a1", Port: "out"}, To: cardgraph.PortRef{Node: "a2", Port: "value"}},
			{From: cardgraph.PortRef{Node: "b1", Port: "out"}, To: cardgraph.PortRef{Node: "b2", Port: "value"}},
			{From: cardgraph.PortRef{Node: "a2", Port: "out"}, To: cardgraph.PortRef{Node: "st", Port: "block1"}},
			{From: cardgraph.PortRef{Node: "b2", Port: "out"}, To: cardgraph.PortRef{Node: "st", Port: "block2"}},
		},
		Root: "st",
		Cols: 4, Rows: 1,
	}

	// ── Dashboard-parity presets: every remaining dashboard widget class as a
	// from-scratch recipe (scalar/molecule → viz, dataset → list, meter). Each is
	// a teaching graph the user can reshape with formulas and custom values. ──────

	// Safe to spend — the safe_to_spend molecule as a hero KPI.
	p["safetospend"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": "safe_to_spend"}, Pos: cardgraph.Point{X: 40, Y: 40}},
			{ID: "n2", Kind: cardgraph.KindVizKPI, Props: map[string]string{"title": "Safe to spend", "format": "currency", "tone": "auto", "hero": "true"}, Pos: cardgraph.Point{X: 340, Y: 40}},
		},
		Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "value"}}},
		Root:  "n2",
		Cols:  2, Rows: 1,
	}

	// Savings rate — the savings_rate molecule as a percent KPI.
	p["savings-rate"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": "savings_rate"}, Pos: cardgraph.Point{X: 40, Y: 40}},
			{ID: "n2", Kind: cardgraph.KindVizKPI, Props: map[string]string{"title": "Savings rate", "format": "percent", "tone": "auto"}, Pos: cardgraph.Point{X: 340, Y: 40}},
		},
		Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "value"}}},
		Root:  "n2",
		Cols:  2, Rows: 1,
	}

	// Health score — the auditable health_score molecule as a 0–100 KPI.
	p["health-score"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": "health_score"}, Pos: cardgraph.Point{X: 40, Y: 40}},
			{ID: "n2", Kind: cardgraph.KindVizKPI, Props: map[string]string{"title": "Financial health", "format": "number", "tone": ""}, Pos: cardgraph.Point{X: 340, Y: 40}},
		},
		Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "value"}}},
		Root:  "n2",
		Cols:  2, Rows: 1,
	}

	// Budget meter — spending against income as a progress bar (swap either
	// scalar for a per-budget or custom-field figure to meter anything).
	p["budget-meter"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": "expense"}, Pos: cardgraph.Point{X: 40, Y: 30}},
			{ID: "n2", Kind: cardgraph.KindSourceScalar, Props: map[string]string{"name": "income"}, Pos: cardgraph.Point{X: 40, Y: 190}},
			{ID: "n3", Kind: cardgraph.KindVizProgress, Props: map[string]string{"title": "Spent of income", "format": "currency"}, Pos: cardgraph.Point{X: 360, Y: 90}},
		},
		Edges: []cardgraph.Edge{
			{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n3", Port: "value"}},
			{From: cardgraph.PortRef{Node: "n2", Port: "out"}, To: cardgraph.PortRef{Node: "n3", Port: "max"}},
		},
		Root: "n3",
		Cols: 2, Rows: 1,
	}

	// Budgets list — the budgets dataset as a table.
	p["budgets-list"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceDataset, Props: map[string]string{"which": "budgets"}, Pos: cardgraph.Point{X: 40, Y: 40}},
			{ID: "n2", Kind: cardgraph.KindVizList, Props: map[string]string{"title": "Budgets", "limit": "8"}, Pos: cardgraph.Point{X: 340, Y: 40}},
		},
		Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "in"}}},
		Root:  "n2",
		Cols:  2, Rows: 2,
	}

	// To-do list — the tasks dataset as a table.
	p["todo-list"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceDataset, Props: map[string]string{"which": "tasks"}, Pos: cardgraph.Point{X: 40, Y: 40}},
			{ID: "n2", Kind: cardgraph.KindVizList, Props: map[string]string{"title": "To-do", "limit": "8"}, Pos: cardgraph.Point{X: 340, Y: 40}},
		},
		Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "in"}}},
		Root:  "n2",
		Cols:  2, Rows: 2,
	}

	// Upcoming bills — the bills dataset as a table.
	p["bills-list"] = cardgraph.Graph{
		Nodes: []cardgraph.Node{
			{ID: "n1", Kind: cardgraph.KindSourceDataset, Props: map[string]string{"which": "bills"}, Pos: cardgraph.Point{X: 40, Y: 40}},
			{ID: "n2", Kind: cardgraph.KindVizList, Props: map[string]string{"title": "Upcoming bills", "limit": "8"}, Pos: cardgraph.Point{X: 340, Y: 40}},
		},
		Edges: []cardgraph.Edge{{From: cardgraph.PortRef{Node: "n1", Port: "out"}, To: cardgraph.PortRef{Node: "n2", Port: "in"}}},
		Root:  "n2",
		Cols:  2, Rows: 2,
	}
	return p
}

func vbPresetOptions() [][2]string {
	return [][2]string{
		{"networth", "Net worth (KPI)"},
		{"income-stat", "Income (stat)"},
		{"spending", "Spending (stat)"},
		{"assets", "Assets (KPI)"},
		{"assets-card", "Assets + monthly change (KPI)"},
		{"liabilities", "Liabilities (KPI)"},
		{"accounts-count", "Account count (KPI)"},
		{"safetospend", "Safe to spend (KPI)"},
		{"savings-rate", "Savings rate (%)"},
		{"health-score", "Financial health (0–100)"},
		{"cashflow", "Cash flow (income − spending)"},
		{"budget-meter", "Budget meter (progress)"},
		{"budgets-list", "Budgets (list)"},
		{"todo-list", "To-do (list)"},
		{"bills-list", "Upcoming bills (list)"},
		{"spend-by-cat", "Spending by category (bar)"},
		{"spend-trend", "Spending trend (line)"},
		{"spend-donut", "Spending breakdown (donut)"},
		{"networth-trend", "Net worth trend (area)"},
		{"styled-kpi", "Styled KPI (accent color)"},
		{"dual-kpi", "Income + Spending (side by side)"},
		{"recent", "Recent transactions (list)"},
		{"accounts", "Accounts (list)"},
	}
}

// ---- panes ---------------------------------------------------------------------

func vbPalette(onAdd func(string)) ui.Node {
	groups := []string{"Data", "Transform", "Logic", "Display", "Style", "Layout", "Interact"}
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
			Attr("stroke", "var(--text-dim,#6b7280)"), Attr("stroke-width", "2.5"), Attr("stroke-linecap", "round"),
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
			zoomBtn("fit", "⤡"), zoomBtn("out", "−"), zoomBtn("reset", "⤢"), zoomBtn("in", "+")),
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
	if raw := uistate.KVGet(vbViewKey); raw != "" { // dataset app KV; migrates legacy browser-store
		var saved struct{ TX, TY, S float64 }
		if err := json.Unmarshal([]byte(raw), &saved); err == nil {
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
		"border": "1.5px solid var(--border,#3a3a3d)",
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
	header := Div(css.Class("wb-node-head"), Style(map[string]string{"padding": "0.4rem 0.6rem", "border-bottom": "1px solid var(--border,#2a2a2d)"}),
		Span(css.Class("wb-node-kind"), Style(map[string]string{"font-size": "10px", "text-transform": "uppercase", "letter-spacing": "0.05em", "color": "var(--text-faint,#9ca3af)", "display": "block"}), head),
		Span(css.Class("wb-node-val"), Style(map[string]string{"font-size": "13px", "font-weight": "600", "white-space": "nowrap", "overflow": "hidden", "text-overflow": "ellipsis", "display": "block"}), sub),
	)

	// One row per input port: a port dot on the left edge + its label. Each dot is a
	// drag target identified by data-node/data-port (the shim wires on drop).
	rows := []any{header}
	for _, pn := range p.InPorts {
		dot := Span(css.Class("wb-port wb-port-in"), Attr("data-node", string(p.ID)), Attr("data-port", pn), Attr("data-dir", "in"), Attr("aria-hidden", "true"),
			Style(map[string]string{"position": "absolute", "left": "-7px", "top": "50%", "transform": "translateY(-50%)",
				"width": "13px", "height": "13px", "border-radius": "999px", "background": "var(--bg,#0e0e10)", "border": "2px solid var(--text-dim,#6b7280)"}))
		rows = append(rows, Div(css.Class("wb-port-row"), Style(map[string]string{"position": "relative", "padding": "0.25rem 0.6rem", "font-size": "11px", "color": "var(--text-dim,#9ca3af)"}),
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

// vbColumnKeys names the props on a data-shaping node that reference a column
// of its wired input collection — candidates for a schema-fed dropdown.
func vbColumnKeys(kind string) map[string]bool {
	switch kind {
	case cardgraph.KindFilter, cardgraph.KindAggregate:
		return map[string]bool{"col": true}
	case cardgraph.KindGroupBy:
		return map[string]bool{"group": true, "value": true}
	case cardgraph.KindRule:
		return map[string]bool{"textcol": true, "amountcol": true}
	}
	return nil
}

// vbUpstreamColumns resolves the column names available to a data-shaping
// node: it walks the node's "in" wiring upstream — through shape-preserving
// filter/rule steps — to the nearest source.dataset and returns that
// collection's columns. Unwired chains, unknown datasets, and reshaping steps
// (group-by changes the columns) return nil; the caller falls back to a
// free-text field.
func vbUpstreamColumns(g cardgraph.Graph, id cardgraph.NodeID) []string {
	cur := id
	for hops := 0; hops < 12; hops++ {
		var from cardgraph.NodeID
		for _, e := range g.Edges {
			if e.To.Node == cur && e.To.Port == "in" {
				from = e.From.Node
			}
		}
		if from == "" {
			return nil
		}
		advanced := false
		for _, n := range g.Nodes {
			if n.ID != from {
				continue
			}
			switch n.Kind {
			case cardgraph.KindSourceDataset:
				which := strings.TrimSpace(n.Props["which"])
				c, ok := vbDatasets()[which]
				if !ok {
					return nil
				}
				out := make([]string, 0, len(c.Cols))
				for _, col := range c.Cols {
					out = append(out, col.Name)
				}
				return out
			case cardgraph.KindFilter, cardgraph.KindRule:
				cur, advanced = n.ID, true
			default:
				return nil
			}
			break
		}
		if !advanced {
			return nil
		}
	}
	return nil
}

type vbMetricFieldProps struct {
	Value string
	OnSet func(string)
}

// vbMetricField is the Figure node's variable picker: the grouped metric
// catalog ("Group · Label") with a type-ahead filter, the selected metric's
// plain-English description, and — for molecules — the atom-built formula, so
// the canvas explains figures as well as the Design tab's picker does. Raw
// live-surface variables the catalog doesn't label (plan/what-if figures,
// engine internals) stay reachable under "Advanced". Its own component so the
// filter state and hooks are isolated.
func vbMetricField(p vbMetricFieldProps) ui.Node {
	q := ui.UseState("")
	onQ := ui.UseEvent(func(v string) { q.Set(v) })
	onSel := ui.UseEvent(func(e ui.Event) {
		if p.OnSet != nil {
			p.OnSet(e.GetValue())
		}
	})

	needle := strings.ToLower(strings.TrimSpace(q.Get()))
	match := func(parts ...string) bool {
		if needle == "" {
			return true
		}
		for _, s := range parts {
			if strings.Contains(strings.ToLower(s), needle) {
				return true
			}
		}
		return false
	}

	var opts []ui.Node
	seen := map[string]bool{}
	selDoc, selFormula := "", ""
	haveCurrent := p.Value == ""
	for _, m := range studioAllMetrics() {
		if seen[m.Name] {
			continue
		}
		seen[m.Name] = true
		if m.Name == p.Value {
			selDoc = m.Doc
			if m.Molecule {
				selFormula = m.Formula
			}
		}
		if !match(m.Name, m.Label, string(m.Group)) && m.Name != p.Value {
			continue
		}
		if m.Name == p.Value {
			haveCurrent = true
		}
		opts = append(opts, Option(Value(m.Name), SelectedIf(m.Name == p.Value), string(m.Group)+" · "+m.Label))
	}
	var extras []string
	for n := range vbVariableSurface() {
		if !seen[n] {
			extras = append(extras, n)
		}
	}
	sort.Strings(extras)
	for _, n := range extras {
		lbl := "Advanced · " + strings.ReplaceAll(n, "_", " ")
		if !match(n, lbl) && n != p.Value {
			continue
		}
		if n == p.Value {
			haveCurrent = true
		}
		opts = append(opts, Option(Value(n), SelectedIf(n == p.Value), lbl))
	}
	// Named text surfaces (the net-worth month-over-month subline) so a KPI's
	// "sub" line can clone the dashboard's assets tile verbatim.
	if match("net_worth_sub", "net worth subline") || p.Value == "net_worth_sub" {
		if p.Value == "net_worth_sub" {
			haveCurrent = true
		}
		opts = append(opts, Option(Value("net_worth_sub"), SelectedIf(p.Value == "net_worth_sub"), uistate.T("vbld.textSubline")))
	}
	// The current value always stays visible/selected even when the filter
	// would hide it — a filtered select must never LOOK re-bound.
	if !haveCurrent {
		opts = append([]ui.Node{Option(Value(p.Value), SelectedIf(true), strings.ReplaceAll(p.Value, "_", " "))}, opts...)
	}

	kids := []any{ClassStr("wb-field"),
		Span(css.Class("wb-field-label"), uistate.T("vbld.figure")),
		Input(css.Class("set-input"), Type("search"), Placeholder(uistate.T("vbld.filterMetrics")), Attr("aria-label", uistate.T("vbld.filterMetrics")), Value(q.Get()), OnInput(onQ)),
		Select(css.Class("set-input"), Attr("aria-label", uistate.T("vbld.figure")), OnChange(onSel), opts),
	}
	if selDoc != "" {
		kids = append(kids, Span(css.Class("vb-metric-doc"), selDoc))
	}
	if selFormula != "" {
		kids = append(kids, Span(css.Class("vb-metric-formula"), uistate.T("vbld.builtFromAtoms")+"  "+prettyFormula(selFormula)))
	}
	return Div(kids...)
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
	// Params. Column-referencing props on data-shaping nodes become dropdowns
	// of the wired dataset's real fields — nobody should have to KNOW that the
	// transactions collection calls its category column "category".
	upstreamCols := vbUpstreamColumns(g, node.ID)
	colKeys := vbColumnKeys(node.Kind)
	for _, pm := range vbParamSchema(node.Kind) {
		pm := pm
		switch {
		case pm.Kind == "metric":
			children = append(children, ui.CreateElement(vbMetricField, vbMetricFieldProps{Value: node.Props[pm.Key],
				OnSet: func(v string) { setProp(node.ID, pm.Key, v) }}))
		case colKeys[pm.Key] && len(upstreamCols) > 0:
			cur := node.Props[pm.Key]
			opts := [][2]string{{"", "— pick a column —"}}
			found := cur == ""
			for _, c := range upstreamCols {
				opts = append(opts, [2]string{c, c})
				if c == cur {
					found = true
				}
			}
			if !found {
				opts = append(opts, [2]string{cur, cur + " (not in source)"})
			}
			children = append(children, ui.CreateElement(vbSelectField, vbSelectFieldProps{Label: pm.Label, Value: cur, Opts: opts,
				OnSet: func(v string) { setProp(node.ID, pm.Key, v) }}))
		case pm.Kind == "select":
			children = append(children, ui.CreateElement(vbSelectField, vbSelectFieldProps{Label: pm.Label, Value: node.Props[pm.Key], Opts: pm.Opts,
				OnSet: func(v string) { setProp(node.ID, pm.Key, v) }}))
		default:
			children = append(children, ui.CreateElement(vbTextField, vbTextFieldProps{Label: pm.Label, Value: node.Props[pm.Key], Numeric: pm.Kind == "number", Color: pm.Kind == "color",
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
	Color                     bool
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
	} else if p.Color {
		typ = "color"
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

// vbRenderTile renders the card body at the given row height (rowSpan, 1..3) so
// height-sensitive content (lists) can show more rows in a taller tile — the card
// respecting its size, not just the grid cell.
func vbRenderTile(res cardgraph.Result, g cardgraph.Graph, rows int) ui.Node {
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
		Div(css.Class("wbody"), vbRenderViz(v, format, rows)),
	)
}

func vbRenderViz(v *cardgraph.VizBlock, format string, rows int) ui.Node {
	switch v.Kind {
	case "text":
		return P(css.Class("t-body"), v.Text)
	case "badge":
		accent := vbAccentOr(v.Accent, "var(--accent,#3b82f6)")
		st := map[string]string{"display": "inline-block", "padding": "0.25rem 0.7rem", "border-radius": "999px", "font-weight": "600",
			"background": "color-mix(in srgb, " + accent + " 18%, transparent)", "color": accent}
		if v.Tone == "up" {
			st["background"], st["color"] = "color-mix(in srgb, var(--up,#16a34a) 18%, transparent)", "var(--up,#16a34a)"
		} else if v.Tone == "down" {
			st["background"], st["color"] = "color-mix(in srgb, var(--down,#dc2626) 18%, transparent)", "var(--down,#dc2626)"
		}
		return Span(Style(st), v.Text)
	case "progress":
		fillW := strconv.FormatFloat(v.Pct*100, 'f', 1, 64) + "%"
		track := map[string]string{"width": "100%", "height": "10px", "border-radius": "999px", "background": "color-mix(in srgb, var(--text-dim,#6b7280) 25%, transparent)", "overflow": "hidden", "margin-top": "0.4rem"}
		fill := map[string]string{"width": fillW, "height": "100%", "background": vbAccentOr(v.Accent, "var(--accent,#3b82f6)")}
		if v.Tone == "up" {
			fill["background"] = "var(--up,#16a34a)"
		}
		return Div(
			Div(css.Class("fig t-figure"+vbToneClass(v.Tone), tw.FontDisplay), vbMoneyFmt(v.Text, format)),
			P(css.Class("t-caption", tw.TextDim), vbMoneyFmt(v.Sub, format)),
			Div(css.Class("wb-bar"), Style(track), Div(css.Class("wb-bar-fill"), Style(fill))),
		)
	case "stat":
		figStyle := map[string]string{}
		// Accent recolors the figure only when there's no semantic ± tone (tone wins,
		// since red/green carry meaning a decorative accent shouldn't override).
		if v.Accent != "" && v.Tone == "" {
			figStyle["color"] = v.Accent
		}
		return Div(
			Div(css.Class("fig t-figure"+vbToneClass(v.Tone), tw.FontDisplay), Style(figStyle), vbMoneyFmt(v.Text, format)),
			P(css.Class("t-caption"+vbToneClass(v.Tone)), v.Sub),
		)
	case "chart":
		return vbChart(v)
	case "list":
		return vbList(v, rows)
	case "stack":
		// Composite tile: render each child block. Direction = column (top→bottom,
		// default) or row (side by side) per the layout node.
		blocks := make([]ui.Node, 0, len(v.Blocks))
		rowDir := v.Dir == "row"
		for i := range v.Blocks {
			b := v.Blocks[i]
			margin := map[string]string{"margin-bottom": "0.6rem"}
			if rowDir {
				margin = map[string]string{"flex": "1", "min-width": "0"}
			}
			blocks = append(blocks, Div(Style(margin), vbRenderViz(&b, format, rows)))
		}
		stackStyle := map[string]string{}
		if rowDir {
			stackStyle = map[string]string{"display": "flex", "gap": "0.6rem", "align-items": "flex-start"}
		}
		return Div(css.Class("vb-stack"), Style(stackStyle), blocks)
	case "button":
		return ui.CreateElement(vbActionButton, vbActionButtonProps{Label: v.Text, Action: v.Action})
	case "toggle":
		return ui.CreateElement(vbToggle, vbActionButtonProps{Label: v.Text, Action: v.Action})
	default: // kpi — render through the dashboard's own KPI body so a clone matches 1:1.
		figTone := ""
		if v.Tone == "up" {
			figTone = "text-up"
		} else if v.Tone == "down" {
			figTone = "text-down"
		}
		fig := vbMoneyFmt(v.Text, format)
		var body ui.Node
		if v.Hero {
			body = kpiBodyHero(fig, figTone, v.Sub, "text-dim")
		} else {
			body = kpiBody(fig, figTone, v.Sub, "text-dim")
		}
		// Accent recolors the KPI figure when no ± tone applies: the figure inherits
		// color (ColorClass("") sets none), so a colored wrapper cascades onto it.
		if v.Accent != "" && figTone == "" {
			return Div(Style(map[string]string{"color": v.Accent}), body)
		}
		return body
	}
}

// vbAccentOr returns the explicit accent color if set, else the fallback (the theme
// accent var). Centralizes the "honor a wired style.accent, else theme" rule.
func vbAccentOr(accent, fallback string) string {
	if strings.TrimSpace(accent) != "" {
		return accent
	}
	return fallback
}

// vbChart renders through the SAME D3 stack (uiw.Chart + chartspec) the dashboard
// uses, so a cloned chart tile is visually identical (axes, area fill, animation,
// theme color) rather than a separate CSS approximation.
func vbChart(v *cardgraph.VizBlock) ui.Node {
	if len(v.Series) == 0 {
		return P(css.Class("t-caption", tw.TextDim), "No data to chart.")
	}
	if v.Chart == "segbar" {
		return vbSegBar(v)
	}
	kind := chartspec.Bar
	switch v.Chart {
	case "line":
		kind = chartspec.Line
	case "area":
		kind = chartspec.Area
	case "donut":
		kind = chartspec.Donut
	}
	pts := make([]chartspec.Point, len(v.Series))
	for i, p := range v.Series {
		pts[i] = chartspec.Point{X: float64(i), Y: p.Value, Label: p.Label}
	}
	sym := currency.Symbol(vbBaseCurrency())
	yFmt := ".3~s"
	if sym == "$" {
		yFmt = "$.3~s"
	}
	spec := chartspec.Spec{
		Kind:   kind,
		Series: []chartspec.Series{{Color: v.Accent, Points: pts}},
		X:      chartspec.Axis{Label: v.XLabel},
		Y:      chartspec.Axis{Format: yFmt},
	}
	if kind == chartspec.Donut {
		spec.Legend = true
	}
	return uiw.Chart(uiw.ChartProps{Spec: spec, Height: "100%", Class: "vb-chart", CurrencySymbol: sym})
}

// vbSegBar renders a percentage-segmented horizontal bar + legend (the breakdown tile).
func vbSegBar(v *cardgraph.VizBlock) ui.Node {
	total := 0.0
	for _, p := range v.Series {
		total += p.Value
	}
	if total == 0 {
		total = 1
	}
	tones := []string{"var(--accent,#3b82f6)", "var(--up,#16a34a)", "var(--warn,#f59e0b)", "var(--down,#dc2626)", "#8b5cf6", "#06b6d4"}
	segs := make([]ui.Node, 0, len(v.Series))
	legend := make([]ui.Node, 0, len(v.Series))
	for i, p := range v.Series {
		c := tones[i%len(tones)]
		w := strconv.FormatFloat(p.Value/total*100, 'f', 2, 64) + "%"
		segs = append(segs, Div(Style(map[string]string{"width": w, "background": c, "height": "100%"})))
		legend = append(legend, Div(Style(map[string]string{"display": "flex", "align-items": "center", "gap": "0.35rem", "font-size": "12px"}),
			Span(Style(map[string]string{"width": "10px", "height": "10px", "border-radius": "3px", "background": c})),
			Span(css.Class("t-caption", tw.TextDim), p.Label)))
	}
	return Div(css.Class("vb-chart vb-segbar"),
		Div(Style(map[string]string{"display": "flex", "width": "100%", "height": "14px", "border-radius": "999px", "overflow": "hidden", "margin-bottom": "0.6rem"}), segs),
		Div(Style(map[string]string{"display": "flex", "flex-direction": "column", "gap": "0.25rem"}), legend),
	)
}

// vbActionButtonProps configures an interactive button node.
type vbActionButtonProps struct {
	Label, Action string
}

// vbActionButton renders the ui.button node: a button that runs a workflow action
// (postRecurring / applyRules / addTask) against app state on click — the builder's
// basic interactivity, the same class of action the dashboard To-do tile performs.
func vbActionButton(p vbActionButtonProps) ui.Node {
	action := p.Action
	on := ui.UseEvent(func() { vbRunAction(action) })
	return Button(css.Class("data-btn"), Type("button"), Attr("data-vb-action", action), OnClick(on), p.Label)
}

func vbBoolAttr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// vbToggle renders the ui.toggle node: a stateful checkbox that runs its action on
// change (the To-do tile's interactivity class — a checkbox, not just a button). Its
// checked state is persisted to localStorage keyed by action so it survives the tile
// re-evaluating/remounting (which happens on any graph edit or data-revision bump) the
// way the To-do tile's checkbox derives its state from the stored task, not ephemeral
// component state.
func vbToggle(p vbActionButtonProps) ui.Node {
	action := p.Action
	checked := ui.UseState(vbToggleState(action))
	on := ui.UseEvent(func() {
		next := !checked.Get()
		checked.Set(next)
		vbSetToggleState(action, next)
		vbRunAction(action)
	})
	box := "☐"
	if checked.Get() {
		box = "☑"
	}
	return Button(css.Class("data-btn"), Type("button"), Attr("role", "switch"), Attr("aria-checked", vbBoolAttr(checked.Get())),
		Attr("data-vb-action", action), OnClick(on), box+" "+p.Label)
}

const vbTogglePrefix = "cashflux:wb-toggle:"

// vbToggleState reads a toggle's persisted checked state from the dataset KV
// (a legacy browserstore value migrates in on first read).
func vbToggleState(action string) bool {
	return uistate.KVGet(vbTogglePrefix+action) == "1"
}

// vbSetToggleState persists a toggle's checked state into the dataset KV.
func vbSetToggleState(action string, on bool) {
	val := "0"
	if on {
		val = "1"
	}
	uistate.KVSet(vbTogglePrefix+action, val)
}

// vbRunAction applies a builder button/toggle action to app state, then bumps the data
// revision so the result shows immediately (like the To-do tile).
func vbRunAction(action string) {
	app := appstate.Default
	if app == nil {
		return
	}
	switch action {
	case "postRecurring":
		_, _ = app.PostDueRecurring(time.Now())
	case "applyRules":
		_, _ = app.ApplyRules()
	case "addTask":
		_, _ = app.CreateFreshnessReminderTask("From a custom widget")
	default:
		return
	}
	uistate.BumpDataRevision()
}

// vbList renders a Collection as the dashboard's "recent transactions" tile renders
// it — headerless, t-body table, dimmed leading date column, and numeric columns shown
// as accounting money (currency symbol, parentheses for negatives) toned green/red via
// the same fmtMoney/figTone/ColorClass path the dashboard uses — so a list clone is
// byte-for-byte the same DOM, not a look-alike.
func vbList(v *cardgraph.VizBlock, tileRows int) ui.Node {
	if len(v.Rows) == 0 {
		return P(css.Class("empty t-body", tw.TextDim), "No rows.")
	}
	// Respect the tile's height: show as many rows as actually fit. The tile is
	// tileRows grid cells tall (vbCellPx each + gaps); subtract the header band and
	// divide by a table row's height. The engine's "limit" prop remains the hard
	// ceiling — this only trims to what fits, so a 1-tall list isn't a clipped overflow
	// and a 3-tall list grows toward its limit (≈3 rows at 1-tall, ≈12 at 3-tall).
	src := v.Rows
	if tileRows >= 1 {
		const vbListHeaderPx, vbListRowPx = 44, 34
		tilePx := tileRows*vbCellPx + (tileRows-1)*vbGapPx
		fit := (tilePx - vbListHeaderPx) / vbListRowPx
		if fit < 1 {
			fit = 1
		}
		if fit < len(src) {
			src = src[:fit]
		}
	}
	rows := make([]ui.Node, 0, len(src))
	for _, r := range src {
		cells := make([]ui.Node, 0, len(v.Cols))
		for i, c := range v.Cols {
			cell := r[c.Name]
			switch {
			case c.Type == cardgraph.TypeNumber:
				m := vbMoneyMajor(cell.Num)
				cells = append(cells, Td(ClassStr("fig "+tw.Fold(tw.Py25, tw.TextRight, tw.FontDisplay)+" "+tw.ColorClass(figTone(m))), fmtMoney(m)))
			case i == 0 && (c.Name == "date" || c.Name == "month"):
				// Leading date column: dimmed, fixed-width — matches the recent tile.
				cells = append(cells, Td(css.Class("fig", tw.Py25, tw.TextDim, tw.W16), cell.Str))
			default:
				cells = append(cells, Td(css.Class(tw.Py25), cell.Str))
			}
		}
		rows = append(rows, Tr(css.Class(tw.BorderB, tw.BorderLine70), cells))
	}
	return Table(css.Class("vb-table t-body", tw.WFull), Tbody(rows))
}

// vbMoneyMajor rebuilds a Money in the base currency from a major-unit float (the form
// vbDatasets exposes), so list/figure clones format through the canonical fmtMoney.
func vbMoneyMajor(major float64) money.Money {
	base := vbBaseCurrency()
	mult := 1.0
	for i := 0; i < currency.Decimals(base); i++ {
		mult *= 10
	}
	return money.New(int64(math.Round(major*mult)), base)
}
