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

  window.cashfluxRenderChart = function (el, specJSON) {
    if (!el || typeof d3 === "undefined") return;
    var spec;
    try {
      spec = JSON.parse(specJSON);
    } catch (e) {
      return;
    }
    el.innerHTML = "";
    var series = spec.series || [];
    if (!series.length) return;

    // Animate the chart in only on its first draw (not on every data tick — el
    // persists across re-renders), and never under reduced motion (§6.16).
    var animate = !el.hasAttribute("data-cf-drawn") &&
      !(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches);
    el.setAttribute("data-cf-drawn", "1");

    var W = el.clientWidth || 320;
    var H = el.clientHeight || 160;
    var fg = cssVar("--text-faint", "#888890");
    var grid = cssVar("--border", "#2a2a2c");
    var defColor = cssVar("--accent", "#2e8b57");

    var svg = d3.select(el).append("svg")
      .attr("width", W).attr("height", H).attr("role", "img");

    if (spec.kind === "donut") {
      renderDonut(svg, series[0], W, H, defColor);
      return;
    }

    var m = { top: 8, right: 10, bottom: 20, left: 44 };
    var iw = Math.max(10, W - m.left - m.right);
    var ih = Math.max(10, H - m.top - m.bottom);
    var g = svg.append("g").attr("transform", "translate(" + m.left + "," + m.top + ")");

    var allPts = [];
    series.forEach(function (s) { (s.points || []).forEach(function (p) { allPts.push(p); }); });
    if (!allPts.length) return;

    var xs = allPts.map(function (p) { return p.x; });
    var ys = allPts.map(function (p) { return p.y; });
    var x = d3.scaleLinear().domain([d3.min(xs), d3.max(xs)]).range([0, iw]);
    // Bars must grow from a 0 baseline; line/area trend charts should fill the
    // plot using the data's own range, else a net-worth line (~$350k on a 0–$400k
    // axis) flattens against the top with dead space below (C51).
    var yMin = (spec.kind === "bar") ? Math.min(0, d3.min(ys)) : d3.min(ys);
    var y = d3.scaleLinear().domain([yMin, d3.max(ys)]).nice().range([ih, 0]);

    function styleAxis(sel) {
      sel.selectAll("text").attr("fill", fg).attr("font-size", "10px");
      sel.selectAll("line,path").attr("stroke", grid);
    }
    // Honor the optional per-axis d3 format hint (chartspec.Axis.Format), e.g.
    // "$.2~s" to render Y ticks as compact currency ("$20k") instead of raw
    // numbers that overflow the narrow margin. Invalid specs fall back silently.
    function tickFormatter(axisSpec) {
      if (axisSpec && axisSpec.format) {
        try { return d3.format(axisSpec.format); } catch (e) { return null; }
      }
      return null;
    }
    var xAxis = d3.axisBottom(x).ticks(4).tickSizeOuter(0);
    var xf = tickFormatter(spec.x);
    if (xf) xAxis.tickFormat(xf);
    var yAxis = d3.axisLeft(y).ticks(4).tickSizeOuter(0);
    var yf = tickFormatter(spec.y);
    if (yf) yAxis.tickFormat(yf);
    g.append("g").attr("transform", "translate(0," + ih + ")").call(xAxis).call(styleAxis);
    g.append("g").call(yAxis).call(styleAxis);

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
          .attr("fill", color);
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
    });
  };

  function renderDonut(svg, s, W, H, defColor) {
    var pts = (s && s.points) || [];
    if (!pts.length) return;
    var r = Math.min(W, H) / 2;
    var g = svg.append("g").attr("transform", "translate(" + (W / 2) + "," + (H / 2) + ")");
    var pie = d3.pie().value(function (p) { return Math.abs(p.y); })(pts);
    var arc = d3.arc().innerRadius(r * 0.6).outerRadius(Math.max(0, r - 2));
    var palette = d3.scaleOrdinal(d3.schemeTableau10 || [defColor]);
    g.selectAll("path").data(pie).enter().append("path")
      .attr("d", arc)
      .attr("fill", function (d, i) { return d.data.color || palette(i); });
  }
})();
