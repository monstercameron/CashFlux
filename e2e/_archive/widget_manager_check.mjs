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

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

// Layout state now persists into the SQLite dataset (appkv → IndexedDB), not
// localStorage, so assert through the LEDGER UI, which reads the same shared
// atoms the dashboard does: row order and each row's rendered W×H value.
const ledgerNames = (page) => page.locator(".wman-ledger .wm-row .wm-name").allTextContents();
const ledgerSpan = async (page, name) => {
  const t = await page.locator(".wman-ledger .wm-row").filter({ hasText: name }).first().locator(".wm-static").first().textContent();
  const m = /(\d+)×(\d+)/.exec(t || "");
  return m ? [Number(m[1]), Number(m[2])] : null;
};

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
  await page.evaluate(() => { history.pushState({}, "", "/widget-manager"); dispatchEvent(new PopStateEvent("popstate")); });
  await page.waitForSelector(".wm-row", { timeout: 10000 });
  await page.waitForTimeout(400);

  // 1) Hide "Recent transactions" → it leaves the dashboard.
  const recentRow = page.locator(".wm-row").filter({ hasText: "Recent transactions" }).first();
  if ((await recentRow.count()) === 0) fail("manager is missing the Recent transactions row");
  await recentRow.locator(".switch").first().click();
  await page.waitForTimeout(300);

  // 2) Resize "To-do": bump its width and confirm the shared layout state changed.
  const todoBefore = (await ledgerSpan(page, "To-do")) || [1, 1];
  await page.locator(".wm-row").filter({ hasText: "To-do" }).first().locator('button[aria-label="Wider"]').click();
  await page.waitForTimeout(250);
  const todoAfter = await ledgerSpan(page, "To-do");
  if (!todoAfter || todoAfter[0] !== Math.min(4, todoBefore[0] + 1)) {
    fail(`resize did not apply: To-do width ${todoBefore} -> ${todoAfter}`);
  }

  // 3) Reorder: move the first widget down and confirm the order changed.
  const orderBefore = await ledgerNames(page);
  await page.locator(".wm-row").first().locator('button[aria-label="Move down"]').click();
  await page.waitForTimeout(250);
  const orderAfter = await ledgerNames(page);
  if (orderBefore[0] === orderAfter[0]) fail(`reorder did not move the first widget: ${orderBefore[0]} still first`);
  if (orderAfter[1] !== orderBefore[0]) fail(`reorder put the wrong widget second: ${orderAfter.slice(0, 2)} vs expected ${orderBefore[0]} at index 1`);

  // Back to the dashboard: hidden widget is gone, others remain.
  await page.locator('a[title="Dashboard"]').first().click();
  await page.waitForSelector(".bento", { timeout: 10000 });
  await page.waitForTimeout(500);
  if (await bentoHas("Recent transactions")) fail("hidden widget still renders on the dashboard");
  if (!(await bentoHas("Needs attention"))) fail("a visible widget went missing after hiding another");

  // Styling sanity: the size controls are the compact bordered steppers (not the
  // wide period pills), the ledger + board map render, and nothing overflows the
  // page horizontally. (The .wm-table DataTable became the bespoke .wman ledger.)
  await page.evaluate(() => { history.pushState({}, "", "/widget-manager"); dispatchEvent(new PopStateEvent("popstate")); });
  await page.waitForSelector(".wm-row", { timeout: 10000 });
  const styleOK = await page.evaluate(() => {
    const ledger = document.querySelector(".wman-ledger");
    const map = document.querySelectorAll(".wman-map .wman-map-tile").length;
    const rows = document.querySelectorAll(".wman-ledger .wm-row").length;
    const sizeCell = document.querySelector(".wm-row .wm-col-size");
    const steps = document.querySelectorAll(".wm-row .wm-step").length;
    const strayPill = document.querySelector(".wm-size .rpill"); // the old janky control
    const pageClip = document.documentElement.scrollWidth > document.documentElement.clientWidth + 2;
    return {
      ledger: !!ledger, map, rows,
      sizeW: sizeCell ? Math.round(sizeCell.getBoundingClientRect().width) : 0,
      steps, strayPill: !!strayPill, pageClip,
    };
  });
  if (styleOK.strayPill) fail("size control is still the wide period pill (.rpill), not the compact stepper");
  if (styleOK.steps === 0) fail("compact size steppers (.wm-step) are missing");
  if (!styleOK.ledger) fail("the .wman-ledger is missing");
  if (styleOK.map === 0) fail("the board map (.wman-map-tile) is missing");
  if (styleOK.map !== styleOK.rows) fail(`board map tiles (${styleOK.map}) != ledger rows (${styleOK.rows})`);
  if (styleOK.pageClip) fail("manager overflows the page horizontally");
  if (styleOK.sizeW > 260) fail(`size column is too wide (${styleOK.sizeW}px) — controls look stretched`);

  if (!process.exitCode) console.log("PASS: manager hide removes the dashboard tile; resize + reorder persist to the layout.");
} finally {
  await browser.close();
}

