// L5 E2E story - "the debt plan shows a balance burn-down chart". With a viable
// plan, the debt card renders an area chart of the remaining total balance falling
// to zero over the payoff timeline.
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

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[aria-label="Extra monthly payment"]', { timeout: 60000 });
  await page.locator('input[aria-label="Extra monthly payment"]').fill("800");
  await page.waitForTimeout(700);

  if ((await page.getByText("Balance burn-down to zero", { exact: false }).count()) === 0) {
    fail("the 'Balance burn-down to zero' heading is missing");
  }
  const chart = page.locator('.cf-chart[aria-label*="Debt balance falling to zero"]');
  if ((await chart.count()) === 0) fail("the burn-down chart (.cf-chart) did not render");
  await page.waitForTimeout(800); // let d3 draw
  await page.screenshot({ path: path.join(__dirname, "payoff-chart.png") });

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: the debt plan renders a balance burn-down chart over the payoff timeline.");
} finally {
  await browser.close();
}
