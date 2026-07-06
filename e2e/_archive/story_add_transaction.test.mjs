// B16 E2E story — "add a transaction" (the canonical journey). Logs an expense
// via the Transactions screen's add form the standard way, and asserts both UX
// (the flow is reachable and completes) and correctness (the transaction shows in
// the ledger with its amount, persists to the dataset store, and survives a
// reload). Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const DESC = "E2E-ADD-TXN-7271";
const AMOUNT = "12.34";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};
try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForFunction(() => document.querySelector("h1")?.textContent?.includes("Transactions"), { timeout: 8000 }).catch(() => {});

  // Precondition: our unique description isn't in the ledger yet.
  if ((await page.getByText(DESC).count()) !== 0) fail("test description already present before adding");

  // Fill the Transactions add form the standard way: description (#txn-add) and
  // amount. The account defaults to the first account and the date to today, so
  // the standard path needs only these two fields for an expense.
  await page.waitForSelector("#txn-add", { timeout: 8000 });
  await page.locator("#txn-add").fill(DESC);
  await page.locator('input[type="number"][aria-required="true"]').fill(AMOUNT);
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(600);

  // UX + display correctness: the row now shows in the ledger with its amount.
  if ((await page.getByText(DESC).count()) === 0) fail("transaction did not appear in the ledger after save");
  if ((await page.getByText(AMOUNT, { exact: false }).count()) === 0) fail(`ledger should show the amount ${AMOUNT}`);

  // Persistence to the dataset store (autosave is on a short ticker + pagehide).
  await page.waitForTimeout(2500);
  const persisted = await page.evaluate(() => localStorage.getItem("cashflux:dataset") || "");
  if (!persisted.includes(DESC)) fail("transaction was not autosaved to the dataset store");

  // Survives a reload.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(800);
  if ((await page.getByText(DESC).count()) === 0) fail("transaction did not survive a reload");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: added "${DESC}" (${AMOUNT}) — shows in ledger, persists, survives reload.`);
} finally {
  await browser.close();
}
