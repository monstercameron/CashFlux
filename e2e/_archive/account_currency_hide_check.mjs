// L37 gate — currency select hidden for single-currency households.
// Forces a single-currency household (strip FX rates, all accounts in the base
// currency) via a one-shot addInitScript so the dataset the app BOOTS with matches
// what we assert, then opens the +Add → Account modal and asserts the currency
// <select> is absent. (The seeded demo data is multi-currency — it carries FX
// rates — so the hide branch must be exercised with deterministic single-currency
// data.)
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

  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  // Arm a one-shot injection that makes the household single-currency on the next
  // boot: clear FX rates and coerce every account to the base currency. addInitScript
  // re-applies it at document-start so the navigation's autosave can't clobber it.
  await page.evaluate(() => localStorage.setItem("e2e-single-ccy", "1"));
  await page.addInitScript(() => {
    if (!localStorage.getItem("e2e-single-ccy")) return;
    localStorage.removeItem("e2e-single-ccy"); // one-shot
    try {
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      ds.settings = ds.settings || {};
      const base = (ds.settings.baseCurrency || "USD");
      ds.settings.fxRates = {};
      (ds.accounts || []).forEach((a) => { a.currency = base; });
      localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
    } catch (e) { /* ignore */ }
  });

  // Reload so the app boots single-currency, then open the add-account modal.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });
  await page.waitForTimeout(500);
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /account/i }).first().click();
  await page.waitForSelector('[data-testid="account-add-form"]', { timeout: 10000 });
  await page.waitForTimeout(300);

  const currencyCount = await page.locator('[data-testid="account-currency-select"]').count();
  if (currencyCount !== 0) {
    fail("currency select should be hidden for a single-currency household but it is present");
  }

  if (!process.exitCode) console.log("PASS: currency select is hidden for a single-currency household (L37).");
} finally {
  await browser.close();
}
