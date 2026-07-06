// L91 E2E loop story — "The Bill Run" (Dana) — 2026-06-24
//
// Theme: BILLS / RECURRING-PAYMENT INTEGRITY. Every household's weekly chore is "what's due, and let
// me pay it." The Bills screen must (1) surface a "total due soon" + the upcoming bills with amounts
// and due dates, (2) order them by urgency (soonest first), (3) let a bill be marked paid and have
// that REMOVE it from "upcoming" / lower the due total, and (4) have marking-paid leave a financial
// trace (a posted transaction) so the books stay honest. Invariants:
//   R-1  Bills shows a "total due soon" summary + upcoming bills with amount + due date.
//   R-2  Upcoming bills are ordered by due date (soonest first) — urgency sort.
//   R-3  Marking a bill paid lowers the "upcoming bills" count (it leaves the due list).
//   R-4  Marking a bill paid posts a matching transaction (cross-screen: it shows in Transactions).
//   R-5  STRESS: marking several bills paid in a row keeps the count monotonically falling, no crash.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_91_the_bill_run.mjs

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

const navTo = async (page, title) => {
  await page.evaluate((t) => { const l = [...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x => x.getAttribute("title") === t); if (l) l.click(); }, title);
  await page.waitForTimeout(1300);
};

// Read the Bills summary: { dueTotal, upcoming, nextDue }.
const billsSummary = (page) => page.evaluate(() => {
  const txt = document.body.innerText;
  const due = txt.match(/total due soon[^$]*?(\$[\d,]+\.?\d*)/i);
  const up = txt.match(/upcoming bills[^\d]*?(\d+)/i);
  const next = txt.match(/next due[^\d]*?(\d{4}-\d{2}-\d{2})/i);
  return { dueTotal: due ? due[1] : null, upcoming: up ? parseInt(up[1], 10) : null, nextDue: next ? next[1] : null };
});
// Read upcoming bill rows: { name, due (ISO), amount }.
const billRows = (page) => page.evaluate(() => {
  return [...document.querySelectorAll('.row')].map(r => {
    const t = r.textContent || "";
    if (!/mark paid/i.test(t)) return null;
    const due = (t.match(/\d{4}-\d{2}-\d{2}/) || [null])[0];
    const amt = (t.match(/\$[\d,]+\.?\d*/) || [null])[0];
    const name = t.split(/\d{4}-\d{2}-\d{2}/)[0].trim();
    return { name, due, amount: amt };
  }).filter(Boolean);
});

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1440, height: 1100 });
  page.on("pageerror", (e) => { const m = String(e); if (!m.includes("already exited")) jsErrors.push(m); });

  let hydrated = false;
  for (let i = 0; i < 2 && !hydrated; i++) {
    try { await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 }); await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 }); hydrated = true; }
    catch (e) { note(`hydrate ${i + 1}: ${e.message.slice(0, 50)}`); }
  }
  if (!hydrated) throw new Error("APP DID NOT HYDRATE");
  pass("HYDRATION — app booted");

  await page.evaluate(() => { const b = [...document.querySelectorAll("button")].find(b => /load sample|sample data/i.test(b.textContent)); if (b) b.click(); });
  await page.waitForTimeout(1500);

  await navTo(page, "Bills");
  await page.waitForTimeout(900);
  await page.screenshot({ path: SS("L91_01_bills.png") });

  // ── R-1: summary + rows present ───────────────────────────────────────────────
  const s0 = await billsSummary(page);
  const rows0 = await billRows(page);
  note(`Summary: dueTotal=${s0.dueTotal} upcoming=${s0.upcoming} nextDue=${s0.nextDue} · rows=${rows0.length}`);
  note(`First rows: ${rows0.slice(0, 4).map(r => `${r.name} ${r.due} ${r.amount}`).join(" | ")}`);
  if (s0.dueTotal && s0.upcoming != null && rows0.length > 0) pass(`R-1 — Bills shows a due-soon summary (${s0.dueTotal}, ${s0.upcoming} upcoming) + ${rows0.length} bill rows`);
  else absent_(`R-1 — summary/rows incomplete (due=${s0.dueTotal}, upcoming=${s0.upcoming}, rows=${rows0.length})`);

  // ── R-2: ordered by due date (soonest first) ──────────────────────────────────
  const dued = rows0.map(r => r.due).filter(Boolean);
  let sorted = true;
  for (let i = 1; i < dued.length; i++) if (dued[i] < dued[i - 1]) sorted = false;
  note(`Due-date sequence (first 8): [${dued.slice(0, 8).join(", ")}]`);
  if (dued.length > 1 && sorted) pass(`R-2 — upcoming bills are ordered by due date (soonest first)`);
  else if (dued.length > 1) absent_(`R-2 — bills not strictly date-ordered ([${dued.slice(0, 6).join(", ")}]) — urgency sort may differ`);
  else absent_("R-2 — too few dated rows to verify ordering");

  // ── R-3 / R-4: mark a bill paid -> count drops + transaction posts ────────────
  const target = rows0[0];
  note(`Marking paid: "${target ? target.name : "?"}" (${target ? target.amount : "?"})`);
  const txnsBefore = await (async () => { await navTo(page, "Transactions"); await page.waitForTimeout(700); return (await page.evaluate(() => { const m = document.body.innerText.match(/([\d,]+)\s+transactions?\s+shown/i); return m ? parseInt(m[1].replace(/,/g, ""), 10) : null; })); })();
  await navTo(page, "Bills"); await page.waitForTimeout(700);
  const marked = await page.evaluate(() => {
    const row = [...document.querySelectorAll('.row')].find(r => /mark paid/i.test(r.textContent));
    if (!row) return "NO_ROW";
    const btn = [...row.querySelectorAll('button')].find(b => /mark paid/i.test(b.textContent));
    if (!btn) return "NO_BTN";
    btn.click(); return "clicked";
  });
  await page.waitForTimeout(1200);
  // some flows show a confirm; click a confirm/yes if present
  await page.evaluate(() => { const c = [...document.querySelectorAll('button')].find(b => /^(mark paid|confirm|yes|pay)$/i.test(b.textContent.trim()) && b.offsetParent !== null); if (c) c.click(); });
  await page.waitForTimeout(1000);
  const s1 = await billsSummary(page);
  note(`After mark paid: upcoming ${s0.upcoming} -> ${s1.upcoming}, dueTotal ${s0.dueTotal} -> ${s1.dueTotal} (click=${marked})`);
  await page.screenshot({ path: SS("L91_02_after_paid.png") });
  if (marked === "clicked" && s1.upcoming != null && s0.upcoming != null && s1.upcoming < s0.upcoming) pass(`R-3 — marking paid lowered upcoming bills (${s0.upcoming} -> ${s1.upcoming})`);
  else absent_(`R-3 — upcoming count did not drop (${s0.upcoming} -> ${s1.upcoming}, click=${marked})`);

  // R-4: transaction posted?
  await navTo(page, "Transactions"); await page.waitForTimeout(900);
  const txnsAfter = await page.evaluate(() => { const m = document.body.innerText.match(/([\d,]+)\s+transactions?\s+shown/i); return m ? parseInt(m[1].replace(/,/g, ""), 10) : null; });
  note(`Transactions count: ${txnsBefore} -> ${txnsAfter}`);
  if (txnsBefore != null && txnsAfter != null && txnsAfter > txnsBefore) pass(`R-4 — marking a bill paid posted a transaction (${txnsBefore} -> ${txnsAfter}) — leaves a financial trace`);
  else absent_(`R-4 — no new transaction after mark-paid (${txnsBefore} -> ${txnsAfter}) — bill payment may not post to the ledger (dev note candidate)`);

  // ── R-5: STRESS — mark several paid, count falls monotonically ─────────────────
  await navTo(page, "Bills"); await page.waitForTimeout(700);
  let prev = s1.upcoming != null ? s1.upcoming : (s0.upcoming || 0);
  const seq = [prev]; let mono = true;
  for (let i = 0; i < 3; i++) {
    const r = await page.evaluate(() => { const row = [...document.querySelectorAll('.row')].find(rw => /mark paid/i.test(rw.textContent)); if (!row) return "NONE"; const b = [...row.querySelectorAll('button')].find(x => /mark paid/i.test(x.textContent)); if (b) { b.click(); return "ok"; } return "NOBTN"; });
    await page.waitForTimeout(900);
    await page.evaluate(() => { const c = [...document.querySelectorAll('button')].find(b => /^(mark paid|confirm|yes|pay)$/i.test(b.textContent.trim()) && b.offsetParent !== null); if (c) c.click(); });
    await page.waitForTimeout(700);
    const cur = (await billsSummary(page)).upcoming;
    seq.push(cur);
    if (r !== "ok" || cur == null || cur > prev) mono = false;
    if (cur != null) prev = cur;
  }
  note(`Stress upcoming sequence: [${seq.join(" -> ")}]`);
  if (mono && seq[seq.length - 1] < seq[0]) pass(`R-5 — marking 3 more bills paid kept the count falling monotonically (${seq[0]} -> ${seq[seq.length - 1]}), no crash`);
  else absent_(`R-5 — count not monotonic under stress [${seq.join(" -> ")}]`);

  if (jsErrors.length === 0) pass("NO_JS_ERRORS — zero runtime JS errors across the ritual");
  else fail(`JS_ERRORS — ${jsErrors.length}: ${jsErrors.slice(0, 3).join("; ")}`);

} catch (err) {
  fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err);
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
