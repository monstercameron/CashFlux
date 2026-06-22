// L18 gate — "report / total transactions by a custom field". Asserts that the
// Reports screen shows a "Spending by <field>" section when transaction custom
// fields exist, that the field selector switches the active grouping, and that a
// CSV download button is present.
//
// Seeded data relied on (from internal/store/sample.go):
//   - CustomField def  id="cf-txn-project"  key="project"  label="Project"
//     type=select  options=[Personal, Freelance, "Side hustle"]  entity=transaction
//   - CustomField def  id="cf-txn-reimbursable"  key="reimbursable"  label="Reimbursable"
//     type=bool  entity=transaction
//   - Transactions with Custom["project"] values "Freelance" and "Personal" are
//     present in the sample (e.g. tx-2025-10 / tx-laptop-2025-11 / tx-medical-2025-09).
//
// The test navigates to /reports in "All time" (or any period that catches the
// seeded transactions), then verifies the custom-field section renders and the
// field selector + CSV button are present.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(
  path.join(__dirname, "..", ".tools", "package.json")
);
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // Navigate to /reports and wait for the main reports content.
  await page.goto(BASE + "/reports", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("[data-testid='customfield-spend-section']", {
    timeout: 60000,
  });

  // ── Section title ─────────────────────────────────────────────────────────
  // The first transaction custom field in the sample is "project" (label "Project"),
  // so the section title should contain "Project".
  const sectionTitle = await page
    .locator("[data-testid='customfield-spend-section'] .card-title")
    .innerText();
  if (!sectionTitle.toLowerCase().includes("project")) {
    fail(
      `section title should mention the active field label "Project", got: "${sectionTitle}"`
    );
  }

  // ── Field selector ────────────────────────────────────────────────────────
  const selector = page.locator("[data-testid='cf-field-select']");
  const selectorCount = await selector.count();
  if (selectorCount === 0) {
    fail("field selector [data-testid='cf-field-select'] not found");
  }

  // The selector should have at least two options (project + reimbursable).
  const optionCount = await selector.locator("option").count();
  if (optionCount < 2) {
    fail(`expected at least 2 field options, got ${optionCount}`);
  }

  // ── Grouped rows ──────────────────────────────────────────────────────────
  // At least one row should appear in the section (the seeded sample has expenses
  // with "Freelance" and "Personal" project values in range when the period is
  // wide enough — the default period covers recent months; if the sample loaded,
  // at least the "(no value)" bucket will appear for transactions without the field).
  const rows = page.locator(
    "[data-testid='customfield-spend-section'] .rows .row"
  );
  const rowCount = await rows.count();
  if (rowCount === 0) {
    fail(
      "no grouped rows found in the custom-field spend section; seeded data may not match the active period"
    );
  }

  // ── Switch field via selector ─────────────────────────────────────────────
  // Select "Reimbursable" (the bool field) and confirm the section title updates.
  await selector.selectOption({ label: "Reimbursable" });
  await page.waitForFunction(
    () => {
      const el = document.querySelector(
        "[data-testid='customfield-spend-section'] .card-title"
      );
      return el && el.textContent.toLowerCase().includes("reimbursable");
    },
    { timeout: 10000 }
  );
  const updatedTitle = await page
    .locator("[data-testid='customfield-spend-section'] .card-title")
    .innerText();
  if (!updatedTitle.toLowerCase().includes("reimbursable")) {
    fail(
      `after switching to Reimbursable, title should update; got: "${updatedTitle}"`
    );
  }

  // ── CSV download button ───────────────────────────────────────────────────
  const csvBtn = page.locator("[data-testid='cf-download-csv']");
  const csvBtnCount = await csvBtn.count();
  if (csvBtnCount === 0) {
    fail("CSV download button [data-testid='cf-download-csv'] not found");
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));

  if (!process.exitCode)
    console.log(
      `PASS: custom-field spend section renders with ${rowCount} grouped rows, field selector (${optionCount} options), and CSV button.`
    );
} finally {
  await browser.close();
}
