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

  // Match the diagram to the app theme (C69/C70). Mermaid's stock "dark" theme
  // DARKENS the whole palette, so the money-flow Sankey's flow bands rendered
  // near-black on a dark card (v1.0 QA finding). Instead we keep the vivid
  // multi-colour "default" palette in BOTH themes and, in dark mode, override
  // only the label/value text to a light colour via themeCSS — so the bands stay
  // legible and the numbers read on the dark surface. Re-initialise when the
  // shell theme (or the Sankey value prefix) changes.
  function shellIsDark() {
    return document.documentElement.getAttribute("data-theme") !== "light";
  }

  var lastPrefix = null;

  function ensureInit(valuePrefix) {
    if (!window.mermaid) return false;
    var dark = shellIsDark();
    var themeKey = dark ? "dark" : "light";
    var prefix = valuePrefix || "";
    // The Go side passes the base-currency symbol ("$"/"€"/"£") so the money-flow
    // Sankey reads "Income $4068" rather than a bare number; per-render config.
    if (themeKey !== lastTheme || prefix !== lastPrefix) {
      var cfg = {
        startOnLoad: false,
        securityLevel: "strict",
        theme: "default", // vivid multi-colour bands in both themes
        flowchart: { useMaxWidth: true, htmlLabels: false },
        sankey: { useMaxWidth: true, prefix: prefix, showValues: true },
      };
      if (dark) {
        // Light labels/values + transparent canvas so the "default" palette's
        // dark text doesn't vanish on the dark card.
        cfg.themeCSS =
          "svg{background:transparent!important}" +
          "text,.sankey-node text,.node-label,.messageText,tspan{fill:#e6e6e6!important}";
      }
      window.mermaid.initialize(cfg);
      lastTheme = themeKey;
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
