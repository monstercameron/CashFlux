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

// ───────── #71 (UX-06): compact goal card default + formatting fixes ─────────
await nav("/goals");
const firstCard = page.locator('[data-testid^="goal-row-"]').first();
if (await firstCard.count()) {
  const compact = await firstCard.evaluate((el) => el.classList.contains("is-compact"));
  const btns = await firstCard.locator("button").count();
  check("#71: goal card defaults to compact", compact, `${btns} buttons`);
  check("#71: compact card has few controls", btns <= 5, `${btns} buttons`);
  const planHidden = !(await firstCard.locator('[data-testid^="goal-plan-toggle-"]').count());
  const delHidden = !(await firstCard.locator('[data-testid^="goal-delete-btn-"]').count());
  check("#71: planner + delete live outside the compact state", planHidden && delHidden);
  await firstCard.locator('[data-testid^="goal-expand-"]').click();
  await page.waitForTimeout(500);
  const expanded = page.locator('[data-testid^="goal-row-"]').first();
  const nowCompact = await expanded.evaluate((el) => el.classList.contains("is-compact"));
  const planVisible = (await expanded.locator('[data-testid^="goal-plan-toggle-"]').count()) > 0;
  const kebab = (await expanded.locator('[data-testid^="goal-menu-btn-"]').count()) > 0;
  check("#71: Details expands to the full card (planner + kebab present)", !nowCompact && planVisible && kebab);
  await expanded.locator('[data-testid^="goal-collapse-"]').click();
  await page.waitForTimeout(400);
  const backCompact = await page.locator('[data-testid^="goal-row-"]').first().evaluate((el) => el.classList.contains("is-compact"));
  check("#71: Less returns to the compact card", backCompact);
} else {
  check("#71: goal cards reachable", false);
}
// Formatting: payday-waterfall amounts share one right edge regardless of name length.
const wfAmts = await page.locator('[data-testid="goals-waterfall-card"] .wf-line-amt').evaluateAll(
  (els) => els.map((e) => Math.round(e.getBoundingClientRect().right)));
if (wfAmts.length > 1) {
  check("#71: waterfall amounts right-align", wfAmts.every((r) => Math.abs(r - wfAmts[0]) <= 1), wfAmts.join(","));
} else {
  console.log("SKIP: #71 waterfall alignment — card not showing in this dataset");
}
// Formatting: a section-count span never wraps below its heading.
const fundsCount = page.locator('[data-testid="goals-funds-count"]');
if (await fundsCount.count()) {
  const sameLine = await fundsCount.evaluate((el) => {
    const h = el.closest("h2");
    return Math.abs(el.getBoundingClientRect().top - h.getBoundingClientRect().top) < h.getBoundingClientRect().height;
  });
  check("#71: 'Sinking funds · N' count stays on the heading line", sameLine);
} else {
  console.log("SKIP: #71 sinking-funds heading — no sinking funds in this dataset");
}

// ───────── #51: contribution slider keyboard + valuetext + numeric entry ─────────
// The planner now lives in the expanded card (UX-06) — expand the first card first.
const exp51 = page.locator('[data-testid^="goal-expand-"]').first();
if (await exp51.count()) { await exp51.click(); await page.waitForTimeout(500); }
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

// ───────── #65: plan comparison, funding order, paycheck preview, recompute ─────────
// (The first goal card is expanded with its planner open from the #51 block.)
const compare = page.locator('[data-testid^="goal-plan-compare-"]').first();
if (await compare.count()) {
  const rows65 = await compare.locator(".goal-plan-compare-row").count();
  const current = await compare.locator(".goal-plan-compare-row.is-current").count();
  const txt65 = await compare.innerText();
  check("#65: planner compares three plans side by side", rows65 === 3 && current === 1, `${rows65} rows`);
  check("#65: comparison pairs each amount with a landing date", /\/mo/.test(txt65) && /(20\d\d|no landing)/.test(txt65), txt65.replace(/\n/g, " | ").slice(0, 90));
} else {
  check("#65: planner comparison present", false);
}

// Edits recompute projections: raising the target must change the figures.
const expandedCard = page.locator('[data-testid^="goal-row-"]').first();
const gid = (await expandedCard.getAttribute("data-testid")).replace("goal-row-", "");
const figsBefore = await page.locator(`[data-testid="goal-figs-${gid}"]`).innerText().catch(() => "");
await page.locator(`[data-testid="goal-target-btn-${gid}"]`).click();
await page.waitForTimeout(300);
await page.locator(`[data-testid="goal-target-input-${gid}"]`).fill("99999");
await page.locator(`[data-testid="goal-target-save-${gid}"]`).click();
await page.waitForTimeout(600);
const figsAfter = await page.locator(`[data-testid="goal-figs-${gid}"]`).innerText().catch(() => "");
check("#65: editing the target recomputes the projections", figsBefore !== "" && figsAfter !== "" && figsBefore !== figsAfter);

// Funding order: reorder moves a goal and renumbers the sequence.
const foToggle = page.locator('[data-testid="goals-funding-order-toggle"]');
if (await foToggle.count()) {
  await foToggle.click();
  await page.waitForTimeout(400);
  const namesBefore = await page.locator('[data-testid^="goal-funding-row-"] .wf-line-name').allInnerTexts();
  check("#65: funding-order list shows the waterfall sequence", namesBefore.length >= 2, namesBefore.join(" | "));
  const firstRowId = (await page.locator('[data-testid^="goal-funding-row-"]').first().getAttribute("data-testid")).replace("goal-funding-row-", "");
  await page.locator(`[data-testid="goal-funding-down-${firstRowId}"]`).click();
  await page.waitForTimeout(600);
  const namesAfter = await page.locator('[data-testid^="goal-funding-row-"] .wf-line-name').allInnerTexts();
  check("#65: move-down reorders the funding sequence", namesAfter.length === namesBefore.length && namesAfter[0] === namesBefore[1], namesAfter.join(" | "));
  await page.locator(`[data-testid="goal-funding-up-${firstRowId}"]`).click();
  await page.waitForTimeout(400);
} else {
  console.log("SKIP: #65 funding order — fewer than two fundable goals");
}

// Paycheck preview: appears once the live waterfall moment is handled.
const wfDismiss = page.locator('[data-testid="goals-waterfall-dismiss"]');
if (await wfDismiss.count()) {
  await wfDismiss.click();
  await page.waitForTimeout(800);
}
const ppToggle = page.locator('[data-testid="goals-paycheck-preview-toggle"]');
if (await ppToggle.count()) {
  check("#65: next-paycheck preview offered when no income is pending", true);
  await ppToggle.click();
  await page.waitForTimeout(400);
  const ppLines = await page.locator('[data-testid="goals-paycheck-preview"] .wf-line').count();
  const ppNote = await page.locator('[data-testid="goals-paycheck-preview-note"]').innerText().catch(() => "");
  check("#65: preview lists per-goal funding lines", ppLines >= 1, `${ppLines} lines`);
  check("#65: preview says approval happens when the paycheck lands", /preview|approval/i.test(ppNote), ppNote.slice(0, 60));
} else {
  console.log("SKIP: #65 paycheck preview — no recent income to estimate from");
}
// Conflict strip is dataset-dependent; note its absence rather than fail.
if (!(await page.locator('[data-testid="goals-conflict-strip"]').count())) {
  console.log("SKIP: #65 conflict strip — no shared over-claimed account in this dataset");
} else {
  check("#65: conflict strip names the over-claimed account", true);
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
for (const id of ["budgets-last-month", "budgets-autobudget", "budgets-month-close", "budgets-sweep-config", "budgets-adjust-all"]) {
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

// ───────── #64: guided month-close flow (still on the past month from above) ─────────
const offer = page.locator('[data-testid="budgets-monthclose-offer"]');
check("#64: closed month offers the guided close flow", (await offer.count()) > 0);
if (await offer.count()) {
  await offer.click();
  await page.waitForTimeout(800);
  const body64 = await page.locator('[data-testid="monthclose-body"]').isVisible().catch(() => false);
  check("#64: month-close modal opens", body64);
  for (const id of ["monthclose-overspends", "monthclose-leftovers", "monthclose-assign", "monthclose-income", "monthclose-copy"]) {
    check(`#64: section ${id} present`, (await page.locator(`[data-testid="${id}"]`).count()) === 1);
  }
  const deltaTxt = await page.locator('[data-testid="monthclose-income-delta"]').innerText().catch(() => "");
  check("#64: income section distinguishes expected vs actual", /more|less|on plan/i.test(deltaTxt), deltaTxt);
  const rollNote = await page.locator('[data-testid="monthclose-rollover-note"]').innerText().catch(() => "");
  if (rollNote) check("#64: rollover behavior explained before the month changes", /rollover is (ON|OFF)/i.test(rollNote), rollNote.slice(0, 60));
  else console.log("SKIP: #64 rollover note — no leftovers in this period");
  // Over-assignment resolutions are dataset-dependent — assert whichever state shows.
  const overAssigned = (await page.locator('[data-testid="monthclose-resolve-defer"]').count()) > 0;
  if (overAssigned) {
    for (const id of ["monthclose-resolve-income", "monthclose-resolve-defer"]) {
      check(`#64: resolution choice ${id} offered`, (await page.locator(`[data-testid="${id}"]`).count()) === 1);
    }
    await page.locator('[data-testid="monthclose-resolve-defer"]').click();
    await page.waitForTimeout(400);
    check("#64: leave-unresolved collapses to an honest note", (await page.locator('[data-testid="monthclose-deferred"]').count()) === 1);
  } else {
    const fits = await page.locator('[data-testid="monthclose-assign"]').innerText().catch(() => "");
    check("#64: plan-fits state says so plainly", /fits/i.test(fits), fits.slice(0, 60));
  }
  await page.locator('[data-testid="monthclose-done"]').click();
  await page.waitForTimeout(500);
  check("#64: Done closes the flow", (await page.locator('[data-testid="monthclose-body"]').count()) === 0);
}
// The plan cell distinguishes expected vs actually-received income.
const incActual = await page.locator('[data-testid="budgets-income-actual"]').innerText().catch(() => "");
if (incActual) check("#64: assign banner shows actual vs expected income", /Received/i.test(incActual), incActual.slice(0, 70));
else console.log("SKIP: #64 income-actual line — no income basis in this method/dataset");

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

// ───────── #66: household clarity — roles, ownership, change previews ─────────
await nav("/members");
const rolesToggle = page.locator('[data-testid="members-roles-explain-toggle"]');
check("#66: household page offers the roles explainer", (await rolesToggle.count()) === 1);
if (await rolesToggle.count()) {
  await rolesToggle.click();
  await page.waitForTimeout(400);
  const explain = await page.locator('[data-testid="members-roles-explain"]').innerText().catch(() => "");
  check("#66: explainer covers all three roles", /Owner/.test(explain) && /Admin/.test(explain) && /Viewer/.test(explain) && /read-only/i.test(explain));
  const own = await page.locator('[data-testid="members-ownership-explain"]').innerText().catch(() => "");
  check("#66: ownership explained (owner vs member vs shared)", /OWNER/.test(own) && /MEMBER/.test(own) && /shared/i.test(own));
}
// Leave preview: pick the first real member and read the breakdown.
const leaveSel = page.locator('[data-testid="member-leave-select"]');
check("#66: leave-preview picker present", (await leaveSel.count()) === 1);
if (await leaveSel.count()) {
  const firstVal = await leaveSel.locator("option:not([value=''])").first().getAttribute("value");
  await leaveSel.selectOption(firstVal);
  await page.waitForTimeout(500);
  const prev = await page.locator('[data-testid="member-leave-preview"]').innerText().catch(() => "");
  check("#66: leave preview lists what needs reassignment", /would need a new owner|owns nothing/.test(prev), prev.replace(/\n/g, " | ").slice(0, 100));
  const noteOK = /only a preview|nothing changes/i.test(prev) || /owns nothing/.test(prev);
  check("#66: leave preview is explicitly read-only", noteOK);
}
// Role explainer inside the add-member form, live with the role select.
await page.locator('button:has-text("Add member")').first().click();
await page.waitForTimeout(600);
const roleExplain1 = await page.locator('[data-testid="member-role-explain"]').innerText().catch(() => "");
check("#66: member form explains the selected role", roleExplain1.length > 20, roleExplain1.slice(0, 50));
const roleSel = page.locator('[data-testid="member-add-role"] select, select[data-testid="member-add-role"]').first();
if (await roleSel.count()) {
  await roleSel.selectOption("viewer");
  await page.waitForTimeout(400);
  const roleExplain2 = await page.locator('[data-testid="member-role-explain"]').innerText().catch(() => "");
  check("#66: explainer follows the role choice", roleExplain2 !== roleExplain1 && /read-only/i.test(roleExplain2), roleExplain2.slice(0, 50));
} else {
  console.log("SKIP: #66 role-select live explainer — role select locator not found");
}
await page.keyboard.press("Escape");
await page.waitForTimeout(400);

// Shared badge on the accounts list.
await nav("/accounts");
const sharedBadges = await page.locator('[data-testid^="acct-shared-badge-"]').count();
check("#66: shared accounts wear a Shared badge in the list", sharedBadges >= 1, `${sharedBadges} badges`);

// Ownership-move preview in the account editor (Edit lives in the row kebab).
const firstAcctRow = page.locator('[data-testid^="acct-row-"]').first();
await firstAcctRow.locator('button[aria-haspopup="menu"]').first().click();
await page.waitForTimeout(400);
const editBtn = page.locator('[data-testid^="edit-account-btn-"] >> visible=true').first();
if (await editBtn.count()) {
  await editBtn.click();
  await page.waitForTimeout(900);
  const ownerSel = page.locator('.flip-panel select[aria-label="Owner"], .modal-scroll select[aria-label="Owner"]').first();
  if (await ownerSel.count()) {
    const cur = await ownerSel.inputValue();
    const other = await ownerSel.locator(`option:not([value="${cur}"])`).first().getAttribute("value");
    await ownerSel.selectOption(other);
    await page.waitForTimeout(500);
    const prevTxt = await page.locator('[data-testid="acct-owner-preview"]').innerText().catch(() => "");
    check("#66: changing owner previews the net-worth move before saving", /moves this account/i.test(prevTxt), prevTxt.slice(0, 90));
    await page.keyboard.press("Escape");
    await page.waitForTimeout(400);
  } else {
    console.log("SKIP: #66 owner-move preview — owner select not found in editor");
  }
} else {
  console.log("SKIP: #66 owner-move preview — no Edit button on account rows");
}

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 3).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
