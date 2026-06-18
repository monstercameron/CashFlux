// cashfluxFlipBento() animates dashboard tiles smoothly when the bento layout
// changes (drag-reorder, resize, or an auto-layout switch). CSS grid placement
// changes don't transition, so this uses the FLIP technique: it remembers each
// tile's last screen position, and on the next call measures the new position,
// jumps the tile back to where it was (no transition), then on the next frame
// transitions the offset to zero — so it appears to glide to its new slot.
//
// State (the previous positions) lives here in JS, so the Go side just calls
// this after each layout-changing render — no per-move callbacks to leak.
// Honors prefers-reduced-motion (then it only records positions, no animation).
(function () {
  var prev = {}; // data-widget id -> { x, y }

  window.cashfluxFlipBento = function () {
    var nodes = document.querySelectorAll(".bento > .w[data-widget]");
    var next = {};
    var reduce = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    for (var i = 0; i < nodes.length; i++) {
      var el = nodes[i];
      var id = el.getAttribute("data-widget");
      if (!id) continue;
      var r = el.getBoundingClientRect();
      next[id] = { x: r.left, y: r.top };
      var old = prev[id];
      if (old && !reduce) {
        var dx = old.x - r.left;
        var dy = old.y - r.top;
        if (dx || dy) {
          el.style.transition = "none";
          el.style.transform = "translate(" + dx + "px," + dy + "px)";
          el.getBoundingClientRect(); // force reflow so the offset is painted first
          requestAnimationFrame(
            (function (node) {
              return function () {
                node.style.transition = "transform .22s ease";
                node.style.transform = "";
              };
            })(el)
          );
        }
      }
    }
    prev = next;
  };
})();
