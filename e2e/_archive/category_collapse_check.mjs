// L28 gate — "the category tree is collapsible." Collapsing a parent hides its
// children; expanding shows them again. Uses the seeded "Utilities" parent
// (children: Electricity, Internet).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const PARENT = "Utilities";
const CHILDREN = ["Electricity", "Internet"];
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

// Is a category row with this exact name currently visible?
const childVisible = (page, name) =>
  page.locator(".rows .row .row-desc", { hasText: new RegExp(`^\\s*${name}\\s*$`) }).first().isVisible().catch(() => false);

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('[data-testid^="cat-toggle-"]', { timeout: 60000 });
  await page.waitForTimeout(400);

  // Children visible initially.
  for (const c of CHILDREN) {
    if (!(await childVisible(page, c))) { fail(`child "${c}" not visible before collapse`); process.exit(1); }
  }

  // The toggle inside the Utilities parent row.
  const parentRow = page.locator(".rows .row", { has: page.locator(".row-desc", { hasText: new RegExp(`^\\s*${PARENT}\\s*$`) }) }).first();
  const toggle = parentRow.locator('[data-testid^="cat-toggle-"]').first();
  if ((await toggle.count()) === 0) { fail(`no collapse toggle on the "${PARENT}" parent row`); process.exit(1); }

  // Collapse → children hidden.
  await toggle.click();
  await page.waitForTimeout(400);
  for (const c of CHILDREN) {
    if (await childVisible(page, c)) fail(`child "${c}" still visible after collapsing "${PARENT}"`);
  }
  if ((await parentRow.locator('[aria-expanded="false"]').count()) === 0) {
    fail("collapsed toggle should report aria-expanded=false");
  }

  // Expand → children return.
  await toggle.click();
  await page.waitForTimeout(400);
  for (const c of CHILDREN) {
    if (!(await childVisible(page, c))) fail(`child "${c}" did not return after expanding "${PARENT}"`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: collapsing "${PARENT}" hides ${CHILDREN.join("/")}, expanding restores them.`);
} finally {
  await browser.close();
}
