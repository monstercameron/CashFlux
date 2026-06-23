// C52 — To-do screen priority filter.
// Adds tasks at each priority level, then uses the priority filter select to
// verify that selecting "High" shows only high-priority tasks and hides the
// others, and that selecting "All priorities" restores all of them.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const TS = Date.now();
const HIGH_TITLE = "HighPriTask_" + TS;
const LOW_TITLE  = "LowPriTask_"  + TS;

try {
  const page = await (await browser.newContext()).newPage();
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });

  // Inject two tasks with different priorities directly into localStorage.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  // Stash the tasks and re-apply them at document-start on the next navigation
  // (a plain localStorage edit gets clobbered by the in-memory store on the next
  // autosave; addInitScript seeds them before wasm boot so the store hydrates them).
  await page.evaluate(([hi, lo]) => {
    localStorage.setItem("e2e-prio-tasks", JSON.stringify({ hi, lo }));
  }, [HIGH_TITLE, LOW_TITLE]);
  await page.addInitScript(() => {
    const raw = localStorage.getItem("e2e-prio-tasks");
    if (!raw) return;
    localStorage.removeItem("e2e-prio-tasks"); // one-shot
    try {
      const { hi, lo } = JSON.parse(raw);
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      ds.tasks = ds.tasks || [];
      ds.tasks.push({ id: "t_hi_e2e", title: hi, status: "open", priority: "high", source: "manual" });
      ds.tasks.push({ id: "t_lo_e2e", title: lo, status: "open", priority: "low", source: "manual" });
      localStorage.setItem("cashflux:dataset", JSON.stringify(ds));
    } catch (e) { /* ignore */ }
  });

  // Navigate to the To-do screen (fresh load → addInitScript seeds the tasks).
  await page.goto(BASE + "/todo", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(800);

  // Priority filter select should be present.
  const filterSel = page.locator('[data-testid="todo-filter-prio"]');
  if ((await filterSel.count()) === 0) fail("todo-filter-prio select not found (C52)");

  // Both tasks should be visible by default ("All priorities").
  const hasHigh = async () => (await page.locator('.row', { hasText: HIGH_TITLE }).count()) > 0;
  const hasLow  = async () => (await page.locator('.row', { hasText: LOW_TITLE  }).count()) > 0;

  if (!(await hasHigh())) fail(`high-priority task "${HIGH_TITLE}" not visible by default`);
  if (!(await hasLow()))  fail(`low-priority task "${LOW_TITLE}" not visible by default`);

  // Select "High" priority filter.
  await filterSel.selectOption("high");
  await page.waitForTimeout(550);

  if (!(await hasHigh())) fail(`high-priority task should be visible when filtering to High`);
  if (await hasLow())     fail(`low-priority task should be hidden when filtering to High`);

  // Select "Low" priority filter.
  await filterSel.selectOption("low");
  await page.waitForTimeout(550);

  if (await hasHigh())    fail(`high-priority task should be hidden when filtering to Low`);
  if (!(await hasLow()))  fail(`low-priority task should be visible when filtering to Low`);

  // Reset to "All priorities". Poll the restored rows — the re-render after the
  // select change can lag a frame in an isolated browser context.
  await filterSel.selectOption("");
  const poll = async (fn) => { for (let i = 0; i < 20; i++) { if (await fn()) return true; await page.waitForTimeout(200); } return false; };
  if (!(await poll(hasHigh))) fail(`high-priority task should be visible again after filter reset`);
  if (!(await poll(hasLow)))  fail(`low-priority task should be visible again after filter reset`);

  if (!process.exitCode) console.log("PASS: To-do priority filter shows/hides tasks by priority level and resets correctly (C52).");
} finally {
  await browser.close();
}
