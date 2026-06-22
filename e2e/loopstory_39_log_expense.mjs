// L39 E2E loop story — "Logging Today's Coffee" (everyday expense entry end-to-end).
// Adds a single expense via the Transactions add form, verifies UX feedback
// (row appears in ledger, shows correct amount), confirms autosave to localStorage,
// then checks cross-page consistency: dashboard shows a spending figure, budgets page
// loads, and the transaction survives a reload.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const DESC = "Morning coffee";
const AMOUNT = "5.00";
const SCREENSHOT = (name) => path.join(__dirname, name);

const browser = await chromium.launch({ headless: true });
let passed = 0;
let failed = 0;

const pass = (label) => { console.log(`PASS: ${label}`); passed++; };
const fail = (label) => { console.error(`FAIL: ${label}`); failed++; };

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 800 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // ── Step 1: Navigate to /transactions ────────────────────────────────────────
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForFunction(
    () => document.querySelector("h1")?.textContent?.includes("Transactions"),
    { timeout: 8000 }
  ).catch(() => {});

  const h1Text = await page.evaluate(() => document.querySelector("h1")?.textContent?.trim() ?? "");
  if (h1Text.includes("Transactions")) {
    pass("Step 1 — /transactions loaded with correct h1");
  } else {
    fail(`Step 1 — expected h1 'Transactions', got '${h1Text}'`);
  }

  // ── Step 2: Screenshot before add ────────────────────────────────────────────
  await page.screenshot({ path: SCREENSHOT("loop39-01-transactions-before.png"), fullPage: false });
  pass("Step 2 — screenshot loop39-01-transactions-before.png taken");

  // ── Step 3: Count rows before add ────────────────────────────────────────────
  const rowsBefore = await page.evaluate(() => document.querySelectorAll("tbody tr").length);
  pass(`Step 3 — ledger row count before add: ${rowsBefore}`);

  // ── Step 4: Fill description and amount, submit ───────────────────────────────
  await page.waitForSelector("#txn-add", { timeout: 8000 });
  await page.locator("#txn-add").fill(DESC);
  await page.locator('input[type="number"][aria-required="true"]').fill(AMOUNT);
  await page.locator('form button[type="submit"]').click();

  // ── Step 5: Wait 800ms + screenshot after add ─────────────────────────────────
  await page.waitForTimeout(800);
  await page.screenshot({ path: SCREENSHOT("loop39-02-after-add.png"), fullPage: false });
  pass("Step 5 — screenshot loop39-02-after-add.png taken");

  // ── Step 6: Verify "Morning coffee" and "5.00" appear in ledger ───────────────
  const descCount = await page.getByText(DESC).count();
  if (descCount > 0) {
    pass(`Step 6a — "${DESC}" appeared in ledger`);
  } else {
    fail(`Step 6a — "${DESC}" NOT found in ledger after submit`);
  }

  const amountCount = await page.getByText("5.00", { exact: false }).count();
  if (amountCount > 0) {
    pass(`Step 6b — amount "5.00" visible in ledger`);
  } else {
    fail(`Step 6b — amount "5.00" NOT visible in ledger`);
  }

  const rowsAfter = await page.evaluate(() => document.querySelectorAll("tbody tr").length);
  if (rowsAfter > rowsBefore) {
    pass(`Step 6c — row count increased from ${rowsBefore} → ${rowsAfter}`);
  } else {
    fail(`Step 6c — row count did not increase (before=${rowsBefore}, after=${rowsAfter})`);
  }

  // ── Step 7: Wait 3s for autosave; check localStorage ─────────────────────────
  await page.waitForTimeout(3000);
  const persisted = await page.evaluate(() => localStorage.getItem("cashflux:dataset") || "");
  if (persisted.includes(DESC)) {
    pass(`Step 7 — "${DESC}" found in cashflux:dataset (autosaved)`);
  } else {
    fail(`Step 7 — "${DESC}" NOT found in cashflux:dataset after 3s autosave wait`);
  }

  // ── Step 8: Navigate to /dashboard ───────────────────────────────────────────
  await page.goto(BASE + "/dashboard", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SCREENSHOT("loop39-03-dashboard.png"), fullPage: false });
  pass("Step 8 — screenshot loop39-03-dashboard.png taken");

  // ── Step 9: Check dashboard shows SOME spending figure ($) ───────────────────
  const dashText = await page.evaluate(() => document.body.innerText);
  const hasDollar = /\$[\d,]+(\.\d+)?/.test(dashText);
  if (hasDollar) {
    pass("Step 9 — dashboard shows at least one $ money figure");
  } else {
    fail("Step 9 — dashboard shows NO $ money figure (spending/balance tiles missing)");
  }

  // ── Step 10: Navigate to /budgets ────────────────────────────────────────────
  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  await page.waitForTimeout(1200);
  await page.screenshot({ path: SCREENSHOT("loop39-04-budgets.png"), fullPage: false });
  pass("Step 10 — screenshot loop39-04-budgets.png taken");

  // ── Step 11: Check budgets page loaded ───────────────────────────────────────
  const budgetsLoaded = await page.evaluate(() => {
    const h1 = document.querySelector("h1")?.textContent ?? "";
    const hasBudget = /budget/i.test(document.body.innerText.substring(0, 2000));
    return { h1, hasBudget };
  });
  if (budgetsLoaded.hasBudget) {
    pass(`Step 11 — budgets page loaded (h1: "${budgetsLoaded.h1}")`);
  } else {
    fail(`Step 11 — budgets page missing Budget content (h1: "${budgetsLoaded.h1}")`);
  }

  // ── Step 12: Reload /transactions and verify coffee survived ──────────────────
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  await page.waitForTimeout(1000);

  const survivedCount = await page.getByText(DESC).count();
  if (survivedCount > 0) {
    pass(`Step 12 — "${DESC}" transaction survived page reload`);
  } else {
    fail(`Step 12 — "${DESC}" NOT present after reload (persistence failure)`);
  }

  // ── Page error guard ─────────────────────────────────────────────────────────
  if (errors.length === 0) {
    pass("Page errors — none detected during run");
  } else {
    fail(`Page errors — ${errors.length} JS error(s): ${errors.slice(0, 3).join(" | ")}`);
  }

} finally {
  await browser.close();
  console.log(`\n── Summary: ${passed} passed, ${failed} failed ──`);
  if (failed > 0) process.exit(1);
}
