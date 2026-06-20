// L5 E2E story - "track payoff progress against a baseline". Starting tracking
// snapshots today's total debt; the card then shows "Paid off $X of $Y (NN%) since
// <date>" and the baseline persists across reloads.
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
const baseline = (d) => (d.settings || {}).payoffBaseline;

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("text=Start tracking progress", { timeout: 60000 });

  await page.getByText("Start tracking progress", { exact: false }).first().click();
  const d = await waitForDataset(page, (dd) => baseline(dd) && baseline(dd).totalOwed > 0);
  if (!baseline(d)) fail("no payoff baseline was saved");

  await page.waitForTimeout(300);
  if ((await page.getByText("Paid off", { exact: false }).count()) === 0) {
    fail("the 'Paid off …' progress strip is not shown after starting tracking");
  }
  await page.screenshot({ path: path.join(__dirname, "payoff-progress.png") });

  // Survives reload: the baseline persists and the strip is still there.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1200);
  const d2 = await dataset(page);
  if (!baseline(d2) || baseline(d2).totalOwed !== baseline(d).totalOwed) {
    fail("the payoff baseline did not survive a reload");
  }
  if ((await page.getByText("Paid off", { exact: false }).count()) === 0) {
    fail("the progress strip did not survive a reload");
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: started payoff tracking (baseline $${(baseline(d).totalOwed / 100).toFixed(2)}); the progress strip shows and persists across reload.`);
} finally {
  await browser.close();
}
