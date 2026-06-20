// B16 E2E story — "category reassign-on-delete (no orphan)". Adds a category,
// assigns a transaction to it, then deletes the category choosing a reassignment
// target, and asserts the deleted category is gone AND the transaction was moved
// to the chosen target (never orphaned). Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const CAT = "ZZCAT-DEL";
const TXN = "ZZCATTXN-9";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const dataset = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
async function waitForDataset(page, pred, timeoutMs = 7000) {
  let d = {};
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    d = await dataset(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return d;
}
const railTo = (page, title) => page.locator(`nav[aria-label="Main navigation"] a[title="${title}"]`).click();

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#cat-add", { timeout: 60000 });

  // 1. Add an expense category (the default kind).
  await page.locator("#cat-add").fill(CAT);
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(500);

  // 2. Assign a transaction to it (so deleting must reassign).
  await railTo(page, "Transactions");
  await page.waitForSelector("#txn-add", { timeout: 8000 });
  await page.locator("#txn-add").fill(TXN);
  await page.locator('input[type="number"][aria-required="true"]').fill("7.50");
  await page.locator('select[aria-label="Category"]').selectOption({ label: CAT });
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(500);

  // Confirm the category is in use.
  const d0 = await waitForDataset(page, (d) => {
    const c = (d.categories || []).find((x) => x.name === CAT);
    const t = (d.transactions || []).find((x) => x.desc === TXN);
    return c && t && t.categoryId === c.id;
  });
  const cat = (d0.categories || []).find((x) => x.name === CAT);
  if (!cat) fail("category not found / not in use before delete");
  const catId = cat && cat.id;

  // 3. Delete it -> the reassign panel opens (it is in use).
  await railTo(page, "Categories");
  await page.waitForSelector("#cat-add", { timeout: 8000 });
  await page.locator(".rows .row", { hasText: CAT }).locator('button[aria-label="Delete category"]').first().click();
  await page.getByRole("button", { name: "Move and delete", exact: true }).waitFor({ timeout: 8000 });

  // 4. Pick a reassignment target (first real option) and confirm.
  const reassignSelect = page.locator('.card', { hasText: "Move and delete" }).locator("select").first();
  await reassignSelect.selectOption({ index: 1 });
  const targetId = await reassignSelect.inputValue();
  await page.getByRole("button", { name: "Move and delete", exact: true }).click();

  // 5. The category is gone and the transaction moved to the target — no orphan.
  const d1 = await waitForDataset(page, (d) => !(d.categories || []).some((x) => x.id === catId));
  if ((d1.categories || []).some((x) => x.name === CAT)) fail("deleted category still present");
  const txn = (d1.transactions || []).find((x) => x.desc === TXN);
  if (!txn) fail("transaction vanished");
  else if (txn.categoryId === catId) fail("transaction still points at the deleted category (orphan)");
  else if (txn.categoryId !== targetId) fail(`transaction categoryId = ${txn.categoryId}, want the reassign target ${targetId}`);
  else if (!(d1.categories || []).some((x) => x.id === txn.categoryId)) fail("transaction points at a non-existent category");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: deleted in-use category "${CAT}"; its transaction was reassigned to the chosen target (no orphan).`);
} finally {
  await browser.close();
}
