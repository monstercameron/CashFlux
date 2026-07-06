// C61 gate — "Customize formats numbers instead of raw floats". The variables
// reference and formula results used to print raw floats (354070); they now
// thousands-separate (354,070). Exits non-zero on any failure.
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

  await page.goto(BASE + "/customize", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".amount.fig", { timeout: 60000 });
  await page.waitForTimeout(300);

  const vals = (await page.locator(".amount.fig").allInnerTexts()).map((t) => t.trim());
  // At least one variable value is a 4+ digit figure and must be comma-grouped.
  const grouped = vals.some((v) => /\d,\d{3}/.test(v));
  const rawLong = vals.find((v) => /(?<!,)\b\d{4,}(\.\d+)?\b/.test(v) && !/,/.test(v));
  if (!grouped) fail(`expected a thousands-grouped variable value, saw: ${vals.join(", ")}`);
  if (rawLong) fail(`a raw ungrouped 4+ digit value remains: "${rawLong}"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: Customize variables are thousands-grouped (e.g. ${vals.find((v) => /\d,\d{3}/.test(v))}).`);
} finally {
  await browser.close();
}
