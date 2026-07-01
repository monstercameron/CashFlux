import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const SD = "C:/Users/mreca/AppData/Local/Temp/claude/C--Users-mreca-Desktop/5aacab8d-c372-4a7d-97dc-bfed206563c6/scratchpad";
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1000 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = [];
p.on("pageerror", e => errs.push(String(e)));

await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1000);
if (await p.locator('[data-testid="hero-load-sample"]').count()) {
  await p.locator('[data-testid="hero-load-sample"]').click();
  await p.waitForTimeout(1500);
}
await p.goto(URL + "/todo", { waitUntil: "domcontentloaded" });
await p.waitForTimeout(1200);

// T1: tasks render as editorial .todo-item rows (not cards, not the old .row).
check("T1 task rows render", await p.locator('.todo-item').count() >= 3);

// T2: summary loader tile present (widgetized).
check("T2 summary loader tile", await p.locator('.bento-todo .budget-loader').count() === 1);

// T3: priority is encoded in the checkbox ring (p-high/med/low), not a badge.
check("T3 priority-ring checkbox present", await p.locator('.todo-check.p-high, .todo-check.p-med, .todo-check.p-low').count() >= 1);

// T4: a linked goal renders as an accent text-link (.todo-link.is-goal).
const goalChip = p.locator('.todo-link.is-goal').first();
check("T4 goal link (is-goal) present", await goalChip.count() >= 1);
if (await goalChip.count()) {
  const c = await goalChip.evaluate(el => getComputedStyle(el).color);
  check("T4b goal link is accent-coloured (seagreen)", c.replace(/\s/g, "") === "rgb(46,139,87)", c);
}

// T5/T6: the circular checkbox toggles the top task done. A completed task sinks below
// all open ones (and, with pagination, off the first page) — so the top row changes.
const firstTitle = (await p.locator('.todo-item .todo-title').first().textContent()) || "";
const firstCheck = p.locator('[data-testid^="task-check-"]').first();
check("T5 circular checkbox present", await firstCheck.count() >= 1);
await firstCheck.scrollIntoViewIfNeeded();
await firstCheck.click({ force: true });
await p.waitForTimeout(800);
const newFirstTitle = (await p.locator('.todo-item .todo-title').first().textContent()) || "";
const doneSomewhere = await p.locator('.todo-item.is-done').count();
check("T6 toggling the checkbox completes the task (it sinks / a done row appears)",
  newFirstTitle !== firstTitle || doneSomewhere >= 1, `${firstTitle} → ${newFirstTitle}`);

// T7: a due date carries a state modifier (overdue/today) somewhere.
check("T7 due-state present", await p.locator('.todo-due.is-overdue, .todo-due.is-today').count() >= 1);

check("T8 no page errors", errs.length === 0, errs.slice(0, 3).join(" | "));

await p.screenshot({ path: SD + "/todo_redesign.png", fullPage: false });
const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
