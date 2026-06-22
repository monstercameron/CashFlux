// L25 CI gate — "bulk ops affect EXACTLY the selected set." Rows carry data-id
// (the transaction ID). We select two specific rows, bulk-recategorize them, and
// assert exactly those two changed category while all others are untouched; then
// select two other rows, bulk-delete, and assert exactly those two are removed.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const txnMap = (page) => page.evaluate(() => {
  const d = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  const m = {};
  for (const t of d.transactions || []) m[t.id] = t.categoryId || "";
  return m;
});
async function flush(page) {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("tr.row[data-id]", { timeout: 60000 });
  await flush(page);

  const rows = page.locator("tr.row[data-id]");
  // Pick two non-transfer rows by data-id (rows on the first page).
  const id0 = await rows.nth(0).getAttribute("data-id");
  const id1 = await rows.nth(1).getAttribute("data-id");
  const before = await txnMap(page);

  // --- Bulk recategorize the two selected rows. ---
  await rows.nth(0).locator("button.check").click();
  await rows.nth(1).locator("button.check").click();
  await page.waitForTimeout(150);
  const catSel = page.locator('select[aria-label="Category to apply"]');
  const targetCat = await catSel.evaluate((el) => {
    const opt = [...el.options].find((o) => o.value);
    el.value = opt.value;
    el.dispatchEvent(new Event("change", { bubbles: true }));
    return opt.value;
  });
  await page.locator('button[title="Set this category on the selected transactions"]').click();
  await flush(page);

  let after = await txnMap(page);
  // Exactly id0 and id1 are now the target category; everything else unchanged.
  for (const [id, cat] of Object.entries(after)) {
    if (id === id0 || id === id1) {
      if (cat !== targetCat) fail(`row ${id} should be recategorized to ${targetCat}, got ${cat}`);
    } else if (cat !== before[id]) {
      fail(`row ${id} category changed (${before[id]} -> ${cat}) but was NOT selected`);
    }
  }

  // --- Bulk delete two OTHER rows. ---
  const id2 = await rows.nth(2).getAttribute("data-id");
  const id3 = await rows.nth(3).getAttribute("data-id");
  await rows.nth(2).locator("button.check").click();
  await rows.nth(3).locator("button.check").click();
  await page.waitForTimeout(150);
  await page.locator('button[title="Delete the selected transactions"]').click();
  await flush(page);

  after = await txnMap(page);
  if (id2 in after || id3 in after) fail("bulk delete did not remove exactly the selected rows");
  // id0/id1 (recategorized earlier) and other rows still present.
  if (!(id0 in after) || !(id1 in after)) fail("bulk delete removed rows that were not selected");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: bulk recategorize + delete each affected exactly the selected rows.");
} finally {
  await browser.close();
}
