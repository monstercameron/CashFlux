// L8 E2E story - "suggested starter questions beat the blank box". The Insights
// Ask section offers tappable starter questions (tailored to the user's top spend
// category); tapping one fills the question box.
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

  await page.goto(BASE + "/insights", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.waitForTimeout(700);

  // Starter chips render and one is tailored to a spend category.
  const chips = page.locator("button.chip-suggest");
  const n = await chips.count();
  if (n < 2) fail(`expected at least 2 starter-question chips, got ${n}`);
  const texts = await chips.allInnerTexts();
  if (!texts.some((t) => /last month\?$/.test(t.trim()))) {
    fail(`no recognizable starter question among: ${JSON.stringify(texts)}`);
  }

  await page.screenshot({ path: path.join(__dirname, "insights-starters.png") });

  // Tapping a chip fills the Ask box (the box reflects the picked question even on
  // the no-key preview path).
  const pick = texts[0].trim();
  await chips.first().click();
  await page.waitForTimeout(300);
  const val = await page.evaluate(() => {
    const inputs = [...document.querySelectorAll("input.field-wide")];
    const filled = inputs.map((i) => i.value).filter(Boolean);
    return filled[0] || "";
  });
  if (val.trim() !== pick) fail(`tapping a chip should fill the Ask box with "${pick}", box has "${val}"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: ${n} starter-question chips render; tapping one fills the Ask box ("${pick}").`);
} finally {
  await browser.close();
}
