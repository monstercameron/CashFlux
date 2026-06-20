// C58 gate — "split has select-all and a result summary". Entering an amount and
// selecting all sharers should show a legible "$X split among N → $Y each" line.
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

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // Sample data seeds a single member; add a second so the household has 2+ and
  // the Select-all affordance applies.
  await page.goto(BASE + "/members", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#member-add", { timeout: 60000 });
  await page.locator("#member-add").fill("ZZSPLITMATE");
  await page.locator("form button[type='submit']").first().click();
  await page.waitForTimeout(500);

  await page.goto(BASE + "/split", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="number"]', { timeout: 60000 });

  await page.locator('input[type="number"]').first().fill("100");
  const selectAll = page.getByRole("button", { name: "Select all" });
  if ((await selectAll.count()) === 0) {
    fail("no 'Select all' button (need a household with 2+ members)");
    throw new Error("stop");
  }
  await selectAll.click();
  await page.waitForTimeout(400);

  const summary = await page.getByText(/split among \d+/).count();
  if (summary === 0) fail("expected a 'split among N' summary after selecting all");
  const text = await page.getByText(/split among \d+/).first().innerText();
  if (!/each/.test(text)) fail(`summary should show the per-person 'each' figure, got "${text}"`);

  // Clear removes the selection (and thus the summary).
  await page.getByRole("button", { name: "Clear" }).click();
  await page.waitForTimeout(400);
  if ((await page.getByText(/split among \d+/).count()) !== 0) fail("Clear should remove the summary (no sharers selected)");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: split select-all + summary ("${text.trim()}"); Clear resets it.`);
} finally {
  await browser.close();
}
