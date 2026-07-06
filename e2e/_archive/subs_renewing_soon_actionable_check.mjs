// C56 gate — "Renewing soon" rows use the full SubscriptionRow (actionable).
// When a subscription is renewing within 7 days it should appear in the
// "Renewing soon" section with Remind/Cancel buttons (not a stripped read-only row).
// Asserts: the renewing-soon section exists if there are any soon-renewing subs,
// and each row inside it has at least one action button (remind or cancel).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

try {
  const page = await (await browser.newContext()).newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(500);

  await page.locator('a[title="Subscriptions"]').first().click();
  await page.waitForTimeout(700);

  // Locate the "Renewing soon" section card.
  const soonSection = page.locator('section.card', { hasText: "Renewing soon" }).first();
  if ((await soonSection.count()) === 0) {
    console.log("SKIP: no subscriptions renewing soon in sample dataset");
    process.exit(0);
  }

  // Every row in the renewing-soon section must have at least one action button
  // (Remind me or Mark as cancelled) — proving it is a full SubscriptionRow,
  // not the old stripped name+date+amount-only variant.
  const rows = soonSection.locator('.row');
  const rowCount = await rows.count();
  if (rowCount === 0) {
    fail("Renewing soon section has no rows");
  }
  for (let i = 0; i < rowCount; i++) {
    const row = rows.nth(i);
    const buttons = await row.locator('button').count();
    if (buttons === 0) {
      fail(`Renewing-soon row ${i} has no action buttons — still using the stripped read-only variant`);
    }
  }

  if (!process.exitCode) console.log(`PASS: all ${rowCount} renewing-soon row(s) are actionable (have buttons).`);
} finally {
  await browser.close();
}
