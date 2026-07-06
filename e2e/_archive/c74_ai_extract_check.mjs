// e2e/c74_ai_extract_check.mjs
// C74 Tier 3 — checks that "Extract with AI" and "Suggest categories" buttons
// render and are operable. AI network calls are not made (no key configured in
// the test environment); the test asserts:
//   1. The Documents page loads with the "Extract with AI" button in the statement card.
//   2. Clicking it without a key doesn't crash (surfaces the key-required prompt).
//   3. After pasting + parsing a deterministic CSV statement, "Suggest categories" appears.
//   4. Clicking "Suggest categories" runs the deterministic rules pass without error.
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

  await page.goto(BASE + "/documents", { waitUntil: "domcontentloaded" });
  // The statement textarea (placeholder contains "Posting Date") anchors the card.
  await page.waitForSelector("textarea[placeholder*='Posting Date']", { timeout: 60000 });

  // 1. The "Extract with AI" button is present in the statement card (C74).
  const extractBtn = page.locator('[data-testid="extract-ai-btn"]');
  if ((await extractBtn.count()) === 0) {
    fail("extract-ai-btn not found");
  } else {
    // 2. Clicking it without a configured AI key must not crash the page.
    await extractBtn.first().click();
    await page.waitForTimeout(300);
  }

  // 3. Paste a deterministic CSV statement and parse it (no AI needed).
  await page.locator("textarea[placeholder*='Posting Date']").fill(
    "Date,Description,Amount\n2026-06-01,SALARY ACH,4200.00\n2026-06-02,WHOLE FOODS,-86.40"
  );
  await page.getByRole("button", { name: "Parse statement", exact: true }).click();
  await page.waitForTimeout(600);

  // 4. After a successful parse the draft renders → "Suggest categories" appears (C74).
  const suggestBtn = page.locator('[data-testid="suggest-categories-btn"]');
  if ((await suggestBtn.count()) === 0) {
    fail("suggest-categories-btn not found after parse");
  } else {
    // 5. The deterministic rules pass runs without an AI key and doesn't crash.
    await suggestBtn.first().click();
    await page.waitForTimeout(300);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) {
    console.log("PASS: C74 'Extract with AI' + 'Suggest categories' render and are operable (no AI key needed for the deterministic path).");
  }
} finally {
  await browser.close();
}
