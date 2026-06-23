// B16 E2E story — "set the default member". Adds a member, marks them the default,
// and asserts exactly one member is the default and it's the right one. Exits
// non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const MEM = "ZZDEFMEM";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const members = (page) => page.evaluate(() => (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").members) || []);
async function waitForMembers(page, pred, timeoutMs = 7000) {
  let ms = [];
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    ms = await members(page);
    if (pred(ms)) return ms;
    await page.waitForTimeout(400);
  }
  return ms;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/members", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  // Add a member (not the default yet) via +Add modal.
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /member/i }).first().click();
  await page.waitForSelector("#member-add", { timeout: 10000 });
  await page.locator("#member-add").fill(MEM);
  await page.locator('[data-testid="member-add-form"] button[type="submit"]').first().click();
  await page.waitForTimeout(400);
  // Navigate away and back so the members list re-reads the updated state.
  await page.locator('nav[aria-label="Main navigation"] a[title="Accounts"]').click();
  await page.waitForTimeout(400);
  await page.locator('nav[aria-label="Main navigation"] a[title="Members"]').click();
  await page.waitForTimeout(600);
  const added = await waitForMembers(page, (ms) => ms.some((m) => m.name === MEM));
  const mem = added.find((m) => m.name === MEM);
  if (!mem) fail("member not created");
  else if (mem.isDefault) fail("new member should not already be the default");

  // Make them the default (the row's Make-default button).
  await page
    .locator(".row")
    .filter({ hasText: MEM })
    .filter({ has: page.locator('button[title="Make default member"]') })
    .locator('button[title="Make default member"]')
    .first()
    .click();

  // Exactly one member is the default, and it is ours.
  const after = await waitForMembers(page, (ms) => (ms.find((m) => m.name === MEM) || {}).isDefault === true);
  const defaults = after.filter((m) => m.isDefault);
  if (defaults.length !== 1) fail(`expected exactly one default member, got ${defaults.length}`);
  else if (defaults[0].name !== MEM) fail(`default member is "${defaults[0].name}", want "${MEM}"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: set "${MEM}" as the default member (exactly one default).`);
} finally {
  await browser.close();
}
