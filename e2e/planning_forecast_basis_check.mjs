// L27 gate — forecast hint shows trailing-average basis text.
//
// The 12-month net-worth forecast on /planning was updated (L27) to use a
// trailing 3-month average of net cash flow rather than the current month's
// figure, and the hint text was updated to reflect this.  This gate asserts:
//   1. The forecast card is present.
//   2. The hint text no longer says "this month's net cash flow" (old wording).
//   3. The hint text says "last 3 months" (new wording).
//   4. The data-testid="forecast-basis" element is present and reads
//      "3-month trailing average".
//
// Selectors:
//   section.card:has-text("Net worth in 12 months")   — the forecast card
//   [data-testid="forecast-basis"]                     — the basis label
//
// Exits non-zero on any assertion failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // Navigate to the planning screen.
  await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('section.card', { timeout: 60000 });
  await page.waitForTimeout(600);

  // 1. The forecast card must be present.
  const forecastCard = page.locator('section.card', { hasText: "Net worth in 12 months" });
  if ((await forecastCard.count()) === 0) {
    fail("forecast card ('Net worth in 12 months') not found on /planning");
    process.exit(1);
  }
  console.log("  PASS: forecast card present");

  // 2. The hint must NOT say "this month's net cash flow" (old, incorrect wording).
  const cardText = await forecastCard.first().innerText();
  if (/this month'?s net cash flow/i.test(cardText)) {
    fail(`hint still says "this month's net cash flow" — old wording not updated.\nCard text snippet: ${cardText.slice(0, 200)}`);
  } else {
    console.log("  PASS: old 'this month's net cash flow' wording absent");
  }

  // 3. The hint must say "last 3 months" (new trailing-average wording).
  if (!/last 3 months/i.test(cardText)) {
    fail(`hint does not say "last 3 months" — new trailing-average wording missing.\nCard text snippet: ${cardText.slice(0, 200)}`);
  } else {
    console.log("  PASS: 'last 3 months' wording present in forecast hint");
  }

  // 4. The basis label element must be present.
  const basisEl = page.locator('[data-testid="forecast-basis"]');
  if ((await basisEl.count()) === 0) {
    fail("forecast-basis element ([data-testid=\"forecast-basis\"]) not found");
  } else {
    const basisText = await basisEl.first().innerText();
    if (!/3.month trailing average/i.test(basisText)) {
      fail(`forecast-basis text does not say "3-month trailing average"; got: "${basisText}"`);
    } else {
      console.log(`  PASS: forecast-basis element shows "${basisText}"`);
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: planning forecast trailing-average basis text correct.");
} finally {
  await browser.close();
}
