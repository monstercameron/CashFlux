// L13 gate — runway warning + suggested action. Adds a large recurring outflow
// (mirrors runway_check) to force the projected liquid balance below the buffer,
// then asserts the runway card shows BOTH the dip warning and a suggested-action
// line (naming a source account + amount, or the no-source note).
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
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));
  await page.goto(BASE + "/planning", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[placeholder^="Label (e.g."]', { timeout: 60000 });

  // Force a breach: a large recurring outflow drives the projected balance under buffer.
  const recForm = page.locator("form").filter({ has: page.locator('input[placeholder^="Label (e.g."]') });
  await recForm.locator('input[placeholder^="Label (e.g."]').fill("L13 breach test");
  await recForm.getByLabel(/Amount/).fill("-99999999");
  await recForm.locator('button[type=submit]').click();
  await page.waitForTimeout(500);

  // Dip warning appears.
  const breach = page.locator('[data-testid="runway-breach"]');
  await breach.first().waitFor({ state: "visible", timeout: 10000 }).catch(() => fail("no runway dip warning after a large outflow"));

  // Suggested-action line appears beside it.
  const suggest = page.locator('[data-testid="runway-suggest"]');
  if ((await suggest.count()) === 0) fail("no suggested-action line beside the runway dip warning");
  else {
    const txt = (await suggest.first().innerText().catch(() => "")) || "";
    if (!/move|\$|no liquid account|delay/i.test(txt)) fail(`suggestion text not meaningful: "${txt}"`);
  }

  if (!process.exitCode) console.log("PASS: runway dip warning shows a suggested action.");
} finally {
  await browser.close();
}
