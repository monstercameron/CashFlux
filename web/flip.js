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
  var rafToken = 0;
  var drag = null;
  var pendingSourceId = "";
  var retargetingDrag = false;
  var scrollLockRAF = 0;

  function widgetNodes() {
    return document.querySelectorAll(".bento > .w[data-widget]");
  }

  function center(rect) {
    return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
  }

  function contains(rect, x, y, pad) {
    return x >= rect.left - pad && x <= rect.right + pad && y >= rect.top - pad && y <= rect.bottom + pad;
  }

  function scrollHost() {
    return document.querySelector("main.cf-scroll") || document.scrollingElement || document.documentElement;
  }

  function restoreDragScroll() {
    var state = drag || pendingScroll;
    if (!state) return;
    if (state.scrollEl && state.scrollEl.scrollTop !== state.scrollTop) state.scrollEl.scrollTop = state.scrollTop;
    if (window.scrollY !== state.windowY) window.scrollTo(window.scrollX, state.windowY);
  }

  function startScrollLockLoop() {
    if (scrollLockRAF) return;
    var tick = function () {
      if (!drag && !pendingScroll) {
        scrollLockRAF = 0;
        return;
      }
      restoreDragScroll();
      scrollLockRAF = requestAnimationFrame(tick);
    };
    scrollLockRAF = requestAnimationFrame(tick);
  }

  var pendingScroll = null;

  function armScrollLock(sourceId) {
    var scroller = scrollHost();
    pendingSourceId = sourceId || "";
    pendingScroll = {
      scrollEl: scroller,
      scrollTop: scroller ? scroller.scrollTop : 0,
      windowY: window.scrollY,
    };
    document.documentElement.setAttribute("data-bento-dragging", sourceId || "true");
    startScrollLockLoop();
  }

  function tileByID(id) {
    if (!id) return null;
    return document.querySelector('.bento > .w[data-widget="' + id.replace(/"/g, '\\"') + '"]');
  }

  function clearScrollLock() {
    pendingSourceId = "";
    pendingScroll = null;
    if (!drag) document.documentElement.removeAttribute("data-bento-dragging");
  }

  // Snapshot the grid before live preview reflows begin. During drag, hit-testing
  // against moving DOM nodes can cause A/B/A target oscillation; this stable map
  // keeps the insertion target tied to pointer travel, not to animated siblings.
  window.cashfluxBentoDragStart = function (sourceId) {
    if (!sourceId && pendingSourceId) sourceId = pendingSourceId;
    var zones = [];
    var nodes = widgetNodes();
    for (var i = 0; i < nodes.length; i++) {
      var el = nodes[i];
      var id = el.getAttribute("data-widget");
      if (!id || id === sourceId) continue;
      var r = el.getBoundingClientRect();
      zones.push({
        id: id,
        left: r.left,
        right: r.right,
        top: r.top,
        bottom: r.bottom,
        width: r.width,
        height: r.height,
        c: center(r),
      });
    }
    var scroller = (pendingScroll && pendingScroll.scrollEl) || scrollHost();
    drag = {
      sourceId: sourceId,
      zones: zones,
      targetId: "",
      lastX: 0,
      lastY: 0,
      scrollEl: scroller,
      scrollTop: pendingScroll ? pendingScroll.scrollTop : scroller ? scroller.scrollTop : 0,
      windowY: pendingScroll ? pendingScroll.windowY : window.scrollY,
    };
    document.documentElement.setAttribute("data-bento-dragging", sourceId || "true");
    startScrollLockLoop();
  };

  window.cashfluxBentoDragEnd = function () {
    restoreDragScroll();
    drag = null;
    clearScrollLock();
  };

  window.cashfluxBentoDragTarget = function (x, y) {
    if (!drag || !drag.zones.length) return "";
    restoreDragScroll();
    x = Number(x);
    y = Number(y);
    if (!isFinite(x) || !isFinite(y)) return drag.targetId || "";

    // Hysteresis: keep the existing target while the pointer remains near its
    // original zone. This prevents flicker when FLIP-animated tiles pass under
    // the pointer and briefly become the browser's dragover target.
    if (drag.targetId) {
      for (var i = 0; i < drag.zones.length; i++) {
        if (drag.zones[i].id === drag.targetId && contains(drag.zones[i], x, y, 18)) {
          drag.lastX = x;
          drag.lastY = y;
          return drag.targetId;
        }
      }
    }

    var best = null;
    var bestScore = Infinity;
    for (var j = 0; j < drag.zones.length; j++) {
      var z = drag.zones[j];
      var inside = contains(z, x, y, 0);
      var dx = x - z.c.x;
      var dy = y - z.c.y;
      var score = dx * dx + dy * dy;
      if (inside) score -= 1000000;
      if (score < bestScore) {
        bestScore = score;
        best = z;
      }
    }
    drag.targetId = best ? best.id : "";
    drag.lastX = x;
    drag.lastY = y;
    return drag.targetId;
  };

  window.cashfluxFlipBento = function () {
    restoreDragScroll();
    var token = ++rafToken;
    var nodes = widgetNodes();
    var next = {};
    var reduce = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    var draggingId = drag && drag.sourceId;
    for (var i = 0; i < nodes.length; i++) {
      var el = nodes[i];
      var id = el.getAttribute("data-widget");
      if (!id) continue;
      var r = el.getBoundingClientRect();
      next[id] = { x: r.left, y: r.top };
      if (draggingId && id === draggingId) {
        el.style.transition = "";
        el.style.transform = "";
        continue;
      }
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
                if (token !== rafToken) return;
                node.style.transition = "transform .16s cubic-bezier(.2,.8,.2,1)";
                node.style.transform = "";
              };
            })(el)
          );
        }
      }
    }
    prev = next;
  };

  document.addEventListener(
    "pointerdown",
    function (event) {
      var tile = event.target && event.target.closest && event.target.closest(".bento > .w[data-widget]");
      if (!tile) return;
      if (event.button !== 0) return;
      if (event.target && event.target.closest && event.target.closest("button,a,input,select,textarea")) return;
      var id = tile.getAttribute("data-widget") || "";
      armScrollLock(id);
    },
    true
  );
  document.addEventListener(
    "mousedown",
    function (event) {
      var tile = event.target && event.target.closest && event.target.closest(".bento > .w[data-widget]");
      if (!tile) return;
      armScrollLock(tile.getAttribute("data-widget") || "");
    },
    true
  );
  document.addEventListener(
    "dragstart",
    function (event) {
      var tile = event.target && event.target.closest && event.target.closest(".bento > .w[data-widget]");
      if (!tile) return;
      window.cashfluxBentoDragStart(tile.getAttribute("data-widget") || "");
    },
    true
  );
  document.addEventListener(
    "dragover",
    function (event) {
      restoreDragScroll();
      if (retargetingDrag || !drag) return;
      var stableId = window.cashfluxBentoDragTarget(event.clientX, event.clientY);
      if (!stableId) return;
      var tile = event.target && event.target.closest && event.target.closest(".bento > .w[data-widget]");
      var currentId = tile && tile.getAttribute("data-widget");
      if (!currentId || currentId === stableId) return;
      var stableTile = document.querySelector('.bento > .w[data-widget="' + stableId.replace(/"/g, '\\"') + '"]');
      if (!stableTile) return;
      event.preventDefault();
      event.stopImmediatePropagation();
      retargetingDrag = true;
      try {
        stableTile.dispatchEvent(
          new DragEvent("dragover", {
            bubbles: true,
            cancelable: true,
            clientX: event.clientX,
            clientY: event.clientY,
            dataTransfer: event.dataTransfer || new DataTransfer(),
          })
        );
      } finally {
        retargetingDrag = false;
      }
    },
    true
  );
  document.addEventListener("scroll", restoreDragScroll, true);
  document.addEventListener(
    "drop",
    function (event) {
      if (!retargetingDrag && drag) {
        var stableId = window.cashfluxBentoDragTarget(event.clientX, event.clientY);
        var tile = event.target && event.target.closest && event.target.closest(".bento > .w[data-widget]");
        var currentId = tile && tile.getAttribute("data-widget");
        if (stableId && currentId && stableId !== currentId) {
          var stableTile = document.querySelector('.bento > .w[data-widget="' + stableId.replace(/"/g, '\\"') + '"]');
          if (stableTile) {
            event.preventDefault();
            event.stopImmediatePropagation();
            retargetingDrag = true;
            try {
              stableTile.dispatchEvent(
                new DragEvent("drop", {
                  bubbles: true,
                  cancelable: true,
                  clientX: event.clientX,
                  clientY: event.clientY,
                  dataTransfer: event.dataTransfer || new DataTransfer(),
                })
              );
            } finally {
              retargetingDrag = false;
              window.cashfluxBentoDragEnd();
            }
            return;
          }
        }
      }
      window.cashfluxBentoDragEnd();
    },
    true
  );
  document.addEventListener("dragend", window.cashfluxBentoDragEnd, true);
  document.addEventListener("mouseup", clearScrollLock, true);
  document.addEventListener("pointerup", clearScrollLock, true);
  document.addEventListener("pointercancel", clearScrollLock, true);
})();
