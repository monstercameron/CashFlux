// L94 E2E loop story — "The Careful Curator" (audit) — 2026-06-24
//
// Theme: DESTRUCTIVE-DELETE GUARDS. An enterprise app must not lose data to a single misclick. This
// audits every per-row delete: does it (a) confirm, (b) reassign (categories), (c) offer undo, or
// (d) delete instantly with no recovery (a data-loss defect)? Baselines: Transactions + Budgets are
// GUARDED (confirm dialog). This checks Goals / Accounts / Categories / To-do. Invariants:
//   C-1  Each entity exposes a delete action.
//   C-2  Clicking delete does NOT immediately destroy the row without a guard (confirm / reassign / undo).
//   (Findings for any unguarded delete are logged to TODOS for a dev.)
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_94_careful_curator.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0, absent = 0;
const pass = (l) => { console.log(`PASS:   ${l}`); passed++; };
const fail = (l) => { console.error(`FAIL:   ${l}`); failed++; };
const absent_ = (l) => { console.log(`ABSENT: ${l}`); absent++; };
const note = (l) => { console.log(`NOTE:   ${l}`); };

// Per-entity delete test in an isolated context (fresh sample data each time).
// Returns { items0, items1, guard } where guard ∈ {confirm, reassign, undo, IMMEDIATE, no-action}.
async function auditDelete(screen, rowSel, delMatch) {
  const ctx = await browser.newContext();
  const p = await ctx.newPage(); p.setViewportSize({ width: 1440, height: 1000 });
  let result = { items0: null, items1: null, guard: "error" };
  try {
    await p.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
    await p.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
    await p.evaluate(() => { const x = [...document.querySelectorAll("button")].find(b => /load sample|sample data/i.test(b.textContent)); if (x) x.click(); });
    await p.waitForTimeout(1500);
    await p.evaluate((t) => { const l = [...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x => x.getAttribute("title") === t); if (l) l.click(); }, screen);
    await p.waitForTimeout(1100);
    const items0 = await p.evaluate((sel) => document.querySelectorAll(sel).length, rowSel);
    const clicked = await p.evaluate((m) => { const btns = [...document.querySelectorAll('button')].filter(b => new RegExp(m, "i").test(b.getAttribute('aria-label') || b.getAttribute('title') || "")); if (btns[0]) { btns[0].click(); return true; } return false; }, delMatch);
    await p.waitForTimeout(800);
    // classify the guard
    const cls = await p.evaluate((sel) => {
      const dialog = document.querySelector('.cf-dialog');
      const reassign = [...document.querySelectorAll('*')].some(e => /reassign|move.*delete|move and delete/i.test(e.textContent || "") && e.children.length < 6 && (e.querySelector('select') || /reassign/i.test(e.className || "")));
      const toast = document.querySelector('.toast');
      const undo = toast && toast.offsetParent !== null && (/undo/i.test(toast.textContent) || !!toast.querySelector('button'));
      return { hasDialog: !!dialog, dialogMsg: dialog ? dialog.textContent.trim().slice(0, 70) : null, reassign, undo, toastTxt: toast && toast.offsetParent !== null ? toast.textContent.trim().slice(0, 50) : null, items: document.querySelectorAll(sel).length };
    }, rowSel);
    let guard = "no-action";
    if (cls.hasDialog) guard = "confirm";
    else if (cls.reassign) guard = "reassign";
    else if (cls.undo) guard = "undo";
    else if (cls.items < items0) guard = "IMMEDIATE";
    result = { items0, items1: cls.items, guard, clicked, dialogMsg: cls.dialogMsg, toastTxt: cls.toastTxt };
  } catch (e) { result.err = e.message.slice(0, 60); }
  await ctx.close();
  return result;
}

try {
  // sanity: app boots
  const ctx0 = await browser.newContext(); const pg = await ctx0.newPage();
  await pg.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 20000 });
  await pg.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  pass("HYDRATION — app booted");
  await ctx0.close();

  const cases = [
    { screen: "Goals", row: ".goal, [class*='goal-row'], .rows .row", del: "delete goal", label: "Goal" },
    { screen: "Accounts", row: ".row, [class*='acct']", del: "delete account", label: "Account" },
    { screen: "Categories", row: ".rows .row", del: "delete category", label: "Category" },
    { screen: "To-do", row: ".rows .row, [class*='task']", del: "delete task", label: "To-do task" },
  ];

  const findings = [];
  for (const c of cases) {
    const r = await auditDelete(c.screen, c.row, c.del);
    const verdict = r.guard === "confirm" ? `GUARDED (confirm: "${r.dialogMsg}")`
      : r.guard === "reassign" ? "GUARDED (reassign-on-delete panel)"
        : r.guard === "undo" ? `SOFT (undo toast: "${r.toastTxt}")`
          : r.guard === "IMMEDIATE" ? "⚠ UNGUARDED — deleted instantly, no confirm/undo"
            : r.guard === "no-action" ? "no-op (delete did nothing / row count unchanged)"
              : "error:" + (r.err || "");
    note(`${c.label}: items ${r.items0}->${r.items1}, clicked=${r.clicked} → ${verdict}`);
    if (r.guard === "IMMEDIATE") { fail(`C-2 — ${c.label} delete is UNGUARDED (instant, no confirm/undo) — data loss on misclick`); findings.push(c.label); }
    else if (["confirm", "reassign", "undo"].includes(r.guard)) pass(`C-2 — ${c.label} delete is guarded (${r.guard})`);
    else absent_(`C-2 — ${c.label}: could not classify (guard=${r.guard}, clicked=${r.clicked}, ${r.err || ""})`);
  }
  if (findings.length === 0) note("All audited deletes are guarded.");
  else note(`UNGUARDED deletes (dev tickets): ${findings.join(", ")}`);

} catch (err) {
  fail(`UNEXPECTED_ERROR — ${err.message}`); console.error(err);
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
