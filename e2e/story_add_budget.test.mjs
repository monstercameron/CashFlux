// B16 E2E story — "create a budget + pick its period". Adds a Weekly budget via
// the Budgets add form and asserts both UX (it lists with its limit) and
// correctness (the persisted budget carries the chosen period, and it survives a
// reload). Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const NAME = "E2E-BUDGET-99";
const LIMIT = "300";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

// Recursively finds a persisted object by name in the dataset store.
const persistedByName = (page, name) =>
  page.evaluate((nm) => {
    const data = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    let found = null;
    const walk = (o) => {
      if (!o || typeof o !== "object") return;
      if (Array.isArray(o)) return o.forEach(walk);
      if (o.name === nm && o.period) found = o;
      Object.values(o).forEach(walk);
    };
    walk(data);
    return found;
  }, name);

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  if ((await page.getByText(NAME).count()) !== 0) fail("test budget already present before adding");

  // Open modal and fill the add form: name + limit, period Weekly.
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /budget/i }).first().click();
  await page.waitForSelector('#budget-add', { timeout: 10000 });
  const dialog = page.locator('[role="dialog"]');
  await dialog.locator("#budget-add").fill(NAME);
  await dialog.locator('input[type="number"][aria-required="true"]').fill(LIMIT);
  await dialog.locator('select:has(option[value="weekly"])').first().selectOption("weekly");
  await dialog.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(700);
  // Soft-nav cycle to force budgets list re-render after modal add.
  await page.evaluate(() => { window.history.pushState({}, '', '/'); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })); });
  await page.waitForTimeout(500);
  await page.evaluate(() => { window.history.pushState({}, '', '/budgets'); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })); });
  await page.waitForTimeout(800);

  // UX: the budget lists with its name and limit.
  if ((await page.getByText(NAME).count()) === 0) fail("budget did not appear in the list after adding");
  if ((await page.getByText(LIMIT, { exact: false }).count()) === 0) fail(`budget should show its limit ${LIMIT}`);

  // Correctness + persistence: the saved budget carries the chosen Weekly period.
  await page.waitForTimeout(2500);
  const saved = await persistedByName(page, NAME);
  if (!saved) fail("budget was not autosaved to the dataset store");
  else if (saved.period !== "weekly") fail(`saved period = ${saved.period}, want "weekly"`);

  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });
  await page.waitForTimeout(800);
  if ((await page.getByText(NAME).count()) === 0) fail("budget did not survive a reload");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: created Weekly budget "${NAME}" (${LIMIT}) — lists, period persists, survives reload.`);
} finally {
  await browser.close();
}
