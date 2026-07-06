// C65 gate — "Workflows: condition variable reference / insert affordance appears
// near the condition input on /workflows". The condition input was previously
// placeholder-only with no guidance; C65 adds an inline variable-reference
// section with click-to-insert buttons and example conditions.
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

  await page.goto(BASE + "/workflows", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('[aria-label="Condition (optional)"]', { timeout: 60000 });

  // Assert all four variable-reference buttons are present (one per condition var).
  const vars = ["txn_abs", "txn_amount", "txn_payee", "txn_category"];
  for (const v of vars) {
    const btn = page.locator(`[data-testid="cond-var-${v}"]`);
    if ((await btn.count()) === 0) {
      fail(`expected a cond-var button for "${v}" but found none`);
    }
  }

  // Assert the hint text ("Available variables") is visible.
  const hint = page.locator("text=/available variables/i");
  if ((await hint.count()) === 0) {
    fail('expected "Available variables" hint text near the condition input');
  }

  // Assert the examples line is visible.
  const examples = page.locator("text=/txn_abs > 200/i");
  if ((await examples.count()) === 0) {
    fail('expected example condition hint (e.g. "txn_abs > 200") but found none');
  }

  // Click txn_abs and assert it inserts into the condition input.
  await page.locator('[data-testid="cond-var-txn_abs"]').click();
  await page.waitForTimeout(200);
  const condValue = await page.locator('[aria-label="Condition (optional)"]').inputValue();
  if (!condValue.includes("txn_abs")) {
    fail(`clicking txn_abs button did not insert "txn_abs" into the condition input; got: "${condValue}"`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: workflow condition variable reference is present; insert-on-click works.");
} finally {
  await browser.close();
}
