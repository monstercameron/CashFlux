// C39 E2E check — recent-payee autocomplete in Quick-Add.
//
// Verifies that:
//   1. The Quick-Add panel contains a Payee <input> with data-testid="txn-add-payee"
//      wired to a <datalist id="qa-payees"> via list="qa-payees".
//   2. After adding a transaction with a payee, re-opening Quick-Add shows that payee
//      as a datalist <option> (autocomplete suggestion).
//
// Limitation: browsers hide the datalist dropdown in headless mode, so we cannot
// simulate the user picking a suggestion from the popup. We verify the DOM wiring
// (datalist presence, list attribute, option values) which is sufficient to confirm
// the feature is correctly rendered.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const PAYEE = "C39-Payee-AutocompleteTest";
const DESC = "C39 autocomplete check expense";
const AMOUNT = "9.99";

const browser = await chromium.launch({ headless: true });
let passed = 0;
let failed = 0;
const pass = (label) => { console.log(`PASS: ${label}`); passed++; };
const fail = (label) => { console.error(`FAIL: ${label}`); failed++; process.exitCode = 1; };

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 800 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });

  // Load sample data so there is a usable account for the Quick-Add form.
  const hasSample = await page.evaluate(() => {
    const raw = localStorage.getItem("cashflux:dataset") || "{}";
    try { return Object.keys(JSON.parse(raw).accounts || {}).length > 0; } catch { return false; }
  });
  if (!hasSample) {
    const loadBtn = page.locator('button', { hasText: "Load sample" });
    if (await loadBtn.count() > 0) {
      await loadBtn.first().click();
      await page.waitForTimeout(1000);
    }
  }

  // ── Step 1: open Quick-Add and check Payee field + datalist wiring. ──
  const qaBtn = page.locator('[data-testid="quick-add-open"], button[aria-label*="quick-add"], button[aria-label*="transaction"]').first();
  // Fallback: try Alt+N shortcut which opens Quick-Add.
  await page.keyboard.press("Alt+n");
  await page.waitForTimeout(600);

  const payeeInput = page.locator('[data-testid="txn-add-payee"]');
  if (await payeeInput.count() === 0) {
    fail("Payee input (data-testid=txn-add-payee) not found in Quick-Add panel");
  } else {
    pass("Payee input is present in Quick-Add panel");
  }

  // Verify the list attribute points to qa-payees.
  const listAttr = await payeeInput.getAttribute("list").catch(() => null);
  if (listAttr === "qa-payees") {
    pass("Payee input list attribute is wired to 'qa-payees'");
  } else {
    fail(`Payee input list attribute is '${listAttr}', expected 'qa-payees'`);
  }

  // Verify the datalist element exists in the DOM.
  const datalist = page.locator("datalist#qa-payees");
  if (await datalist.count() > 0) {
    pass("datalist#qa-payees is present in the DOM");
  } else {
    fail("datalist#qa-payees not found in the DOM");
  }

  // ── Step 2: submit a transaction with a known payee, then re-open and verify ──
  //    that payee appears as an option in the datalist.
  await page.locator('[data-testid="txn-add-payee"]').fill(PAYEE);
  await page.locator('[data-testid="txn-add-desc"]').fill(DESC);
  await page.locator('[data-testid="txn-add-amount"]').fill(AMOUNT);
  await page.waitForTimeout(200);

  // Click Save (the panel footer Save button).
  const saveBtn = page.locator('button[type="button"]', { hasText: /^Save$/ }).first();
  await saveBtn.click();
  await page.waitForTimeout(800);

  // Re-open Quick-Add.
  await page.keyboard.press("Alt+n");
  await page.waitForTimeout(600);

  // Check the datalist now contains our payee as an option.
  const optionCount = await page.evaluate((payee) => {
    const dl = document.getElementById("qa-payees");
    if (!dl) return -1;
    const opts = Array.from(dl.options || dl.querySelectorAll("option"));
    return opts.filter((o) => o.value === payee).length;
  }, PAYEE);

  if (optionCount > 0) {
    pass(`Datalist contains recently-used payee '${PAYEE}'`);
  } else if (optionCount === -1) {
    fail("datalist#qa-payees not found on second open");
  } else {
    fail(`Payee '${PAYEE}' not found in datalist options after transaction was saved`);
  }

  if (errors.length) fail("Page errors: " + errors.join(" | "));
} finally {
  await browser.close();
}

console.log(`\nResults: ${passed} passed, ${failed} failed.`);
if (failed > 0) process.exitCode = 1;
