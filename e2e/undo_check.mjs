// C78 — undo/redo (Ctrl+Z / Ctrl+Shift+Z). Adds a task, flushes autosave so
// captureUndoPoint fires, presses Ctrl+Z, and asserts the task is gone.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
try {
  const page = await (await browser.newContext()).newPage();
  page.on("dialog", async (d) => { fail("a NATIVE dialog opened: " + d.type()); await d.dismiss(); });
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title], .bento', { timeout: 60000 });
  await page.waitForTimeout(700);

  // Navigate to the To-do screen via the nav link.
  await page.locator('a[title="To-do"]').first().click();
  await page.waitForTimeout(500);

  // Add a uniquely-labelled task so we can assert its presence/absence reliably.
  const taskLabel = "UndoE2ETask_" + Date.now();
  const addInput = page.locator("#task-add");
  await addInput.waitFor({ timeout: 8000 });
  await addInput.fill(taskLabel);
  await page.getByRole("button", { name: "Add", exact: true }).first().click();
  await page.waitForTimeout(400);

  // Verify the task was added before undoing.
  const taskBefore = page.locator(`text=${taskLabel}`);
  if ((await taskBefore.count()) === 0) {
    fail("task was not added — cannot test undo");
  }

  // Flush autosave so the mutation is captured as an undo point.
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(300);

  // Undo the add: the task should disappear.
  await page.keyboard.press("Control+z");
  await page.waitForTimeout(500);

  const taskAfter = page.locator(`text=${taskLabel}`);
  if ((await taskAfter.count()) !== 0) {
    fail("task still visible after Ctrl+Z — undo did not revert the add");
  }

  if (!process.exitCode) {
    console.log("PASS: task added, autosave captured, Ctrl+Z removed it (undo works end-to-end).");
  }
} finally {
  await browser.close();
}
