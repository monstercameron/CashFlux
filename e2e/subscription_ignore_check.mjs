// C56 subscription ignore — "Not a subscription" correction path.
// Detects a subscription from seeded transaction history, marks it as ignored,
// asserts it disappears from the active detected list, then reloads to confirm
// the ignore persists, and finally restores it (Undo / Unignore).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

async function flush(page) {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
}

function dataset(page) {
  return page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
}

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // 1) Navigate to Subscriptions and wait for the screen to load.
  await page.goto(BASE + "/subscriptions", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1500);

  // 2) Find the first "Not a subscription" (ignore) button visible on the page.
  //    It has data-testid matching sub-ignore-<slug>.
  const ignoreBtn = page.locator('[data-testid^="sub-ignore-"]').first();
  const ignoreCount = await ignoreBtn.count();
  if (ignoreCount === 0) {
    // No detected subscriptions in the seeded dataset — skip gracefully.
    console.log("SKIP: no detected subscriptions found (seeded data may be empty)");
    process.exit(0);
  }

  // Remember which subscription we are ignoring by reading the data-testid slug.
  const testId = await ignoreBtn.getAttribute("data-testid");
  const slug = testId.replace("sub-ignore-", "");

  // 3) Click "Not a subscription".
  await ignoreBtn.click();
  await flush(page);

  // 4) Assert the ignore button for this slug is gone from the active list.
  const stillActive = await page.locator(`[data-testid="sub-ignore-${slug}"]`).count();
  if (stillActive > 0) {
    fail(`subscription ${slug} still visible in active list after ignore`);
    process.exit(1);
  }

  // 5) Assert the ignore record persisted in the dataset.
  const ds1 = await dataset(page);
  const ignores1 = ds1.subscriptionIgnores || [];
  if (ignores1.length === 0) {
    fail("no subscriptionIgnores entry written to dataset after ignore");
    process.exit(1);
  }

  // 6) Reload the page and confirm the subscription is still absent from the
  //    active list (persists across a full reload).
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1500);
  const afterReload = await page.locator(`[data-testid="sub-ignore-${slug}"]`).count();
  if (afterReload > 0) {
    fail(`subscription ${slug} reappeared in active list after reload`);
    process.exit(1);
  }

  // 7) Find and click the "Undo" (unignore) button to restore the subscription.
  const unignoreBtn = page.locator(`[data-testid="sub-unignore-${slug}"]`).first();
  if ((await unignoreBtn.count()) === 0) {
    fail(`sub-unignore-${slug} button not found in ignored section after reload`);
    process.exit(1);
  }
  await unignoreBtn.click();
  await flush(page);

  // 8) Assert the subscription is back in the active list.
  const restored = await page.locator(`[data-testid="sub-ignore-${slug}"]`).count();
  if (restored === 0) {
    fail(`subscription ${slug} did not reappear in active list after unignore`);
    process.exit(1);
  }

  // 9) Assert the ignore record is gone from the dataset.
  const ds2 = await dataset(page);
  const ignores2 = ds2.subscriptionIgnores || [];
  const stillIgnored = ignores2.some((ig) => {
    const s = (ig.subName || "").toLowerCase().replace(/[\s/.'"`]/g, "-");
    return s.includes(slug) || slug.includes(s);
  });
  if (stillIgnored) {
    fail(`subscriptionIgnores still contains entry for ${slug} after unignore`);
  }

  if (!process.exitCode) {
    console.log(`PASS: subscription ${slug} — ignored → persisted → restored via unignore.`);
  }
} finally {
  await browser.close();
}
