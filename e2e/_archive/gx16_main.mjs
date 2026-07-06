/**
 * GX16 — Charts & data-viz light-mode probe
 * Navigates to root (SPA), sets theme, uses hash/history navigation.
 * Run: node C:\Users\mreca\Desktop\CashFlux\e2e\gx16_main.mjs
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

const screenshots = [];

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await ctx.newPage();

  // Set light theme BEFORE loading so inline script sees it
  await page.addInitScript(() => {
    window.__gx16_setTheme = "light";
    const orig = Storage.prototype.getItem;
    Storage.prototype.getItem = function(k) {
      if (k === "cashflux:prefs") return JSON.stringify({ theme: "light" });
      return orig.call(this, k);
    };
  });

  await page.goto(BASE + "/");
  await page.waitForTimeout(2000);

  const dataTheme = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
  console.log("data-theme after load:", dataTheme);

  // Check if wasm booted
  const wasmBooted = await page.evaluate(() => !!document.querySelector(".topbar, .rail, nav"));
  console.log("Wasm booted:", wasmBooted);

  const bodyHTML = await page.evaluate(() => document.body.innerHTML.slice(0, 500));
  console.log("Body HTML[:500]:", bodyHTML);

  // Take screenshots in whatever state we're in
  screenshots.push(await ss(page, "gx16_light_1280_root.png"));

  // If wasm not loaded, that's GI0 blocker — document it
  if (!wasmBooted) {
    console.log("\nGI0 CONFIRMED: Wasm not booting — no .topbar/.rail rendered.");
    console.log("Performing CSS-only analysis from source instead.");
  }

  // Measure CSS vars from the loaded stylesheet (even without wasm)
  const styleSheetStats = await page.evaluate(() => {
    const sheets = Array.from(document.styleSheets);
    const stats = sheets.map(s => {
      let count = 0;
      try { count = s.cssRules.length; } catch {}
      return { href: s.href, count };
    });
    return stats;
  });
  console.log("\nStylesheets:", JSON.stringify(styleSheetStats));

  // Try to get any CSS from the inline <style> tags
  const inlineStyles = await page.evaluate(() => {
    return Array.from(document.querySelectorAll("style")).map(s => s.textContent.length);
  });
  console.log("Inline <style> tag lengths:", inlineStyles);

  // Check <link> stylesheet elements
  const linkEls = await page.evaluate(() => {
    return Array.from(document.querySelectorAll("link[rel=stylesheet]")).map(l => l.href);
  });
  console.log("Linked stylesheets:", linkEls);

  // Try to apply data-theme and measure
  await page.evaluate(() => {
    document.documentElement.setAttribute("data-theme", "light");
  });
  await page.waitForTimeout(200);

  const cssVarsAfterTheme = await page.evaluate(() => {
    const cs = getComputedStyle(document.documentElement);
    const vars = ["--text", "--text-faint", "--border", "--accent", "--bg", "--bg-elev"];
    const r = {};
    for (const v of vars) r[v] = cs.getPropertyValue(v).trim();
    return r;
  });
  console.log("\nCSS vars after setAttribute('data-theme','light'):", JSON.stringify(cssVarsAfterTheme));

  // Inject test divs and check computed styles
  const injectedMeasurements = await page.evaluate(() => {
    const results = {};

    function measure(cls) {
      const d = document.createElement("div");
      d.className = cls;
      document.body.appendChild(d);
      const cs = getComputedStyle(d);
      const r = {
        bg: cs.backgroundColor,
        color: cs.color,
        border: cs.borderTopColor,
      };
      document.body.removeChild(d);
      return r;
    }

    ["bar", "bar-fill", "stat", "card-title", "page-title", "row-desc"].forEach(c => {
      results[c] = measure(c);
    });

    return results;
  });
  console.log("\nInjected element measurements (light data-theme on html):");
  console.log(JSON.stringify(injectedMeasurements, null, 2));

  // Dark theme check
  await page.evaluate(() => {
    document.documentElement.setAttribute("data-theme", "dark");
  });
  await page.waitForTimeout(200);

  const darkMeasurements = await page.evaluate(() => {
    function measure(cls) {
      const d = document.createElement("div");
      d.className = cls;
      document.body.appendChild(d);
      const cs = getComputedStyle(d);
      const r = { bg: cs.backgroundColor, color: cs.color, border: cs.borderTopColor };
      document.body.removeChild(d);
      return r;
    }
    const r = {};
    ["bar", "bar-fill", "stat"].forEach(c => { r[c] = measure(c); });
    return r;
  });
  console.log("\nDark measurements:");
  console.log(JSON.stringify(darkMeasurements, null, 2));

  screenshots.push(await ss(page, "gx16_dark_1280_root.png"));

  await ctx.close();
  await browser.close();

  console.log("\n=== Screenshots ===");
  screenshots.forEach(s => console.log(" ", s));
  console.log("\nExit 0");
  process.exit(0);
})();
