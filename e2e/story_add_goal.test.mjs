// B16 E2E story — "create a goal + contribute". Adds a savings goal, contributes
// to it, and asserts both UX (it lists with progress, the contribute flow works)
// and correctness (the contribution advances the saved amount and it persists).
// Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const NAME = "E2E-GOAL-55";
const TARGET = "1000";
const START = "100";
const CONTRIB = "250"; // 100 + 250 = 350 saved

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#goal-add", { timeout: 60000 });
  if ((await page.getByText(NAME).count()) !== 0) fail("test goal already present before adding");

  // Create the goal: name, target (aria-required), and an initial saved amount
  // (the second, non-required number field).
  await page.locator("#goal-add").fill(NAME);
  await page.locator('input[type="number"][aria-required="true"]').fill(TARGET);
  await page.locator('input[type="number"]').nth(1).fill(START);
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(700);

  const row = page.locator(".budget", { hasText: NAME });
  if ((await row.count()) === 0) fail("goal did not appear after adding");

  // Contribute: open the contribute form (the row's first action button), enter
  // an amount, and save.
  await row.getByRole("button").first().click();
  await page.waitForSelector('input[id^="goal-contrib-"]', { timeout: 8000 });
  await page.locator('input[id^="goal-contrib-"]').fill(CONTRIB);
  await page.locator(".budget", { hasText: NAME }).locator('button[type="submit"]').first().click();
  await page.waitForTimeout(700);

  // Correctness: the saved amount advanced to 350 (100 + 250).
  const rowText = (await page.locator(".budget", { hasText: NAME }).first().textContent()) || "";
  if (!rowText.includes("350")) fail(`goal should show advanced progress (350) after contributing, got: ${rowText.replace(/\s+/g, " ").trim()}`);

  // Persist + survive reload.
  await page.waitForTimeout(2500);
  const persisted = await page.evaluate(() => localStorage.getItem("cashflux:dataset") || "");
  if (!persisted.includes(NAME)) fail("goal was not autosaved to the dataset store");
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#goal-add", { timeout: 60000 });
  await page.waitForTimeout(800);
  const afterReload = (await page.locator(".budget", { hasText: NAME }).first().textContent().catch(() => "")) || "";
  if ((await page.getByText(NAME).count()) === 0) fail("goal did not survive a reload");
  if (!afterReload.includes("350")) fail(`contributed amount should persist (350) after reload, got: ${afterReload.replace(/\s+/g, " ").trim()}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: created goal "${NAME}", contributed ${CONTRIB} → 350 saved, persists across reload.`);
} finally {
  await browser.close();
}
