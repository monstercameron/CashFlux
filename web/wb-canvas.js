// Widget Builder canvas drag shim. The builder renders draggable node boxes
// (.wb-node, each with a data-step) on a 2D surface (.wb-canvas), joined by SVG
// bezier wires (path.wb-wire, each with data-from / data-to step ids). Pointer
// dragging is handled here (not in Go) for smooth, live movement: while dragging we
// update the node's left/top and re-route the connected wires directly in the DOM;
// on release we persist the node's position to localStorage so the Go re-render
// keeps it. Uses document-level delegation so it survives the Go virtual-DOM
// re-rendering the canvas. Pure DOM, no dependencies.
(function () {
  var POS_KEY = "cashflux:wb-canvas-pos";
  var NODE_W = 156, NODE_H = 66;
  var drag = null;

  function load() {
    try { return JSON.parse(localStorage.getItem(POS_KEY) || "{}"); } catch (e) { return {}; }
  }
  function save(p) {
    try { localStorage.setItem(POS_KEY, JSON.stringify(p)); } catch (e) {}
  }

  // Recompute every wire's path from the current node positions, so wires follow
  // nodes live during a drag.
  function reroute(canvas) {
    var ports = {};
    canvas.querySelectorAll(".wb-node").forEach(function (n) {
      var step = n.getAttribute("data-step");
      var x = parseFloat(n.style.left) || 0, y = parseFloat(n.style.top) || 0;
      ports[step] = { inX: x, inY: y + NODE_H / 2, outX: x + NODE_W, outY: y + NODE_H / 2 };
    });
    canvas.querySelectorAll("path.wb-wire").forEach(function (p) {
      var f = ports[p.getAttribute("data-from")], t = ports[p.getAttribute("data-to")];
      if (!f || !t) return;
      var x1 = f.outX, y1 = f.outY, x2 = t.inX, y2 = t.inY;
      var dx = (x2 - x1) / 2; if (dx < 50) dx = 50;
      p.setAttribute("d", "M " + x1 + " " + y1 + " C " + (x1 + dx) + " " + y1 + ", " + (x2 - dx) + " " + y2 + ", " + x2 + " " + y2);
    });
  }

  document.addEventListener("mousedown", function (e) {
    var node = e.target.closest ? e.target.closest(".wb-node") : null;
    if (!node) return;
    var canvas = node.closest(".wb-canvas");
    if (!canvas) return;
    var rect = node.getBoundingClientRect();
    drag = {
      step: node.getAttribute("data-step"), el: node, canvas: canvas,
      offX: e.clientX - rect.left, offY: e.clientY - rect.top, moved: false,
    };
    e.preventDefault();
  });

  document.addEventListener("mousemove", function (e) {
    if (!drag) return;
    var c = drag.canvas.getBoundingClientRect();
    var nx = e.clientX - c.left - drag.offX;
    var ny = e.clientY - c.top - drag.offY;
    if (nx < 0) nx = 0;
    if (ny < 0) ny = 0;
    drag.el.style.left = nx + "px";
    drag.el.style.top = ny + "px";
    drag.moved = true;
    reroute(drag.canvas);
  });

  document.addEventListener("mouseup", function () {
    if (!drag) return;
    if (drag.moved) {
      var p = load();
      p[drag.step] = { x: parseFloat(drag.el.style.left) || 0, y: parseFloat(drag.el.style.top) || 0 };
      save(p);
      // Let Go know positions changed so its next render reads them (optional —
      // the DOM is already updated, this just keeps state in sync).
      window.dispatchEvent(new CustomEvent("cashflux-wb-moved"));
    }
    drag = null;
  });
})();
