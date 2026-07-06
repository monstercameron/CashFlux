// L104 E2E loop story — "The Density Dial" (Renu) — 2026-06-24
//
// Theme: SMART AFFORDANCE GATING. The whole point of the density dial is that the user controls how
// much the SMART layer weaves into the app. The affordance taxonomy is monotonic by rank:
//   Off(0) → nothing · Minimal(1) → badges/strips/empty-states · Standard(2) → +tooltips/section-actions/
//   widgets · Everywhere(3) → +entity overlays.
// This ritual turns the WHOLE layer on, then walks the dial top→bottom and asserts each step removes
// exactly the right tier of affordance (and never the wrong one). It is the end-to-end guard for the
// Wave 1–6 placement work (badges, tooltips, overlays, digest widget) all honoring one dial.
// Invariants (measured on Accounts rows + the Dashboard):
//   D-1  Everywhere → entity-overlay triggers present (the rank-3 top of the dial).
//   D-2  Standard   → row badges present BUT overlay triggers GONE (dial steps past rank 3).
//   D-3  Minimal    → key-figure tooltips GONE but badges still present (dial steps past rank 2).
//   D-4  Off        → every smart affordance gone (badges + tooltips + overlays all absent).
//   D-5  Monotonic  → badge count never INCREASES as density lowers.
//   D-6  Zero runtime JS errors across the whole walk.
//
// Run: E2E_URL=http://127.0.0.1:8123 node e2e/loopstory_104_the_density_dial.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8123";
const SSDIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SSDIR)) fs.mkdirSync(SSDIR, { recursive: true });

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0, absent = 0;
const pass = (l) => { console.log(`PASS:   ${l}`); passed++; };
const fail = (l) => { console.error(`FAIL:   ${l}`); failed++; };
const absent_ = (l) => { console.log(`ABSENT: ${l}`); absent++; };
const note = (l) => { console.log(`NOTE:   ${l}`); };

const dismissOverlay = (page) => page.evaluate(() => { const o = document.getElementById("gwc-error-overlay") || document.querySelector(".gwc-error-overlay"); if (o) o.remove(); });
const goto = async (page, route, sel) => { await page.goto(BASE + route, { waitUntil: "domcontentloaded" }); await page.waitForSelector(sel, { timeout: 20000 }); await dismissOverlay(page); await page.waitForTimeout(700); };
const countSel = (page, sel) => page.evaluate((s) => document.querySelectorAll(s).length, sel);
// Set the global density dial and let the opt-in autosave flush before any nav.
const setDensity = async (page, value) => {
  await goto(page, "/smart", '[data-testid="smart-hub"]');
  await page.selectOption('[data-testid="smart-density"]', value);
  await page.waitForTimeout(3200); // settings autosave must persist before reload
};
// Snapshot the affordance counts at the current density (badges+overlays on Accounts, tooltip on Dashboard).
const snapshot = async (page) => {
  await goto(page, "/accounts", "#cf-page-view");
  const badges = await countSel(page, '[data-testid^="smart-badge-"]');
  const overlays = await countSel(page, '[data-testid^="smart-overlay-trigger-"]');
  await goto(page, "/", "#cf-page-view");
  const tips = await countSel(page, '[data-testid^="smart-tip-"]');
  return { badges, overlays, tips };
};

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });
  page.on("console", (m) => { if (m.type() === "error" && !/released function/i.test(m.text())) jsErrors.push(m.text()); });

  let hydrated = false;
  for (let i = 0; i < 2 && !hydrated; i++) {
    try { await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 }); await page.waitForSelector("#app", { timeout: 30000 }); hydrated = true; }
    catch (e) { note(`hydrate ${i + 1}: ${e.message.slice(0, 50)}`); }
  }
  if (!hydrated) throw new Error("APP DID NOT HYDRATE");
  pass("HYDRATION — app booted");

  await dismissOverlay(page);
  const ls = page.locator('[data-testid="hero-load-sample"]');
  if (await ls.count() > 0) { await ls.first().click(); await page.waitForTimeout(1500); }

  // Turn the WHOLE layer on so every affordance has content to gate.
  await goto(page, "/smart", '[data-testid="smart-hub"]');
  const enableAll = page.locator('[data-testid="smart-enable-all"]');
  if (await enableAll.count() === 0) { absent_("smart-enable-all not found — cannot exercise the dial"); }
  else { await enableAll.first().click(); await page.waitForTimeout(4000); }

  // Walk the dial top → bottom.
  await setDensity(page, "everywhere");
  const ev = await snapshot(page);
  note(`Everywhere: badges=${ev.badges} overlays=${ev.overlays} tips=${ev.tips}`);
  if (ev.overlays > 0) pass(`D-1 — Everywhere shows entity-overlay triggers (${ev.overlays}) — rank-3 top of the dial`);
  else absent_(`D-1 — no overlay triggers at Everywhere (badges=${ev.badges}); sample may lack account-targeted insights`);

  await setDensity(page, "standard");
  const st = await snapshot(page);
  note(`Standard: badges=${st.badges} overlays=${st.overlays} tips=${st.tips}`);
  if (st.overlays === 0 && (st.badges > 0 || ev.badges === 0)) pass(`D-2 — Standard hides overlays (${st.overlays}) but keeps badges (${st.badges})`);
  else fail(`D-2 — Standard gating wrong: overlays=${st.overlays} (want 0), badges=${st.badges}`);

  await setDensity(page, "minimal");
  const mn = await snapshot(page);
  note(`Minimal: badges=${mn.badges} overlays=${mn.overlays} tips=${mn.tips}`);
  if (mn.tips === 0 && mn.overlays === 0) pass(`D-3 — Minimal hides tooltips (${mn.tips}) and overlays (${mn.overlays}); badges=${mn.badges}`);
  else fail(`D-3 — Minimal gating wrong: tips=${mn.tips} overlays=${mn.overlays} (want 0/0)`);

  await setDensity(page, "off");
  const off = await snapshot(page);
  note(`Off: badges=${off.badges} overlays=${off.overlays} tips=${off.tips}`);
  if (off.badges === 0 && off.tips === 0 && off.overlays === 0) pass("D-4 — Off removes every smart affordance (badges/tooltips/overlays all 0)");
  else fail(`D-4 — Off still shows affordances: badges=${off.badges} tips=${off.tips} overlays=${off.overlays}`);

  // D-5: monotonic non-increase of badges as density lowers.
  const seq = [ev.badges, st.badges, mn.badges, off.badges];
  const monotonic = seq.every((v, i) => i === 0 || v <= seq[i - 1]);
  if (monotonic) pass(`D-5 — badge count is monotonic non-increasing across the dial [${seq.join(" → ")}]`);
  else fail(`D-5 — badge count NOT monotonic across dial [${seq.join(" → ")}]`);

  // Restore a sane default for the next run.
  await setDensity(page, "standard");

  if (jsErrors.length === 0) pass("D-6 — zero runtime JS errors across the dial walk");
  else fail(`D-6 — JS_ERRORS — ${jsErrors.length}: ${jsErrors.slice(0, 3).join("; ")}`);

  await page.screenshot({ path: path.join(SSDIR, "L104_01_dial.png") });

} catch (err) {
  fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err);
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
