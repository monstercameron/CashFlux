// B16 E2E story — "reconcile: toggle cleared". Adds a transaction, toggles its
// cleared status, and asserts both UX (the cleared-status filter includes/excludes
// it accordingly) and correctness (the cleared flag persists to the dataset and
// survives a reload). Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const DESC = "ZZCLEAR-7";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

// Finds a transaction in the dataset store by its description.
const txnByDesc = (page, desc) =>
  page.evaluate((d) => {
    const data = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    let found = null;
    const walk = (o) => {
      if (!o || typeof o !== "object") return;
      if (Array.isArray(o)) return o.forEach(walk);
      if (o.desc === d) found = o;
      Object.values(o).forEach(walk);
    };
    walk(data);
    return found;
  }, desc);

const rowCount = (page) => page.locator(".txn-table .row-desc").count();

// Polls the dataset store (autosave is on a short ticker) until the transaction
// matches the predicate, or times out — returns the last seen value either way.
async function waitForTxn(page, desc, pred, timeoutMs = 7000) {
  let t = null;
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    t = await txnByDesc(page, desc);
    if (pred(t)) return t;
    await page.waitForTimeout(400);
  }
  return t;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#txn-add", { timeout: 60000 });

  // Seed a uniquely described transaction and filter the ledger to it.
  await page.locator("#txn-add").fill(DESC);
  await page.locator('input[type="number"][aria-required="true"]').fill("5.00");
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(600);
  await page.locator('input[type="search"]').first().fill(DESC);
  await page.waitForTimeout(400);
  if ((await rowCount(page)) !== 1) fail("expected exactly the seeded row after filtering");

  // It starts not-cleared (poll the store, which autosaves on a short ticker).
  let txn = await waitForTxn(page, DESC, (t) => !!t);
  if (!txn) fail("seeded transaction not found in the dataset");
  else if (txn.cleared) fail("transaction should start not-cleared");

  // Toggle cleared (the per-row reconcile button).
  await page.locator('button[title="Toggle reconciled (cleared) status"]').first().click();
  txn = await waitForTxn(page, DESC, (t) => t && t.cleared === true);
  if (!txn || txn.cleared !== true) fail(`transaction should be cleared after toggling, got cleared=${txn && txn.cleared}`);

  // The cleared-status filter now excludes it under "not cleared" and includes it
  // under "cleared".
  const clearedSelect = page.locator('select:has(option[value="yes"])').first();
  await clearedSelect.selectOption("no");
  await page.waitForTimeout(400);
  if ((await rowCount(page)) !== 0) fail('a cleared txn should be hidden under the "not cleared" filter');
  await clearedSelect.selectOption("yes");
  await page.waitForTimeout(400);
  if ((await rowCount(page)) !== 1) fail('the cleared txn should show under the "cleared" filter');

  // Persist + survive reload (cleared flag).
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#txn-add", { timeout: 60000 });
  await page.waitForTimeout(800);
  const after = await txnByDesc(page, DESC);
  if (!after || after.cleared !== true) fail("cleared flag did not survive a reload");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: toggled "${DESC}" cleared — filter reflects it and the flag persists across reload.`);
} finally {
  await browser.close();
}
