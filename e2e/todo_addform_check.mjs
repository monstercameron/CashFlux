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
await p.waitForTimeout(1000);

// Open the redesigned add-task modal.
await p.locator('[data-testid="todo-add"]').click();
await p.waitForTimeout(600);
const form = p.locator('[data-testid="task-add-form"]');
check("T1 add-task form opens", await form.count() === 1);

// T2: hero (Fraunces) title field + segmented priority control present.
check("T2 hero title field", await form.locator('.tc-title').count() === 1);
check("T3 segmented priority present", await form.locator('.task-seg [data-testid^="task-prio-"]').count() === 3);
// signature: the writing zone's priority "spine" reflects the selected priority.
check("T3b writing zone carries priority class", await form.locator('.tc-write.p-med').count() === 1);
// Medium is the default active segment.
check("T4 medium is default active", (await p.locator('[data-testid="task-prio-med"]').getAttribute('class') || '').includes('is-active'));

// T5: clicking High activates that segment, deactivates Medium, and tints the spine red.
await p.locator('[data-testid="task-prio-high"]').click();
await p.waitForTimeout(200);
check("T5 High activates + spine turns high",
  (await p.locator('[data-testid="task-prio-high"]').getAttribute('class') || '').includes('is-active') &&
  !(await p.locator('[data-testid="task-prio-med"]').getAttribute('class') || '').includes('is-active') &&
  await p.locator('.tc-write.p-high').count() === 1);

// T5c: the footer live-summary reflects the chosen priority.
check("T5c live summary shows priority", (await p.locator('[data-testid="task-summary"]').textContent() || '').toLowerCase().includes('high'));

// T6: a quick-date chip fills the due input.
await p.locator('[data-testid="task-quick-today"]').click();
await p.waitForTimeout(200);
const dueVal = await form.locator('input[type=date]').inputValue();
check("T6 'Today' quick chip fills the date", !!dueVal && /\d{4}-\d{2}-\d{2}/.test(dueVal), dueVal);
// Clear empties it.
await p.locator('[data-testid="task-quick-clear"]').click();
await p.waitForTimeout(200);
check("T7 'Clear' empties the date", (await form.locator('input[type=date]').inputValue()) === "");

// T8: empty title blocks submit with an inline error.
await p.locator('[data-testid="task-add-submit"]').click();
await p.waitForTimeout(300);
check("T8 empty title blocked (form still open)", await p.locator('[data-testid="task-add-form"]').count() === 1);

// T9: filling the title + Add creates the task and closes the modal.
const uniq = "Book the newborn photoshoot QA";
await form.locator('#task-add').fill(uniq);
await p.locator('[data-testid="task-add-submit"]').click();
await p.waitForTimeout(900);
check("T9 modal closes after add", await p.locator('[data-testid="task-add-form"]').count() === 0);
// The new undated task sorts to the end under Smart order (may land on page 2); sort A–Z
// so "Book…" surfaces near the top of page 1, proving it was actually persisted.
await p.locator('[data-testid="todo-sort"]').selectOption("az");
await p.waitForTimeout(700);
check("T10 new task appears in the list", (await p.locator('#app').textContent() || '').includes(uniq));

// T11: reopen + Cancel closes without adding.
await p.locator('[data-testid="todo-add"]').click();
await p.waitForTimeout(500);
await p.locator('.tc-foot-actions .btn:not(.btn-primary)').first().click();
await p.waitForTimeout(400);
check("T11 Cancel closes the modal", await p.locator('[data-testid="task-add-form"]').count() === 0);

check("T12 no page errors", errs.length === 0, errs.slice(0, 3).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
