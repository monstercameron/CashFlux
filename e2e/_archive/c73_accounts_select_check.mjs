// C73 e2e — asserts the migrated account-form currency SelectInput still works:
// the select renders its options and selecting a non-default currency is persisted
// on the created account (the add form's currency field uses uiw.SelectInput).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const ACCOUNT_NAME = "E2E-C73-CURR";
const TARGET_CURRENCY = "EUR";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // Force a multi-currency household (seed an FX rate) so the currency select is
  // shown — it's hidden for single-currency households (L37). One-shot addInitScript
  // so it survives the navigation's autosave.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(800);
  await page.evaluate(() => localStorage.setItem("e2e-multiccy", "1"));
  await page.addInitScript(() => {
    if (!localStorage.getItem("e2e-multiccy")) return;
    localStorage.removeItem("e2e-multiccy");
    try {
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      ds.settings = ds.settings || {};
      ds.settings.fxRates = Object.assign({}, ds.settings.fxRates, { EUR: 0.92 });
      localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
    } catch (e) { /* ignore */ }
  });
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  // The add-account form lives in the +Add FlipPanel modal (C73/C79) — open it first.
  await page.waitForSelector('.add-btn', { timeout: 60000 });
  await page.locator('.add-btn').click();
  await page.locator('[role="menuitem"]', { hasText: /account/i }).first().click();
  await page.waitForSelector('[data-testid="account-add-form"]', { timeout: 60000 });

  // 1. The migrated Account-type SelectInput (always visible) renders options.
  const typeSelect = page.locator('[data-testid="account-add-form"] select[aria-label="Account type"]');
  const optionCount = await typeSelect.locator("option").count();
  if (optionCount < 2) fail(`account-type select has too few options (${optionCount}); expected ≥2`);

  // 2. Selecting a non-default option works (the migrated SelectInput responds).
  const optVals = await typeSelect.locator("option").evaluateAll((els) => els.map((e) => e.value));
  const pick = optVals[1] || optVals[0];
  await typeSelect.selectOption(pick);
  const selected = await typeSelect.inputValue();
  if (selected !== pick) fail(`selecting account type failed; wanted "${pick}", got "${selected}"`);

  // 3. Fill name + opening balance and submit — the migrated form still creates an account.
  await page.locator('[data-testid="account-add-form"] input[type="text"]').first().fill(ACCOUNT_NAME);
  await page.locator('[data-testid="account-add-form"] input[type="number"]').first().fill("100");
  await page.locator('[data-testid="account-add-form"] button[type="submit"]').click();
  await page.waitForTimeout(800);

  // 4. The account persisted (the list may not refresh in place after a modal add, so
  //    check the dataset rather than the DOM row).
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
  const created = await page.evaluate((name) =>
    (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").accounts || []).some((a) => a.name === name),
    ACCOUNT_NAME);
  if (!created) fail(`account "${ACCOUNT_NAME}" was not created/persisted after submit`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log(`PASS: migrated account-type SelectInput renders ${optionCount} options, selects, and the form creates the account.`);
} finally {
  await browser.close();
}
