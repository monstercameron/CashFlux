// L4 E2E story - "add a non-base-currency account via the validated picker". The
// account currency is now a <select> (was free text), so an expat can pick EUR/GBP
// without typos. Adds a EUR account, asserts it persists with Currency "EUR", and
// survives a reload.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const NAME = "ZZEUR-ACCT";

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

  // The currency control is a <select> with ISO options (validated picker).
  const curSelect = page.locator('select:has(option[value="EUR"])');
  if ((await curSelect.count()) === 0) fail("currency picker (a <select> with an EUR option) not found");

  await page.locator('input[type="text"][aria-required="true"]').fill(NAME);
  await curSelect.selectOption("EUR");
  await page.locator('input[type="number"]').first().fill("1000");
  await page.locator('button[type="submit"]').first().click();

  const d = await waitForDataset(page, (dd) => (dd.accounts || []).some((a) => a.name === NAME));
  const acct = (d.accounts || []).find((a) => a.name === NAME);
  if (!acct) fail("the EUR account was not saved");
  else if (acct.currency !== "EUR") fail(`account currency = ${acct.currency}, want EUR`);

  await page.screenshot({ path: path.join(__dirname, "account-currency.png") });

  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForTimeout(800);
  const d2 = await dataset(page);
  const acct2 = (d2.accounts || []).find((a) => a.name === NAME);
  if (!acct2 || acct2.currency !== "EUR") fail("the EUR account did not survive a reload");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: added EUR account "${NAME}" via the currency picker; persists across reload.`);
} finally {
  await browser.close();
}
