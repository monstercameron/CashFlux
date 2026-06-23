// GLAMOR G6 — To-do page visual + structural review for "The Money To-Do List" (Nina).
// Takes screenshots at 1280, 1440, and 768 px in dark + light themes.
// Audits: row/checkbox layout, nesting (C72), due-date cues, overdue ordering,
// unlabelled controls, light-mode contrast, tap-target sizes.
// Writes into e2e/screenshots/glamor_06_todo_*.png and glamor_06_todo_dom.json.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOTS = path.join(__dirname, "screenshots");
if (!fs.existsSync(SHOTS)) fs.mkdirSync(SHOTS, { recursive: true });

const shot = (name) => path.join(SHOTS, `glamor_06_todo_${name}.png`);

const browser = await chromium.launch({ headless: true });
const errors = [];

try {
  // ── DARK THEME ───────────────────────────────────────────────────────────────
  const dark = await browser.newPage();
  dark.on("pageerror", (e) => errors.push("dark: " + String(e)));

  // Seed dark theme in localStorage before loading.
  // Boot via root to let wasm load, then in-app navigate to /todo.
  await dark.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await dark.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await dark.waitForTimeout(600);
  await dark.locator('nav a[title="To-do"]').first().click();
  // Wait for task list card to render
  await dark.waitForSelector('.card .rows, .card .budget-head', { timeout: 30000 });
  await dark.waitForTimeout(800); // let seeded data render

  // 1280 dark — viewport + full page
  await dark.setViewportSize({ width: 1280, height: 900 });
  await dark.waitForTimeout(300);
  await dark.screenshot({ path: shot("dark_1280") });
  await dark.screenshot({ path: shot("dark_1280_full"), fullPage: true });

  // 1440 dark
  await dark.setViewportSize({ width: 1440, height: 900 });
  await dark.waitForTimeout(300);
  await dark.screenshot({ path: shot("dark_1440") });

  // 768 dark
  await dark.setViewportSize({ width: 768, height: 1024 });
  await dark.waitForTimeout(300);
  await dark.screenshot({ path: shot("dark_768") });

  // ── DOM AUDIT ────────────────────────────────────────────────────────────────
  // Reset to 1280 for the DOM audit.
  await dark.setViewportSize({ width: 1280, height: 900 });
  await dark.waitForTimeout(200);

  const domAudit = await dark.evaluate(() => {
    const rows = [...document.querySelectorAll(".row")];
    const checkboxes = [...document.querySelectorAll('input[type="checkbox"], .cb, .check, [role="checkbox"]')];
    const subTasks = [...document.querySelectorAll(".sub-task, .subtask, .child-task, .nested")];
    const dueDates = [...document.querySelectorAll(".row-meta, .due, .due-date, [class*='due']")];
    const overdueEls = [...document.querySelectorAll(".text-down, .overdue, [class*='overdue']")];
    const addForm = document.querySelector("form");
    const addInput = document.querySelector("#task-add") || document.querySelector('[data-testid="task-add-form"] input');
    const prioritySelect = document.querySelector('select[aria-label="Priority"]');
    const dueDateInput = document.querySelector('input[aria-label="Due date"]');
    const unlabelledSelects = [...document.querySelectorAll("select:not([aria-label]):not([aria-labelledby])")].map(el => ({
      name: el.name,
      id: el.id,
      options: [...el.options].map(o => o.text),
    }));
    const unlabelledButtons = [...document.querySelectorAll("button:not([aria-label]):not([title])")].filter(b => !b.textContent.trim()).map(b => ({
      class: b.className,
      html: b.outerHTML.slice(0, 100),
    }));
    const filterControls = [...document.querySelectorAll(".filter, [class*='filter'], select, input[type='text']:not(#task-add)")].map(el => ({
      tag: el.tagName,
      aria: el.getAttribute("aria-label"),
      id: el.id,
      class: el.className.slice(0, 60),
    }));
    const completedRows = [...document.querySelectorAll(".row.done, .row.completed, .row[class*='done'], .row[class*='complete']")];
    const sortControls = [...document.querySelectorAll(".sort, [class*='sort'], select")].map(el => ({
      tag: el.tagName,
      text: el.textContent.trim().slice(0, 80),
      aria: el.getAttribute("aria-label"),
    }));

    // Measure first checkbox tap target if possible.
    const firstCb = document.querySelector('input[type="checkbox"], .cb, [role="checkbox"]');
    let cbRect = null;
    if (firstCb) {
      const r = firstCb.getBoundingClientRect();
      cbRect = { width: Math.round(r.width), height: Math.round(r.height) };
    }

    // Row count above the fold.
    const aboveFold = rows.filter(r => r.getBoundingClientRect().bottom < window.innerHeight).length;

    return {
      rowCount: rows.length,
      checkboxCount: checkboxes.length,
      subTaskCount: subTasks.length,
      dueDateCount: dueDates.length,
      overdueCount: overdueEls.length,
      hasAddForm: !!addForm,
      hasAddInput: !!addInput,
      hasPrioritySelect: !!prioritySelect,
      hasDueDateInput: !!dueDateInput,
      unlabelledSelects,
      unlabelledButtons: unlabelledButtons.slice(0, 10),
      filterControls,
      completedRowCount: completedRows.length,
      sortControls,
      cbRect,
      aboveFoldRows: aboveFold,
      dataTheme: document.documentElement.getAttribute("data-theme"),
      // Sample first 5 row texts.
      sampleRows: rows.slice(0, 5).map(r => ({
        text: r.textContent.trim().slice(0, 120),
        class: r.className.slice(0, 80),
        hasMeta: !!r.querySelector(".row-meta"),
        metaText: r.querySelector(".row-meta")?.textContent?.trim().slice(0, 60) || null,
        metaClass: r.querySelector(".row-meta")?.className || null,
      })),
    };
  });

  fs.writeFileSync(
    path.join(SHOTS, "glamor_06_todo_dom.json"),
    JSON.stringify(domAudit, null, 2)
  );
  console.log("DOM audit:", JSON.stringify(domAudit, null, 2));

  await dark.close();

  // ── LIGHT THEME ──────────────────────────────────────────────────────────────
  const light = await browser.newPage();
  light.on("pageerror", (e) => errors.push("light: " + String(e)));

  await light.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await light.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await light.waitForTimeout(600);

  // Switch to light theme via localStorage (exact recipe from G4/G5).
  await light.evaluate(() =>
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" }))
  );
  await light.reload({ waitUntil: "domcontentloaded" });
  await light.waitForFunction(
    () => document.documentElement.getAttribute("data-theme") === "light"
  );
  await light.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await light.waitForTimeout(600);
  await light.locator('nav a[title="To-do"]').first().click();
  await light.waitForSelector('.card .rows, .card .budget-head', { timeout: 30000 });
  await light.waitForTimeout(800);

  // 1280 light
  await light.setViewportSize({ width: 1280, height: 900 });
  await light.waitForTimeout(300);
  await light.screenshot({ path: shot("light_1280") });

  // 1440 light
  await light.setViewportSize({ width: 1440, height: 900 });
  await light.waitForTimeout(300);
  await light.screenshot({ path: shot("light_1440") });

  // 768 light
  await light.setViewportSize({ width: 768, height: 1024 });
  await light.waitForTimeout(300);
  await light.screenshot({ path: shot("light_768") });

  const lightTheme = await light.evaluate(
    () => document.documentElement.getAttribute("data-theme")
  );
  console.log("light theme attr:", lightTheme);

  await light.close();

  // ── SUMMARY ──────────────────────────────────────────────────────────────────
  console.log("page errors:", errors.length ? errors.join(" | ") : "none");
  console.log("screenshots written:");
  [
    "dark_1280", "dark_1280_full", "dark_1440", "dark_768",
    "light_1280", "light_1440", "light_768",
  ].forEach((n) => console.log(" ", shot(n)));
} finally {
  await browser.close();
}
