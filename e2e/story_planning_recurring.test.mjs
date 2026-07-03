// B16 E2E story — "add a recurring flow". Adds a recurring cash-flow on the
// /recurring Scheduled surface (via the add flip modal) and asserts it lists and
// survives a reload (the persistence proof — the dataset lives in IndexedDB, so
// UI-after-reload is the observable). Exits non-zero on any failure.
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

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/recurring", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".bento-recurring", { timeout: 60000 });

  if ((await page.getByText(LABEL).count()) !== 0) fail("test recurring item already present before adding");

  // The add form lives in a flip modal; open it and wait past the 550ms flip
  // animation (the back face isn't hit-testable mid-flip).
  await page.locator('[data-testid="recurring-add"]').first().click();
  await page.waitForTimeout(800);
  const form = page.locator("form.form-grid", { has: page.locator('select[aria-label="How often"]') });
  await form.waitFor({ timeout: 10000 });

  await form.getByPlaceholder("Label (e.g. Rent, Salary)").fill(LABEL);
  await page.locator('[data-testid="rec-amount"]').fill("50");
  await page.locator('[data-testid="rec-save"]').click();
  await page.waitForTimeout(800);

  // Lists (the modal closes and the flow card appears).
  if ((await page.getByText(LABEL).count()) === 0) {
    await page.waitForTimeout(700);
    if ((await page.getByText(LABEL).count()) === 0) fail("recurring item did not appear in the list after adding");
  }

  // Survives reload — the persistence proof.
  await page.waitForTimeout(1200); // let the autosave flush
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector(".bento-recurring", { timeout: 60000 });
  await page.waitForTimeout(800);
  if ((await page.getByText(LABEL).count()) === 0) fail("recurring item did not survive a reload");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: added recurring "${LABEL}" via the modal — lists and survives reload.`);
} finally {
  await browser.close();
}
