// L57/L30 gate — reconcile "Update balance" delta preview + category field.
// Verifies: (a) the inline "Update balance" form shows a live delta-preview
// element once a parseable value is typed; (b) a category picker is present;
// (c) saving posts the adjustment transaction with the chosen category; (d) the
// form has a unique id so selectors resolve unambiguously (L44 fix).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const txns = (page) => page.evaluate(() =>
  JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []);
async function flush(page) {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
}

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });

  // Open the ⋯ menu and click "Update balance" on the first non-archived account.
  await page.waitForSelector('.row [aria-haspopup="menu"]', { timeout: 60000 });
  await page.locator('.row [aria-haspopup="menu"]').first().click();
  await page.waitForTimeout(200);

  // The overflow menu button label for "Update balance" comes from i18n key
  // accounts.updateBalance.  Find it inside the open menu.
  const updateBtn = page.locator('[role="menu"] [role="menuitem"]').filter({ hasText: /update balance/i });
  if ((await updateBtn.count()) === 0) { fail("Update balance menu item not found"); process.exit(1); }
  await updateBtn.first().click();
  await page.waitForTimeout(300);

  // L44: the form must have a unique id starting with "acct-setbal-form-".
  const form = page.locator('[id^="acct-setbal-form-"]');
  if ((await form.count()) === 0) { fail("Update balance form missing id=acct-setbal-form-*"); }

  // The balance input is within this form.
  const balInput = page.locator('input[id^="acct-setbal-"]').first();

  // Type a value — any value that differs from the current balance — to trigger
  // the delta preview.  Use a very large number so it's virtually guaranteed to
  // differ from the actual current balance.
  await balInput.fill("999999.99");
  await page.waitForTimeout(300);

  // L57/L30: delta preview element must appear.
  const preview = page.locator('[data-testid="setbal-delta-preview"]');
  if ((await preview.count()) === 0) {
    fail("delta preview element not shown after typing a new balance");
  }

  // L57/L30: category picker must be present.
  const catSelect = page.locator('[data-testid="setbal-cat-select"]');
  if ((await catSelect.count()) === 0) { fail("category picker (setbal-cat-select) not found"); }

  // Pick the first non-empty category.
  const catOptions = await catSelect.locator("option").all();
  let catVal = "";
  for (const opt of catOptions) {
    const v = await opt.getAttribute("value");
    if (v && v.length > 0) { catVal = v; break; }
  }
  if (catVal) {
    await catSelect.selectOption(catVal);
  }

  const txnsBefore = await txns(page);

  // Submit the form.
  await form.locator('button[type="submit"]').first().click();
  await flush(page);

  // An adjustment transaction must appear.
  let all = await txns(page);
  for (let i = 0; i < 10 && all.length <= txnsBefore.length; i++) { await flush(page); all = await txns(page); }

  const adj = all.find((t) =>
    !txnsBefore.find((p) => p.id === t.id) &&
    t.desc && t.desc.toLowerCase().includes("adjustment")
  );
  if (!adj) { fail("adjustment transaction not found after saving Update balance"); }

  // If a category was selected, verify it is set on the adjustment.
  if (catVal && adj && adj.categoryId !== catVal) {
    fail(`adjustment category: expected ${catVal}, got ${adj && adj.categoryId}`);
  }

  if (!process.exitCode) {
    console.log(`PASS: Update balance — delta preview shown, category picker present, adjustment posted` +
      (catVal ? ` with category ${catVal}` : " (no categories available to assign)") + ".");
  }
} finally {
  await browser.close();
}
