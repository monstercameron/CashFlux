// C77 — dashboard To-do widget: a progress line, inline complete checkboxes that
// toggle the task (and update the count), and a title that drills into /todo.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

try {
  const page = await (await browser.newContext()).newPage();
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".bento", { timeout: 60000 });
  await page.waitForTimeout(700);

  const tile = page.locator('.w[data-widget="todo"]').first();
  if ((await tile.count()) === 0) fail("To-do widget tile is missing");

  // Progress line "N left · M done".
  const progress = await tile.locator("p", { hasText: "left ·" }).first().textContent().catch(() => "");
  if (!/\d+ left · \d+ done/.test(progress || "")) fail(`missing/odd progress line: ${progress}`);
  const leftBefore = parseInt(progress.match(/(\d+) left/)[1], 10);

  // Inline checkbox completes a task → "left" count drops by one.
  const check = tile.locator(".dash-check").first();
  if ((await check.count()) === 0) fail("no inline complete checkbox in the To-do widget");
  await check.click();
  await page.waitForTimeout(400);
  const progress2 = await page.locator('.w[data-widget="todo"] p', { hasText: "left ·" }).first().textContent().catch(() => "");
  const leftAfter = parseInt((progress2.match(/(\d+) left/) || [0, "0"])[1], 10);
  if (leftAfter !== leftBefore - 1) fail(`completing a task did not drop the open count: ${leftBefore} -> ${leftAfter}`);

  // Clicking a task title drills into /todo.
  await page.locator('.w[data-widget="todo"] button.dash-task').first().click();
  await page.waitForFunction(() => location.pathname.replace(/\/$/, "").endsWith("/todo"), { timeout: 5000 })
    .catch(() => fail("clicking a task title did not navigate to /todo"));

  if (!process.exitCode) console.log("PASS: To-do widget shows progress, completes tasks inline (count updates), and drills into /todo.");
} finally {
  await browser.close();
}
