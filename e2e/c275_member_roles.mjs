// C275 gate — role selector in add-member and edit-member forms.
// Steps:
//   1. Navigate to Members screen.
//   2. Open the add-member modal, choose "Viewer" role, submit.
//   3. Confirm the new member exists in the dataset with role="viewer".
//   4. Open the inline edit form for that member, change role to "Owner".
//   5. Confirm the updated role persists after a page reload.
//   6. Take a screenshot.
//
// Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const MEMBER_NAME = "ZZRoleTest-C275-" + Date.now();
const SCREENSHOT = path.join(__dirname, "c275_member_roles.png");

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const dataset = (page) =>
  page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));

async function waitForDataset(page, pred, timeoutMs = 8000) {
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    const d = await dataset(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return await dataset(page);
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // ── Step 1: navigate to Members ──────────────────────────────────────────────
  await page.goto(BASE + "/members", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });

  // ── Step 2: open add-member modal and choose "Viewer" role ───────────────────
  // The add button opens the AddHost modal — look for the "+" or "Add" trigger.
  const addTrigger = page.locator('button[aria-label*="Add"], button:has-text("Add member"), button:has-text("Add first member"), button[title*="Add"]').first();
  const addCount = await addTrigger.count();
  if (addCount === 0) {
    // Try the global "+" menu button.
    const plusBtn = page.locator('button[aria-label*="menu"], button[title*="Add"], button:has-text("+")').first();
    if ((await plusBtn.count()) > 0) {
      await plusBtn.click();
      await page.waitForTimeout(300);
      const memberItem = page.locator('[role="menuitem"]:has-text("Member"), button:has-text("Member")').first();
      if ((await memberItem.count()) > 0) await memberItem.click();
    }
  } else {
    await addTrigger.click();
  }
  await page.waitForTimeout(400);

  // The add form should now be visible (modal or inline).
  const addForm = page.locator('[data-testid="member-add-form"]');
  if ((await addForm.count()) === 0) {
    fail("member-add-form not found after clicking add trigger");
  }

  // Fill in the name.
  const nameInput = addForm.locator('input[type="text"]').first();
  await nameInput.fill(MEMBER_NAME);

  // Choose "Viewer" in the role selector.
  const roleSelect = addForm.locator('[data-testid="member-add-role"]');
  if ((await roleSelect.count()) === 0) {
    fail("role selector (data-testid=member-add-role) not found in add form");
  } else {
    await roleSelect.selectOption({ value: "viewer" });
  }

  // Submit.
  await addForm.locator('button[type="submit"]').click();
  await page.waitForTimeout(600);

  // ── Step 3: confirm role="viewer" persisted in dataset ───────────────────────
  const d1 = await waitForDataset(
    page,
    (d) => (d.members || []).some((m) => m.name === MEMBER_NAME),
    7000
  );
  const added = (d1.members || []).find((m) => m.name === MEMBER_NAME);
  if (!added) {
    fail(`member "${MEMBER_NAME}" not found in dataset after add`);
  } else if (added.role !== "viewer") {
    fail(`expected role="viewer" after add, got "${added.role}"`);
  } else {
    console.log(`PASS step 3: member added with role="${added.role}"`);
  }

  // ── Step 4: edit the member — change role to "Owner" ─────────────────────────
  // Find and click the Edit button for the new member row.
  // MemberRow renders a button with title from members.editTitle (typically "Edit").
  const editBtn = page.locator(`button[title*="Edit"], button:has-text("Edit")`).last();
  if (!process.exitCode && (await editBtn.count()) > 0) {
    await editBtn.click();
    await page.waitForTimeout(400);
  }

  // The inline edit form: the role select is data-testid="member-edit-role-<id>".
  const editRoleSelect = page.locator(`[data-testid="member-edit-role-${added ? added.id : ""}"]`);
  if (!process.exitCode) {
    if ((await editRoleSelect.count()) === 0) {
      fail(`role selector (data-testid=member-edit-role-${added?.id}) not found in edit form`);
    } else {
      await editRoleSelect.selectOption({ value: "owner" });
      // Submit the edit form.
      const saveBtn = page.locator('button[type="submit"]:has-text("Save"), button[type="submit"]').first();
      await saveBtn.click();
      await page.waitForTimeout(600);
    }
  }

  // ── Step 5: confirm role persists after reload ───────────────────────────────
  if (!process.exitCode) {
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });

    const d2 = await waitForDataset(
      page,
      (d) => (d.members || []).some((m) => m.name === MEMBER_NAME),
      5000
    );
    const updated = (d2.members || []).find((m) => m.name === MEMBER_NAME);
    if (!updated) {
      fail(`member "${MEMBER_NAME}" missing after reload`);
    } else if (updated.role !== "owner") {
      fail(`expected role="owner" after edit+reload, got "${updated.role}"`);
    } else {
      console.log(`PASS step 5: role persisted as "${updated.role}" across reload`);
    }
  }

  // ── Step 6: screenshot ───────────────────────────────────────────────────────
  await page.goto(BASE + "/members", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1000);
  await page.screenshot({ path: SCREENSHOT, fullPage: false });
  console.log(`Screenshot: ${SCREENSHOT}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: C275 role selector add/edit/persist verified.");
} finally {
  await browser.close();
}
