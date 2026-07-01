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

const titles = async () => p.locator('.todo-item .todo-title').allInnerTexts();

// T1: sort + pager controls exist (sample data has > 20 tasks → 2 pages).
check("T1 sort control present", await p.locator('[data-testid="todo-sort"]').count() === 1);
const hasPager = await p.locator('[data-testid="todo-pager-range"]').count() === 1;
check("T2 pager present (multi-page dataset)", hasPager);

// T3: A–Z sort orders the visible titles alphabetically (first page).
await p.locator('[data-testid="todo-sort"]').selectOption("az");
await p.waitForTimeout(700);
const az = await titles();
const azSorted = [...az].sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
check("T3 A–Z sort orders titles alphabetically", JSON.stringify(az) === JSON.stringify(azSorted), az.slice(0, 3).join(" | "));

// T4: sorting resets to page 1.
const rangeText = async () => (await p.locator('[data-testid="todo-pager-range"]').textContent().catch(() => "")) || "";
check("T4 sort reset to page 1", (await rangeText()).trim().startsWith("1–"), await rangeText());

// T5: Next advances the page; the first title changes and the range moves.
const page1First = (await titles())[0];
const range1 = await rangeText();
await p.locator('[data-testid="todo-next"]').click();
await p.waitForTimeout(700);
const page2First = (await titles())[0];
const range2 = await rangeText();
check("T5 Next changes the page (range moved)", range1 !== range2 && !range2.trim().startsWith("1–"), `${range1} → ${range2}`);
check("T6 page 2 shows different tasks", page1First !== page2First, `${page1First} vs ${page2First}`);

// T7: on the last page, Next is disabled.
check("T7 Next disabled on last page", await p.locator('[data-testid="todo-next"]').isDisabled());

// T8: Prev returns to page 1.
await p.locator('[data-testid="todo-prev"]').click();
await p.waitForTimeout(700);
check("T8 Prev returns to page 1", (await rangeText()).trim().startsWith("1–"), await rangeText());
check("T9 Prev disabled on page 1", await p.locator('[data-testid="todo-prev"]').isDisabled());

// T10: Priority sort puts a high-priority ring on the first row.
await p.locator('[data-testid="todo-sort"]').selectOption("priority");
await p.waitForTimeout(700);
check("T10 priority sort → first row is high priority", await p.locator('.todo-item').first().locator('.todo-check.p-high').count() === 1);

check("T11 no page errors", errs.length === 0, errs.slice(0, 3).join(" | "));

await p.screenshot({ path: SD + "/todo_sort_page.png", fullPage: false });
const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
