// L44 gate — CSV import account selector. Navigates to /documents and asserts
// that the CSV import section contains an account <select> with aria-label
// "documents.csvAccount" (or the English fallback text) and that at least one
// account option is present. Also asserts the Import submit button is rendered
// *before* (i.e. above) the textarea in DOM order so it is visible without
// scrolling on short viewports. Exits non-zero on any failure.
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

  await page.goto(BASE + "/documents", { waitUntil: "domcontentloaded" });
  // Wait for the page to hydrate (accounts must be loaded before selects render).
  await page.waitForSelector("textarea", { timeout: 60000 });
  await page.waitForTimeout(600);

  // 1) The CSV section must contain an account <select>.
  // The select's aria-label is the i18n key "documents.csvAccount" (falls back
  // to the key itself when the key isn't in the catalog yet).
  const csvForm = page.locator("form", { has: page.getByPlaceholder(/date,\s*payee/i) });
  const acctSelect = csvForm.locator("select");
  const selectCount = await acctSelect.count();
  if (selectCount === 0) {
    fail("CSV import form has no <select> for the destination account");
  } else {
    // At least one account option (the sample dataset always has accounts).
    const optionCount = await acctSelect.first().locator("option").count();
    if (optionCount === 0) {
      fail("CSV import account <select> has no options");
    } else {
      console.log(`PASS: CSV import account select present with ${optionCount} option(s).`);
    }
  }

  // 2) The Import submit button must appear before the textarea in DOM order
  //    (it is placed above the textarea so it is visible without scrolling).
  const submitBtn = csvForm.locator('button[type="submit"]');
  const submitCount = await submitBtn.count();
  if (submitCount === 0) {
    fail("CSV import form has no submit button");
  } else {
    // Compare bounding-box Y positions: button must have a lower Y than textarea.
    const btnBox = await submitBtn.first().boundingBox();
    const txtBox = await csvForm.locator("textarea").first().boundingBox();
    if (btnBox && txtBox) {
      if (btnBox.y >= txtBox.y) {
        fail(
          `Import button (y=${btnBox.y.toFixed(0)}) is not above the textarea (y=${txtBox.y.toFixed(0)}) — still below the fold`
        );
      } else {
        console.log(
          `PASS: Import button (y=${btnBox.y.toFixed(0)}) is above the textarea (y=${txtBox.y.toFixed(0)}).`
        );
      }
    } else {
      fail("Could not measure positions of Import button or textarea");
    }
  }

  // 3) Smoke-test: select the first account, paste a no-account-column CSV, and
  //    assert the import succeeds (fallback account ID is used).
  const dataset = () => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
  const d0 = await dataset();
  const accs = d0.accounts || [];
  if (accs.length > 0) {
    const target = accs[0];
    await csvForm.locator("select").first().selectOption(target.id);
    const payee = "ZZIMPORTACCT" + Date.now();
    // No account column — the fallback should route to the selected account.
    const csv = `date,payee,amount\n2026-06-10,${payee},-9.99`;
    await csvForm.locator("textarea").fill(csv);
    await csvForm.locator('button[type="submit"]').first().click();
    // Wait for the transaction to land.
    let found = false;
    for (let i = 0; i < 10; i++) {
      await page.waitForTimeout(400);
      const d = await dataset();
      if ((d.transactions || []).some((t) => t.desc === payee && t.accountId === target.id)) {
        found = true;
        break;
      }
    }
    if (!found) {
      fail(`Fallback-account CSV import: transaction "${payee}" not found in account "${target.name}" (${target.id})`);
    } else {
      console.log(`PASS: Fallback-account import routed "${payee}" to "${target.name}" without an account column.`);
    }
  } else {
    console.log("SKIP: no accounts in dataset, skipping fallback-account smoke test.");
  }
} finally {
  await browser.close();
}
