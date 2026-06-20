// C63/B15 gate — "the category add form's controls are all labelled". Asserts the
// kind and parent selects carry accessible names (aria-label), closing the
// systemic placeholder/label gap. Exits non-zero on any failure.
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
  await page.waitForSelector("#cat-add", { timeout: 60000 });

  // Every select in the add form must have an accessible name.
  const form = page.locator("form", { has: page.locator("#cat-add") });
  const selects = form.locator("select");
  const n = await selects.count();
  if (n < 2) fail(`expected the add form to have the kind + parent selects, found ${n}`);
  for (let i = 0; i < n; i++) {
    const label = await selects.nth(i).getAttribute("aria-label");
    if (!label || !label.trim()) fail(`add-form select #${i} has no aria-label`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: category add-form selects are all labelled (${n} selects).`);
} finally {
  await browser.close();
}
