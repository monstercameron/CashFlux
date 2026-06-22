// L28 gate — "deleting a parent category re-homes its children, never orphans
// them." Creates a parent + a child under it, deletes the parent, and asserts the
// child still exists with its parentId re-homed to root (not a dangling pointer).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const PAR = "ZZPAR-" + Date.now();
const CHI = "ZZCHI-" + Date.now();
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const cats = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").categories || []);
async function waitCats(page, pred, timeoutMs = 10000) {
  let c = [];
  for (let w = 0; w < timeoutMs; w += 400) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    c = await cats(page);
    if (pred(c)) return c;
    await page.waitForTimeout(400);
  }
  return c;
}

async function addCategory(page, name, parentValue) {
  await page.fill("#cat-add", name);
  if (parentValue) {
    await page.locator('select[aria-label="Parent category (optional)"]').selectOption(parentValue);
  }
  await page.locator('form button[type="submit"]').first().click();
  await page.waitForTimeout(300);
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#cat-add", { timeout: 60000 });

  // Create the parent, then the child under it.
  await addCategory(page, PAR);
  const afterPar = await waitCats(page, (c) => c.some((x) => x.name === PAR));
  const parent = afterPar.find((x) => x.name === PAR);
  if (!parent) { fail("parent category was not created"); process.exit(1); }

  // Select the parent by its id (the option value).
  await addCategory(page, CHI, parent.id);
  const afterChi = await waitCats(page, (c) => c.some((x) => x.name === CHI));
  const child = afterChi.find((x) => x.name === CHI);
  if (!child) { fail("child category was not created"); process.exit(1); }
  if (child.parentId !== parent.id) { fail(`child parentId=${child.parentId}, want ${parent.id}`); process.exit(1); }

  // Delete the parent: click the btn-del on the parent's row.
  const parentRow = page.locator(".rows .row", { has: page.locator(".row-desc", { hasText: PAR }) }).first();
  await parentRow.locator("button.btn-del").click();
  await page.waitForTimeout(500);

  // Parent gone; child survives, re-homed to root (no dangling parentId).
  const after = await waitCats(page, (c) => !c.some((x) => x.id === parent.id));
  if (after.some((x) => x.id === parent.id)) fail("parent was not deleted");
  const survivor = after.find((x) => x.id === child.id);
  // Re-homed to root means parentId is empty — and since the field is omitempty,
  // an empty parent serializes as absent (undefined), which is exactly root.
  if (!survivor) fail("child was orphaned/removed when the parent was deleted");
  else if (survivor.parentId === parent.id) fail("child still points at the deleted parent (orphaned dangling parentId)");
  else if (survivor.parentId) fail(`child re-homed to ${survivor.parentId}, want root (empty)`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: deleting the parent re-homed its child to root (no orphan).`);
} finally {
  await browser.close();
}
