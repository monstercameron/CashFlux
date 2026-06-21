// Widget Manager (Phase 1): hiding a widget removes it from the dashboard;
// resizing and reordering in the manager persist to the layout the dashboard
// reads. Proves the manager's controls are wired back into the dashboard. Exits
// non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

// dashlayout.Item has no JSON tags, so localStorage keys are the Go field names.
const layoutIDs = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:layout") || "[]").map((i) => i.ID));
const layoutSpan = (page, id) => page.evaluate((wid) => {
  const it = JSON.parse(localStorage.getItem("cashflux:layout") || "[]").find((x) => x.ID === wid);
  return it ? [it.ColSpan || 1, it.RowSpan || 1] : null;
}, id);

try {
  const page = await (await browser.newContext()).newPage();
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".bento", { timeout: 60000 });
  await page.waitForTimeout(800);

  // Baseline: the Recent transactions tile is on the dashboard.
  const bentoHas = (t) => page.evaluate((s) => (document.querySelector(".bento")?.innerText || "").includes(s), t);
  if (!(await bentoHas("Recent transactions"))) fail("baseline: Recent transactions tile should be on the dashboard");

  // Open the manager.
  await page.locator('a[title="Widget manager"]').first().click();
  await page.waitForSelector(".wm-row", { timeout: 10000 });
  await page.waitForTimeout(400);

  // 1) Hide "Recent transactions" → it leaves the dashboard.
  const recentRow = page.locator(".wm-row").filter({ hasText: "Recent transactions" }).first();
  if ((await recentRow.count()) === 0) fail("manager is missing the Recent transactions row");
  await recentRow.locator(".switch").first().click();
  await page.waitForTimeout(300);

  // 2) Resize "To-do": bump its width and confirm the layout persists the change.
  const todoBefore = (await layoutSpan(page, "todo")) || [1, 1]; // unsaved layout = defaults (todo is 1x1)
  await page.locator(".wm-row").filter({ hasText: "To-do" }).first().locator('button[aria-label="Wider"]').click();
  await page.waitForTimeout(250);
  const todoAfter = await layoutSpan(page, "todo");
  if (!todoAfter || todoAfter[0] !== Math.min(4, todoBefore[0] + 1)) {
    fail(`resize did not persist: todo width ${todoBefore} -> ${todoAfter}`);
  }

  // 3) Reorder: move the first widget down and confirm the order persists.
  const orderBefore = await layoutIDs(page);
  await page.locator(".wm-row").first().locator('button[aria-label="Move down"]').click();
  await page.waitForTimeout(250);
  const orderAfter = await layoutIDs(page);
  if (orderBefore[0] === orderAfter[0]) fail(`reorder did not move the first widget: ${orderBefore[0]} still first`);
  if (orderAfter[1] !== orderBefore[0]) fail(`reorder put the wrong widget second: ${orderAfter.slice(0, 2)} vs expected ${orderBefore[0]} at index 1`);

  // Back to the dashboard: hidden widget is gone, others remain.
  await page.locator('a[title="Dashboard"]').first().click();
  await page.waitForSelector(".bento", { timeout: 10000 });
  await page.waitForTimeout(500);
  if (await bentoHas("Recent transactions")) fail("hidden widget still renders on the dashboard");
  if (!(await bentoHas("Needs attention"))) fail("a visible widget went missing after hiding another");

  if (!process.exitCode) console.log("PASS: manager hide removes the dashboard tile; resize + reorder persist to the layout.");
} finally {
  await browser.close();
}
