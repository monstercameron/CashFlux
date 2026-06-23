// add_modal_entities_check — verifies that the +Add menu opens the modal add
// form for task, category, member, and rule. Checks that submitting empty stays
// open (validation error) and submitting valid data closes the modal.
//
// NOTE: The following existing e2e files use the OLD inline form IDs and will
// need updating to use the modal flow after this refactor:
//   e2e/story_todo_toggle.test.mjs         — uses #task-add
//   e2e/recurring_task_check.mjs           — uses #task-add
//   e2e/todo_nesting_check.mjs             — uses #task-add
//   e2e/todo_overdue_check.mjs             — uses #task-add
//   e2e/todo_labels_check.mjs              — uses #task-add
//   e2e/undo_check.mjs                     — uses #task-add
//   e2e/task_entity_link_check.mjs         — uses #task-add
//   e2e/loopstory_62_money_question.mjs    — uses #task-add
//   e2e/story_category_reassign.test.mjs   — uses #cat-add
//   e2e/story_subcategory.test.mjs         — uses #cat-add
//   e2e/categories_labels_check.mjs        — uses #cat-add
//   e2e/category_parent_delete_check.mjs   — uses #cat-add
//   e2e/loopstory_42_add_category.mjs      — uses #cat-add
//   e2e/story_member_reassign.test.mjs     — uses #member-add
//   e2e/story_member_default.test.mjs      — uses #member-add
//   e2e/story_settle_up.test.mjs           — uses #member-add
//   e2e/split_summary_check.mjs            — uses #member-add
//   e2e/split_diagram_check.mjs            — uses #member-add
//   e2e/loopstory_48_settle_up.mjs         — uses #member-add
//   e2e/rules_check.mjs                    — uses #rule-add
//   e2e/rules_diagram_check.mjs            — uses #rule-add
//   e2e/rules_live_count_check.mjs         — uses #rule-add
//   e2e/rules_preview_check.mjs            — uses #rule-add
//   e2e/create_rule_from_txn_check.mjs     — uses #rule-add
//   e2e/loopstory_52_automator.mjs         — uses #rule-add

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

// openAddMenu clicks the "+Add" button to open the add menu.
async function openAddMenu(page) {
  await page.getByRole("button", { name: "Add something new" }).click();
  await page.waitForTimeout(200);
}

// openModal clicks a menu item by its visible label, then waits for the flip-panel.
async function openModal(page, label) {
  await page.getByRole("menuitem", { name: label }).click();
  // The FlipPanel renders as a dialog or a panel overlay. Wait for it to appear.
  await page.waitForSelector(".flip-panel, [role=dialog]", { timeout: 8000 })
    .catch(() => fail(`modal did not open for "${label}"`));
  await page.waitForTimeout(200);
}

// closeModal clicks the Close button on the flip-panel.
async function closeModal(page) {
  const closeBtn = page.locator(".flip-panel button", { hasText: /close/i }).first();
  if (await closeBtn.count() > 0) {
    await closeBtn.click();
  } else {
    // Fallback: press Escape
    await page.keyboard.press("Escape");
  }
  await page.waitForTimeout(200);
}

try {
  const page = await (await browser.newContext()).newPage();
  page.on("dialog", async (d) => { fail("native dialog opened: " + d.type()); await d.dismiss(); });
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title], .bento', { timeout: 60000 });
  await page.waitForTimeout(700);

  // ── Task ──────────────────────────────────────────────────────────────────
  await openAddMenu(page);
  await openModal(page, "New task");

  // The task add form must have #task-add.
  await page.waitForSelector("#task-add", { timeout: 5000 })
    .catch(() => fail("task-add-form: #task-add input not found"));

  // Submit empty → stays open (title is required).
  await page.locator('[data-testid="task-add-form"] button[type=submit]').click();
  await page.waitForTimeout(300);
  const taskFormStillOpen = await page.locator("#task-add").isVisible();
  if (!taskFormStillOpen) fail("task-add-form: empty submit should keep form open");

  // Fill valid data and submit.
  await page.locator("#task-add").fill("E2E test task");
  await page.locator('[data-testid="task-add-form"] button[type=submit]').click();
  await page.waitForTimeout(500);
  // Modal should close (flip-panel gone).
  const taskModalGone = (await page.locator(".flip-panel").count()) === 0 ||
    !(await page.locator(".flip-panel").isVisible());
  if (!taskModalGone) fail("task-add-form: modal did not close after valid submit");

  // ── Category ──────────────────────────────────────────────────────────────
  await openAddMenu(page);
  await openModal(page, "New category");

  await page.waitForSelector("#cat-add", { timeout: 5000 })
    .catch(() => fail("category-add-form: #cat-add input not found"));

  // Submit empty → stays open (name is required).
  await page.locator('[data-testid="category-add-form"] button[type=submit]').click();
  await page.waitForTimeout(300);
  const catFormStillOpen = await page.locator("#cat-add").isVisible();
  if (!catFormStillOpen) fail("category-add-form: empty submit should keep form open");

  // Fill valid data and submit.
  await page.locator("#cat-add").fill("E2E test category");
  await page.locator('[data-testid="category-add-form"] button[type=submit]').click();
  await page.waitForTimeout(500);
  const catModalGone = (await page.locator(".flip-panel").count()) === 0 ||
    !(await page.locator(".flip-panel").isVisible());
  if (!catModalGone) fail("category-add-form: modal did not close after valid submit");

  // ── Member ────────────────────────────────────────────────────────────────
  await openAddMenu(page);
  await openModal(page, "New member");

  await page.waitForSelector("#member-add", { timeout: 5000 })
    .catch(() => fail("member-add-form: #member-add input not found"));

  // Submit empty → stays open (name is required).
  await page.locator('[data-testid="member-add-form"] button[type=submit]').click();
  await page.waitForTimeout(300);
  const memberFormStillOpen = await page.locator("#member-add").isVisible();
  if (!memberFormStillOpen) fail("member-add-form: empty submit should keep form open");

  // Fill valid data and submit.
  await page.locator("#member-add").fill("E2E test member");
  await page.locator('[data-testid="member-add-form"] button[type=submit]').click();
  await page.waitForTimeout(500);
  const memberModalGone = (await page.locator(".flip-panel").count()) === 0 ||
    !(await page.locator(".flip-panel").isVisible());
  if (!memberModalGone) fail("member-add-form: modal did not close after valid submit");

  // ── Rule ──────────────────────────────────────────────────────────────────
  await openAddMenu(page);
  await openModal(page, "New rule");

  await page.waitForSelector("#rule-add", { timeout: 5000 })
    .catch(() => fail("rule-add-form: #rule-add input not found"));

  // Submit empty → stays open (match phrase and category are required).
  await page.locator('[data-testid="rule-add-form"] button[type=submit]').click();
  await page.waitForTimeout(300);
  const ruleFormStillOpen = await page.locator("#rule-add").isVisible();
  if (!ruleFormStillOpen) fail("rule-add-form: empty submit should keep form open");

  // A rule needs both a match phrase AND a category. Without any categories in a
  // fresh dataset we can only check the match-required error (form stays open).
  // If a category exists from the earlier step, also pick it and verify close.
  await page.locator("#rule-add").fill("E2E test rule phrase");
  // Try to pick the first available category option (if any).
  const catSelect = page.locator('[data-testid="rule-add-form"] select[aria-label]').first();
  const catOptCount = await catSelect.locator("option").count();
  if (catOptCount > 1) {
    // Pick the first real option (index 1; index 0 is the blank prompt).
    await catSelect.selectOption({ index: 1 });
    await page.locator('[data-testid="rule-add-form"] button[type=submit]').click();
    await page.waitForTimeout(500);
    const ruleModalGone = (await page.locator(".flip-panel").count()) === 0 ||
      !(await page.locator(".flip-panel").isVisible());
    if (!ruleModalGone) fail("rule-add-form: modal did not close after valid submit");
  } else {
    // No categories — just close the modal manually.
    await closeModal(page);
    console.log("  (rule-add-form: skipped close-on-success check — no categories available)");
  }

  if (!process.exitCode) {
    console.log("PASS: add-modal entity forms (task, category, member, rule) open via +Add menu, validate on empty submit, and close on success.");
  }
} finally {
  await browser.close();
}
