// L23 — CSV import resilience end-to-end. Pastes a CSV with 3 valid rows and 2
// malformed rows (one missing amount, one non-numeric amount) into the Documents
// CSV importer, submits it, and asserts that 3 transactions landed in
// localStorage and a "Skipped 2" message is shown.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

// CSV body is built after we discover a real account name (rows need a valid
// account to pass the validated write path). 3 valid rows + 2 malformed: one
// missing amount, one non-numeric amount.
const csvFor = (acct) =>
  [
    "date,payee,amount,account",
    `2026-06-01,ZZRESIL-Salary,2500.00,${acct}`,
    `2026-06-02,ZZRESIL-Groceries,-86.40,${acct}`,
    `2026-06-03,ZZRESIL-Gas,-45.00,${acct}`,
    "2026-06-04,ZZRESIL-BadRow1,,",
    "2026-06-05,ZZRESIL-BadRow2,notanumber,",
  ].join("\n");

try {
  const page = await (await browser.newContext()).newPage();
  page.on("dialog", async (d) => { fail("native dialog opened: " + d.type()); await d.dismiss(); });
  page.on("pageerror", (e) => fail("page error: " + e.message));
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(500);

  // Navigate to Documents. It lives under the "Data & import" Tools sub-section;
  // expand that sub-section if it is collapsed.
  if ((await page.locator('nav a[title="Documents"]').count()) === 0) {
    await page.locator(".rail-subhead", { hasText: "Data & import" }).first().click();
    await page.waitForTimeout(250);
  }
  await page.locator('nav a[title="Documents"]').first().click();
  await page.waitForTimeout(500);

  // The CSV importer textarea has a placeholder documenting "date,payee,amount,account".
  // Discover a real, comma-free account name for the valid rows.
  const dataset = () => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
  let acctName = "";
  for (let i = 0; i < 20 && !acctName; i++) {
    const d0 = await dataset();
    const a = (d0.accounts || []).find((x) => x.name && !x.name.includes(","));
    if (a) acctName = a.name;
    else await page.waitForTimeout(300);
  }
  if (!acctName) fail("no usable account found for the import");

  const csvTextarea = page.locator("textarea[placeholder*='date,payee,amount']");
  await csvTextarea.waitFor({ timeout: 10000 });
  await csvTextarea.fill(csvFor(acctName));

  // Submit the CSV importer form.
  await page.getByRole("button", { name: "Import", exact: true }).click();
  await page.waitForTimeout(600);

  // Assert the "Skipped 2" message is visible somewhere on the page.
  const skippedVisible = await page.locator("text=/Skipped 2/i").count();
  if (skippedVisible === 0) fail('expected a "Skipped 2" message but none was found');

  // Flush autosave and poll localStorage for the 3 valid transactions.
  const matching = (d) =>
    (d.transactions || []).filter(
      (t) => /ZZRESIL-(Salary|Groceries|Gas)/.test(t.desc || t.payee || "")
    );

  let txns = [];
  for (let i = 0; i < 20; i++) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    txns = matching(await dataset());
    if (txns.length >= 3) break;
    await page.waitForTimeout(300);
  }
  if (txns.length !== 3) fail(`expected 3 valid transactions in localStorage, got ${txns.length}`);

  // Confirm the malformed rows are NOT present.
  const bad = (await dataset()).transactions || [];
  const badRows = bad.filter((t) => /ZZRESIL-BadRow/.test(t.desc || t.payee || ""));
  if (badRows.length !== 0) fail(`malformed rows appeared in localStorage: ${JSON.stringify(badRows)}`);

  if (!process.exitCode) console.log("PASS: 3 valid transactions imported, 2 malformed rows skipped, Skipped message shown.");
} finally {
  await browser.close();
}
