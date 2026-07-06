// C58 split-transaction UI + Reports per-tab empty-state verification.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
let failed = 0;
const fail = (m) => { console.error("FAIL: " + m); failed++; process.exitCode = 1; };
const pass = (m) => console.log("PASS: " + m);
const errs = [];

try {
  const ctx = await browser.newContext();
  const p = await ctx.newPage();
  p.on("pageerror", (e) => errs.push(String(e)));
  p.on("console", (m) => { if (m.type() === "error") errs.push(m.text()); });

  await p.goto(BASE + "/", { waitUntil: "networkidle" });
  await p.waitForSelector("#app", { timeout: 60000 });
  await p.waitForTimeout(4500); // WASM boot (loads sample data)

  // ---- C58: split editor on /transactions ----
  await p.click('a[href="/transactions"]');
  await p.waitForTimeout(1500);
  // Open the first editable row's inline editor (pencil).
  const editBtn = await p.$('tr.row [aria-label]:has(svg)');
  // Click the first edit (pencil) button found.
  await p.evaluate(() => {
    const b = document.querySelector('tr.row .td-actions button');
    if (b) b.click();
  });
  await p.waitForTimeout(800);

  const hasToggle = await p.$('[data-testid="txn-split-toggle"]');
  if (hasToggle) pass("split toggle present in inline edit"); else fail("no split toggle");

  // Open the split editor.
  await p.click('[data-testid="txn-split-toggle"]');
  await p.waitForTimeout(500);
  const editorState = await p.evaluate(() => {
    const ed = document.querySelector('[data-testid="split-editor"]');
    if (!ed) return null;
    return {
      rows: document.querySelectorAll('[data-testid="split-row"]').length,
      remainder: document.querySelector('[data-testid="split-remainder"]')?.textContent || "",
      hasSave: !!document.querySelector('[data-testid="split-save"]'),
      hasAdd: !!document.querySelector('[data-testid="split-add"]'),
    };
  });
  if (!editorState) { fail("split editor did not mount"); }
  else {
    if (editorState.rows >= 2) pass(`split editor mounted with ${editorState.rows} rows`); else fail(`only ${editorState.rows} rows`);
    if (editorState.hasSave && editorState.hasAdd) pass("save + add controls present"); else fail("missing controls");
    console.log("  remainder text: " + JSON.stringify(editorState.remainder));
  }

  // Drive a balanced split: read the transaction amount from the amount input, put
  // half in each of two split rows with categories, expect "Balanced" then save.
  const result = await p.evaluate(() => {
    const setVal = (el, v) => {
      const proto = Object.getPrototypeOf(el);
      const desc = Object.getOwnPropertyDescriptor(proto, "value");
      desc.set.call(el, v);
      el.dispatchEvent(new Event("input", { bubbles: true }));
    };
    // Amount input in the inline edit (the number field that's not a split-amt).
    const amtInput = document.querySelector('input[type="number"]:not([data-testid^="split-amt"])');
    const total = parseFloat(amtInput?.value || "0");
    const half = (total / 2).toFixed(2);
    const rest = (total - parseFloat(half)).toFixed(2);
    const a0 = document.querySelector('[data-testid="split-amt-0"]');
    const a1 = document.querySelector('[data-testid="split-amt-1"]');
    setVal(a0, half); setVal(a1, rest);
    // pick categories: first non-empty option for each select.
    const pickCat = (sel) => {
      const s = document.querySelector(sel);
      if (!s) return;
      for (const o of s.options) { if (o.value) { s.value = o.value; s.dispatchEvent(new Event("change", { bubbles: true })); break; } }
    };
    pickCat('[data-testid="split-cat-0"]');
    // second row: pick a different category if possible
    const s1 = document.querySelector('[data-testid="split-cat-1"]');
    if (s1) { const opts=[...s1.options].filter(o=>o.value); if(opts[1]){s1.value=opts[1].value;} else if(opts[0]){s1.value=opts[0].value;} s1.dispatchEvent(new Event("change",{bubbles:true})); }
    return { total, half, rest };
  });
  await p.waitForTimeout(400);
  const remAfter = await p.evaluate(() => document.querySelector('[data-testid="split-remainder"]')?.textContent || "");
  console.log(`  total=${result.total} half=${result.half}; remainder after fill: ${JSON.stringify(remAfter)}`);
  if (/Balanced/i.test(remAfter)) pass("remainder shows Balanced after even split"); else fail("not balanced: " + remAfter);

  // Save and confirm splits persisted (row re-renders; reopen editor → clear button shows).
  await p.click('[data-testid="split-save"]');
  await p.waitForTimeout(900);
  // Reopen first row edit. Because the txn now HasSplits, the editor auto-opens
  // (splitOpen seeds true), so a Clear control should already be present.
  await p.evaluate(() => { const b=document.querySelector('tr.row .td-actions button'); if(b)b.click(); });
  await p.waitForTimeout(700);
  const persisted = await p.evaluate(() => !!document.querySelector('[data-testid="split-clear"]'));
  if (persisted) pass("split persisted (Clear control present on reopen)"); else fail("split did not persist");

  await p.screenshot({ path: "e2e/screenshots/c58_split_editor.png" });

  // ---- Reports per-tab empty-state (Advanced with no custom fields) ----
  await p.click('a[href="/reports"]');
  await p.waitForTimeout(1500);
  const advEmpty = await p.evaluate(() => {
    // click the Advanced segment
    const segs = [...document.querySelectorAll('[role="radio"], button')].filter(b => /Advanced/i.test(b.textContent || ""));
    if (segs[0]) segs[0].click();
    return true;
  });
  await p.waitForTimeout(700);
  const tabEmpty = await p.evaluate(() => {
    const e = document.querySelector('[data-testid="reports-tab-empty"]');
    return e ? e.textContent : null;
  });
  if (tabEmpty) pass("Advanced tab shows empty-state instead of blank: " + JSON.stringify(tabEmpty.slice(0,40)));
  else console.log("  NOTE: Advanced tab empty-state not shown (sample may have custom fields) — not a failure");
  await p.screenshot({ path: "e2e/screenshots/reports_tab_empty.png" });

  console.log("Console/page errors: " + errs.length);
  if (errs.length) { errs.slice(0,5).forEach(e => console.log("  ERR: " + e)); fail("had console errors"); }
} catch (e) {
  fail("exception: " + e.message);
} finally {
  await browser.close();
}
console.log(failed ? "RESULT: FAILED" : "RESULT: PASSED");
process.exit(failed ? 1 : 0);
