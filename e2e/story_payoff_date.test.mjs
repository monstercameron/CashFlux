// L5 E2E story - "the debt strategy shows a calendar debt-free date". The plan card
// used to show only a bare month count ("170 months"); it now also reads
// "Debt-free by <Month Year>" and dates each debt in the payoff order. Seeded data
// has liabilities, so the strategy card renders.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
  // Commit an extra monthly amount so the multi-debt plan is viable + diverges.
  await page.waitForSelector('input[aria-label="Extra monthly payment"]', { timeout: 60000 });
  await page.locator('input[aria-label="Extra monthly payment"]').fill("800");
  await page.waitForTimeout(500);
  // The debt-strategy card prints a calendar debt-free date.
  await page.waitForSelector("text=Debt-free by", { timeout: 60000 });
  const dateText = (await page.getByText("Debt-free by", { exact: false }).first().innerText()).replace(/\s+/g, " ");
  // Must name a month + a 4-digit year (e.g. "Aug 2031"), not just a month count.
  if (!/[A-Z][a-z]{2}\s+\d{4}/.test(dateText)) {
    fail(`debt-free line should show a Month Year date, got: ${dateText}`);
  }
  // The payoff order dates each debt.
  const orderText = (await page.getByText("Payoff order:", { exact: false }).first().innerText()).replace(/\s+/g, " ");
  if (!/\([A-Z][a-z]{2}\s+\d{4}\)/.test(orderText)) {
    fail(`payoff order should date each debt, got: ${orderText}`);
  }

  await page.screenshot({ path: path.join(__dirname, "payoff-date.png") });

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: debt strategy shows a calendar debt-free date ("${dateText}") and dates each debt in the order.`);
} finally {
  await browser.close();
}
