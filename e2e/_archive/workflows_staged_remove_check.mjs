// C65 gate — "a staged workflow action can be removed before saving". The action
// builder used to only add (a mistake meant starting over). Each staged action now
// has a remove button. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const TAG = "ZZACT-REMOVE-ME";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/workflows", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[placeholder="Workflow name"]', { timeout: 60000 });

  // Default action kind is "create task" → a text param input. Stage one action.
  await page.locator('input[placeholder="Task title / message / tag"]').first().fill(TAG);
  await page.getByRole("button", { name: "Add action" }).click();
  await page.waitForTimeout(300);

  const stagedRow = page.locator(".row", { hasText: TAG });
  if ((await stagedRow.count()) === 0) fail("the action did not stage (no row with the action text)");
  if ((await stagedRow.locator(".btn-del").count()) === 0) fail("staged action has no remove button");

  // Remove it.
  await stagedRow.locator(".btn-del").first().click();
  await page.waitForTimeout(300);
  if ((await page.locator(".row", { hasText: TAG }).count()) !== 0) fail("the staged action was not removed");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: a staged workflow action can be removed before saving.");
} finally {
  await browser.close();
}
