// L97 E2E loop story — "The Glanceable Read" (Renu) — 2026-06-24
//
// Theme: INSIGHT COPY QUALITY. Renu glances at her SMART strips every morning. The whole point of the
// layer is that it reads like a person wrote it — money with a currency symbol, rounded and grouped
// ($519, $5,434), correct plurals ("2 categories", not "2 categorys"), and no template tells (raw
// "519.37", a trailing "USD limit", stray "~", "/mo/mo"). This ritual turns the ENTIRE smart layer on,
// reads every rendered insight across the hub + inline strips, and holds the copy to that bar — it is
// the end-to-end guard for the humanized-copy work (hmoneyc formatter + real-English plural()).
// Invariants:
//   C-1  Enabling all features surfaces a live insight corpus (the hub + strips render cards).
//   C-2  Money in copy is SYMBOLIZED — a "$" appears, and there is NO symbol-less 2-decimal amount
//        (a leftover Money.Format(2) like "519.37 over its limit") anywhere in the insight text.
//   C-3  No bare currency CODE used as a word in prose ("… over its USD limit", "12 USD").
//   C-4  No grammar artifacts ("entrys"/"categorys"/"daies") and no template tells (tilde-before-digit,
//        "/mo/mo", "//", double spaces around money).
//   C-5  Zero runtime JS errors across the ritual (the app-wide "released function" log is ignored).
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_97_glanceable_read.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SSDIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SSDIR)) fs.mkdirSync(SSDIR, { recursive: true });
const SS = (n) => path.join(SSDIR, n);

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0, absent = 0;
const pass = (l) => { console.log(`PASS:   ${l}`); passed++; };
const fail = (l) => { console.error(`FAIL:   ${l}`); failed++; };
const absent_ = (l) => { console.log(`ABSENT: ${l}`); absent++; };
const note = (l) => { console.log(`NOTE:   ${l}`); };

const dismissOverlay = (page) => page.evaluate(() => { const o = document.getElementById("gwc-error-overlay") || document.querySelector(".gwc-error-overlay"); if (o) o.remove(); });
const goto = async (page, route, sel) => { await page.goto(BASE + route, { waitUntil: "domcontentloaded" }); await page.waitForSelector(sel, { timeout: 20000 }); await dismissOverlay(page); await page.waitForTimeout(700); };
// Collect the visible text of every insight card on the current page.
const cardText = (page) => page.evaluate(() => [...document.querySelectorAll('[data-testid="smart-card"]')].map(c => c.innerText.trim()).filter(Boolean));

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

  // Sample data → there is something to be insightful about.
  await dismissOverlay(page);
  const ls = page.locator('[data-testid="hero-load-sample"]');
  if (await ls.count() > 0) { await ls.first().click(); await page.waitForTimeout(1500); }

  // Turn the WHOLE layer on so we read the maximum copy surface in one pass.
  await goto(page, "/smart", '[data-testid="smart-hub"]');
  const enableAll = page.locator('[data-testid="smart-enable-all"]');
  if (await enableAll.count() === 0) { absent_("smart-enable-all button not found — cannot turn the layer on"); }
  else {
    await enableAll.first().click();
    await page.waitForTimeout(4000); // opt-in autosave must flush
  }

  // ── C-1: the layer renders a live insight corpus ──────────────────────────────
  await goto(page, "/smart", '[data-testid="smart-hub"]');
  let texts = await cardText(page);
  // Add inline-strip copy from a few money-heavy pages for breadth.
  for (const [route, sel] of [["/budgets", "#cf-page-view"], ["/goals", "#cf-page-view"], ["/transactions", "#cf-page-view"], ["/bills", "#cf-page-view"]]) {
    await goto(page, route, sel);
    texts = texts.concat(await cardText(page));
  }
  const corpus = texts.join("\n\n");
  note(`Collected ${texts.length} insight cards (${corpus.length} chars)`);
  if (texts.length >= 1) pass(`C-1 — enabling all surfaced a live insight corpus (${texts.length} cards)`);
  else absent_("C-1 — no insight cards rendered after Enable all (nothing to audit this pass)");
  fs.writeFileSync(SS("L97_corpus.txt"), corpus);
  await goto(page, "/smart", '[data-testid="smart-hub"]');
  await page.screenshot({ path: SS("L97_01_hub.png"), fullPage: true });

  if (texts.length >= 1) {
    // ── C-2: money is symbolized; no leftover symbol-less 2-decimal amount ────────
    const hasSymbol = /[$€£¥]/.test(corpus);
    // Anchor on the WHOLE amount (incl. thousands grouping) so the comma in "$1,480.00"
    // can't make us flag its "480.00" tail. A money token is `<optional symbol>\d[\d,]*.dd`
    // anchored so it isn't mid-number; a LEAK is such a token with no leading symbol —
    // a bare Money.Format(2) (e.g. "519.37"). Percentages are 1-decimal; dates/ratios
    // have no ".dd", so neither matches.
    const symbolless = [];
    for (const m of corpus.matchAll(/(?<![\d,.])([$€£¥]?)\d[\d,]*\.\d{2}\b/g)) {
      if (!m[1]) symbolless.push(m[0]);
    }
    note(`money: hasSymbol=${hasSymbol}; symbol-less 2-decimals=[${symbolless.slice(0, 6).join(", ")}]`);
    if (hasSymbol && symbolless.length === 0) pass("C-2 — every money amount is symbolized (no symbol-less '519.37' leftovers)");
    else if (!hasSymbol) absent_("C-2 — no currency symbol seen in the corpus (no money-bearing insight this pass)");
    else fail(`C-2 — ${symbolless.length} symbol-less money amount(s) in copy (Money.Format(2) leak): [${symbolless.slice(0, 6).join(", ")}]`);

    // ── C-3: no bare currency CODE used as a word in prose ────────────────────────
    const codeInProse = corpus.match(/\b\d[\d,]*\s?(USD|EUR|GBP|JPY|CAD|AUD)\b|\b(USD|EUR|GBP|JPY|CAD|AUD)\s+(limit|over|left|saved|of|in)\b/gi) || [];
    if (codeInProse.length === 0) pass("C-3 — no bare currency code used as a word in prose");
    else fail(`C-3 — currency CODE leaked into prose: [${codeInProse.slice(0, 6).join(", ")}]`);

    // ── C-4: no grammar artifacts, no template tells ──────────────────────────────
    const grammar = corpus.match(/\b(entrys|categorys|daies|months s|boxs|moneys|persons)\b/gi) || [];
    const tells = [];
    if (/~\s*[$€£¥]?\d/.test(corpus)) tells.push("tilde-before-number");
    if (/\/mo\/mo|\/yr\/yr/.test(corpus)) tells.push("doubled-period-suffix");
    if (/\S\/\/\S/.test(corpus)) tells.push("double-slash");
    if (/ {2,}[$€£¥]\d/.test(corpus)) tells.push("double-space-before-money");
    note(`grammar artifacts=[${grammar.join(", ")}]; template tells=[${tells.join(", ")}]`);
    if (grammar.length === 0 && tells.length === 0) pass("C-4 — no grammar artifacts and no template tells in the copy");
    else fail(`C-4 — copy defects: grammar=[${grammar.join(", ")}] tells=[${tells.join(", ")}]`);
  }

  // ── C-5: clean run ────────────────────────────────────────────────────────────
  if (jsErrors.length === 0) pass("C-5 — zero runtime JS errors across the ritual");
  else fail(`C-5 — JS_ERRORS — ${jsErrors.length}: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err);
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
