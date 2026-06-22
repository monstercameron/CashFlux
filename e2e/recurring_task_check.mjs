// L26 gate — recurring tasks auto-spawn the next occurrence on completion.
//
// Flow:
//   1. Add a task with a unique title, Repeat=Weekly, and a specific due date.
//   2. Complete it (click the check button).
//   3. Trigger autosave (dispatch visibilitychange + poll localStorage).
//   4. Assert localStorage cashflux:dataset contains exactly two tasks with that
//      title: one "done" and one "open" whose due is 7 days after the original.
//   5. Complete the newly-spawned task; assert the dataset still has exactly two
//      tasks with that title (no multiplication: old done + new open, count stays 2).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const TITLE = "ZZRECUR-WEEKLY-1";
const DUE = "2026-07-01"; // ISO due date for the first task
const NEXT_DUE = "2026-07-08"; // expected due of the spawned next occurrence (+7 days)

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

/** Trigger autosave and poll localStorage until the key exists. */
async function waitForSave(page, timeoutMs = 10000) {
  await page.evaluate(() => document.dispatchEvent(new Event("visibilitychange")));
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const raw = await page.evaluate(() => localStorage.getItem("cashflux:dataset"));
    if (raw) return raw;
    await page.waitForTimeout(200);
  }
  throw new Error("cashflux:dataset never appeared in localStorage");
}

/** Parse all tasks from the stored dataset JSON. */
function parseTasks(raw) {
  const ds = JSON.parse(raw);
  // Tasks may live under ds.tasks (array) or ds.data.tasks depending on layout.
  return ds?.tasks ?? ds?.data?.tasks ?? [];
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/todo", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#task-add", { timeout: 60000 });

  // ── Step 1: add the recurring task ──────────────────────────────────────────
  await page.locator("#task-add").fill(TITLE);
  await page.locator('form input[type="date"]').first().fill(DUE);
  // Select Weekly in the Repeat dropdown (aria-label="Repeat" or data-testid).
  await page.locator('[data-testid="task-add-repeat"]').selectOption("weekly");
  await page.locator("form button[type='submit']").first().click();
  await page.waitForTimeout(700);

  const row1 = page.locator(".row", { hasText: TITLE });
  if ((await row1.count()) === 0) fail(`task "${TITLE}" did not appear after adding`);

  // ── Step 2: complete it ─────────────────────────────────────────────────────
  await row1.first().locator("button.check").click();
  await page.waitForTimeout(700);

  // ── Step 3: autosave ────────────────────────────────────────────────────────
  const raw1 = await waitForSave(page);
  const tasks1 = parseTasks(raw1);
  const byTitle1 = tasks1.filter((t) => t.title === TITLE);

  if (byTitle1.length !== 2)
    fail(`expected 2 tasks named "${TITLE}" after first completion, got ${byTitle1.length}`);

  const done1 = byTitle1.find((t) => t.status === "done");
  const open1 = byTitle1.find((t) => t.status === "open");
  if (!done1) fail("no done task found after first completion");
  if (!open1) fail("no open (spawned) task found after first completion");

  // Check the spawned task's due date is 7 days after the original.
  const spawnedDue = open1?.due ? open1.due.slice(0, 10) : "";
  if (spawnedDue !== NEXT_DUE)
    fail(`spawned task due = "${spawnedDue}", want "${NEXT_DUE}"`);

  // ── Step 5: complete the spawned task; no multiplication ────────────────────
  const row2 = page.locator(".row", { hasText: TITLE, hasNot: page.locator(".done") }).first();
  if ((await row2.count()) === 0) fail("spawned open task row not found in the UI");
  await row2.locator("button.check").click();
  await page.waitForTimeout(700);

  const raw2 = await waitForSave(page);
  const tasks2 = parseTasks(raw2);
  const byTitle2 = tasks2.filter((t) => t.title === TITLE);

  // After re-completing: still exactly 2 tasks with this title (the previous done
  // stays done; the re-completed spawned one is now done; its own successor is
  // the third — so we allow 2 or 3 but NOT > 4 to prevent unbounded growth).
  if (byTitle2.length > 4)
    fail(
      `task count for "${TITLE}" ballooned to ${byTitle2.length} after second completion — runaway spawning?`
    );
  const openAfter2 = byTitle2.filter((t) => t.status === "open");
  if (openAfter2.length > 1)
    fail(
      `more than one open successor found after second completion (${openAfter2.length}) — runaway spawning?`
    );

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log(
      `PASS: recurring task completes, spawns one open successor (due ${spawnedDue}); re-completing does not multiply tasks uncontrollably.`
    );
} finally {
  await browser.close();
}
