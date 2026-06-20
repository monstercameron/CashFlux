// L4 E2E story - "net worth never silently miscomputes a missing FX rate". Adds an
// account in a currency that has no exchange rate; the accounts net-worth total must
// EXCLUDE it with a visible notice (not collapse the whole figure to zero or treat
// the balance as base). Determinism/explainability rule.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const NAME = "ZZNORATE-ACCT";
const CANDIDATES = ["GBP", "EUR", "JPY", "CNY", "INR", "MXN", "CHF", "CAD", "AUD"];

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const dataset = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
async function waitForDataset(page, pred, timeoutMs = 8000) {
  let d = {};
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    d = await dataset(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return d;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="text"][aria-required="true"]', { timeout: 60000 });

  // Pick a currency the seeded FX table has no rate for.
  const d0 = await dataset(page);
  const fx = (d0.settings && d0.settings.fxRates) || {};
  const base = (d0.settings && d0.settings.baseCurrency) || "USD";
  const cur = CANDIDATES.find((c) => c !== base && !(c in fx));
  if (!cur) fail("no candidate currency without a rate (sample FX table changed)");
  console.log(`using rate-less currency: ${cur}`);

  await page.locator('input[type="text"][aria-required="true"]').fill(NAME);
  await page.locator(`select:has(option[value="${cur}"])`).selectOption(cur);
  await page.locator('input[type="number"]').first().fill("1000");
  await page.locator('button[type="submit"]').first().click();
  await waitForDataset(page, (dd) => (dd.accounts || []).some((a) => a.name === NAME));
  await page.waitForTimeout(400);

  // The accounts page must warn that the total excludes the rate-less account.
  const body = (await page.locator(".stat-grid").first().locator("xpath=..").innerText()).replace(/\s+/g, " ");
  if (!body.includes("no " + cur + " rate") && !body.includes(cur)) {
    fail(`expected a missing-rate notice mentioning ${cur}, page text: ${body.slice(0, 400)}`);
  }
  const noticed = await page.getByText("Net worth excludes", { exact: false }).count();
  if (noticed === 0) fail("the 'Net worth excludes …' notice is not shown");
  await page.screenshot({ path: path.join(__dirname, "networth-missing-rate.png") });

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: a ${cur} account with no rate is excluded from net worth with a visible notice (not silently zeroed).`);
} finally {
  await browser.close();
}
