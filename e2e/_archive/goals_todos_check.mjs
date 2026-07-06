import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const SD = "C:/Users/mreca/AppData/Local/Temp/claude/C--Users-mreca-Desktop/5aacab8d-c372-4a7d-97dc-bfed206563c6/scratchpad";
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1100 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = [];
p.on("pageerror", e => errs.push(String(e)));

await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) {
  await p.locator('[data-testid="hero-load-sample"]').click();
  await p.waitForTimeout(1500);
}
await p.goto(URL + "/goals", { waitUntil: "domcontentloaded" });
await p.waitForTimeout(1500);

// T1: goals with linked to-dos render a TO-DOS section (sample: Baby fund, Emergency fund).
check("T1 linked-todo sections render", await p.locator('[data-testid^="goal-todos-"]').count() >= 2);
check("T2 linked todo rows render", await p.locator('.goal-todo').count() >= 3);

// Emergency fund has exactly one linked to-do (0/1), open.
const emCard = p.locator('.goal-card').filter({ hasText: "Emergency fund (3 months)" }).first();
const emTodos = emCard.locator('[data-testid^="goal-todos-"]').first();
check("T3 Emergency fund shows its linked to-do", (await emTodos.textContent() || "").includes("Build a real emergency fund"));
check("T4 count reads 0/1", (await emCard.locator('.goal-todos-count').textContent() || "").trim() === "0/1");
check("T5 a done linked to-do is struck through somewhere", await p.locator('.goal-todo-title.is-done').count() >= 1);

// T6: toggling the linked to-do marks it done → count becomes 1/1 (live).
const chk = emCard.locator('.goal-todo-check').first();
await chk.scrollIntoViewIfNeeded();
await chk.click({ force: true });
await p.waitForTimeout(700);
const emCard2 = p.locator('.goal-card').filter({ hasText: "Emergency fund (3 months)" }).first();
check("T6 toggling updates the count to 1/1", (await emCard2.locator('.goal-todos-count').textContent() || "").trim() === "1/1", await emCard2.locator('.goal-todos-count').textContent());

// T7: no page errors.
check("T7 no page errors", errs.length === 0, errs.slice(0, 3).join(" | "));

await p.screenshot({ path: SD + "/goals_todos.png", fullPage: false });
const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
