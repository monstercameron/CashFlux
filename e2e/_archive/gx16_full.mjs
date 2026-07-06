/**
 * GX16 full chart audit — navigates via History API, audits each page.
 * Run: node C:\Users\mreca\Desktop\CashFlux\e2e\gx16_full.mjs
 */
import { chromium } from "playwright";
import fs from "fs";
import path from "path";

const BASE = "http://localhost:8080";
const SS_DIR = "C:\\Users\\mreca\\Desktop\\CashFlux\\e2e\\screenshots";
fs.mkdirSync(SS_DIR, { recursive: true });

const ss = async (page, name) => {
  const p = path.join(SS_DIR, name);
  await page.screenshot({ path: p, fullPage: false });
  console.log("  screenshot:", name);
  return name;
};

async function navigate(page, route) {
  await page.evaluate((r) => {
    window.history.pushState({}, "", r);
    window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
  }, route);
  await page.waitForTimeout(800);
}

async function setTheme(page, theme) {
  await page.evaluate((t) => {
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: t }));
    document.documentElement.setAttribute("data-theme", t);
    // Try to trigger any theme-change listeners
    window.dispatchEvent(new CustomEvent("cashflux:themechange", { detail: { theme: t } }));
  }, theme);
  await page.waitForTimeout(300);
}

async function measureCharts(page, pageLabel, theme, width) {
  const label = `${pageLabel}_${theme}_${width}`;
  console.log(`\n--- ${label} ---`);
  const r = {};

  // SVG count
  r.svgCount = await page.evaluate(() => document.querySelectorAll("svg").length);
  console.log(`  SVGs: ${r.svgCount}`);

  // D3 .cf-chart containers
  r.cfCharts = await page.evaluate(() => document.querySelectorAll(".cf-chart").length);
  console.log(`  .cf-chart: ${r.cfCharts}`);

  // Axis tick text — fill attr and computed color
  r.axisTexts = await page.evaluate(() => {
    const els = Array.from(document.querySelectorAll(".x-axis text, .y-axis text")).slice(0, 6);
    return els.map(el => ({
      attrFill: el.getAttribute("fill"),
      computedColor: getComputedStyle(el).color,
      text: el.textContent.trim().slice(0, 30),
    }));
  });
  console.log(`  Axis texts (${r.axisTexts.length}):`);
  r.axisTexts.forEach((t, i) => console.log(`    [${i}] fill="${t.attrFill}" color="${t.computedColor}" text="${t.text}"`));

  // Axis domain/tick lines
  r.axisLines = await page.evaluate(() => {
    const els = Array.from(document.querySelectorAll(".x-axis path.domain, .y-axis path.domain, .x-axis line, .y-axis line")).slice(0, 4);
    return els.map(el => ({
      tag: el.tagName,
      attrStroke: el.getAttribute("stroke"),
      computedStroke: getComputedStyle(el).stroke,
    }));
  });
  console.log(`  Axis lines (${r.axisLines.length}):`);
  r.axisLines.forEach((l, i) => console.log(`    [${i}] <${l.tag}> attr-stroke="${l.attrStroke}" computed="${l.computedStroke}"`));

  // SVG area/line paths
  r.svgPaths = await page.evaluate(() => {
    const els = Array.from(document.querySelectorAll("svg path")).slice(0, 8);
    return els.map(el => ({
      attrFill: el.getAttribute("fill"),
      attrStroke: el.getAttribute("stroke"),
      computedFill: getComputedStyle(el).fill,
      computedStroke: getComputedStyle(el).stroke,
    }));
  });
  console.log(`  SVG paths (${r.svgPaths.length}):`);
  r.svgPaths.forEach((p, i) => console.log(`    [${i}] fill="${p.attrFill}" stroke="${p.attrStroke}"`));

  // Bar tracks
  r.bars = await page.evaluate(() => {
    const els = Array.from(document.querySelectorAll(".bar")).slice(0, 5);
    return els.map(el => ({
      bg: getComputedStyle(el).backgroundColor,
      border: getComputedStyle(el).borderTopColor,
    }));
  });
  console.log(`  .bar tracks (${r.bars.length}):`);
  r.bars.forEach((b, i) => console.log(`    [${i}] bg="${b.bg}" border="${b.border}"`));

  // Bar fills
  r.barFills = await page.evaluate(() => {
    const els = Array.from(document.querySelectorAll(".bar-fill")).slice(0, 5);
    return els.map(el => ({
      bg: getComputedStyle(el).backgroundColor,
    }));
  });
  console.log(`  .bar-fill (${r.barFills.length}):`);
  r.barFills.forEach((b, i) => console.log(`    [${i}] bg="${b.bg}"`));

  // Mermaid SVGs
  r.mermaid = await page.evaluate(() => {
    const svgs = Array.from(document.querySelectorAll("svg"));
    const mermaidSvg = svgs.find(s =>
      s.querySelector(".node, .sankey-node, .label, [class*='mermaid']") ||
      s.id.startsWith("mermaid")
    );
    if (!mermaidSvg) return null;
    const rect = mermaidSvg.querySelector("rect");
    const text = mermaidSvg.querySelector("text");
    return {
      id: mermaidSvg.id,
      bg: getComputedStyle(mermaidSvg).backgroundColor,
      rectAttrFill: rect ? rect.getAttribute("fill") : null,
      rectComputedFill: rect ? getComputedStyle(rect).fill : null,
      textAttrFill: text ? text.getAttribute("fill") : null,
      textComputedColor: text ? getComputedStyle(text).color : null,
      textContent: text ? text.textContent.trim().slice(0, 30) : null,
    };
  });
  if (r.mermaid) {
    console.log(`  Mermaid SVG: id="${r.mermaid.id}" bg="${r.mermaid.bg}" rectFill="${r.mermaid.rectAttrFill}" textFill="${r.mermaid.textAttrFill}" textColor="${r.mermaid.textComputedColor}"`);
  } else {
    console.log(`  Mermaid: none found`);
  }

  // D3 SVG backgrounds (chart container SVGs)
  r.chartSvgBgs = await page.evaluate(() => {
    return Array.from(document.querySelectorAll(".cf-chart svg")).slice(0, 3).map(svg => ({
      computedBg: getComputedStyle(svg).backgroundColor,
      w: svg.getAttribute("width"),
      h: svg.getAttribute("height"),
      firstRectFill: (svg.querySelector("rect") || {}).getAttribute?.("fill") ?? null,
    }));
  });
  console.log(`  D3 chart SVG backgrounds (${r.chartSvgBgs.length}):`);
  r.chartSvgBgs.forEach((s, i) => console.log(`    [${i}] bg="${s.computedBg}" firstRectFill="${s.firstRectFill}" ${s.w}×${s.h}`));

  // CSS custom props read by chart.js
  r.chartJsReads = await page.evaluate(() => {
    function cssVar(name, fb) {
      const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
      return v || fb;
    }
    return {
      fg: cssVar("--text-faint", "#888890"),
      grid: cssVar("--border", "#2a2a2c"),
      defColor: cssVar("--accent", "#2e8b57"),
    };
  });
  console.log(`  chart.js reads: fg="${r.chartJsReads.fg}" grid="${r.chartJsReads.grid}" defColor="${r.chartJsReads.defColor}"`);

  // Planning-specific: x-axis tick text content (L61)
  if (pageLabel === "planning") {
    r.planningXTicks = await page.evaluate(() => {
      return Array.from(document.querySelectorAll(".x-axis .tick text, .x-axis text")).map(el => ({
        text: el.textContent.trim(),
        fill: el.getAttribute("fill"),
      }));
    });
    console.log(`  Planning X-axis ticks (L61 check): ${JSON.stringify(r.planningXTicks)}`);
  }

  return r;
}

(async () => {
  // Set light theme via init script
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await ctx.newPage();

  await page.addInitScript(() => {
    const orig = Storage.prototype.getItem;
    Storage.prototype.getItem = function(k) {
      if (k === "cashflux:prefs") return JSON.stringify({ theme: "light" });
      return orig.call(this, k);
    };
  });

  await page.goto(BASE + "/");
  await page.waitForTimeout(2500);

  const booted = await page.evaluate(() => !!document.querySelector(".topbar, .rail, nav, [class*='nav']"));
  console.log("Wasm booted:", booted);
  const dataTheme = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
  console.log("data-theme:", dataTheme);

  const allResults = {};

  // LIGHT pass
  const lightRoutes = [
    { route: "/dashboard", name: "dashboard" },
    { route: "/reports", name: "reports" },
    { route: "/planning", name: "planning" },
    { route: "/goals", name: "goals" },
    { route: "/budgets", name: "budgets" },
  ];

  for (const { route, name } of lightRoutes) {
    await navigate(page, route);
    await ss(page, `gx16_light_${name}_1280.png`);
    await page.evaluate(() => window.scrollTo(0, 400));
    await page.waitForTimeout(400);
    await ss(page, `gx16_light_${name}_1280_scroll.png`);
    await page.evaluate(() => window.scrollTo(0, 0));

    allResults[`${name}_light`] = await measureCharts(page, name, "light", 1280);
  }

  // DARK spot-check on reports
  await setTheme(page, "dark");
  await navigate(page, "/reports");
  await ss(page, `gx16_dark_reports_1280.png`);
  await page.evaluate(() => window.scrollTo(0, 400));
  await page.waitForTimeout(400);
  await ss(page, `gx16_dark_reports_1280_scroll.png`);
  allResults["reports_dark"] = await measureCharts(page, "reports", "dark", 1280);

  // Back to light for 1440
  await setTheme(page, "light");
  await ctx.close();
  const ctx2 = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page2 = await ctx2.newPage();
  await page2.addInitScript(() => {
    const orig = Storage.prototype.getItem;
    Storage.prototype.getItem = function(k) {
      if (k === "cashflux:prefs") return JSON.stringify({ theme: "light" });
      return orig.call(this, k);
    };
  });
  await page2.goto(BASE + "/");
  await page2.waitForTimeout(2000);
  await navigate(page2, "/reports");
  await ss(page2, `gx16_light_reports_1440.png`);
  await page2.evaluate(() => window.scrollTo(0, 400));
  await page2.waitForTimeout(400);
  await ss(page2, `gx16_light_reports_1440_scroll.png`);
  allResults["reports_light_1440"] = await measureCharts(page2, "reports", "light", 1440);
  await ctx2.close();

  await browser.close();

  // Save full results
  fs.writeFileSync(
    path.join(SS_DIR, "gx16_audit.json"),
    JSON.stringify(allResults, null, 2)
  );

  console.log("\n=== Done ===");
  console.log("Results:", path.join(SS_DIR, "gx16_audit.json"));
  process.exit(0);
})();
