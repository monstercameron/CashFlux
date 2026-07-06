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
await p.goto(URL + "/goals", { waitUntil: "domcontentloaded" });
await p.waitForTimeout(1200);

// Helper: open the Add goal modal, fill common name + a kind, submit.
async function addGoal({ name, kind, habitTarget }) {
  await p.locator('[data-testid="goals-add"]').first().click();
  await p.waitForTimeout(500);
  const form = p.locator('[data-testid="goal-add-form"]');
  await form.waitFor({ timeout: 8000 });
  await form.locator('#goal-add').fill(name);
  await form.locator('[data-testid="goal-add-kind"]').selectOption(kind);
  await p.waitForTimeout(300);
  if (kind === "financial") {
    await form.locator('input[type=number]').first().fill("500");
  }
  if (kind === "habit") {
    await form.locator('[data-testid="goal-add-cadence"]').selectOption("weekly");
    await form.locator('[data-testid="goal-add-habit-target"]').fill(String(habitTarget || 4));
  }
  await form.locator('button[type=submit]').click();
  await p.waitForTimeout(900);
}

// T1: kind selector present + hint updates when kind changes.
await p.locator('[data-testid="goals-add"]').first().click();
await p.waitForTimeout(500);
const kindSel = p.locator('[data-testid="goal-add-kind"]');
check("T1 kind selector exists in add modal", await kindSel.count() === 1);
const financialHint = await p.locator('[data-testid="goal-add-kind-hint"]').textContent().catch(() => "");
await kindSel.selectOption("checklist");
await p.waitForTimeout(300);
const checklistHint = await p.locator('[data-testid="goal-add-kind-hint"]').textContent().catch(() => "");
check("T2 hint changes with kind", financialHint !== checklistHint, JSON.stringify({ financialHint, checklistHint }));
// habit reveals cadence + target fields
await kindSel.selectOption("habit");
await p.waitForTimeout(300);
check("T3 habit reveals cadence + target fields",
  (await p.locator('[data-testid="goal-add-cadence"]').count()) === 1 &&
  (await p.locator('[data-testid="goal-add-habit-target"]').count()) === 1);
// financial hides them again
await kindSel.selectOption("financial");
await p.waitForTimeout(300);
check("T4 financial hides habit fields", (await p.locator('[data-testid="goal-add-cadence"]').count()) === 0);
// close modal (press Escape / click backdrop)
await p.keyboard.press("Escape").catch(() => {});
await p.waitForTimeout(400);
// if still open, submit-cancel by reloading goals
await p.goto(URL + "/goals", { waitUntil: "domcontentloaded" });
await p.waitForTimeout(800);

// T5: create a CHECKLIST goal → card shows data-kind + "No steps linked yet".
await addGoal({ name: "Plan the trip", kind: "checklist" });
const checklistCard = p.locator('.goal-card[data-kind="checklist"]').filter({ hasText: "Plan the trip" });
check("T5 checklist goal card created", await checklistCard.count() >= 1);
check("T6 checklist card shows 'No steps linked yet'", (await checklistCard.first().textContent() || "").includes("No steps"));

// T7: create a MILESTONE goal → mark done → shows "Done".
await addGoal({ name: "Renew passport", kind: "milestone" });
const msCard = () => p.locator('.goal-card[data-kind="milestone"]').filter({ hasText: "Renew passport" }).first();
check("T7 milestone card created", await p.locator('.goal-card[data-kind="milestone"]').filter({ hasText: "Renew passport" }).count() >= 1);
const markBtn = msCard().locator('[data-testid^="goal-markdone-"]');
check("T8 milestone shows Mark done action", await markBtn.count() === 1);
await markBtn.click();
await p.waitForTimeout(900);
check("T9 milestone reads Done after marking", (await msCard().textContent() || "").includes("Done"));
check("T10 milestone now offers Reopen", await msCard().locator('[data-testid^="goal-reopen-"]').count() === 1);

// T11: create a HABIT goal → check in → shows check-ins + streak.
await addGoal({ name: "Weekly review", kind: "habit", habitTarget: 4 });
const habitCard = () => p.locator('.goal-card[data-kind="habit"]').filter({ hasText: "Weekly review" }).first();
check("T11 habit card created", await p.locator('.goal-card[data-kind="habit"]').filter({ hasText: "Weekly review" }).count() >= 1);
const checkinBtn = habitCard().locator('[data-testid^="goal-checkin-"]');
check("T12 habit shows Check in action", await checkinBtn.count() === 1);
const before = await habitCard().textContent() || "";
check("T13 habit shows '0 of 4 check-ins' before check-in", before.includes("0 of 4"));
await checkinBtn.click();
await p.waitForTimeout(900);
const after = await habitCard().textContent() || "";
check("T14 habit shows '1 of 4 check-ins' after check-in", after.includes("1 of 4"));
check("T15 habit shows a streak chip", await habitCard().locator('[data-testid^="goal-streak-"]').count() === 1);

// T16: no page errors.
check("T16 no page errors", errs.length === 0, errs.slice(0, 3).join(" | "));

await p.screenshot({ path: SD + "/goals_kinds.png", fullPage: false });
const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
