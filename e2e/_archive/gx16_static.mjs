/**
 * GX16 static probe — screenshots the shell in light/dark,
 * measures CSS custom properties, and checks what little the
 * static page (pre-wasm-boot) exposes.
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

async function setThemeViaStorage(page, theme) {
  // Set in localStorage (runs before wasm boot)
  await page.evaluate((t) =>
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: t })),
    theme
  );
  await page.reload();
  // The inline <script> sets data-theme synchronously from localStorage before wasm
  await page.waitForTimeout(1000);
  return page.evaluate(() => document.documentElement.getAttribute("data-theme"));
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await ctx.newPage();

  // ---- DARK (default) ----
  await page.goto(BASE + "/dashboard");
  await page.waitForTimeout(1200);
  const darkTheme = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
  console.log("Default theme:", darkTheme);
  await ss(page, "gx16_dark_shell_1280.png");

  // ---- LIGHT ----
  const lightTheme = await setThemeViaStorage(page, "light");
  console.log("Light theme set:", lightTheme);
  await ss(page, "gx16_light_shell_1280.png");

  // Measure CSS custom properties in light mode
  const lightVars = await page.evaluate(() => {
    const cs = getComputedStyle(document.documentElement);
    const vars = [
      "--text", "--text-dim", "--text-faint", "--border",
      "--accent", "--accent-dim", "--bg", "--bg-elev",
      "--bg-card", "--up", "--down", "--danger", "--warn"
    ];
    const result = {};
    for (const v of vars) {
      result[v] = cs.getPropertyValue(v).trim();
    }
    return result;
  });
  console.log("\n=== CSS vars in light mode ===");
  for (const [k, v] of Object.entries(lightVars)) {
    console.log(`  ${k}: "${v}"`);
  }

  // Measure what the boot splash looks like (that's all we get without wasm)
  const bootEl = await page.evaluate(() => {
    const boot = document.querySelector("#boot");
    if (!boot) return null;
    const cs = getComputedStyle(boot);
    return {
      bg: cs.backgroundColor,
      color: cs.color,
      display: cs.display,
    };
  });
  console.log("\nBoot element:", bootEl);

  // Check data-theme is set before wasm
  const dataTheme = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
  console.log("data-theme on html:", dataTheme);

  // .bar and .bar-fill classes — these exist in CSS so we can check their computed values
  // even without wasm, by injecting test elements
  const barMeasure = await page.evaluate(() => {
    // create a test .bar and .bar-fill in the document to measure computed styles
    const div = document.createElement("div");
    div.className = "bar";
    div.innerHTML = '<div class="bar-fill" style="width:60%"></div>';
    document.body.appendChild(div);
    const barCs = getComputedStyle(div);
    const fillCs = getComputedStyle(div.firstChild);
    const result = {
      barBg: barCs.backgroundColor,
      barBorder: barCs.borderColor,
      fillBg: fillCs.backgroundColor,
    };
    document.body.removeChild(div);
    return result;
  });
  console.log("\n=== .bar/.bar-fill computed (injected) in light ===");
  console.log(JSON.stringify(barMeasure, null, 2));

  // Measure .stat tile
  const statMeasure = await page.evaluate(() => {
    const div = document.createElement("div");
    div.className = "stat";
    document.body.appendChild(div);
    const cs = getComputedStyle(div);
    const r = { bg: cs.backgroundColor, border: cs.borderColor };
    document.body.removeChild(div);
    return r;
  });
  console.log("\n=== .stat computed (injected) in light ===");
  console.log(JSON.stringify(statMeasure, null, 2));

  // Measure chart.js cssVar resolution for --text-faint and --border in light
  // (these are what cashfluxRenderChart uses for axis/grid)
  const chartJsVars = await page.evaluate(() => {
    function cssVar(name, fallback) {
      try {
        var v = getComputedStyle(document.documentElement).getPropertyValue(name);
        v = (v || "").trim();
        return v || fallback;
      } catch (e) { return fallback; }
    }
    return {
      "--text-faint (fg)": cssVar("--text-faint", "#888890"),
      "--border (grid)": cssVar("--border", "#2a2a2c"),
      "--accent (defColor)": cssVar("--accent", "#2e8b57"),
    };
  });
  console.log("\n=== chart.js cssVar reads in light mode ===");
  for (const [k, v] of Object.entries(chartJsVars)) {
    console.log(`  ${k}: "${v}"`);
  }

  // Simulate what chart.js axis style would render
  // fg (--text-faint) = tick text fill
  // grid (--border) = axis line stroke
  console.log("\n=== chart.js would render axis text fill as:", chartJsVars["--text-faint (fg)"]);
  console.log("=== chart.js would render axis line stroke as:", chartJsVars["--border (grid)"]);

  // Screenshot at 1440
  const ctx2 = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page2 = await ctx2.newPage();
  await page2.goto(BASE + "/dashboard");
  await page2.waitForTimeout(500);
  await setThemeViaStorage(page2, "light");
  await ss(page2, "gx16_light_shell_1440.png");
  await ctx2.close();

  await ctx.close();
  await browser.close();

  console.log("\nDone. Screenshots in:", SS_DIR);
  console.log("Exit code: 0");
})();
