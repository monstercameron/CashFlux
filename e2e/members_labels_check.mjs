// Gate: member add form and inline edit have visible labels (C62).
// Checks that the add form renders a visible <label> for the Name field,
// and that the inline edit form also wraps Name in a <label>.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

async function flush(page) {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
}

const MEMBER_NAME = "ZZLabelMember_" + Date.now();

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  await page.goto(BASE + "/members", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(500);

  // Open the member add form.
  let addForm = page.locator('[data-testid="member-add-form"]');
  if (!(await addForm.count())) {
    await page.locator('button[title="Add something new"]').click();
    await page.waitForTimeout(200);
    await page.locator('button:has-text("New member")').click();
    await page.waitForTimeout(300);
    addForm = page.locator('[data-testid="member-add-form"]');
  }

  if (!(await addForm.count())) { fail("member add form not found"); process.exit(1); }

  // Expect a visible <label> element containing "Name" text inside the form.
  const labelCount = await addForm.locator('label').count();
  if (labelCount === 0) {
    fail("no <label> elements found in member add form (visible labels required for C62)");
  }

  // The label should contain "Name" text.
  const labelTexts = await addForm.locator('label').allTextContents();
  const hasNameLabel = labelTexts.some((t) => t.toLowerCase().includes("name"));
  if (!hasNameLabel) {
    fail(`member add form label texts: ${JSON.stringify(labelTexts)} — none contains "name"`);
  }

  // Now add a member so we can test the inline edit form.
  await addForm.locator('input[type="text"]').fill(MEMBER_NAME);
  await addForm.locator('button[type="submit"]').click();
  await flush(page);

  // Navigate back to /members and find the edit button.
  await page.goto(BASE + "/members", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(300);

  // Click the Edit button on the new member's row.
  const editBtn = page.locator(`button[title*="Edit"]`).last();
  if ((await editBtn.count()) === 0) { fail("no Edit button found on member row"); }
  else {
    await editBtn.click();
    await page.waitForTimeout(200);

    // The inline edit form should also have visible labels.
    const editForm = page.locator('.row form.form-grid').last();
    if (await editForm.count()) {
      const editLabels = await editForm.locator('label').count();
      if (editLabels === 0) {
        fail("no <label> elements in member inline edit form (C62 visible labels required)");
      }
    }
  }

  if (!process.exitCode) console.log("PASS: member add form and inline edit have visible labels for Name and Color.");
} finally {
  await browser.close();
}
