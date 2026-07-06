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
  await page.waitForSelector(".bento-allocate", { timeout: 60000 });

  // The emergency-buffer / cap-per-destination inputs (and their labeled-field labels) live in
  // the "Adjust strategy" flip modal (a redesign UX change), so open it first.
  const edit = page.locator('[data-testid="allocate-edit-strategy"]');
  if (await edit.count()) { await edit.click({ force: true }); await page.waitForTimeout(600); }
  await page.waitForSelector(".labeled-field", { timeout: 10000 }).catch(() => {});

  // Visible labeled-field labels for the two advanced amount inputs.
  const texts = (await page.locator(".labeled-field span").allInnerTexts()).map((t) => t.trim());
  for (const want of ["Emergency buffer", "Cap per destination"]) {
    if (!texts.includes(want)) {
      fail(`allocate advanced form should show a visible "${want}" label (saw: ${texts.join(", ")})`);
    }
  }

  // Every amount input carries an aria-label (the hero amount is labelled via its caption +
  // aria-label rather than a labeled-field).
  for (const label of ["Amount to allocate", "Emergency buffer", "Cap per destination"]) {
    if ((await page.locator(`input[aria-label="${label}"]`).count()) === 0) {
      fail(`input missing aria-label="${label}"`);
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log("PASS: allocate amount/reserve/max-per inputs have accessible labels.");
} finally {
  await browser.close();
}
