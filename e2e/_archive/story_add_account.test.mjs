// B16 E2E story — "add an account". Adds an asset account with an opening
// balance via the Accounts add form and asserts both UX (the flow completes) and
// correctness (the account shows in the list, the net-worth summary rises by the
// opening balance, and it survives a reload). Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const NAME = "E2E-ACCT-3344";
const OPENING = 5000;

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

// Reads the "Net worth" summary figure as a number.
const netWorth = (page) =>
  page.evaluate(() => {
    const stats = [...document.querySelectorAll(".stat")];
    const s = stats.find((el) => /net worth/i.test(el.querySelector(".stat-label")?.textContent || ""));
    if (!s) return null;
    const n = parseFloat((s.querySelector(".stat-value")?.textContent || "").replace(/[^0-9.-]/g, ""));
    return Number.isNaN(n) ? null : n;
  });

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="text"][aria-required="true"]', { timeout: 60000 });

  if ((await page.getByText(NAME).count()) !== 0) fail("test account already present before adding");
  const before = await netWorth(page);
  if (before === null) fail("could not read the net-worth summary");

  // Fill the add form: name + opening balance. Type defaults to a checking asset,
  // currency to USD, owner to the household — the standard path for an asset.
  await page.locator('input[type="text"][aria-required="true"]').fill(NAME);
  await page.locator('input[type="number"]').first().fill(String(OPENING));
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(700);

  // The account appears in the list, and net worth rose by the opening balance.
  if ((await page.getByText(NAME).count()) === 0) fail("account did not appear in the list after adding");
  const after = await netWorth(page);
  if (after === null) fail("could not read net worth after adding");
  else if (Math.abs(after - before - OPENING) > 1) fail(`net worth should rise by ${OPENING}: ${before} → ${after}`);

  // Persist + survive reload.
  await page.waitForTimeout(2500);
  const persisted = await page.evaluate(() => localStorage.getItem("cashflux:dataset") || "");
  if (!persisted.includes(NAME)) fail("account was not autosaved to the dataset store");
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="text"][aria-required="true"]', { timeout: 60000 });
  await page.waitForTimeout(800);
  if ((await page.getByText(NAME).count()) === 0) fail("account did not survive a reload");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: added asset "${NAME}" (+${OPENING}) — shows in list, net worth ${before} → ${after}, persists.`);
} finally {
  await browser.close();
}
