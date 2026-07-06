// B16 E2E story — "account archive + restore". Adds an account, archives it via
// the row's More-actions menu (asserting it's flagged archived), then restores it
// (asserting it's active again). Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const NAME = "ZZARCH-1";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const acctByName = (page, name) =>
  page.evaluate((n) => {
    const d = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    return (d.accounts || []).find((a) => a.name === n) || null;
  }, name);
async function waitForAcct(page, name, pred, timeoutMs = 7000) {
  let a = null;
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    a = await acctByName(page, name);
    if (pred(a)) return a;
    await page.waitForTimeout(400);
  }
  return a;
}

// Opens the row's More-actions menu and clicks the menu item with the given title.
async function rowMenuAction(page, rowText, itemTitle) {
  const row = page.locator(".row", { hasText: rowText });
  await row.locator('button[aria-label="More actions"]').first().click();
  await page.waitForTimeout(200);
  await row.locator(`button[title="${itemTitle}"]`).first().click();
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="text"][aria-required="true"]', { timeout: 60000 });

  // Add an account.
  await page.locator('input[type="text"][aria-required="true"]').fill(NAME);
  await page.locator('button[type="submit"]').first().click();
  const created = await waitForAcct(page, NAME, (a) => !!a);
  if (!created) fail("account not found after adding");
  else if (created.archived) fail("new account should not be archived");

  // Archive it.
  await rowMenuAction(page, NAME, "Archive account");
  const archived = await waitForAcct(page, NAME, (a) => a && a.archived === true);
  if (!archived || archived.archived !== true) fail(`account should be archived, got archived=${archived && archived.archived}`);

  // Restore it.
  await rowMenuAction(page, NAME, "Restore account");
  const restored = await waitForAcct(page, NAME, (a) => a && !a.archived);
  if (!restored || restored.archived) fail(`account should be restored (not archived), got archived=${restored && restored.archived}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: archived and restored account "${NAME}" (archived true -> false).`);
} finally {
  await browser.close();
}
