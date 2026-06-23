// C54 gate — "allocate amount / reserve / max-per inputs are labelled". The
// three split-amount inputs previously had only placeholder text and no
// persistent label or aria-label. Asserts each now has a visible labeledField
// label and a matching aria-label attribute. Exits non-zero on any failure.
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
  await page.waitForSelector(".labeled-field", { timeout: 60000 });

  // Collect all visible labeled-field span texts on the page.
  const texts = (await page.locator(".labeled-field span").allInnerTexts()).map(
    (t) => t.trim()
  );

  for (const want of [
    "Amount to allocate",
    "Emergency buffer",
    "Cap per destination",
  ]) {
    if (!texts.includes(want)) {
      fail(`allocate form should show a visible "${want}" label (saw: ${texts.join(", ")})`);
    }
  }

  // Each input also carries an aria-label matching its visible label.
  for (const label of [
    "Amount to allocate",
    "Emergency buffer",
    "Cap per destination",
  ]) {
    if (
      (await page.locator(`input[aria-label="${label}"]`).count()) === 0
    ) {
      fail(`input missing aria-label="${label}"`);
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log(
      "PASS: allocate amount/reserve/max-per inputs have visible labels + aria-labels."
    );
} finally {
  await browser.close();
}
