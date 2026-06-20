// B16 E2E story — "customize: formula evaluates + saves". Types an arithmetic
// expression into the formula calculator and asserts the live result, then saves
// the formula and asserts it persists to the dataset. Exits non-zero on failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const NAME = "ZZFORMULA";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const formulaByName = (page, name) =>
  page.evaluate((n) => {
    const d = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    return (d.formulas || []).find((f) => f.name === n) || null;
  }, name);
async function waitForFormula(page, name, timeoutMs = 7000) {
  let f = null;
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    f = await formulaByName(page, name);
    if (f) return f;
    await page.waitForTimeout(400);
  }
  return f;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/customize", { waitUntil: "domcontentloaded" });
  await page.getByPlaceholder("round((income").waitFor({ timeout: 60000 });

  // Type an arithmetic expression and check the live result.
  await page.getByPlaceholder("round((income").fill("6 * 7");
  await page.waitForTimeout(400);
  const result = (await page.locator(".stat-value").first().textContent())?.trim();
  if (result !== "42") fail(`formula "6 * 7" should evaluate to 42, got "${result}"`);

  // Save the formula and confirm it persists.
  await page.getByPlaceholder("Name this formula").fill(NAME);
  await page
    .locator("form", { has: page.getByPlaceholder("Name this formula") })
    .locator('button[type="submit"]')
    .first()
    .click();
  const saved = await waitForFormula(page, NAME);
  if (!saved) fail("saved formula was not persisted to the dataset");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: formula "6 * 7" = 42 and saved formula "${NAME}" persists.`);
} finally {
  await browser.close();
}
