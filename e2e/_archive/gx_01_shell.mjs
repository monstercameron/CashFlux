// GX1 — App shell (top bar + left rail + breadcrumb) visual review.
// Story: "Twenty Trips a Day" — the chrome that frames every page must be
// calm, legible, consistent, and unobtrusive in both themes.
//
// Captures screenshots and measures computed styles at 1280 + 768 in both themes.
// Saves to e2e/screenshots/gx01_*.png  (prefix gx01_ to avoid clashing with G1).
// Exit code 0 — this is an evidence-harvest / audit script, not a pass/fail gate.

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOTS_DIR = path.join(__dirname, "screenshots");
fs.mkdirSync(SHOTS_DIR, { recursive: true });

const WIDTHS = [1280, 768];
const THEMES = ["dark", "light"];

// Boot a fresh page with the requested theme pre-set in localStorage.
async function bootWithTheme(browser, width, theme) {
  const ctx = await browser.newContext({ viewport: { width, height: 900 } });
  const page = await ctx.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });

  // Set both the standalone atom and the prefs blob so the app picks up the theme.
  await page.evaluate((t) => {
    localStorage.setItem("cashflux:theme", JSON.stringify(t));
    try {
      const raw = localStorage.getItem("cashflux:prefs");
      if (raw) { const p = JSON.parse(raw); p.theme = t; localStorage.setItem("cashflux:prefs", JSON.stringify(p)); }
    } catch (_) {}
  }, theme);

  // Hard reload so WASM boots with the theme applied.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  // Wait for the rail nav to appear — confirms shell is rendered.
  await page.waitForSelector('aside.rail, nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(900); // settle WASM + transitions

  return { page, ctx, errors };
}

// Measure a set of CSS properties on a selector — returns plain-object of results.
async function measure(page, selector, props) {
  return page.evaluate(({ sel, props }) => {
    const el = document.querySelector(sel);
    if (!el) return { _missing: true };
    const cs = getComputedStyle(el);
    const out = {};
    for (const p of props) out[p] = cs.getPropertyValue(p).trim();
    return out;
  }, { sel: selector, props });
}

// Save a screenshot and log the filename.
async function shot(page, name) {
  const p = path.join(SHOTS_DIR, name);
  await page.screenshot({ path: p, fullPage: false });
  console.log(`  shot: ${name}`);
  return name;
}

const browser = await chromium.launch({ headless: true });
const report = { screenshots: [], measurements: {} };

try {
  for (const theme of THEMES) {
    for (const width of WIDTHS) {
      const tag = `${width}_${theme}`;
      console.log(`\n=== ${tag} ===`);

      const { page, ctx, errors } = await bootWithTheme(browser, width, theme);

      // ── BASELINE: topbar + rail at rest (Dashboard) ────────────────────────
      await shot(page, `gx01_shell_${tag}.png`);
      report.screenshots.push(`gx01_shell_${tag}.png`);

      // ── MEASUREMENTS: topbar ───────────────────────────────────────────────
      const topbarM = await measure(page, ".topbar", [
        "background-color", "border-bottom", "padding-top", "padding-bottom",
        "padding-left", "padding-right", "height", "min-height", "gap",
      ]);
      console.log("  topbar:", JSON.stringify(topbarM));
      report.measurements[`topbar_${tag}`] = topbarM;

      // ── MEASUREMENTS: muzak/notify/add buttons ─────────────────────────────
      const addBtnM = await measure(page, ".add-btn", [
        "width", "height", "background-color", "color", "border", "border-radius",
      ]);
      console.log("  .add-btn:", JSON.stringify(addBtnM));
      report.measurements[`add_btn_${tag}`] = addBtnM;

      const muzakM = await measure(page, ".muzak-btn", [
        "width", "height", "background-color", "color",
      ]);
      console.log("  .muzak-btn:", JSON.stringify(muzakM));
      report.measurements[`muzak_btn_${tag}`] = muzakM;

      const notifyM = await measure(page, ".notify-btn", [
        "width", "height", "background-color", "color",
      ]);
      console.log("  .notify-btn:", JSON.stringify(notifyM));
      report.measurements[`notify_btn_${tag}`] = notifyM;

      // ── MEASUREMENTS: rail ─────────────────────────────────────────────────
      const railM = await measure(page, "aside.rail", [
        "width", "background-color", "border-right",
      ]);
      console.log("  rail:", JSON.stringify(railM));
      report.measurements[`rail_${tag}`] = railM;

      // ── MEASUREMENTS: active nav item ──────────────────────────────────────
      const activeNavM = await measure(page, "aside.rail .nv.active, aside.rail a.active, aside.rail [aria-current]", [
        "background-color", "color", "border-radius",
      ]);
      console.log("  active nav:", JSON.stringify(activeNavM));
      report.measurements[`active_nav_${tag}`] = activeNavM;

      // ── MEASUREMENTS: household card ───────────────────────────────────────
      const hhM = await measure(page, ".hh", [
        "background-color", "color", "padding-top", "padding-bottom", "border-top",
      ]);
      console.log("  household card (.hh):", JSON.stringify(hhM));
      report.measurements[`hh_card_${tag}`] = hhM;

      // ── A11Y: aria-current on active nav ───────────────────────────────────
      const ariaCurrentEl = await page.evaluate(() => {
        const el = document.querySelector('[aria-current="page"]');
        return el ? { tag: el.tagName, text: el.innerText?.trim()?.slice(0, 40) } : null;
      });
      console.log("  aria-current:", JSON.stringify(ariaCurrentEl));
      report.measurements[`aria_current_${tag}`] = ariaCurrentEl;

      // ── A11Y: focus ring check on first nav item ───────────────────────────
      const focusRingM = await page.evaluate(() => {
        const el = document.querySelector("aside.rail .nv, aside.rail a");
        if (!el) return null;
        el.focus();
        const cs = getComputedStyle(el);
        return {
          outline: cs.getPropertyValue("outline").trim(),
          outlineOffset: cs.getPropertyValue("outline-offset").trim(),
        };
      });
      console.log("  focus ring:", JSON.stringify(focusRingM));
      report.measurements[`focus_ring_${tag}`] = focusRingM;

      // ── BREADCRUMB ─────────────────────────────────────────────────────────
      const breadcrumbM = await measure(page, ".breadcrumb, [aria-label='breadcrumb'], nav.breadcrumb", [
        "color", "font-size", "background-color",
      ]);
      console.log("  breadcrumb:", JSON.stringify(breadcrumbM));
      report.measurements[`breadcrumb_${tag}`] = breadcrumbM;

      // ── +ADD MENU OPEN ─────────────────────────────────────────────────────
      try {
        await page.click(".add-btn", { timeout: 5000 });
        await page.waitForTimeout(300);
        await shot(page, `gx01_add_menu_${tag}.png`);
        report.screenshots.push(`gx01_add_menu_${tag}.png`);

        const addMenuM = await measure(page, ".add-menu", [
          "background-color", "border", "border-radius", "box-shadow", "z-index",
        ]);
        console.log("  .add-menu:", JSON.stringify(addMenuM));
        report.measurements[`add_menu_${tag}`] = addMenuM;

        // Close by pressing Escape.
        await page.keyboard.press("Escape");
        await page.waitForTimeout(200);
      } catch (e) {
        console.warn("  +Add click failed:", e.message);
      }

      // ── NOTICES PANEL ──────────────────────────────────────────────────────
      try {
        await page.click(".notify-btn", { timeout: 5000 });
        await page.waitForTimeout(300);
        await shot(page, `gx01_notices_${tag}.png`);
        report.screenshots.push(`gx01_notices_${tag}.png`);
        await page.keyboard.press("Escape");
        await page.waitForTimeout(200);
      } catch (e) {
        console.warn("  notify-btn click failed:", e.message);
      }

      // ── COLLAPSED RAIL ─────────────────────────────────────────────────────
      // Only on 1280 (at 768 the rail is auto-collapsed by media query).
      if (width === 1280) {
        try {
          // Click the collapse toggle (railhead button / chevron).
          await page.click("aside.rail button.collapse-toggle, aside.rail .railhead button, aside.rail button[title], aside.rail button:first-of-type", { timeout: 5000 });
          await page.waitForTimeout(350);
          await shot(page, `gx01_rail_collapsed_${tag}.png`);
          report.screenshots.push(`gx01_rail_collapsed_${tag}.png`);

          const collRailM = await measure(page, "aside.rail.collapsed", [
            "width", "background-color",
          ]);
          console.log("  collapsed rail:", JSON.stringify(collRailM));
          report.measurements[`rail_collapsed_${tag}`] = collRailM;

          // Re-expand.
          await page.click("aside.rail button.collapse-toggle, aside.rail .railhead button, aside.rail button[title], aside.rail button:first-of-type", { timeout: 5000 });
          await page.waitForTimeout(350);
        } catch (e) {
          console.warn("  collapse toggle failed:", e.message);
          // Try alt approach — look for any button in railhead.
          try {
            const toggled = await page.evaluate(() => {
              const rail = document.querySelector("aside.rail");
              const btn = rail?.querySelector("button");
              if (btn) { btn.click(); return true; }
              return false;
            });
            if (toggled) {
              await page.waitForTimeout(350);
              await shot(page, `gx01_rail_collapsed_${tag}.png`);
              report.screenshots.push(`gx01_rail_collapsed_${tag}.png`);
              // Re-expand.
              await page.evaluate(() => {
                const btn = document.querySelector("aside.rail button");
                if (btn) btn.click();
              });
              await page.waitForTimeout(350);
            }
          } catch (e2) {
            console.warn("  collapse fallback also failed:", e2.message);
          }
        }
      }

      // ── CHECK: topbar bg is not transparent (shell must be opaque) ─────────
      const topbarBg = topbarM["background-color"] || "";
      const topbarOk = topbarBg.startsWith("rgb") && !topbarBg.includes("rgba(0, 0, 0, 0)");
      console.log(`  topbar bg opaque: ${topbarOk} (${topbarBg})`);

      // ── CHECK: rail bg matches theme expectation ───────────────────────────
      const railBg = railM["background-color"] || "";
      console.log(`  rail bg: ${railBg} (theme=${theme})`);

      // ── ICON SIZES: all topbar icon buttons should be 30×30 ───────────────
      const iconSizes = await page.evaluate(() => {
        const sels = [".muzak-btn", ".notify-btn", ".add-btn"];
        return sels.map(s => {
          const el = document.querySelector(s);
          if (!el) return { sel: s, missing: true };
          const r = el.getBoundingClientRect();
          return { sel: s, w: Math.round(r.width), h: Math.round(r.height) };
        });
      });
      console.log("  icon sizes:", JSON.stringify(iconSizes));
      report.measurements[`icon_sizes_${tag}`] = iconSizes;

      // ── TOPBAR HEIGHT ──────────────────────────────────────────────────────
      const topbarRect = await page.evaluate(() => {
        const el = document.querySelector(".topbar");
        if (!el) return null;
        const r = el.getBoundingClientRect();
        return { h: Math.round(r.height) };
      });
      console.log("  topbar height:", JSON.stringify(topbarRect));
      report.measurements[`topbar_rect_${tag}`] = topbarRect;

      // ── RAIL WIDTH ─────────────────────────────────────────────────────────
      const railRect = await page.evaluate(() => {
        const el = document.querySelector("aside.rail");
        if (!el) return null;
        const r = el.getBoundingClientRect();
        return { w: Math.round(r.width) };
      });
      console.log("  rail width:", JSON.stringify(railRect));
      report.measurements[`rail_rect_${tag}`] = railRect;

      // ── PAGE ERRORS ────────────────────────────────────────────────────────
      if (errors.length) console.warn(`  page errors: ${errors.join(" | ")}`);
      report.measurements[`page_errors_${tag}`] = errors;

      await ctx.close();
    }
  }
} finally {
  await browser.close();
}

// Write JSON summary for the ticket author.
const summaryPath = path.join(__dirname, "screenshots", "gx01_measurements.json");
fs.writeFileSync(summaryPath, JSON.stringify(report, null, 2));
console.log(`\nMeasurements written to ${path.basename(summaryPath)}`);
console.log("Screenshots:", report.screenshots.join(", "));
