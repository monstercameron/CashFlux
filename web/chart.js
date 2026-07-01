// cashfluxRenderChart(el, specJSON) renders a chartspec.Spec (JSON) into the
// element `el` using D3 (loaded separately, pinned). The Go ui.Chart component
// calls this from an effect keyed on the spec, so it's safe to call repeatedly:
// each call clears `el` and redraws. It is theme-aware — axis/grid/default colors
// are read from the app's CSS custom properties, so charts match light/dark.
//
// JSON contract (matches internal/chartspec with json tags):
//   { kind: "line|area|bar|donut",
//     series: [ { name, color, points: [ {x, y, label} ] } ],
//     x: {label, format}, y: {label, format}, stacked, legend }
(function () {
  function cssVar(name, fallback) {
    try {
      var v = getComputedStyle(document.documentElement).getPropertyValue(name);
      v = (v || "").trim();
      return v || fallback;
    } catch (e) {
      return fallback;
    }
  }

  window.cashfluxRenderChart = function (el, specJSON, currencySymbol) {
    if (!el || typeof d3 === "undefined") return;
    var spec;
    try {
      spec = JSON.parse(specJSON);
    } catch (e) {
      return;
    }
    el.__cfChartSpecJSON = specJSON;
    // Base-currency symbol for the "money" axis format. Persisted on the element so
    // the ResizeObserver re-render (which calls without the arg) keeps using it.
    if (currencySymbol != null && currencySymbol !== "") el.__cfCurSym = currencySymbol;
    var curSym = el.__cfCurSym || "$";
    if (!el.__cfChartResizeObserver && typeof ResizeObserver !== "undefined") {
      var resizeFrame = 0;
      el.__cfChartResizeObserver = new ResizeObserver(function () {
        if (resizeFrame) cancelAnimationFrame(resizeFrame);
        resizeFrame = requestAnimationFrame(function () {
          resizeFrame = 0;
          if (!el.isConnected) {
            if (el.__cfChartResizeObserver) el.__cfChartResizeObserver.disconnect();
            el.__cfChartResizeObserver = null;
            return;
          }
          if (el.__cfChartSpecJSON) window.cashfluxRenderChart(el, el.__cfChartSpecJSON);
        });
      });
      el.__cfChartResizeObserver.observe(el);
    }
    el.innerHTML = "";
    var series = spec.series || [];
    if (!series.length) return;

    // Animate the chart in only on its first draw (not on every data tick — el
    // persists across re-renders), never under reduced motion, AND never when the
    // WONDER dial is off (--wonder-on: 0 via [data-wonder="off"]). Mirrors
    // countup.js's wonderEnabled() so charts honor the same toggle as every other
    // flourish — "reduced-motion + data-wonder=off → fully static" (§6.16 / WONDER).
    var reduceMotion = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    var wonderRaw = parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--wonder-on").trim());
    var wonderOn = !isNaN(wonderRaw) && wonderRaw > 0;
    var animate = !el.hasAttribute("data-cf-drawn") && !reduceMotion && wonderOn;
    el.setAttribute("data-cf-drawn", "1");

    var W = el.clientWidth || 320;
    var H = el.clientHeight || 160;
    var fg = cssVar("--text-faint", "#888890");
    var grid = cssVar("--border", "#2a2a2c");
    var defColor = cssVar("--accent", "#2e8b57");

    var svg = d3.select(el).append("svg")
      .attr("width", W).attr("height", H).attr("role", "img");

    if (spec.kind === "donut") {
      renderDonut(svg, series[0], W, H, defColor, curSym, animate);
      return;
    }

    var hideX = spec.x && spec.x.format === "hidden";
    var tiny = H < 96;
    var narrow = W < 280;
    var m = tiny
      ? { top: 4, right: 4, bottom: 4, left: 4 }
      : narrow
        ? { top: 6, right: 6, bottom: 8, left: 34 }
        : { top: 8, right: 10, bottom: 20, left: 44 };
    var iw = Math.max(10, W - m.left - m.right);
    var ih = Math.max(10, H - m.top - m.bottom);
    var g = svg.append("g").attr("transform", "translate(" + m.left + "," + m.top + ")");

    var allPts = [];
    series.forEach(function (s) { (s.points || []).forEach(function (p) { allPts.push(p); }); });
    if (!allPts.length) return;

    var xs = allPts.map(function (p) { return p.x; });
    var ys = allPts.map(function (p) { return p.y; });
    var x = d3.scaleLinear().domain([d3.min(xs), d3.max(xs)]).range([0, iw]);
    var labelsByX = {};
    allPts.forEach(function (p) {
      if (p.label) labelsByX[p.x] = p.label;
    });
    // Bars must grow from a 0 baseline; line/area trend charts should fill the
    // plot using the data's own range, else a net-worth line (~$350k on a 0–$400k
    // axis) flattens against the top with dead space below (C51).
    var yMin = (spec.kind === "bar") ? Math.min(0, d3.min(ys)) : d3.min(ys);
    var yMax = d3.max(ys);
    // C239: guard a degenerate domain (all-zero or all-equal data) — without a span,
    // d3.scaleLinear maps every value to the same/NaN position, which produced negative
    // or NaN <rect height> SVG errors on the bar charts. Give it a minimal positive span.
    if (!(yMax > yMin)) { yMax = yMin + 1; }
    var y = d3.scaleLinear().domain([yMin, yMax]).nice().range([ih, 0]);

    function styleAxis(sel) {
      sel.selectAll("text").attr("fill", fg).attr("font-size", "10px");
      sel.selectAll("line,path").attr("stroke", grid);
    }
    // Honor the optional per-axis d3 format hint (chartspec.Axis.Format), e.g.
    // "$.2~s" to render Y ticks as compact currency ("$20k") instead of raw
    // numbers that overflow the narrow margin. Invalid specs fall back silently.
    function tickFormatter(axisSpec) {
      if (axisSpec && axisSpec.format) {
        // "money" = currency-aware compact ticks ("$1.5k") using the base-currency
        // symbol, so chart axes match the rest of the app's money formatting without
        // hardcoding "$" (which would be wrong for a EUR/GBP/JPY base).
        if (axisSpec.format === "money") {
          var f = d3.format("~s");
          return function (d) { return curSym + f(d); };
        }
        try { return d3.format(axisSpec.format); } catch (e) { return null; }
      }
      return null;
    }
    var xAxis = d3.axisBottom(x).ticks(narrow ? 3 : 4).tickSizeOuter(0);
    var xf = tickFormatter(spec.x);
    if (xf) xAxis.tickFormat(xf);
    else if (Object.keys(labelsByX).length) {
      xAxis.tickFormat(function (d) {
        var nearest = Math.round(d);
        return labelsByX[nearest] || "";
      });
    }
    var yAxis = d3.axisLeft(y).ticks(tiny ? 0 : (narrow ? 3 : 4)).tickSizeOuter(0);
    var yf = tickFormatter(spec.y);
    if (yf) yAxis.tickFormat(yf);
    if (!hideX && !tiny && !narrow) {
      var gx = g.append("g").attr("class", "x-axis").attr("transform", "translate(0," + ih + ")").call(xAxis).call(styleAxis);
      // Keep the edge tick labels inside the plot: d3 centers tick text, so a wide
      // first/last label (e.g. "Jun '26 (so far)") overflows the small side margin
      // and gets clipped by the card. Anchor the first label to its start and the
      // last to its end so both stay within bounds.
      var xticks = gx.selectAll(".tick text");
      var xn = xticks.size();
      xticks.each(function (d, i) {
        if (i === 0) d3.select(this).attr("text-anchor", "start");
        else if (i === xn - 1) d3.select(this).attr("text-anchor", "end");
      });
    }
    if (!tiny) {
      g.append("g").attr("class", "y-axis").call(yAxis).call(styleAxis);
    }

    if (spec.kind === "bar") {
      var groupW = iw / Math.max(1, allPts.length / series.length);
      var bw = Math.max(1, (groupW * 0.8) / series.length);
      series.forEach(function (s, si) {
        var color = s.color || defColor;
        var finalY = function (p) { return Math.min(y(p.y), y(0)); };
        var finalH = function (p) { return Math.abs(y(p.y) - y(0)); };
        var bars = g.selectAll(".bar-" + si).data(s.points || []).enter().append("rect")
          .attr("x", function (p) { return x(p.x) - (bw * series.length) / 2 + si * bw; })
          .attr("width", bw)
          // Per-bar color override (p.color) lets a ranked category chart share its
          // sibling donut's palette; empty falls back to the series color.
          .attr("fill", function (p) { return (p && p.color) || color; });
        // Per-bar native tooltip so each bar is identifiable on hover ("Mortgage — $1,480.00").
        // The bars otherwise carry no label/legend, so without this you can't tell which category
        // a bar is or its exact value. curSym + d3 give app-consistent money formatting.
        bars.append("title").text(function (p) {
          var v = curSym + d3.format(",.2f")(Math.abs(p.y));
          var amt = (p.y < 0) ? "(" + v + ")" : v;
          return (p.label ? p.label + " — " : "") + amt;
        });
        if (animate) {
          // Grow each bar up from the baseline on first paint.
          bars.attr("y", y(0)).attr("height", 0)
            .transition().duration(450).ease(d3.easeCubicOut)
            .attr("y", finalY).attr("height", finalH);
        } else {
          bars.attr("y", finalY).attr("height", finalH);
        }
      });
      return;
    }

    // Optional legend (top-right): a colored dot + the series name for each
    // series, so multi-series charts (e.g. baseline vs scenario) are readable.
    if (spec.legend && series.length > 1) {
      var lg = g.append("g").attr("font-size", "10px");
      series.forEach(function (s, si) {
        var color = s.color || defColor;
        var row = lg.append("g").attr("transform", "translate(0," + si * 13 + ")");
        row.append("rect").attr("x", iw - 9).attr("y", 0).attr("width", 8).attr("height", 8).attr("rx", 2).attr("fill", color);
        row.append("text").attr("x", iw - 13).attr("y", 7).attr("text-anchor", "end").attr("fill", fg).text(s.name || ("Series " + (si + 1)));
      });
    }

    // Full-precision, unit-aware value formatter for per-point hover tooltips (the axis tickFormatter
    // is compact — "$1.5k" — which is wrong for an exact hover read). Money → "$1,480.00"; an explicit
    // d3 format (e.g. percent) is honored; otherwise a plain thousands-separated number.
    function valFmt(v) {
      if (spec.y && spec.y.format === "money") return curSym + d3.format(",.2f")(v);
      if (spec.y && spec.y.format) { try { return d3.format(spec.y.format)(v); } catch (e) { } }
      return d3.format(",")(v);
    }
    // line / area
    series.forEach(function (s) {
      var color = s.color || defColor;
      var pts = s.points || [];
      if (spec.kind === "area") {
        var area = d3.area()
          .x(function (p) { return x(p.x); })
          .y0(ih)
          .y1(function (p) { return y(p.y); });
        g.append("path").datum(pts)
          .attr("fill", color).attr("fill-opacity", 0.18).attr("d", area);
      }
      var line = d3.line()
        .x(function (p) { return x(p.x); })
        .y(function (p) { return y(p.y); });
      var path = g.append("path").datum(pts)
        .attr("fill", "none").attr("stroke", color).attr("stroke-width", 1.6).attr("d", line);
      if (animate) {
        // Draw the line on left-to-right on first paint.
        var total = path.node().getTotalLength();
        path.attr("stroke-dasharray", total + " " + total).attr("stroke-dashoffset", total)
          .transition().duration(600).ease(d3.easeCubicInOut).attr("stroke-dashoffset", 0);
      }
      // Invisible per-point hover targets so each datum shows its period + exact value on hover
      // ("Mar: $1,480.00"). transparent (not "none") so the fill still receives the pointer; purely
      // additive — no visible change to the line/area. Multi-series charts get a target per series.
      g.selectAll(null).data(pts).enter().append("circle")
        .attr("cx", function (p) { return x(p.x); })
        .attr("cy", function (p) { return y(p.y); })
        .attr("r", 7).attr("fill", "transparent")
        .append("title").text(function (p) {
          var lbl = labelsByX[Math.round(p.x)] || p.label || "";
          return (lbl ? lbl + ": " : "") + valFmt(p.y);
        });
    });
  };

  // Re-render every live chart when the app theme flips (data-theme on <html>).
  // The D3 charts bake their text `fill`, grid `stroke`, and default colors from the
  // CSS theme tokens AT RENDER TIME, so a chart drawn in one theme otherwise keeps the
  // other theme's colors after a switch (e.g. dark near-white axis/donut text staying on
  // a white card after toggling to light). The CSS pin in index.html only covers the
  // light direction and only text; re-rendering fixes BOTH directions and grid/line too.
  // Theme changes are rare so a one-shot re-render is cheap, and the existing
  // `data-cf-drawn` guard stops the draw-in animation from replaying on a theme switch.
  if (typeof MutationObserver !== "undefined" && !window.__cfChartThemeObserver) {
    window.__cfChartThemeObserver = new MutationObserver(function (muts) {
      for (var i = 0; i < muts.length; i++) {
        if (muts[i].attributeName === "data-theme") {
          var els = document.querySelectorAll(".cf-chart");
          for (var j = 0; j < els.length; j++) {
            var el = els[j];
            if (el.isConnected && el.__cfChartSpecJSON) {
              window.cashfluxRenderChart(el, el.__cfChartSpecJSON);
            }
          }
          break;
        }
      }
    });
    window.__cfChartThemeObserver.observe(document.documentElement, {
      attributes: true, attributeFilter: ["data-theme"],
    });
  }

  window.cashfluxDisposeChart = function (el) {
    if (!el) return;
    if (el.__cfChartResizeObserver) {
      el.__cfChartResizeObserver.disconnect();
      el.__cfChartResizeObserver = null;
    }
    el.__cfChartSpecJSON = "";
    el.innerHTML = "";
  };

  function renderDonut(svg, s, W, H, defColor, curSym, animate) {
    var pts = (s && s.points) || [];
    if (!pts.length) return;
    var sym = curSym || "$";
    var palette = d3.scaleOrdinal(d3.schemeTableau10 || [defColor]);
    var colorOf = function (p, i) { return (p && p.color) || palette(i); };
    var total = 0;
    pts.forEach(function (p) { total += Math.abs(p.y); });

    // Lay the ring on the left and a legend (swatch · label · share%) on the right
    // when there's room, so the slices are actually identifiable — a bare ring of
    // colours is unreadable. On a narrow box, fall back to just the ring (the
    // adjacent named breakdown carries the detail there).
    var legendW = 160;
    var hasRoom = W > 200 + legendW;
    var r = Math.min(hasRoom ? (W - legendW) : W, H) / 2;
    var cx = hasRoom ? (r + 6) : (W / 2);
    var g = svg.append("g").attr("transform", "translate(" + cx + "," + (H / 2) + ")");
    var pie = d3.pie().value(function (p) { return Math.abs(p.y); }).sort(null)(pts);
    var arc = d3.arc().innerRadius(r * 0.6).outerRadius(Math.max(0, r - 2));
    var slices = g.selectAll("path").data(pie).enter().append("path")
      .attr("fill", function (d, i) { return colorOf(d.data, i); });
    // Per-slice native tooltip ("Mortgage — $1,480.00 (18%)") so slices are identifiable on hover —
    // essential on narrow boxes where the legend is dropped (`if(!hasRoom) return` below), leaving an
    // otherwise unlabeled ring. Mirrors the category-bar tooltips for consistency.
    slices.append("title").text(function (d) {
      var p = d.data;
      var v = sym + d3.format(",.2f")(Math.abs(p.y));
      var pct = total > 0 ? Math.round((Math.abs(p.y) / total) * 100) : 0;
      return (p.label ? p.label + " — " : "") + v + " (" + pct + "%)";
    });
    if (animate) {
      // W-18 donut draw-in: each wedge sweeps open from its own start angle on first paint
      // (gated by the same `animate` flag as the bar/line draw-ins — off under reduced-motion
      // and data-wonder=off). Only the arcs animate; the center total + legend render normally.
      slices.attr("d", function (d) { return arc({ startAngle: d.startAngle, endAngle: d.startAngle }); })
        .transition().duration(600).ease(d3.easeCubicOut)
        .attrTween("d", function (d) {
          var i = d3.interpolate({ startAngle: d.startAngle, endAngle: d.startAngle }, { startAngle: d.startAngle, endAngle: d.endAngle });
          return function (t) { return arc(i(t)); };
        });
    } else {
      slices.attr("d", arc);
    }

    var fg = cssVar("--text", "#e6e6e9");
    var dim = cssVar("--text-dim", "#9a9aa2");
    // Center total — the ring's hole is dead space; show the summed value there as
    // a focal figure (compact currency, e.g. "$4.1k"). Caption only when there's room.
    // Exact total with thousands separators ("$4,068") when it fits the hole;
    // fall back to compact ("$1.2M") for big figures so it never overflows the ring.
    var full = sym + d3.format(",.0f")(total);
    var centerText = full.length <= 8 ? full : sym + d3.format(".2~s")(total);
    var valFs = Math.max(11, Math.min(18, r * 0.3));
    g.append("text").attr("text-anchor", "middle").attr("y", r > 46 ? -1 : 4)
      .attr("font-size", valFs + "px").attr("font-weight", "700").attr("fill", fg)
      .text(centerText);
    if (r > 46) {
      g.append("text").attr("text-anchor", "middle").attr("y", 13)
        .attr("font-size", "9px").attr("letter-spacing", "0.04em").attr("fill", dim).text("total");
    }
    if (!hasRoom) return;

    var rowH = Math.min(18, Math.max(12, (H - 8) / pts.length));
    var lx = 2 * r + 18;
    var startY = (H - rowH * pts.length) / 2 + rowH / 2;
    var lg = svg.append("g");
    pts.forEach(function (p, i) {
      var y = startY + i * rowH;
      var pct = total > 0 ? Math.round((Math.abs(p.y) / total) * 100) : 0;
      var label = p.label || "—";
      if (label.length > 16) label = label.slice(0, 15) + "…";
      lg.append("rect").attr("x", lx).attr("y", y - 5).attr("width", 9).attr("height", 9).attr("rx", 2).attr("fill", colorOf(p, i));
      lg.append("text").attr("x", lx + 14).attr("y", y + 3).attr("font-size", "11px").attr("fill", fg).text(label);
      // Right-align the share% within the legend column (not the far SVG edge) so
      // each "label … NN%" pair reads as one compact row, not split across the card.
      lg.append("text").attr("x", lx + legendW - 10).attr("y", y + 3).attr("font-size", "11px").attr("text-anchor", "end").attr("fill", dim).text(pct + "%");
    });
  }
})();
