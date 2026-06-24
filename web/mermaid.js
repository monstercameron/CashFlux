// CashFlux Mermaid shim — mirrors chart.js for the D3 charts. The Go ui.Mermaid
// component hands a managed container element and a Mermaid source string to
// cashfluxRenderMermaid, which renders it to inline <svg>. The mermaid global is
// provided by ./mermaid.min.js (vendored locally, no CDN — C44).
//
// Security (C45): securityLevel 'strict' — no click-to-run JS, no raw-HTML labels;
// startOnLoad off so we control exactly what renders, and on any parse error we
// clear the box rather than inject error markup.
(function () {
  var seq = 0;
  var lastTheme = null;

  // Match the diagram theme to the app theme (C69/C70): a light shell sets
  // data-theme="light" on <html>, where Mermaid's "default" (light) theme reads
  // far better than dark-on-light. Re-initialise when the theme changes so a
  // diagram rendered after a theme switch picks up the new palette.
  function mermaidTheme() {
    var t = document.documentElement.getAttribute("data-theme");
    return t === "light" ? "default" : "dark";
  }

  var lastPrefix = null;

  function ensureInit(valuePrefix) {
    if (!window.mermaid) return false;
    var theme = mermaidTheme();
    var prefix = valuePrefix || "";
    // Re-initialise when EITHER the theme OR the Sankey value prefix changes. The
    // Go side passes the base-currency symbol ("$"/"€"/"£") so the money-flow
    // Sankey reads "Income $4068" rather than a bare number; it's per-render config
    // (not hardcoded) so non-USD households get the correct symbol.
    if (theme !== lastTheme || prefix !== lastPrefix) {
      window.mermaid.initialize({
        startOnLoad: false,
        securityLevel: "strict",
        theme: theme,
        flowchart: { useMaxWidth: true, htmlLabels: false },
        sankey: { useMaxWidth: true, prefix: prefix, showValues: true },
      });
      lastTheme = theme;
      lastPrefix = prefix;
    }
    return true;
  }

  window.cashfluxRenderMermaid = function (el, source, valuePrefix) {
    if (!el || !source || !ensureInit(valuePrefix)) return;
    var id = "cf-mmd-" + seq++;
    try {
      // Mermaid 11 render is async and returns { svg }.
      window.mermaid
        .render(id, source)
        .then(function (res) {
          el.innerHTML = res && res.svg ? res.svg : "";
        })
        .catch(function () {
          el.textContent = ""; // never inject error HTML (strict)
        });
    } catch (e) {
      el.textContent = "";
    }
  };

  window.cashfluxDisposeMermaid = function (el) {
    if (el) el.innerHTML = "";
  };
})();
