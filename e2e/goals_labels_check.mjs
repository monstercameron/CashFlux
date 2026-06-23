// C51 gate — "goal add/edit/contribute forms have persistent visible labels".
// Asserts the add form wraps controls in .labeled-field with visible text, and the
// inline editor is labeled too. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
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

  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".add-btn", { timeout: 60000 });

  // Open the add modal first so labeled-field elements are visible.
  await page.locator(".add-btn").click();
  await page.locator('[role="menuitem"]', { hasText: /goal/i }).first().click();
  await page.waitForSelector('#goal-add', { timeout: 10000 });
  await page.waitForSelector(".labeled-field", { timeout: 5000 });

  const count = await page.locator(".labeled-field").count();
  if (count < 5) fail(`expected the goal add form to have several labeled fields, got ${count}`);
  const texts = (await page.locator(".labeled-field span").allInnerTexts()).map((t) => t.trim());
  if (!texts.includes("Name")) fail(`add form should have a visible "Name" label (saw: ${texts.join(", ")})`);

  // Close the modal before interacting with the goal list behind it.
  await page.keyboard.press("Escape");
  await page.waitForTimeout(300);

  // Inline editor is labeled too.
  const edit = page.getByRole("button", { name: "Edit" }).first();
  if (await edit.count()) {
    await edit.click();
    await page.waitForSelector(".budget .labeled-field", { timeout: 5000 });
    const editCount = await page.locator(".budget .labeled-field").count();
    if (editCount < 4) fail(`inline editor should have several labeled fields, got ${editCount}`);
  } else {
    fail("no goal row Edit button found to verify inline-edit labels");
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: goal forms labeled — ${count} add-form labeled fields, inline editor labeled.`);
} finally {
  await browser.close();
}
