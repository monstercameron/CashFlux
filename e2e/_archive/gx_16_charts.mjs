/**
 * GX16 â€” Charts & data-viz light-mode probe
 * Run: node C:\Users\mreca\Desktop\CashFlux\e2e\gx_16_charts.mjs
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
  return p;
};

async function setTheme(page, theme) {
  await page.evaluate((t) => {
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: t }));
  }, theme);
  await page.reload();
  await page.waitForFunction(
    (t) => document.documentElement.getAttribute("data-theme") === t,
    theme,
    { timeout: 8000 }
  );
  await page.waitForTimeout(600);
}

async function measure(page, selector, props) {
  return page.evaluate(
    ({ sel, ps }) => {
      const el = document.querySelector(sel);
      if (!el) return { found: false, selector: sel };
      const cs = getComputedStyle(el);
      const result = { found: true, selector: sel, tag: el.tagName };
      for (const p of ps) result[p] = cs[p];
      // also grab fill/stroke from SVG attribute
      result.attrFill = el.getAttribute("fill") || "";
      result.attrStroke = el.getAttribute("stroke") || "";
      result.textContent = el.textContent.trim().slice(0, 60);
      return result;
    },
    { sel: selector, ps: props }
  );
}

async function measureAll(page, selector, props) {
  return page.evaluate(
    ({ sel, ps }) => {
      const els = Array.from(document.querySelectorAll(sel)).slice(0, 5);
      return els.map((el) => {
        const cs = getComputedStyle(el);
        const r = { found: true, selector: sel, tag: el.tagName };
        for (const p of ps) r[p] = cs[p];
        r.attrFill = el.getAttribute("fill") || "";
        r.attrStroke = el.getAttribute("stroke") || "";
        r.textContent = el.textContent.trim().slice(0, 40);
        return r;
      });
    },
    { sel: selector, ps: props }
  );
}

async function auditCharts(page, pageName, theme) {
  const label = `${pageName}_${theme}`;
  console.log(`\n=== ${label} ===`);

  const results = {};

  // SVG presence
  const svgCount = await page.evaluate(() => document.querySelectorAll("svg").length);
  results.svgCount = svgCount;
  console.log(`  SVG elements: ${svgCount}`);

  // D3 chart containers
  const cfChartCount = await page.evaluate(() => document.querySelectorAll(".cf-chart").length);
  results.cfChartCount = cfChartCount;
  console.log(`  .cf-chart containers: ${cfChartCount}`);

  // area chart (Go sparkline)
  const areaSparkline = await measureAll(page, "svg path[stroke]", ["stroke", "fill", "opacity"]);
  results.areaSparkline = areaSparkline;
  console.log(`  SVG stroked paths: ${areaSparkline.length}`);
  areaSparkline.forEach((r, i) => {
    console.log(`    [${i}] attrStroke=${r.attrStroke} attrFill=${r.attrFill}`);
  });

  // D3 axis text (tick labels)
  const axisTexts = await measureAll(page, ".x-axis text, .y-axis text", ["fill", "color", "fontSize"]);
  results.axisTexts = axisTexts;
  console.log(`  Axis tick texts: ${axisTexts.length}`);
  axisTexts.forEach((r, i) => {
    console.log(`    [${i}] fill attr=${r.attrFill} computed color=${r.color} text="${r.textContent}"`);
  });

  // D3 axis lines
  const axisLines = await measureAll(page, ".x-axis line, .y-axis line, .x-axis path, .y-axis path", ["stroke", "color"]);
  results.axisLines = axisLines;
  console.log(`  Axis lines/paths: ${axisLines.length}`);
  axisLines.forEach((r, i) => {
    console.log(`    [${i}] attrStroke=${r.attrStroke}`);
  });

  // progress bars (share / budget)
  const bars = await measureAll(page, ".bar", ["backgroundColor", "borderColor"]);
  results.bars = bars;
  console.log(`  .bar tracks: ${bars.length}`);
  bars.forEach((r, i) => {
    console.log(`    [${i}] bg=${r.backgroundColor} border=${r.borderColor}`);
  });

  const barFills = await measureAll(page, ".bar-fill", ["backgroundColor"]);
  results.barFills = barFills;
  console.log(`  .bar-fill fills: ${barFills.length}`);
  barFills.forEach((r, i) => {
    console.log(`    [${i}] bg=${r.backgroundColor}`);
  });

  // mermaid SVG
  const mermaidSvg = await page.evaluate(() => {
    const svgs = Array.from(document.querySelectorAll("svg"));
    // mermaid SVGs have specific class or structure
    const m = svgs.find(s => s.querySelector(".node, .sankey-node, .flowchart-label, .label"));
    if (!m) return null;
    const cs = getComputedStyle(m);
    const rect = m.querySelector("rect");
    const text = m.querySelector("text");
    return {
      found: true,
      svgBg: cs.backgroundColor,
      rectFill: rect ? getComputedStyle(rect).fill : null,
      rectAttrFill: rect ? rect.getAttribute("fill") : null,
      textFill: text ? getComputedStyle(text).fill : null,
      textColor: text ? getComputedStyle(text).color : null,
    };
  });
  results.mermaidSvg = mermaidSvg;
  if (mermaidSvg) {
    console.log(`  Mermaid SVG: found=true bg=${mermaidSvg.svgBg} rectFill=${mermaidSvg.rectFill} rectAttrFill=${mermaidSvg.rectAttrFill} textFill=${mermaidSvg.textFill}`);
  } else {
    console.log(`  Mermaid SVG: not found on this page`);
  }

  // donut slices
  const donutSlices = await measureAll(page, "svg .arc path, svg path[d]", ["fill"]);
  results.donutSlices = donutSlices.filter(r => r.attrFill && r.attrFill !== "none" && r.attrFill !== "url(#cf-area)");
  console.log(`  Donut/filled path slices: ${results.donutSlices.length}`);
  results.donutSlices.forEach((r, i) => {
    console.log(`    [${i}] attrFill=${r.attrFill} computed fill=${r.fill}`);
  });

  // chart SVG background rect (if any)
  const chartBg = await page.evaluate(() => {
    const cfCharts = document.querySelectorAll(".cf-chart");
    const results = [];
    cfCharts.forEach(c => {
      const svg = c.querySelector("svg");
      if (!svg) return;
      const cs = getComputedStyle(svg);
      // first rect in SVG (may be explicit bg)
      const rect = svg.querySelector("rect");
      results.push({
        svgBg: cs.backgroundColor,
        svgWidth: svg.getAttribute("width"),
        svgHeight: svg.getAttribute("height"),
        firstRectFill: rect ? rect.getAttribute("fill") : null,
        firstRectComputedFill: rect ? getComputedStyle(rect).fill : null,
      });
    });
    return results;
  });
  results.chartBg = chartBg;
  console.log(`  D3 chart SVG backgrounds:`);
  chartBg.forEach((r, i) => {
    console.log(`    [${i}] svgBg=${r.svgBg} firstRectFill=${r.firstRectFill} firstRectComputedFill=${r.firstRectComputedFill}`);
  });

  return results;
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await ctx.newPage();

  // Navigate and set light theme
  await page.goto(BASE + "/dashboard");
  await page.waitForTimeout(2000);
  await setTheme(page, "light");

  const pages = [
    { path: "/dashboard", name: "dashboard" },
    { path: "/reports", name: "reports" },
    { path: "/planning", name: "planning" },
    { path: "/goals", name: "goals" },
    { path: "/budgets", name: "budgets" },
  ];

  const allResults = {};

  for (const pg of pages) {
    await page.goto(BASE + pg.path);
    await page.waitForTimeout(1500);
    await ss(page, `gx16_light_${pg.name}_1280.png`);

    // scroll down to see charts
    await page.evaluate(() => window.scrollTo(0, 500));
    await page.waitForTimeout(400);
    await ss(page, `gx16_light_${pg.name}_1280_scroll.png`);

    const audit = await auditCharts(page, pg.name, "light");
    allResults[pg.name + "_light"] = audit;
  }

  // Dark spot-check on /reports
  await setTheme(page, "dark");
  await page.goto(BASE + "/reports");
  await page.waitForTimeout(1500);
  await ss(page, `gx16_dark_reports_1280.png`);
  await page.evaluate(() => window.scrollTo(0, 500));
  await page.waitForTimeout(400);
  await ss(page, `gx16_dark_reports_1280_scroll.png`);
  const darkReports = await auditCharts(page, "reports", "dark");
  allResults["reports_dark"] = darkReports;

  // Also re-check planning for L61 (axis labels)
  await setTheme(page, "light");
  await page.goto(BASE + "/planning");
  await page.waitForTimeout(1500);

  // Specifically probe planning chart x-axis tick content
  const planningXTicks = await page.evaluate(() => {
    const ticks = Array.from(document.querySelectorAll(".x-axis .tick text, .x-axis text"));
    return ticks.map(t => ({ text: t.textContent.trim(), fill: t.getAttribute("fill") }));
  });
  console.log("\n=== L61 Planning X-axis ticks ===");
  console.log(JSON.stringify(planningXTicks));
  allResults.planningXTicks = planningXTicks;

  // Check planning Y-axis ticks
  const planningYTicks = await page.evaluate(() => {
    const ticks = Array.from(document.querySelectorAll(".y-axis .tick text, .y-axis text"));
    return ticks.map(t => ({ text: t.textContent.trim(), fill: t.getAttribute("fill") }));
  });
  console.log("Planning Y-axis ticks:", JSON.stringify(planningYTicks));

  // --- CSS var resolution in light ---
  const lightCssVars = await page.evaluate(() => {
    const cs = getComputedStyle(document.documentElement);
    return {
      "--text-faint": cs.getPropertyValue("--text-faint").trim(),
      "--border": cs.getPropertyValue("--border").trim(),
      "--accent": cs.getPropertyValue("--accent").trim(),
      "--text": cs.getPropertyValue("--text").trim(),
      "--text-dim": cs.getPropertyValue("--text-dim").trim(),
      "--bg": cs.getPropertyValue("--bg").trim(),
      "--bg-elev": cs.getPropertyValue("--bg-elev").trim(),
    };
  });
  console.log("\n=== CSS vars in light mode ===");
  console.log(JSON.stringify(lightCssVars, null, 2));
  allResults.lightCssVars = lightCssVars;

  // Scroll to top on reports and screenshot full area
  await page.goto(BASE + "/reports");
  await page.waitForTimeout(1500);
  await ss(page, `gx16_light_reports_hero.png`);

  // Extra wide screenshot
  await ctx.close();
  const ctx2 = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page2 = await ctx2.newPage();
  await page2.goto(BASE + "/reports");
  await page2.waitForTimeout(1000);
  await setTheme(page2, "light");
  await page2.waitForTimeout(1000);
  await ss(page2, `gx16_light_reports_1440.png`);
  await page2.evaluate(() => window.scrollTo(0, 600));
  await page2.waitForTimeout(400);
  await ss(page2, `gx16_light_reports_1440_scroll.png`);

  await ctx2.close();
  await browser.close();

  // Write results JSON
  fs.writeFileSync(
    path.join(SS_DIR, "gx16_audit.json"),
    JSON.stringify(allResults, null, 2)
  );
  console.log("\nDone. Results in", SS_DIR);
})();

