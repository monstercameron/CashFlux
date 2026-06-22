// L5 E2E story - "the debt strategy suggests a starting extra (useless at $0)". At
// $0 extra snowball and avalanche tie, so the card prompts a sensible extra; one tap
// fills it and the comparison becomes meaningful (a debt-free date appears).
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
  const extra = page.getByLabel(/Extra per month/);
  await extra.waitFor({ timeout: 60000 });

  // With no extra entered, the card prompts a sensible amount.
  if ((await extra.inputValue()) !== "") fail("extra should start empty");
  const suggest = page.locator('button', { hasText: "Try" }).filter({ hasText: "/mo" });
  if ((await suggest.count()) === 0) fail('no "Try $X/mo" suggestion button at $0 extra');

  await page.screenshot({ path: path.join(__dirname, "payoff-suggest.png") });

  // One tap fills the extra and the comparison becomes meaningful.
  await suggest.first().click();
  await page.waitForTimeout(500);
  const filled = await extra.inputValue();
  if (!filled || Number(filled) <= 0) fail(`suggestion did not fill a positive extra, got "${filled}"`);
  await page.waitForSelector("text=Debt-free by", { timeout: 8000 });

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: $0-extra strategy prompts a sensible amount; tapping it fills ${filled} and produces a real payoff plan.`);
} finally {
  await browser.close();
}
