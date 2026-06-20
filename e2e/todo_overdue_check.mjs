// C52 gate — "overdue tasks stand out". Adds an open task with a past due date and
// asserts its due meta carries the danger tone and an explicit "overdue" word
// (colour + text, B15). Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const TITLE = "ZZOVERDUE-TASK";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/todo", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#task-add", { timeout: 60000 });

  await page.locator("#task-add").fill(TITLE);
  await page.locator('form input[type="date"]').first().fill("2020-01-01");
  await page.locator("form button[type='submit']").first().click();
  await page.waitForTimeout(700);

  const row = page.locator(".row", { hasText: TITLE });
  if ((await row.count()) === 0) fail(`the overdue task "${TITLE}" did not appear`);
  const overdueMeta = row.locator(".row-meta.text-down");
  if ((await overdueMeta.count()) === 0) fail("overdue task's due meta should carry the danger tone (.text-down)");
  const txt = (await overdueMeta.first().innerText()) || "";
  if (!/overdue/i.test(txt)) fail(`overdue meta should contain the word "overdue", got "${txt}"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: overdue task flagged with danger tone + "overdue" ("${txt.trim()}").`);
} finally {
  await browser.close();
}
