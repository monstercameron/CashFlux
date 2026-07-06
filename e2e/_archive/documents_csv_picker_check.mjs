// C60 gate — "CSV import: a Choose CSV file button exists on /documents".
// The Documents screen previously accepted CSV only via paste; C60 adds a file
// picker button so real .csv files can be imported without copy-pasting.
// This check asserts the button is present and correctly labelled.
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
  // Wait for the CSV section to render (the account select aria-label is a stable
  // landmark).
  await page.waitForSelector('[aria-label="Import into account"]', { timeout: 60000 });

  // Assert the file-picker button is present by its data-testid.
  const picker = page.locator('[data-testid="csv-file-picker"]');
  if ((await picker.count()) === 0) {
    fail('expected a [data-testid="csv-file-picker"] button but found none');
  }

  // Also assert by visible text for belt-and-suspenders.
  const byText = page.getByRole("button", { name: /choose csv file/i });
  if ((await byText.count()) === 0) {
    fail('expected a button labelled "Choose CSV file" but found none');
  }

  // The paste textarea must still exist (paste path should not be removed).
  const textarea = page.locator("textarea[placeholder*='date,payee,amount']");
  if ((await textarea.count()) === 0) {
    fail("expected the CSV paste textarea to still exist alongside the file picker");
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: Choose CSV file button is present on /documents; paste textarea is preserved.");
} finally {
  await browser.close();
}
