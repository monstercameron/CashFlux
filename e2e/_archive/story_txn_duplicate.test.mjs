// B16 E2E story — "duplicate a transaction". Seeds a transaction, duplicates it
// via the row's Duplicate button, and asserts a standalone copy is created (same
// description, two ledger rows, and neither is a transfer leg). Exits non-zero on
// any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const DESC = "ZZDUP-5";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const txnsByDesc = (page, desc) =>
  page.evaluate((d) => {
    const data = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    return (data.transactions || []).filter((t) => t.desc === d);
  }, desc);
async function waitForTxns(page, desc, pred, timeoutMs = 7000) {
  let list = [];
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    list = await txnsByDesc(page, desc);
    if (pred(list)) return list;
    await page.waitForTimeout(400);
  }
  return list;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#txn-add", { timeout: 60000 });

  // Seed a transaction and filter the ledger to it.
  await page.locator("#txn-add").fill(DESC);
  await page.locator('input[type="number"][aria-required="true"]').fill("8.88");
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(500);
  await page.locator('input[type="search"]').first().fill(DESC);
  await page.waitForTimeout(400);
  if ((await page.locator(".txn-table .row-desc").count()) !== 1) fail("expected exactly the seeded row before duplicating");

  // Duplicate it.
  await page.locator('button[title="Copy this transaction to today"]').first().click();
  await page.waitForTimeout(500);

  // UX: the filtered ledger now shows two rows with the same description.
  if ((await page.locator(".txn-table .row-desc").count()) !== 2) fail("duplicate did not add a second ledger row");

  // Correctness: two transactions with this description, neither a transfer leg.
  const list = await waitForTxns(page, DESC, (l) => l.length >= 2);
  if (list.length !== 2) fail(`expected 2 transactions named ${DESC}, got ${list.length}`);
  if (list.some((t) => t.transferAccountId)) fail("a duplicate should be a standalone copy, not a transfer leg");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: duplicated "${DESC}" — 2 standalone rows, neither a transfer leg.`);
} finally {
  await browser.close();
}
