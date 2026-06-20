// B16 E2E story — "bulk clear transactions". Seeds two transactions, selects both
// via the per-row select buttons, and uses the bulk "Mark cleared" action, then
// asserts both become cleared in the dataset. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const A = "ZZBULK-1";
const B = "ZZBULK-2";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const txnByDesc = (page, desc) =>
  page.evaluate((d) => {
    const data = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    return (data.transactions || []).find((t) => t.desc === d) || null;
  }, desc);
async function waitForBoth(page, timeoutMs = 7000) {
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    const a = await txnByDesc(page, A);
    const b = await txnByDesc(page, B);
    if (a && b && a.cleared === true && b.cleared === true) return { a, b };
    await page.waitForTimeout(400);
  }
  return { a: await txnByDesc(page, A), b: await txnByDesc(page, B) };
}

async function addTxn(page, desc, amount) {
  await page.locator("#txn-add").fill(desc);
  await page.locator('input[type="number"][aria-required="true"]').fill(amount);
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(400);
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#txn-add", { timeout: 60000 });

  // Seed two transactions and filter the ledger to just them.
  await addTxn(page, A, "3.00");
  await addTxn(page, B, "4.00");
  await page.locator('input[type="search"]').first().fill("ZZBULK");
  await page.waitForTimeout(400);
  if ((await page.locator(".rows .row-desc").count()) !== 2) fail("expected exactly the two seeded rows after filtering");

  // Select both rows, then bulk-clear.
  const checks = page.locator('.rows .row button[title="Select for bulk actions"]');
  const n = await checks.count();
  for (let i = 0; i < n; i++) await checks.nth(i).click();
  await page.locator('button[title="Mark the selected transactions cleared"]').first().click();

  // Both transactions are now cleared.
  const { a, b } = await waitForBoth(page);
  if (!a || a.cleared !== true) fail(`${A} should be cleared, got cleared=${a && a.cleared}`);
  if (!b || b.cleared !== true) fail(`${B} should be cleared, got cleared=${b && b.cleared}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: bulk-cleared ${A} and ${B} (both cleared in the dataset).`);
} finally {
  await browser.close();
}
