// B16 E2E story — "transfer is excluded from income/expense totals". Adds two
// accounts, transfers between them, and asserts both the mechanics (a paired
// transfer is created — two legs flagged as transfers) and the key correctness
// property (the dashboard Income and Spending KPIs do NOT change, because
// transfers are excluded from totals). Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const A = "ZZXFER-A";
const B = "ZZXFER-B";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const dataset = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
async function waitForDataset(page, pred, timeoutMs = 7000) {
  let d = {};
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    d = await dataset(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return d;
}
const railTo = (page, title) => page.locator(`nav[aria-label="Main navigation"] a[title="${title}"]`).click();
const transferLegs = (d) => (d.transactions || []).filter((t) => t.transferAccountId).length;
const kpi = (page, id) =>
  page.evaluate((wid) => {
    const el = document.querySelector(`[data-widget="${wid}"] .fig`);
    return el ? el.textContent.replace(/[^0-9.-]/g, "") : null;
  }, id);

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // Two same-currency accounts to transfer between (USD is the default).
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="text"][aria-required="true"]', { timeout: 60000 });
  for (const name of [A, B]) {
    await page.locator('input[type="text"][aria-required="true"]').fill(name);
    await page.locator('button[type="submit"]').first().click();
    await page.waitForTimeout(500);
  }

  // Baseline: dashboard Income + Spending KPIs, and the transfer-leg count.
  await railTo(page, "Dashboard");
  await page.waitForSelector('[data-widget="kpi-income"] .fig', { timeout: 8000 });
  const incBefore = await kpi(page, "kpi-income");
  const spBefore = await kpi(page, "kpi-spending");
  const d0 = await waitForDataset(page, (d) => (d.accounts || []).some((a) => a.name === B));
  const legsBefore = transferLegs(d0);

  // Add a transfer A -> B.
  await railTo(page, "Transactions");
  await page.waitForSelector("#txn-add", { timeout: 8000 });
  await page.locator('select:has(option[value="Transfer"])').first().selectOption("Transfer");
  await page.waitForSelector('select[aria-label="To account"]', { timeout: 8000 });
  await page.locator('input[type="number"][aria-required="true"]').fill("123.45");
  await page.locator('select[aria-label="From account"]').selectOption({ label: A });
  await page.locator('select[aria-label="To account"]').selectOption({ label: B });
  await page.locator('button[type="submit"]').first().click();

  // Mechanics: two transfer legs were created.
  const d1 = await waitForDataset(page, (d) => transferLegs(d) >= legsBefore + 2);
  if (transferLegs(d1) !== legsBefore + 2) fail(`expected 2 new transfer legs, got ${transferLegs(d1) - legsBefore}`);

  // Correctness: transfers are excluded — the Income/Spending KPIs are unchanged.
  await railTo(page, "Dashboard");
  await page.waitForSelector('[data-widget="kpi-income"] .fig', { timeout: 8000 });
  const incAfter = await kpi(page, "kpi-income");
  const spAfter = await kpi(page, "kpi-spending");
  if (incAfter !== incBefore) fail(`Income KPI changed after a transfer: ${incBefore} -> ${incAfter}`);
  if (spAfter !== spBefore) fail(`Spending KPI changed after a transfer: ${spBefore} -> ${spAfter}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: transfer ${A}->${B} created 2 legs; Income/Spending unchanged (excluded from totals).`);
} finally {
  await browser.close();
}
