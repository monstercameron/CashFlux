/**
 * GX16 CSS probe — loads index.html, waits for stylesheets, injects data-theme,
 * and measures computed styles for all chart-related classes.
 */
import { chromium } from "playwright";
import fs from "fs";
import path from "path";

const BASE = "http://localhost:8080";
const SS_DIR = "C:\\Users\\mreca\\Desktop\\CashFlux\\e2e\\screenshots";
fs.mkdirSync(SS_DIR, { recursive: true });

const ss = async (page, name) => {
  const p = path.join(SS_DIR, name);
  await page.screenshot({ path: p });
  console.log("  screenshot:", name);
  return name;
};

(async () => {
  const browser = await chromium.launch({ headless: false }); // headed so JS runs fully
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await ctx.newPage();

  // Set light prefs BEFORE navigation so the inline script picks it up
  await page.addInitScript(() => {
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" }));
  });

  await page.goto(BASE + "/dashboard");
  // Wait for the inline theme script to run (it's synchronous in <head>)
  await page.waitForTimeout(1500);

  const dataTheme = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
  console.log("data-theme:", dataTheme);

  // Wait for stylesheets to parse
  await page.waitForFunction(() => {
    const sheets = Array.from(document.styleSheets);
    return sheets.some(s => {
      try { return s.cssRules && s.cssRules.length > 0; } catch { return false; }
    });
  }, { timeout: 10000 }).catch(() => console.log("Warning: stylesheet wait timed out"));

  await page.waitForTimeout(500);

  // Now measure CSS vars
  const lightVars = await page.evaluate(() => {
    const cs = getComputedStyle(document.documentElement);
    const vars = [
      "--text", "--text-dim", "--text-faint", "--border",
      "--accent", "--accent-dim", "--bg", "--bg-elev", "--bg-card",
      "--up", "--down", "--danger", "--warn"
    ];
    const r = {};
    for (const v of vars) r[v] = cs.getPropertyValue(v).trim();
    return r;
  });
  console.log("\n=== CSS vars in light mode (with stylesheets) ===");
  for (const [k, v] of Object.entries(lightVars)) {
    console.log(`  ${k}: "${v}"`);
  }

  // Inject test elements to measure computed chart-related styles
  const measurements = await page.evaluate(() => {
    const theme = document.documentElement.getAttribute("data-theme");

    function inject(cls, extra = "") {
      const d = document.createElement("div");
      d.className = cls;
      if (extra) d.setAttribute("style", extra);
      document.body.appendChild(d);
      const cs = getComputedStyle(d);
      const r = {
        class: cls,
        backgroundColor: cs.backgroundColor,
        color: cs.color,
        borderColor: cs.borderTopColor,
        fill: cs.fill,
      };
      document.body.removeChild(d);
      return r;
    }

    function injectSvg(attrs) {
      const ns = "http://www.w3.org/2000/svg";
      const svg = document.createElementNS(ns, "svg");
      const el = document.createElementNS(ns, attrs.tag);
      for (const [k, v] of Object.entries(attrs.attrs || {})) el.setAttribute(k, v);
      svg.appendChild(el);
      document.body.appendChild(svg);
      const cs = getComputedStyle(el);
      const r = {
        tag: attrs.tag,
        attrFill: el.getAttribute("fill"),
        computedFill: cs.fill,
        computedColor: cs.color,
        computedStroke: cs.stroke,
      };
      document.body.removeChild(svg);
      return r;
    }

    return {
      theme,
      bar: inject("bar"),
      barFill: inject("bar-fill"),
      barFillNear: inject("bar-fill near"),
      barFillOver: inject("bar-fill over"),
      stat: inject("stat"),
      cardTitle: inject("card-title"),
      heroNet: inject("hero-net"),
      heroNetPos: inject("hero-net pos"),
      textFg: inject("text-fg"),
      textDim: inject("text-dim"),
      textFaint: inject("text-faint"),
      // chart SVG element with hardcoded stroke (like AreaChart uses)
      svgPath_hardcoded2e8b57: injectSvg({ tag: "path", attrs: { stroke: "#2e8b57", fill: "none" } }),
      svgText_fill888890: injectSvg({ tag: "text", attrs: { fill: "#888890" } }),
      // D3 chart axes — what cssVar("--text-faint") resolves to
      cssTextFaint: getComputedStyle(document.documentElement).getPropertyValue("--text-faint").trim(),
      cssBorder: getComputedStyle(document.documentElement).getPropertyValue("--border").trim(),
      cssAccent: getComputedStyle(document.documentElement).getPropertyValue("--accent").trim(),
    };
  });

  console.log("\n=== Chart element measurements (light) ===");
  console.log(JSON.stringify(measurements, null, 2));

  // Screenshot the boot splash (wasm won't load but that's OK)
  await ss(page, "gx16_light_shell_1280.png");

  // Dark comparison
  await page.evaluate(() => {
    document.documentElement.setAttribute("data-theme", "dark");
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "dark" }));
  });
  await page.waitForTimeout(300);
  await ss(page, "gx16_dark_shell_1280.png");

  const darkMeasurements = await page.evaluate(() => {
    function inject(cls) {
      const d = document.createElement("div");
      d.className = cls;
      document.body.appendChild(d);
      const cs = getComputedStyle(d);
      const r = { class: cls, backgroundColor: cs.backgroundColor, color: cs.color, borderColor: cs.borderTopColor };
      document.body.removeChild(d);
      return r;
    }
    return {
      theme: document.documentElement.getAttribute("data-theme"),
      bar: inject("bar"),
      barFill: inject("bar-fill"),
      cssTextFaint: getComputedStyle(document.documentElement).getPropertyValue("--text-faint").trim(),
      cssBorder: getComputedStyle(document.documentElement).getPropertyValue("--border").trim(),
    };
  });
  console.log("\n=== Dark comparison ===");
  console.log(JSON.stringify(darkMeasurements, null, 2));

  await ctx.close();
  await browser.close();

  console.log("\nDone.");
  process.exit(0);
})();
