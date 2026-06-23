// B16 E2E story — "sub-category nesting". Adds a parent category and a child
// category under it, and asserts the child is linked to the parent (parentId),
// while the parent stays top-level — the data linkage that the category tree's
// rollup is built on. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const PARENT = "ZZPARENT";
const CHILD = "ZZCHILD";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const catByName = (page, name) =>
  page.evaluate((n) => {
    const d = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    return (d.categories || []).find((c) => c.name === n) || null;
  }, name);
async function waitForCat(page, name, pred = (c) => !!c, timeoutMs = 7000) {
  let c = null;
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    c = await catByName(page, name);
    if (pred(c)) return c;
    await page.waitForTimeout(400);
  }
  return c;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  // 1. Add a top-level parent (expense is the default kind; no parent chosen).
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /category/i }).first().click();
  await page.waitForSelector("#cat-add", { timeout: 10000 });
  await page.locator("#cat-add").fill(PARENT);
  await page.locator('[data-testid="category-add-form"] button[type="submit"]').first().click();
  await page.waitForTimeout(500);
  const parent = await waitForCat(page, PARENT);
  if (!parent) fail("parent category not created");
  else if (parent.parentId) fail("parent should be top-level (no parentId)");

  // 2. Add a child under that parent (pick it in the parent select).
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /category/i }).first().click();
  await page.waitForSelector("#cat-add", { timeout: 10000 });
  await page.locator("#cat-add").fill(CHILD);
  const parentSelect = page.locator('[role="dialog"] select').filter({ has: page.getByRole("option", { name: PARENT, exact: true }) });
  await parentSelect.first().selectOption({ label: PARENT });
  await page.locator('[data-testid="category-add-form"] button[type="submit"]').first().click();

  // 3. The child is linked to the parent.
  const child = await waitForCat(page, CHILD, (c) => c && c.parentId);
  if (!child) fail("child category not created");
  else if (child.parentId !== (parent && parent.id)) fail(`child parentId = ${child && child.parentId}, want the parent's id ${parent && parent.id}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: "${CHILD}" nests under "${PARENT}" (parentId linked); parent stays top-level.`);
} finally {
  await browser.close();
}
