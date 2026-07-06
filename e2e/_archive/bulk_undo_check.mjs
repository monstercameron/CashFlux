// L25 gate — "bulk delete is reversible." Selects two ledger rows, deletes them,
// asserts they're gone from the dataset, clicks Undo, asserts they're restored
// with the same IDs. Rows carry data-id (the transaction ID), so we map selected
// rows precisely.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const ids = (page) => page.evaluate(() => {
  const d = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  return (d.transactions || []).map((t) => t.id);
});
async function waitIds(page, pred, timeoutMs = 10000) {
  let v = [];
  for (let w = 0; w < timeoutMs; w += 400) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    v = await ids(page);
    if (pred(v)) return v;
    await page.waitForTimeout(400);
  }
  return v;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("tr.row[data-id]", { timeout: 60000 });
  const before = await waitIds(page, (v) => v.length > 2);
  const n = before.length;

  // Select the first two rows and capture their transaction IDs.
  const rows = page.locator("tr.row[data-id]");
  const id0 = await rows.nth(0).getAttribute("data-id");
  const id1 = await rows.nth(1).getAttribute("data-id");
  await rows.nth(0).locator("button.check").click();
  await rows.nth(1).locator("button.check").click();
  await page.waitForTimeout(200);

  // Delete the selected rows.
  await page.locator('button[title="Delete the selected transactions"]').click();
  const afterDel = await waitIds(page, (v) => !v.includes(id0) && !v.includes(id1));
  if (afterDel.includes(id0) || afterDel.includes(id1)) fail("selected rows were not deleted");
  if (afterDel.length !== n - 2) fail(`expected ${n - 2} txns after delete, got ${afterDel.length}`);

  // Undo restores them (same IDs).
  await page.locator('button[title="Undo the last bulk action"]').click();
  const afterUndo = await waitIds(page, (v) => v.includes(id0) && v.includes(id1));
  if (!afterUndo.includes(id0) || !afterUndo.includes(id1)) fail("Undo did not restore the deleted rows");
  if (afterUndo.length !== n) fail(`expected ${n} txns after undo, got ${afterUndo.length}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: bulk-deleted 2 rows (${n}->${n - 2}) and Undo restored them (${afterUndo.length}) with the same IDs.`);
} finally {
  await browser.close();
}
