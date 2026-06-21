// C72 — to-do nested sub-tasks: add a subtask (indented) and cascade-delete.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
const has = (page, t) => page.evaluate((s) => document.body.innerText.includes(s), t);
try {
  const page = await (await browser.newContext()).newPage();
  page.on("dialog", async (d) => { fail("native dialog: " + d.type()); await d.dismiss(); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(500);
  await page.locator('a[title="To-do"]').first().click();
  await page.waitForSelector("#task-add", { timeout: 10000 });

  // Add a parent task.
  const P = "Parent E2E " + Date.now();
  await page.locator("#task-add").fill(P);
  await page.getByRole("button", { name: "Add", exact: true }).first().click();
  await page.waitForTimeout(400);
  const parentRow = page.locator(".rows .row").filter({ hasText: P }).first();
  if ((await parentRow.count()) === 0) fail("parent task did not appear");

  // Add a sub-task via "+ Sub" → prompt modal.
  await parentRow.locator("button", { hasText: "Sub" }).first().click();
  await page.waitForSelector(".cf-dialog-input", { timeout: 8000 }).catch(() => fail("subtask prompt modal did not open"));
  const C = "Child E2E " + Date.now();
  await page.locator(".cf-dialog-input").fill(C);
  await page.locator("#cf-dialog-confirm").click();
  await page.waitForTimeout(400);
  if (!(await has(page, C))) fail("sub-task did not appear");
  if ((await page.locator(".row.subtask").filter({ hasText: C }).count()) === 0) fail("sub-task is not rendered as an indented subtask");

  // Cascade delete: delete the parent → child goes too.
  await page.locator(".rows .row").filter({ hasText: P }).first().locator('button[aria-label="Delete task"]').click();
  await page.waitForTimeout(400);
  if (await has(page, P)) fail("parent still present after delete");
  if (await has(page, C)) fail("cascade delete did not remove the sub-task");

  if (!process.exitCode) console.log("PASS: sub-tasks nest under their parent and cascade-delete with it.");
} finally {
  await browser.close();
}
