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
  // Asset scoring fields (Liquidity/Stability/…) now sit behind an "Advanced"
  // disclosure (C49) — expand it before asserting their constraints.
  await page.waitForSelector(".cf-adv-toggle", { timeout: 60000 });
  await page.locator(".cf-adv-toggle").first().click();
  await page.waitForSelector('input[placeholder^="Liquidity"]', { timeout: 60000 });

  // Asset (default) form: Liquidity / Stability are 1–5 with the hint.
  for (const name of ["Liquidity", "Stability"]) {
    const a = await attrs(page.locator(`input[placeholder^="${name}"]`).first());
    if (a.min !== "1" || a.max !== "5" || a.step !== "1") fail(`${name}: min/max/step = ${a.min}/${a.max}/${a.step}, want 1/5/1`);
    if (!/\(1–5\)/.test(a.ph || "")) fail(`${name}: placeholder "${a.ph}" should carry the (1–5) hint`);
  }

  // Switch the account type to a liability so the Due day field renders, then
  // assert it's constrained to a valid day-of-month range.
  const typeSel = page.locator("select").first();
  await typeSel.selectOption({ label: "Credit card" }).catch(async () => {
    // Fall back to the first liability-ish option by value if the label differs.
    await typeSel.selectOption({ index: 1 });
  });
  await page.waitForSelector('input[placeholder^="Due day"]', { timeout: 5000 });
  const d = await attrs(page.locator('input[placeholder^="Due day"]').first());
  if (d.min !== "1" || d.max !== "28" || d.step !== "1") fail(`Due day: min/max/step = ${d.min}/${d.max}/${d.step}, want 1/28/1`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: account fields — Liquidity/Stability 1–5 hinted, Due day 1–28 constrained.");
} finally {
  await browser.close();
}
