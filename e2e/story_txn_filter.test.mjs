// B16 E2E story — "transactions filter persists across reload". Adds a uniquely
// described transaction, filters the ledger to it via the search box, and asserts
// the list narrows to the one match AND that the filter (and the narrowed view)
// survive a reload. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const DESC = "ZZFILTERTEST-42";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const rowCount = (page) => page.locator(".rows .row-desc").count();

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#txn-add", { timeout: 60000 });

  // Seed a uniquely described transaction so the filter has a deterministic match.
  await page.locator("#txn-add").fill(DESC);
  await page.locator('input[type="number"][aria-required="true"]').fill("9.99");
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(600);

  const before = await rowCount(page);
  if (before < 2) fail(`expected several ledger rows before filtering, got ${before}`);

  // Filter the ledger to our unique description.
  await page.locator('input[type="search"]').first().fill(DESC);
  await page.waitForTimeout(500);
  const afterFilter = await rowCount(page);
  if (afterFilter !== 1) fail(`filter should narrow the ledger to 1 row, got ${afterFilter}`);
  if ((await page.getByText(DESC).count()) === 0) fail("the matching row should still be visible");

  // The filter is persisted.
  const storedText = await page.evaluate(() => {
    try {
      return (JSON.parse(localStorage.getItem("cashflux:tx-filter") || "{}")).text;
    } catch {
      return null;
    }
  });
  if (storedText !== DESC) fail(`persisted filter text = ${storedText}, want ${DESC}`);

  // Survives reload: the search box keeps the term and the view stays narrowed.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#txn-add", { timeout: 60000 });
  await page.waitForTimeout(800);
  const searchVal = await page.locator('input[type="search"]').first().inputValue();
  if (searchVal !== DESC) fail(`after reload the search box = "${searchVal}", want "${DESC}"`);
  const afterReload = await rowCount(page);
  if (afterReload !== 1) fail(`after reload the ledger should still show 1 filtered row, got ${afterReload}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: filtered ledger to "${DESC}" (${before} → 1 rows); filter persists across reload.`);
} finally {
  await browser.close();
}
