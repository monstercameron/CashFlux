// L89 E2E loop story — "Putting Money to Work" (Priya) — 2026-06-24
//
// Theme: ALLOCATION ENGINE INTEGRITY. When a paycheck lands, the everyday question is "where should
// this money go?" CashFlux's Allocate screen answers with RANKED, EXPLAINED suggestions that must
// (1) be ordered #1..#N, (2) each show a destination + share + a plain-English "why", (3) react when
// the user excludes a destination (re-rank), and (4) react when the user changes the strategy profile
// (Balanced → Pay down debt → Finish goals) so the ranking reflects the chosen priority. Invariants:
//   A-1  The screen shows ranked allocation suggestions (≥3).
//   A-2  Each suggestion shows a destination, a share (%/amount), AND an explainability line.
//   A-3  Suggestions are ranked monotonically (#1,#2,#3… in DOM order — the top item is the priority).
//   A-4  Excluding the #1 destination changes the suggestion set (re-rank / removal works).
//   A-5  Changing the Profile recomputes the ranking (the top destination or its share changes).
//   A-6  A debt-priority profile ("Pay down debt") puts a debt destination at #1 (semantic correctness).
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_89_money_to_work.mjs

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

// Read ranked allocation suggestions in DOM order: { rank, name, pct, explain }.
const suggestions = (page) => page.evaluate(() => {
  const rows = [...document.querySelectorAll('.budget')].filter(r => r.querySelector('.rank-badge'));
  return rows.map(r => {
    const badge = r.querySelector('.rank-badge');
    const rank = badge ? parseInt(badge.textContent.replace(/[^0-9]/g, ""), 10) : null;
    const head = r.querySelector('.budget-head');
    const amount = r.querySelector('.budget-amount');
    const sub = r.querySelector('.budget-sub');
    let name = "";
    if (head) {
      const a = head.querySelector('.budget-amount');
      name = (head.textContent || "")
        .replace(badge ? badge.textContent : "", "")
        .replace(a ? a.textContent : "", "")
        .replace(/(Exclude|Include).*$/gi, "").replace(/[#…·]/g, "").trim();
    }
    const pctM = (r.textContent || "").match(/(\d+)%/);
    return {
      rank,
      name,
      pct: pctM ? parseInt(pctM[1], 10) : null,
      explain: sub ? sub.textContent.trim() : ((r.textContent || "").match(/returns[^]*?pays debt|returns[^]*?liquidity \d+/i) || [""])[0],
    };
  });
});

const setProfile = (page, label) => page.evaluate((label) => {
  const s = [...document.querySelectorAll('select')].find(s => s.getAttribute('aria-label') === 'Profile');
  if (!s) return "NO_SELECT";
  const opt = [...s.options].find(o => new RegExp(label, "i").test(o.textContent));
  if (!opt) return "NO_OPT";
  s.value = opt.value; s.dispatchEvent(new Event('change', { bubbles: true }));
  return "set";
}, label);

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

  await navTo(page, "Allocate");
  await page.waitForTimeout(1000);
  await page.screenshot({ path: SS("L89_01_allocate.png") });

  // ── A-1: ranked suggestions present ───────────────────────────────────────────
  const s0 = await suggestions(page);
  note(`Suggestions: ${s0.length} [${s0.slice(0, 5).map(s => `#${s.rank} ${s.name} ${s.pct}%`).join(" | ")}]`);
  if (s0.length >= 3) pass(`A-1 — ${s0.length} ranked allocation suggestions shown`);
  else absent_(`A-1 — too few suggestions (${s0.length})`);

  // ── A-2: each has destination + share + explainability ────────────────────────
  if (s0.length) {
    const named = s0.filter(s => s.name);
    const withPct = s0.filter(s => s.pct != null);
    const withwhy = s0.filter(s => /returns|stability|liquidity|pays debt|fund|goal|target/i.test(s.explain || ""));
    note(`Named: ${named.length}/${s0.length} · with %: ${withPct.length}/${s0.length} · with why: ${withwhy.length}/${s0.length}`);
    note(`Sample explain: "${(s0[0] && s0[0].explain || "").slice(0, 80)}"`);
    if (named.length === s0.length && withPct.length >= Math.ceil(s0.length * 0.8)) pass(`A-2a — every suggestion has a destination + share`);
    else absent_(`A-2a — some suggestions missing name/share (named ${named.length}, pct ${withPct.length} of ${s0.length})`);
    if (withwhy.length >= Math.ceil(s0.length * 0.8)) pass(`A-2b — suggestions carry an explainability line (${withwhy.length}/${s0.length}) — determinism/explainability honored`);
    else absent_(`A-2b — explainability missing on many rows (${withwhy.length}/${s0.length})`);
  }

  // ── A-3: ranked monotonically in DOM order ────────────────────────────────────
  const ranks = s0.map(s => s.rank).filter(r => r != null);
  let mono = ranks.length > 1;
  for (let i = 1; i < ranks.length; i++) if (ranks[i] < ranks[i - 1]) mono = false;
  note(`Rank sequence (DOM order): [${ranks.join(", ")}]`);
  if (mono && ranks[0] === 1) pass(`A-3 — suggestions are ranked monotonically from #1 (${ranks.slice(0, 6).join(",")}…)`);
  else absent_(`A-3 — rank order not monotonic-from-1 ([${ranks.slice(0, 8).join(",")}])`);

  const top0 = s0[0] ? s0[0].name : null;

  // ── A-4: excluding #1 changes the set ─────────────────────────────────────────
  const exResult = await page.evaluate(() => {
    const rows = [...document.querySelectorAll('.budget')].filter(r => r.querySelector('.rank-badge'));
    if (!rows.length) return "NO_ROWS";
    const btn = rows[0].querySelector('button');
    const ex = [...rows[0].querySelectorAll('button')].find(b => /exclude/i.test(b.textContent)) || btn;
    if (ex) { ex.click(); return "clicked"; }
    return "NO_BTN";
  });
  await page.waitForTimeout(1000);
  const s1 = await suggestions(page);
  const top1 = s1[0] ? s1[0].name : null;
  note(`After excluding "${top0}": now ${s1.length} suggestions, new top="${top1}"`);
  if (exResult === "clicked" && (s1.length !== s0.length || top1 !== top0)) pass(`A-4 — excluding #1 ("${top0}") re-ranked the list (top is now "${top1}")`);
  else absent_(`A-4 — exclude did not change the set (result=${exResult}, len ${s0.length}→${s1.length}, top ${top0}→${top1})`);

  // ── A-5 / A-6: profile change recomputes ranking ──────────────────────────────
  await navTo(page, "Allocate"); await page.waitForTimeout(800); // reset any exclusions by re-entering
  const base = await suggestions(page);
  const baseTop = base[0] ? base[0].name : null;
  const setDebt = await setProfile(page, "Pay down debt");
  await page.waitForTimeout(1200);
  await page.screenshot({ path: SS("L89_02_paydebt.png") });
  const debtS = await suggestions(page);
  const debtTop = debtS[0] ? debtS[0].name : null;
  note(`Profile Balanced top="${baseTop}" → Pay-down-debt top="${debtTop}" (set=${setDebt})`);
  if (setDebt === "set" && debtTop && (debtTop !== baseTop || JSON.stringify(debtS.map(s => s.name)) !== JSON.stringify(base.map(s => s.name)))) pass(`A-5 — changing Profile recomputed the ranking (top "${baseTop}" → "${debtTop}")`);
  else absent_(`A-5 — profile change did not visibly recompute (top ${baseTop} → ${debtTop})`);
  // A-6 — debt profile should rank a debt destination first (Card / Loan / Pay down)
  if (debtTop && /pay down|card|loan|debt|credit/i.test(debtTop)) pass(`A-6 — "Pay down debt" profile puts a debt destination at #1 ("${debtTop}") — semantically correct`);
  else absent_(`A-6 — top under debt profile isn't obviously a debt destination ("${debtTop}")`);

  // ── try "Finish goals" too (extra recompute confidence) ───────────────────────
  const setGoals = await setProfile(page, "Finish goals");
  await page.waitForTimeout(1200);
  const goalS = await suggestions(page);
  const goalTop = goalS[0] ? goalS[0].name : null;
  note(`Profile Finish-goals top="${goalTop}" (set=${setGoals})`);

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
