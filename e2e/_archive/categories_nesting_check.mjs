// C63 gate — "nested categories indent with real spacing, not literal em-dashes".
// Asserts no category row label is prefixed with "— " and that at least one nested
// row carries a real left-padding style. Exits non-zero on any failure.
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

  await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".row-desc", { timeout: 60000 });

  const labels = await page.locator(".row-desc").allInnerTexts();
  for (const t of labels) {
    if (t.trim().startsWith("—")) fail(`row label "${t}" still uses an em-dash prefix; want real indentation`);
  }

  // At least one nested row should carry a left padding (sample has sub-categories).
  const padded = await page.locator(".row-desc").evaluateAll((els) =>
    els.filter((el) => parseFloat(el.style.paddingLeft || "0") > 0).length
  );
  if (padded === 0) fail("expected at least one nested category row with real left padding");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: category tree indents with real spacing (${padded} nested rows padded), no em-dash prefixes.`);
} finally {
  await browser.close();
}
