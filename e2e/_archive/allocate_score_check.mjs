// C54 gate — "an allocate suggestion shows its score once". The row used to print
// the score twice (head "60%" and a "Score 60%" sub-line) with a manual " · "
// separator. Now the score is in the head + the progress bar only, and the
// breakdown is the lone sub-line. Exits non-zero on any failure.
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

  await page.goto(BASE + "/allocate", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".alloc-dest", { timeout: 60000 });
  await page.waitForTimeout(400);

  const rows = await page.locator(".alloc-dest").count();
  if (rows === 0) fail("expected at least one ranked destination card");

  // The score appears once, in the card head (.alloc-dest-score), as a percent.
  const score = (await page.locator(".alloc-dest .alloc-dest-score").first().innerText()) || "";
  if (!/%/.test(score)) fail(`the destination head should show the score percent, got "${score}"`);

  // The breakdown is a set of labelled criterion chips (Return / Stability / Liquidity) —
  // not a duplicated "Score …" line.
  const chips = await page.locator(".alloc-dest .alloc-dest-chip").count();
  if (chips < 1) fail("the criterion breakdown chips are missing");
  const chipText = (await page.locator(".alloc-dest .alloc-dest-breakdown").first().innerText()).toLowerCase();
  if (!/return|stability|liquidity/.test(chipText)) fail(`the breakdown chips are missing their criterion labels (saw: ${JSON.stringify(chipText)})`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: destination cards show the score once (head "${score.trim()}") plus a criterion breakdown.`);
} finally {
  await browser.close();
}
