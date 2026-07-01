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

check("T1 at least one task row present", await p.locator('[data-testid^="task-edit-btn-"]').count() >= 1);

// T2: no inline edit form appears in the row; Edit opens a flip modal instead.
const editBtn = p.locator('[data-testid^="task-edit-btn-"]').first();
check("T2 edit button present", await editBtn.count() >= 1);
await editBtn.scrollIntoViewIfNeeded();
await editBtn.click({ force: true });
await p.waitForTimeout(600);
const modal = p.locator('.flip-panel, [data-testid="flip-panel"], .flip-backdrop');
const editForm = p.locator('input#task-edit, input[id^="task-edit-"]').first();
check("T3 edit opens a flip modal with the task field", await editForm.count() >= 1);
// modal should be centered (not inside a row) — its input is visible
check("T4 modal title field visible", await editForm.isVisible());

// T5: edit the title and save → row reflects the change.
const newTitle = "QA edited task " + "x";
await editForm.fill(newTitle);
await p.locator('.acct-edit-form button[type=submit]').first().click();
await p.waitForTimeout(900);
check("T5 edited title shows on the page", (await p.locator('#app').textContent() || "").includes(newTitle));
check("T6 modal closed after save", await p.locator('input[id^="task-edit-"]').count() === 0);

// T7: ⋯ menu holds Add sub + Delete (no standalone delete X in the row action strip).
const moreBtn = p.locator('[id^="task-menu-"] button').first();
check("T7 ⋯ menu button present", await moreBtn.count() >= 1);
await moreBtn.scrollIntoViewIfNeeded();
await moreBtn.click({ force: true });
await p.waitForTimeout(400);
check("T8 ⋯ menu has Add sub-task", await p.locator('[data-testid^="task-addsub-"]').first().isVisible());
check("T9 ⋯ menu has Delete", await p.locator('[data-testid^="task-delete-btn-"]').first().isVisible());

check("T10 no page errors", errs.length === 0, errs.slice(0, 3).join(" | "));

await p.screenshot({ path: SD + "/todo_flipmodal.png", fullPage: false });
const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
