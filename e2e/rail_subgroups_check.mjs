// C67 — Tools rail sub-sections: sub-group headers group the Tools items and
// collapse/expand their items (persisted across reloads).
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
  await page.waitForTimeout(600);

  const heads = await page.locator(".rail-subhead").allTextContents();
  for (const want of ["Plan & analyze", "Bills & recurring", "Data & import", "Build", "System", "My pages"]) {
    if (!heads.some((h) => h.includes(want))) fail(`collapsible section header missing: ${want}`);
  }
  // System collapses too: collapsing it hides its items (e.g. Members).
  if ((await page.locator('nav a[title="Members"]').count()) === 0) fail("Members not visible initially");
  await page.locator(".rail-subhead", { hasText: "System" }).first().click();
  await page.waitForTimeout(250);
  if ((await page.locator('nav a[title="Members"]').count()) !== 0) fail("collapsing System did not hide its items");
  // "Allocate" lives under "Plan & analyze"; collapsing that section hides it.
  const allocate = page.locator('nav a[title="Allocate"]');
  if ((await allocate.count()) === 0) fail("Allocate tool not visible initially");
  await page.locator(".rail-subhead", { hasText: "Plan & analyze" }).first().click();
  await page.waitForTimeout(300);
  if ((await page.locator('nav a[title="Allocate"]').count()) !== 0) fail("collapsing the sub-section did not hide its items");

  // Persists across reload.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(500);
  if ((await page.locator('nav a[title="Allocate"]').count()) !== 0) fail("collapsed sub-section did not persist across reload");
  // Expand again.
  await page.locator(".rail-subhead", { hasText: "Plan & analyze" }).first().click();
  await page.waitForTimeout(300);
  if ((await page.locator('nav a[title="Allocate"]').count()) === 0) fail("expanding did not restore the items");

  if (!process.exitCode) console.log("PASS: Tools rail sub-sections group items and collapse/expand (persisted).");
} finally {
  await browser.close();
}
