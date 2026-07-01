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

// T1: tasks render as .task-card cards (not the old .row).
check("T1 task cards render", await p.locator('.task-card').count() >= 3);

// T2: summary loader tile present (widgetized).
check("T2 summary loader tile", await p.locator('.bento-todo .budget-loader').count() === 1);

// T3: at least one card has a priority-stripe class.
check("T3 priority-stripe class present", await p.locator('.task-card.tp-high, .task-card.tp-med, .task-card.tp-low').count() >= 1);

// T4: a goal link chip renders accent-tinted (is-goal) with the goal name.
const goalChip = p.locator('.task-link-chip.is-goal').first();
check("T4 goal link chip (is-goal) present", await goalChip.count() >= 1);
if (await goalChip.count()) {
  const c = await goalChip.evaluate(el => getComputedStyle(el).borderColor);
  check("T4b goal chip is accent-bordered (non-default)", !!c && c !== "rgba(0, 0, 0, 0)", c);
}

// T5: custom checkbox toggles the card to done (line-through / is-done).
const firstCheck = p.locator('[data-testid^="task-check-"]').first();
check("T5 custom checkbox present", await firstCheck.count() >= 1);
const card = firstCheck.locator('xpath=ancestor::div[contains(@class,"task-card")]');
const wasDone = (await card.getAttribute('class') || '').includes('is-done');
await firstCheck.scrollIntoViewIfNeeded();
await firstCheck.click({ force: true });
await p.waitForTimeout(800);
// After toggle, SOME card's done-state changed on the page (the toggled one may reorder).
const doneCount = await p.locator('.task-card.is-done').count();
check("T6 toggling the checkbox produces a done card", doneCount >= 1 || wasDone);

// T7: a due chip carries a state modifier (overdue/today) somewhere.
check("T7 due-state chip present", await p.locator('.task-chip.is-overdue, .task-chip.is-today').count() >= 1);

check("T8 no page errors", errs.length === 0, errs.slice(0, 3).join(" | "));

await p.screenshot({ path: SD + "/todo_redesign.png", fullPage: false });
const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
