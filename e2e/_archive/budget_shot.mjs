// One-off visual check for the L1 budget sub-line fix: boot /budgets (seeded) and
// screenshot the budget rows so the status / pace / rollover / envelope sub-lines
// each sit on their own line (no longer glued together). Writes e2e/budgets.png.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1280, height: 1400 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".budget .budget-sub", { timeout: 60000 });
  await page.waitForTimeout(400);

  await page.screenshot({ path: path.join(__dirname, "budgets.png") });
  console.log("budget-sub count:", await page.locator(".budget .budget-sub").count());
  console.log("page errors:", errors.length ? errors.join(" | ") : "none");
} finally {
  await browser.close();
}
