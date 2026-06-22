// L5 E2E story - "exclude a debt from the payoff plan". Real debt-crusher plans
// target consumer debt; the mortgage is excluded by default and any liability can
// be toggled out. Toggling a debt out drops it from the payoff order and persists.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const DEBT = "Auto Loan"; // a seeded non-mortgage liability, included by default

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
const acctFlag = (d, name) => ((d.accounts || []).find((a) => a.name === name) || {}).includeInPayoff;

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
  await page.getByLabel(/Extra per month/).first().waitFor({ timeout: 60000 });
  await page.getByLabel(/Extra per month/).fill("800");
  await page.waitForTimeout(500);

  // The debt is in the payoff order, and has an include toggle (on by default).
  const orderBefore = (await page.getByText("Payoff order:", { exact: false }).first().innerText());
  if (!orderBefore.includes(DEBT)) fail(`"${DEBT}" should be in the payoff order initially: ${orderBefore}`);
  const toggle = page.locator(`[role="switch"][aria-label="${DEBT}"]`);
  if ((await toggle.count()) === 0) fail(`no include toggle for "${DEBT}"`);
  if ((await toggle.first().getAttribute("aria-checked")) !== "true") fail(`"${DEBT}" should be included by default`);

  // Toggle it out: it leaves the plan and the flag persists as false.
  await toggle.first().click();
  await waitForDataset(page, (d) => acctFlag(d, DEBT) === false);
  await page.waitForTimeout(400);
  const orderAfter = (await page.getByText("Payoff order:", { exact: false }).first().innerText());
  if (orderAfter.includes(DEBT)) fail(`"${DEBT}" should be gone from the payoff order after excluding it: ${orderAfter}`);
  await page.screenshot({ path: path.join(__dirname, "payoff-exclude.png") });

  // Survives reload.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1000);
  const d = await dataset(page);
  if (acctFlag(d, DEBT) !== false) fail(`exclude flag did not survive reload (got ${acctFlag(d, DEBT)})`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: excluding "${DEBT}" drops it from the payoff order and persists across reload.`);
} finally {
  await browser.close();
}
