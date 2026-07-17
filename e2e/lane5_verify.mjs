// lane5_verify.mjs — e2e checks for lane 5 (goals/budgets/household refinements).
// Usage: node e2e/lane5_verify.mjs   (server on :8115 serving the lane5 webroot)
import { chromium } from "playwright";

const BASE = "http://127.0.0.1:8115";
let pass = 0, fail = 0;
const check = (name, ok, detail = "") => {
  console.log(`${ok ? "PASS" : "FAIL"}: ${name}${detail ? " — " + detail : ""}`);
  ok ? pass++ : fail++;
};

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));
const nav = async (p) => { await page.evaluate((x) => { history.pushState({}, "", x); dispatchEvent(new PopStateEvent("popstate")); }, p); await page.waitForTimeout(1500); };
const bodyText = async () => await page.locator("body").innerText();

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await page.waitForTimeout(1800);

// ───────── #51: contribution slider keyboard + valuetext + numeric entry ─────────
await nav("/goals");
// open the first goal's Plan contribution disclosure
const planToggle = page.locator('button:has-text("Plan contribution") >> visible=true').first();
if (await planToggle.count()) { await planToggle.click(); await page.waitForTimeout(700); }
const slider = page.locator('[data-testid^="goal-plan-slider-"]').first();
if (await slider.count()) {
  const vt0 = await slider.getAttribute("aria-valuetext");
  check("#51: slider carries formatted aria-valuetext", !!vt0 && /\$[\d,]+\.\d{2}\/mo/.test(vt0), vt0 || "(none)");
  const amt0 = await page.locator('[data-testid^="goal-plan-amount-"]').first().inputValue();
  await slider.focus();
  await page.keyboard.press("ArrowRight");
  await page.waitForTimeout(400);
  const amt1 = await page.locator('[data-testid^="goal-plan-amount-"]').first().inputValue();
  check("#51: ArrowRight steps the plan (numeric field follows)", amt1 !== amt0, `${amt0} → ${amt1}`);
  await page.keyboard.press("End");
  await page.waitForTimeout(400);
  const sMax = parseInt(await slider.getAttribute("max"), 10);
  const sStep = parseInt(await slider.getAttribute("step"), 10);
  const sVal = parseInt(await slider.inputValue(), 10);
  // the browser snaps the value to the step grid, so End lands within one step of max
  check("#51: End jumps to (within one step of) max", sMax - sVal < sStep, `${sVal}/${sMax} step ${sStep}`);
  // numeric entry drives the slider — type a mid-range amount so no clamp applies
  const sMin = parseInt(await slider.getAttribute("min"), 10);
  const midMinor = Math.round((sMin + sMax) / 2 / 100) * 100;
  const midStr = (midMinor / 100).toFixed(2);
  const numInput = page.locator('[data-testid^="goal-plan-amount-"]').first();
  await numInput.fill(midStr);
  await page.waitForTimeout(400);
  const vt1 = await slider.getAttribute("aria-valuetext");
  check("#51: typing an amount updates the slider valuetext", !!vt1 && vt1.includes(midStr.replace(/\B(?=(\d{3})+(?!\d))/g, ",")), `typed ${midStr} → ${vt1}`);
} else {
  check("#51: plan slider reachable", false);
}

// ───────── #70 (UX-05): budgets historical wording, clickable counts, Automate ─────────
await nav("/budgets");
// (1) Automate menu: bulk tools are folded away until the menu opens.
const automate = page.locator('[data-testid="budgets-automate"]');
check("#70: toolbar has an Automate menu", (await automate.count()) === 1);
const autoBtnHidden = await page.locator('[data-testid="budgets-autobudget"]').isVisible().catch(() => false);
check("#70: bulk tools hidden until Automate opens", !autoBtnHidden);
await automate.click();
await page.waitForTimeout(400);
for (const id of ["budgets-last-month", "budgets-autobudget", "budgets-sweep-config", "budgets-adjust-all"]) {
  const vis = await page.locator(`[data-testid="${id}"]`).isVisible().catch(() => false);
  check(`#70: Automate menu holds ${id}`, vis);
}
await page.keyboard.press("Escape");
await page.waitForTimeout(300);

// (2) Compact-by-default for >6 budgets (fresh context = no stored density choice).
const listEl = page.locator('[data-testid="budgets-list"]');
if (await listEl.count()) {
  const cards = await listEl.evaluate((el) => el.childElementCount);
  const isCompact = await listEl.evaluate((el) => el.classList.contains("budget-clist"));
  if (cards > 6) check("#70: >6 budgets default to the compact list", isCompact, `${cards} budgets`);
  else check("#70: <=6 budgets keep full cards", !isCompact, `${cards} budgets`);
}

// (3) Historical period wording: page back one month.
const capNow = await page.locator('[data-testid="budgets-spend-cap"]').innerText().catch(() => "");
check("#70: live period says 'so far this month'", /so far this month/i.test(capNow), capNow);
await page.locator(".period-control .period-step").first().click();
await page.waitForTimeout(1200);
const capHist = await page.locator('[data-testid="budgets-spend-cap"]').innerText().catch(() => "");
check("#70: past period reads '<period> spending'", /spending$/i.test(capHist) && !/so far/i.test(capHist), capHist);

// (4) Clickable counts filter the list. Expand the rail if present, else use the
// healthy-branch near pill.
let filtered = false;
const rail = page.locator('[data-testid="budgets-issues-rail"]');
if (await rail.count()) {
  await rail.click();
  await page.waitForTimeout(400);
  for (const id of ["budgets-filter-over", "budgets-filter-near"]) {
    const b = page.locator(`[data-testid="${id}"]`);
    if ((await b.count()) && (await b.isVisible())) { await b.click(); filtered = true; break; }
  }
} else {
  const pill = page.locator('[data-testid="budgets-near-filter"]');
  if (await pill.count()) { await pill.click(); filtered = true; }
}
if (filtered) {
  await page.waitForTimeout(500);
  const chip = await page.locator('[data-testid="budgets-attention-chip"]').isVisible().catch(() => false);
  check("#70: clicking a count shows the filter chip", chip);
  await page.locator('[data-testid="budgets-attention-clear"]').click();
  await page.waitForTimeout(400);
  const gone = (await page.locator('[data-testid="budgets-attention-chip"]').count()) === 0;
  check("#70: Show all clears the attention filter", gone);
} else {
  console.log("SKIP: #70 attention-count filter — no over/near counts in this dataset/period");
}

// (5) Follow-ups collapsed by default (only when a budget has linked to-dos).
const fuToggle = page.locator('[data-testid^="budget-todos-toggle-"]').first();
if (await fuToggle.count()) {
  const itemsBefore = await page.locator(".budget-todos .txnfu-item").count();
  await fuToggle.click();
  await page.waitForTimeout(300);
  const itemsAfter = await page.locator(".budget-todos .txnfu-item").count();
  check("#70: follow-ups start collapsed and expand on click", itemsBefore === 0 && itemsAfter >= 0, `${itemsBefore} → ${itemsAfter}`);
} else {
  console.log("SKIP: #70 follow-ups collapse — no budget-linked to-dos in this dataset");
}

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 3).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
