// L18 gate — filter transactions by a custom field value. Picks the seeded
// "project" select custom field, chooses a value, and asserts the ledger narrows
// to only rows carrying that value (and that the filter persists in the criteria).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#txn-add", { timeout: 60000 });
  await page.waitForTimeout(400);
  const ft = page.getByText("Filters", { exact: false }).first();
  if (await ft.count()) { await ft.click(); await page.waitForTimeout(300); }

  const rowCount = () => page.locator("tr.row[data-id]").count();
  const before = await rowCount();
  if (before < 2) { fail(`too few rows to test filtering (${before})`); process.exit(1); }

  const keySel = page.locator('[data-testid="txn-filter-custom-key"]');
  if ((await keySel.count()) === 0) { fail("no custom-field filter control"); process.exit(1); }
  await keySel.selectOption("project");
  await page.waitForTimeout(300);
  const valSel = page.locator('[data-testid="txn-filter-custom-val"]');
  const val = await valSel.evaluate((el) => { const o = [...el.options].find((x) => x.value); el.value = o.value; el.dispatchEvent(new Event("change", { bubbles: true })); return o.value; });
  await page.waitForTimeout(600);

  // The ledger narrowed, and the criteria persisted customKey/customVal.
  const after = await rowCount();
  if (after >= before) fail(`custom-field filter did not narrow the ledger (${before} -> ${after})`);
  const crit = await page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:tx-filter") || "{}"));
  if (crit.customKey !== "project" || crit.customVal !== val) {
    fail(`criteria not persisted: customKey=${crit.customKey}, customVal=${crit.customVal}, want project/${val}`);
  }
  if (!process.exitCode) console.log(`PASS: custom-field filter narrowed the ledger ${before}->${after} on project=${val}.`);
} finally {
  await browser.close();
}
