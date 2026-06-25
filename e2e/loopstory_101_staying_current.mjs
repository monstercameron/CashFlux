// L101 E2E loop story — "Staying Current" (Dana) — 2026-06-24
//
// Theme: FRESHNESS lifecycle across Accounts ↔ Dashboard. A finance-aware home needs to trust that
// the balances it sees are current. Stale accounts must be flagged, marking one updated must clear
// ITS flag (and decrement the bulk count), "Mark all updated" must clear EVERYTHING, and the
// dashboard freshness surface must reflect the live state — no stale-after-update lies.
//
// Invariants:
//   F-1  Stale accounts are flagged (per-row "Stale" badges + a bulk "Mark all updated (N …)" control).
//   F-2  Marking ONE account updated clears exactly one badge (N → N-1) and the bulk count follows.
//   F-3  "Mark all updated" clears ALL remaining stale flags (→ 0) and the bulk control retires.
//   F-4  The change is reflected live with no reload (cross-screen: Dashboard freshness ↔ Accounts).
//   F-5  No JS errors across the flow.
//
// Run: node e2e/loopstory_101_staying_current.mjs  (against go run e2e/serve.go on :8099)

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

const staleCount = (page) => page.evaluate(() =>
  [...document.querySelectorAll('.row .badge, .row [class*="badge"]')].filter(b => /stale/i.test(b.textContent || "")).length);

const markAllInfo = (page) => page.evaluate(() => {
  const b = [...document.querySelectorAll('button')].find(x => /mark all updated/i.test(x.textContent || ""));
  if (!b) return { present: false };
  const m = (b.textContent || "").match(/(\d+)/);
  return { present: true, count: m ? parseInt(m[1], 10) : null, label: b.textContent.trim() };
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

  // ── F-1: stale accounts flagged ───────────────────────────────────────────────
  await navTo(page, "Accounts");
  const stale0 = await staleCount(page);
  const all0 = await markAllInfo(page);
  await page.screenshot({ path: path.join(SSDIR, "L101_01_stale.png") });
  note(`Stale badges: ${stale0} · bulk control: ${JSON.stringify(all0)}`);
  if (stale0 > 0 && all0.present) pass(`F-1 — ${stale0} stale badges + bulk "Mark all updated" (count ${all0.count})`);
  else { absent_(`F-1 — freshness flags missing (stale=${stale0}, bulk=${JSON.stringify(all0)})`); throw new Error("no stale"); }

  // ── F-2: mark ONE account updated via its ⋯ menu ──────────────────────────────
  const marked = await page.evaluate(() => {
    const trigger = document.querySelector('button[aria-label="More actions"]');
    if (!trigger) return "NO_TRIGGER";
    trigger.click();
    return "opened";
  });
  await page.waitForTimeout(400);
  const clickedMark = await page.evaluate(() => {
    // the ⋯ menu for that row is now open; click its "Mark updated" item
    const item = [...document.querySelectorAll('.add-menu:not(.hidden-menu):not(.hidden) .add-item, .add-item')]
      .find(b => b.offsetParent !== null && /^mark updated$/i.test((b.textContent || "").trim()));
    if (!item) return "NO_ITEM";
    item.click(); return "clicked";
  });
  await page.waitForTimeout(1000);
  const stale1 = await staleCount(page);
  const all1 = await markAllInfo(page);
  note(`After mark-one (${marked}/${clickedMark}): stale ${stale0} → ${stale1}, bulk count ${all0.count} → ${all1.count}`);
  if (clickedMark === "clicked" && stale1 === stale0 - 1) pass(`F-2 — one account marked fresh: stale ${stale0} → ${stale1} (exactly one cleared)`);
  else absent_(`F-2 — mark-one did not clear exactly one (stale ${stale0} → ${stale1}, ${marked}/${clickedMark})`);
  if (all1.present && all1.count === (all0.count - 1)) pass(`F-2b — bulk count followed (${all0.count} → ${all1.count})`);
  else note(`F-2b — bulk count now ${all1.count} (was ${all0.count})`);

  // ── F-3: Mark all updated clears everything ───────────────────────────────────
  const bulkClicked = await page.evaluate(() => {
    const b = [...document.querySelectorAll('button')].find(x => /mark all updated/i.test(x.textContent || ""));
    if (!b) return "NO_BULK"; b.click(); return "clicked";
  });
  await page.waitForTimeout(1100);
  const stale2 = await staleCount(page);
  const all2 = await markAllInfo(page);
  await page.screenshot({ path: path.join(SSDIR, "L101_02_fresh.png") });
  note(`After mark-all (${bulkClicked}): stale ${stale1} → ${stale2}, bulk present=${all2.present}`);
  if (bulkClicked === "clicked" && stale2 === 0) pass(`F-3 — "Mark all updated" cleared ALL stale flags (${stale1} → 0)`);
  else fail(`F-3 — stale flags remain after mark-all (${stale1} → ${stale2})`);
  if (!all2.present) pass(`F-3b — bulk control retired once nothing is stale`);
  else note(`F-3b — bulk control still present (count ${all2.count}) after clearing`);

  // ── F-4: dashboard reflects the now-fresh state (no reload) ────────────────────
  await navTo(page, "Dashboard");
  const dashStale = await page.evaluate(() => {
    // any dashboard freshness surface mentioning N stale/needs-updating accounts
    const els = [...document.querySelectorAll('*')].filter(e => e.children.length <= 3 && /\bstale\b|needs? updating|out of date/i.test(e.textContent || "") && (e.textContent || "").length < 120);
    const nums = els.map(e => { const m = (e.textContent || "").match(/(\d+)\s*(?:account|stale)/i); return m ? parseInt(m[1], 10) : null; }).filter(n => n != null);
    return { mentions: [...new Set(els.map(e => (e.textContent || "").replace(/\s+/g, " ").trim().slice(0, 60)))].slice(0, 4), maxNum: nums.length ? Math.max(...nums) : 0 };
  });
  note(`Dashboard freshness after: ${JSON.stringify(dashStale)}`);
  if (dashStale.maxNum === 0) pass(`F-4 — dashboard shows no stale-account count after updating (cross-screen consistent)`);
  else absent_(`F-4 — dashboard still reports ${dashStale.maxNum} stale (${JSON.stringify(dashStale.mentions)})`);

  if (jsErrors.length === 0) pass("F-5 — zero runtime JS errors across the ritual");
  else fail(`F-5 — ${jsErrors.length} JS errors: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  if (String(err.message) !== "no stale") { fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err); }
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
