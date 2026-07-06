import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1000 } });
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
await p.goto(URL + "/todo", { waitUntil: "domcontentloaded" });
await p.waitForTimeout(1000);
await p.locator('[data-testid="todo-sort"]').selectOption("az");
await p.waitForTimeout(500);

check("T1 no sub-tasks initially", await p.locator('.todo-item.is-subtask').count() === 0);

// Add a sub-task to the first task via ⋯ → Add sub-task → prompt.
const mb = p.locator('[id^="task-menu-"] button').first();
await mb.scrollIntoViewIfNeeded();
await mb.click({ force: true });
await p.waitForTimeout(250);
const addSubItem = p.locator('[data-testid^="task-addsub-"]').first();
check("T1b menu item reads 'Add sub-task' (not '+ Sub')", ((await addSubItem.textContent()) || "").trim() === "Add sub-task", (await addSubItem.textContent()));
await addSubItem.click({ force: true });
await p.waitForTimeout(350);
await p.locator('#cf-dialog-input').fill("Buy the crib mattress");
await p.locator('#cf-dialog-confirm').click();
await p.waitForTimeout(800);

const r = await p.evaluate(() => {
  const sub = document.querySelector('.todo-item.is-subtask');
  if (!sub) return { found: false };
  const cs = getComputedStyle(sub);
  const parent = sub.previousElementSibling;
  const chk = sub.querySelector('.todo-check');
  const rc = sub.getBoundingClientRect(), pr = parent ? parent.getBoundingClientRect() : null;
  return {
    found: true,
    padLeft: parseFloat(cs.paddingLeft),
    hasArrow: !!sub.querySelector('.todo-subarrow'),
    checkW: chk ? parseFloat(getComputedStyle(chk).width) : 999,
    title: sub.querySelector('.todo-title')?.textContent || "",
    prevTitle: parent ? (parent.querySelector('.todo-title')?.textContent || "") : "",
    below: pr ? rc.top >= pr.top : false,
    leftOK: rc.left >= 0 && rc.left < 300,
    scrollLeft: document.scrollingElement.scrollLeft,
    hOverflow: document.documentElement.scrollWidth > window.innerWidth + 2,
  };
});

check("T2 sub-task renders", r.found && r.title === "Buy the crib mattress", r.title);
check("T3 nested directly under its parent", r.below && r.prevTitle.includes("crib and changing table"), r.prevTitle);
check("T4 indented (padding-left > base)", r.padLeft >= 30, `${r.padLeft}px`);
check("T5 has ↳ connector", r.hasArrow);
check("T6 smaller check ring", r.checkW > 0 && r.checkW <= 21, `${r.checkW}px`);
check("T7 no horizontal page overflow", !r.hOverflow && r.scrollLeft === 0, JSON.stringify({ sl: r.scrollLeft, ov: r.hOverflow }));
check("T8 sub-task not clipped off-screen", r.leftOK);
// --- collapse + summary + container-aware menu ---
// The parent now shows a "N/M" sub-task summary chip and a disclosure chevron.
const parentCard = p.locator('.todo-item:not(.is-subtask)').filter({ hasText: "Assemble the crib and changing table" }).first();
check("T10 parent shows a sub-task summary chip", await parentCard.locator('[data-testid^="task-substat-"]').count() >= 1);
const disclose = parentCard.locator('[data-testid^="task-collapse-"]').first();
check("T11 parent has a disclosure toggle", await disclose.count() === 1);

// Collapsing the parent hides the sub-task; expanding shows it again.
await disclose.scrollIntoViewIfNeeded();
await disclose.click({ force: true });
await p.waitForTimeout(500);
check("T12 collapse hides the sub-task", await p.locator('.todo-item.is-subtask').count() === 0);
check("T13 parent summary still visible when collapsed", await parentCard.locator('[data-testid^="task-substat-"]').count() >= 1);
await disclose.click({ force: true });
await p.waitForTimeout(500);
check("T14 expand shows the sub-task again", await p.locator('.todo-item.is-subtask').count() === 1);

// Container-aware ⋯ menu: opening it must not push the page into horizontal overflow.
const kebab = parentCard.locator('[data-testid^="task-menu-btn-"]').first();
check("T15 kebab menu button present", await kebab.count() === 1);
await kebab.click({ force: true });
await p.waitForTimeout(350);
const menuOv = await p.evaluate(() => ({
  overflow: document.documentElement.scrollWidth > window.innerWidth + 2,
  menuRight: (() => { const m = document.querySelector('.add-menu:not(.hidden-menu)'); return m ? Math.round(m.getBoundingClientRect().right) : -1; })(),
  iw: window.innerWidth,
}));
check("T16 open ⋯ menu does not overflow the viewport", !menuOv.overflow && menuOv.menuRight <= menuOv.iw + 1, JSON.stringify(menuOv));
// close the parent menu
await p.keyboard.press("Escape").catch(() => {});
await p.waitForTimeout(200);

// The SUB-TASK's own ⋯ menu must also be container-aware (flip left / stay in view).
const subKebab = p.locator('.todo-item.is-subtask [data-testid^="task-menu-btn-"]').first();
check("T17 sub-task has its own ⋯ menu", await subKebab.count() === 1);
await subKebab.scrollIntoViewIfNeeded();
await subKebab.click({ force: true });
await p.waitForTimeout(350);
const subMenu = await p.evaluate(() => {
  const m = document.querySelector('.add-menu:not(.hidden-menu)');
  const rc = m ? m.getBoundingClientRect() : null;
  return {
    right: rc ? Math.round(rc.right) : -1,
    inView: rc ? (rc.right <= window.innerWidth + 1 && rc.left >= -1) : false,
    overflow: document.documentElement.scrollWidth > window.innerWidth + 2,
    iw: window.innerWidth,
  };
});
check("T18 sub-task ⋯ menu stays inside the viewport (container-aware)", subMenu.inView && !subMenu.overflow, JSON.stringify(subMenu));

check("T9 no page errors", errs.length === 0, errs.slice(0, 3).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
