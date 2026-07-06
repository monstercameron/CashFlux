// L8 — Insights "The Money Question" e2e check.
//
// Navigates to /insights, types a grounded affordability question into the chat
// input, submits it, and asserts that the deterministic affordability card
// appears — without requiring an AI key (affordability questions are routed
// through the pure internal/insights engine, not OpenAI).
//
// Selectors used:
//   #cf-chat-input          — the Ask / chat input field
//   [data-cf="afford-result"] — the grounded affordability answer card
//
// The card is expected to contain either a surplus line or a shortfall line so
// the assertion is amount-agnostic (the balance varies per run).
//
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

const bodyHas = (page, re) =>
  page.evaluate(
    ({ src, flags }) =>
      new RegExp(src, flags).test(document.body.innerText || ""),
    { src: re.source, flags: re.flags }
  );

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // 1. Navigate to Insights.
  await page.goto(BASE + "/insights", { waitUntil: "domcontentloaded" });

  // 2. Wait for the chat input to be ready.
  //    Selector: #cf-chat-input
  await page.locator("#cf-chat-input").waitFor({ timeout: 60000 });

  // 3. Type an affordability question and submit via Enter.
  //    The question matches the ParseAffordQuery grammar:
  //    "can I afford $<amount> in <N> months"
  await page.locator("#cf-chat-input").fill("Can I afford $500 in 3 months?");
  await page.locator("#cf-chat-input").press("Enter");

  // 4. Wait for the grounded affordability card to appear.
  //    Selector: [data-cf="afford-result"]
  await page.locator('[data-cf="afford-result"]').waitFor({ timeout: 10000 });

  // 5. Assert the card contains either a surplus or a shortfall number, and the
  //    projected-available line — so we know real figures were used.
  const cardText = await page.locator('[data-cf="afford-result"]').innerText();
  const hasSurplusOrShortfall =
    /surplus/i.test(cardText) || /shortfall/i.test(cardText);
  if (!hasSurplusOrShortfall) {
    fail('afford-result card is missing a surplus/shortfall line — got: ' + cardText);
  }

  const hasProjected = /projected/i.test(cardText);
  if (!hasProjected) {
    fail('afford-result card is missing the projected-available line — got: ' + cardText);
  }

  // 6. Confirm no JS page errors occurred.
  if (errors.length) fail("page errors: " + errors.join(" | "));

  if (!process.exitCode) {
    console.log(
      "PASS: Insights grounded affordability card appeared with surplus/shortfall + projected figures."
    );
  }
} finally {
  await browser.close();
}
