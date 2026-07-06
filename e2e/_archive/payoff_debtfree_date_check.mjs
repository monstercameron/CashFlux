// L5 gate — "the payoff calculator shows a calendar DEBT-FREE DATE, not just a
// month count." Fills the debt-payoff calculator (balance / APR / payment) and
// asserts the result includes a "Debt-free by <Mon YYYY>" stat alongside the
// months figure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const labelledNumber = (page, re) =>
  page.locator("label", { hasText: re }).locator('input[type="number"]').first();

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("h1, h2", { timeout: 60000 });

  await labelledNumber(page, /Balance owed/i).fill("5000");
  await labelledNumber(page, /APR/i).fill("18");
  await labelledNumber(page, /Monthly payment/i).fill("250");
  await page.waitForTimeout(400);

  // Find the payoff result stat-grid (the one carrying the debt-free date) and
  // assert it shows a "Debt-free by" label with a Mon YYYY value.
  const grids = page.locator(".stat-grid");
  const n = await grids.count();
  let payoffText = "";
  for (let i = 0; i < n; i++) {
    const t = (await grids.nth(i).innerText()).replace(/\s+/g, " ");
    if (/Debt-free by/i.test(t)) { payoffText = t; break; }
  }
  if (!payoffText) fail('no "Debt-free by" stat after filling the payoff calculator');
  else if (!/Debt-free by\s+[A-Z][a-z]{2}\s+20\d{2}/i.test(payoffText)) {
    fail(`"Debt-free by" is shown but without a Mon YYYY date; grid text: ${payoffText}`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: payoff calculator shows a calendar debt-free date beside the month count.");
} finally {
  await browser.close();
}
