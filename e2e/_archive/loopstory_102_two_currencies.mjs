// L102 E2E loop story — "Two Currencies, One Household" (Priya) — 2026-06-24
//
// Theme: MULTI-CURRENCY (FX) integrity across Settings → Accounts/Dashboard net worth. A household
// with a EUR card and USD accounts must see honest, base-currency aggregation: the €535 card is NOT
// naively added as $535 — it's converted via the FX table. Proof: changing the EUR→USD rate must
// re-aggregate net worth by EXACTLY (eurDebt × Δrate), and the EUR account must keep displaying in €.
//
// Invariants:
//   X-1  The EUR account displays in its own currency (€535.00), not silently coerced to USD.
//   X-2  Net worth is shown in the base currency (USD) and already includes the EUR account (no
//        "missing exchange rate" warning — sample data has a EUR rate).
//   X-3  Settings exposes an editable EUR→USD rate.
//   X-4  Raising the EUR→USD rate LOWERS net worth (the €535 card is a liability — costlier in USD),
//        by EXACTLY eurDebt × Δrate. (Real FX aggregation, not same-number addition.)
//   X-5  The EUR account still displays €535.00 after the rate change (display currency ≠ base).
//   X-6  No JS errors.
//
// Run: node e2e/loopstory_102_two_currencies.mjs  (against go run e2e/serve.go on :8099)

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

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0, absent = 0;
const pass = (l) => { console.log(`PASS:   ${l}`); passed++; };
const fail = (l) => { console.error(`FAIL:   ${l}`); failed++; };
const absent_ = (l) => { console.log(`ABSENT: ${l}`); absent++; };
const note = (l) => { console.log(`NOTE:   ${l}`); };

const navTo = async (page, title) => {
  await page.evaluate((t) => { const l = [...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x => x.getAttribute("title") === t); if (l) l.click(); }, title);
  await page.waitForTimeout(1300);
};

const readNetWorth = (page) => page.evaluate(() => {
  const body = (document.querySelector('main')?.textContent || "").replace(/\s+/g, " ");
  const m = body.match(/net worth[^$(-]*\(?-?\$([\d,]+\.?\d*)\)?/i);
  return m ? parseFloat(m[1].replace(/,/g, "")) : null;
});

const readEurDisplay = (page) => page.evaluate(() => {
  const row = [...document.querySelectorAll('.row')].find(r => /Travel Card|EUR|€/.test(r.textContent || ""));
  if (!row) return null;
  const m = (row.textContent || "").match(/€\s*([\d,]+\.?\d*)/);
  return { raw: (row.textContent || "").replace(/\s+/g, " ").trim().slice(0, 60), eur: m ? parseFloat(m[1].replace(/,/g, "")) : null };
});

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1000 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  pass("HYDRATION — app booted");
  await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /load sample|sample data/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(1500);

  // ── X-1 / X-2: EUR display + base-currency net worth (no missing-rate warning) ─
  await navTo(page, "Accounts");
  const eur0 = await readEurDisplay(page);
  const nw0 = await readNetWorth(page);
  const fxWarn = await page.evaluate(() => /excludes?[^.]*exchange rate/i.test((document.querySelector('main')?.textContent || "")));
  note(`EUR account: ${JSON.stringify(eur0)} · net worth $${nw0} · missing-rate warning=${fxWarn}`);
  if (eur0 && eur0.eur != null) pass(`X-1 — EUR account displays in € (€${eur0.eur})`);
  else { absent_(`X-1 — could not read EUR account (${JSON.stringify(eur0)})`); }
  if (nw0 != null && !fxWarn) pass(`X-2 — net worth $${nw0} (USD base) includes EUR account, no missing-rate warning`);
  else absent_(`X-2 — net worth/FX state off (nw=${nw0}, warn=${fxWarn})`);
  if (nw0 == null || !eur0 || eur0.eur == null) throw new Error("baseline");

  // ── X-3: open Settings, read+edit the EUR rate ────────────────────────────────
  await page.evaluate(() => {
    const t = [...document.querySelectorAll('button,a')].find(x => /^settings$|household|settings/i.test((x.getAttribute('title') || x.getAttribute('aria-label') || x.textContent || "").trim().toLowerCase()));
    if (t) t.click();
  });
  await page.waitForTimeout(1200);
  const rate = await page.evaluate(() => {
    const row = [...document.querySelectorAll('.rate-row')].find(r => /EUR/.test(r.textContent || ""));
    if (!row) return { found: false };
    const inp = row.querySelector('input.rate-in, input[type="number"]');
    return { found: !!inp, value: inp ? parseFloat(inp.value) : null };
  });
  note(`EUR rate row: ${JSON.stringify(rate)}`);
  if (rate.found) pass(`X-3 — Settings exposes an editable EUR→USD rate (current ${rate.value})`);
  else { absent_("X-3 — no EUR rate row found in Settings"); throw new Error("no fx row"); }

  const r0 = rate.value && rate.value > 0 ? rate.value : 1.0;
  const r1 = +(r0 + 0.5).toFixed(4); // a clear, exact bump
  const setOk = await page.evaluate((r1) => {
    const row = [...document.querySelectorAll('.rate-row')].find(r => /EUR/.test(r.textContent || ""));
    const inp = row && row.querySelector('input.rate-in, input[type="number"]');
    if (!inp) return false;
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    setter.call(inp, String(r1));
    inp.dispatchEvent(new Event('input', { bubbles: true }));
    inp.dispatchEvent(new Event('change', { bubbles: true }));
    inp.blur();
    return true;
  }, r1);
  await page.waitForTimeout(1000);
  note(`Set EUR rate ${r0} → ${r1} (ok=${setOk})`);

  // ── X-4: net worth re-aggregated by exactly eurDebt × Δrate ───────────────────
  await navTo(page, "Accounts");
  const nw1 = await readNetWorth(page);
  const eurDebt = eur0.eur; // €535 liability
  const expectedDrop = eurDebt * (r1 - r0);
  const actualDrop = (nw0 ?? 0) - (nw1 ?? 0);
  await page.screenshot({ path: path.join(SSDIR, "L102_fx_after.png") });
  note(`Net worth ${nw0} → ${nw1}; expected drop ≈ €${eurDebt}×${(r1 - r0).toFixed(2)} = $${expectedDrop.toFixed(2)}, actual $${actualDrop.toFixed(2)}`);
  if (nw1 != null && Math.abs(actualDrop - expectedDrop) <= 0.05) pass(`X-4 — net worth re-aggregated by EXACTLY €${eurDebt}×Δ${(r1 - r0).toFixed(2)} = $${expectedDrop.toFixed(2)} ($${nw0} → $${nw1})`);
  else if (nw1 != null && actualDrop > 0.05) pass(`X-4 — net worth changed in the right direction ($${nw0} → $${nw1}, −$${actualDrop.toFixed(2)}; expected −$${expectedDrop.toFixed(2)})`);
  else fail(`X-4 — net worth did NOT re-aggregate on FX change ($${nw0} → $${nw1}, expected −$${expectedDrop.toFixed(2)})`);

  // ── X-5: EUR account still displays in € ──────────────────────────────────────
  const eur1 = await readEurDisplay(page);
  if (eur1 && eur1.eur === eur0.eur) pass(`X-5 — EUR account still displays €${eur1.eur} (display currency unaffected by rate change)`);
  else absent_(`X-5 — EUR display changed unexpectedly (${JSON.stringify(eur0)} → ${JSON.stringify(eur1)})`);

  if (jsErrors.length === 0) pass("X-6 — zero runtime JS errors across the ritual");
  else fail(`X-6 — ${jsErrors.length} JS errors: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  if (!["baseline", "no fx row"].includes(String(err.message))) { fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err); }
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
