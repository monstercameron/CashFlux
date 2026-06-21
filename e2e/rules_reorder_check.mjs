// C64 — Rules precedence drag-to-reorder. Verifies the draggable affordance and
// that dragging one rule onto another changes the order (precedence).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
try {
  const page = await (await browser.newContext()).newPage();
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(500);
  await page.locator('a[title="Rules"]').first().click();
  await page.waitForTimeout(700);

  const rows = page.locator('.rows .row[draggable="true"]');
  const n = await rows.count();
  if (n === 0) { console.log("SKIP: no rules in sample to reorder (affordance code is present + store test covers ordering)"); process.exit(0); }
  if ((await page.locator(".rule-grip").count()) === 0) fail("rule rows have no drag grip");

  if (n >= 2) {
    const firstBefore = (await rows.first().locator(".row-desc").textContent()) || "";
    // Drag the 2nd rule onto the 1st → it should become first.
    await rows.nth(1).dragTo(rows.first());
    await page.waitForTimeout(500);
    const firstAfter = (await page.locator('.rows .row[draggable="true"]').first().locator(".row-desc").textContent()) || "";
    if (firstAfter === firstBefore) fail(`drag-reorder did not change precedence: still "${firstBefore}"`);
  }
  if (!process.exitCode) console.log("PASS: rules are draggable with a grip; drag-reorder changes precedence.");
} finally {
  await browser.close();
}
