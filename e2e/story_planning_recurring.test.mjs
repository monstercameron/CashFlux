// B16 E2E story — "planning: add a recurring item". Adds a recurring cash-flow on
// the Planning screen and asserts it lists and persists to the dataset, surviving
// a reload. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const LABEL = "ZZRECUR-1";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const recurringByLabel = (page, label) =>
  page.evaluate((l) => {
    const d = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    return (d.recurring || []).find((r) => r.label === l) || null;
  }, label);
async function waitForRecurring(page, label, timeoutMs = 7000) {
  let r = null;
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    r = await recurringByLabel(page, label);
    if (r) return r;
    await page.waitForTimeout(400);
  }
  return r;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
  // Scope to the recurring form (the one with the "How often" cadence select).
  const form = page.locator("form.form-grid", { has: page.locator('select[aria-label="How often"]') });
  await form.waitFor({ timeout: 60000 });

  if ((await page.getByText(LABEL).count()) !== 0) fail("test recurring item already present before adding");

  await form.getByPlaceholder("Label (e.g. Rent, Salary)").fill(LABEL);
  await form.locator('input[type="number"]').first().fill("50");
  await form.locator('button[type="submit"]').first().click();

  // Lists + persists.
  if ((await page.getByText(LABEL).count()) === 0) {
    await page.waitForTimeout(500);
    if ((await page.getByText(LABEL).count()) === 0) fail("recurring item did not appear in the list after adding");
  }
  const saved = await waitForRecurring(page, LABEL);
  if (!saved) fail("recurring item was not persisted to the dataset");

  // Survives reload.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.locator("form.form-grid", { has: page.locator('select[aria-label="How often"]') }).waitFor({ timeout: 60000 });
  await page.waitForTimeout(600);
  if ((await page.getByText(LABEL).count()) === 0) fail("recurring item did not survive a reload");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: added recurring "${LABEL}" — lists, persists, survives reload.`);
} finally {
  await browser.close();
}
