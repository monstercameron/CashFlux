// B16 E2E story — "to-do add + complete toggle". Adds a task, marks it complete,
// and asserts both UX (the row shows done) and correctness (the task's status
// flips to "done" in the dataset and survives a reload). Exits non-zero on fail.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const TITLE = "ZZTODO-88";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const taskByTitle = (page, title) =>
  page.evaluate((t) => {
    const data = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    let found = null;
    const walk = (o) => {
      if (!o || typeof o !== "object") return;
      if (Array.isArray(o)) return o.forEach(walk);
      if (o.title === t && o.status) found = o;
      Object.values(o).forEach(walk);
    };
    walk(data);
    return found;
  }, title);

async function waitForTask(page, title, pred, timeoutMs = 7000) {
  let t = null;
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    t = await taskByTitle(page, title);
    if (pred(t)) return t;
    await page.waitForTimeout(400);
  }
  return t;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/todo", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#task-add", { timeout: 60000 });
  if ((await page.getByText(TITLE).count()) !== 0) fail("test task already present before adding");

  // Add the task.
  await page.locator("#task-add").fill(TITLE);
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(600);
  const row = page.locator(".row", { hasText: TITLE });
  if ((await row.count()) === 0) fail("task did not appear after adding");
  if ((await page.locator(".row.done", { hasText: TITLE }).count()) !== 0) fail("task should start not-done");
  const open = await waitForTask(page, TITLE, (t) => !!t);
  if (!open) fail("task not found in the dataset");
  else if (open.status !== "open") fail(`new task status = ${open.status}, want "open"`);

  // Mark it complete (the row's check button).
  await page.locator(".row", { hasText: TITLE }).locator("button.check").first().click();
  const doneTask = await waitForTask(page, TITLE, (t) => t && t.status === "done");
  if (!doneTask || doneTask.status !== "done") fail(`task status after toggle = ${doneTask && doneTask.status}, want "done"`);
  if ((await page.locator(".row.done", { hasText: TITLE }).count()) === 0) fail("the row should show the done state after toggling");

  // Survives reload.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#task-add", { timeout: 60000 });
  await page.waitForTimeout(800);
  const after = await taskByTitle(page, TITLE);
  if (!after || after.status !== "done") fail("done status did not survive a reload");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: added to-do "${TITLE}", marked complete (open → done), persists across reload.`);
} finally {
  await browser.close();
}
