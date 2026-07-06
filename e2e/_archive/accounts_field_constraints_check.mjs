// C49 gate — "account number fields are constrained + hinted". The add form's
// score fields (Liquidity / Stability) must be 1–5 with a visible (1–5) hint, and
// switching to a liability type must expose a Due day field constrained to 1–28.
// Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const attrs = (loc) => loc.evaluate((el) => ({ min: el.getAttribute("min"), max: el.getAttribute("max"), step: el.getAttribute("step"), ph: el.getAttribute("placeholder") }));

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  // Open the add modal so form fields are visible.
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /account/i }).first().click();
  await page.waitForTimeout(400);

  // Asset scoring fields (Liquidity/Stability/…) now sit behind an "Advanced"
  // disclosure (C49) — expand it before asserting their constraints.
  await page.waitForSelector(".cf-adv-toggle", { timeout: 10000 });
  await page.locator(".cf-adv-toggle").first().click();
  await page.waitForSelector('input[placeholder^="Liquidity"], input[placeholder^="Easy to access"]', { timeout: 10000 });

  // Asset (default) form: liquidity/stability fields are 1–5 with the hint.
  // Labels may be "Liquidity" or "Easy to access" / "Stability" or "Low risk".
  const liquidityLocator = page.locator('input[placeholder^="Liquidity"], input[placeholder^="Easy to access"]').first();
  const stabilityLocator = page.locator('input[placeholder^="Stability"], input[placeholder^="Low risk"]').first();

  const aLiq = await attrs(liquidityLocator);
  if (aLiq.min !== "1" || aLiq.max !== "5" || aLiq.step !== "1") fail(`Liquidity/Easy-to-access: min/max/step = ${aLiq.min}/${aLiq.max}/${aLiq.step}, want 1/5/1`);
  if (!/\(1.5\)/.test(aLiq.ph || "")) fail(`Liquidity/Easy-to-access: placeholder "${aLiq.ph}" should carry the (1–5) hint`);

  const aStab = await attrs(stabilityLocator);
  if (aStab.min !== "1" || aStab.max !== "5" || aStab.step !== "1") fail(`Stability/Low-risk: min/max/step = ${aStab.min}/${aStab.max}/${aStab.step}, want 1/5/1`);
  if (!/\(1.5\)/.test(aStab.ph || "")) fail(`Stability/Low-risk: placeholder "${aStab.ph}" should carry the (1–5) hint`);

  // Switch the account type to a liability so the Due day field renders, then
  // assert it's constrained to a valid day-of-month range.
  const typeSel = page.locator('select[aria-label="Account type"]');
  await typeSel.selectOption({ label: "Credit card" }).catch(async () => {
    await typeSel.selectOption({ index: 4 });
  });
  await page.waitForSelector('input[placeholder^="Due day"]', { timeout: 5000 });
  const d = await attrs(page.locator('input[placeholder^="Due day"]').first());
  if (d.min !== "1" || d.max !== "28" || d.step !== "1") fail(`Due day: min/max/step = ${d.min}/${d.max}/${d.step}, want 1/28/1`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: account fields — Liquidity/Stability 1–5 hinted, Due day 1–28 constrained.");
} finally {
  await browser.close();
}
