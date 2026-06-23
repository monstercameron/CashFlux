// B16 E2E story — "member reassign-on-delete (no orphan)". Adds a member, gives
// them an account, then deletes the member choosing a reassignment target, and
// asserts the member is gone AND their account moved to the chosen owner (never
// orphaned). Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const MEM = "ZZMEM-DEL";
const ACCT = "ZZMEMACCT";

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

  await page.goto(BASE + "/members", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  // 1. Add a member via +Add modal.
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /member/i }).first().click();
  await page.waitForSelector("#member-add", { timeout: 10000 });
  await page.locator("#member-add").fill(MEM);
  await page.locator('[data-testid="member-add-form"] button[type="submit"]').first().click();
  await page.waitForTimeout(500);

  // 2. Give them an account (so deleting must reassign) via +Add modal.
  await railTo(page, "Accounts");
  await page.waitForSelector(".add-btn", { timeout: 8000 });
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /account/i }).first().click();
  await page.waitForSelector('[role="dialog"]', { timeout: 10000 });
  const acctDialog = page.locator('[role="dialog"]');
  await acctDialog.locator('input[type="text"][placeholder="Name"]').fill(ACCT);
  await acctDialog.locator('input[type="number"]').first().fill("100");
  await acctDialog.locator('select[aria-label="Owner"]').selectOption({ label: MEM });
  await acctDialog.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(500);

  // Confirm the member owns the account.
  const d0 = await waitForDataset(page, (d) => {
    const m = (d.members || []).find((x) => x.name === MEM);
    const a = (d.accounts || []).find((x) => x.name === ACCT);
    return m && a && a.ownerId === m.id;
  });
  const mem = (d0.members || []).find((x) => x.name === MEM);
  if (!mem) fail("member not found / account not owned by them before delete");
  const memId = mem && mem.id;

  // 3. Delete the member -> reassign panel opens (they own an entity).
  await railTo(page, "Members");
  await page.waitForSelector(".add-btn", { timeout: 8000 });
  const memRow = page
    .locator(".row")
    .filter({ hasText: MEM })
    .filter({ has: page.locator('button[aria-label="Delete member"]') });
  await memRow.locator('button[aria-label="Delete member"]').first().click();
  await page.getByRole("button", { name: "Move and delete", exact: true }).waitFor({ timeout: 8000 });

  // 4. Confirm with the default reassignment target (the household / group).
  const reassignSelect = page.locator('.card', { hasText: "Move and delete" }).locator("select").first();
  const targetId = await reassignSelect.inputValue();
  await page.getByRole("button", { name: "Move and delete", exact: true }).click();

  // 5. Member gone, account moved to the target owner — no orphan.
  const d1 = await waitForDataset(page, (d) => !(d.members || []).some((x) => x.id === memId));
  if ((d1.members || []).some((x) => x.name === MEM)) fail("deleted member still present");
  const acct = (d1.accounts || []).find((x) => x.name === ACCT);
  if (!acct) fail("account vanished");
  else if (acct.ownerId === memId) fail("account still owned by the deleted member (orphan)");
  else if (acct.ownerId !== targetId) fail(`account ownerId = ${acct.ownerId}, want the reassign target ${targetId}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: deleted member "${MEM}" who owned an account; the account was reassigned to "${targetId}" (no orphan).`);
} finally {
  await browser.close();
}
