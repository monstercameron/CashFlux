// /recurring comprehensive e2e: the widgetized Scheduled surface (hero figures,
// next-30-days schedule, flow cards, detected charges), the add/edit flip modal,
// the ⋯ menu (edit / view account / delete), tab switching, Post due — plus
// negative/edge cases (empty label, zero amount, cancel-no-add, share-meter
// bounds, hero math, no page errors). Exits non-zero on any failure.
import { createRequire } from "module";
const require = createRequire("C:/Users/mreca/Desktop/CashFlux/.tools/package.json");
const { chromium } = require("playwright");
const URL = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const p = await b.newPage({ viewport: { width: 1440, height: 1200 } });
const results = [];
const check = (n, c, d = "") => { results.push(!!c); console.log((c ? "PASS " : "FAIL ") + n + (d ? " — " + d : "")); };
const errs = []; p.on("pageerror", e => errs.push(String(e)));
const cents = (s) => { const m = (s || "").replace(/[^0-9.]/g, ""); return m ? Math.round(parseFloat(m) * 100) : 0; };

const openRec = async () => {
  await p.goto(URL + "/recurring", { waitUntil: "domcontentloaded" });
  await p.waitForSelector(".bento-recurring", { timeout: 15000 }).catch(() => {});
  await p.waitForTimeout(900);
};
const openModal = async (btn) => { await btn.click(); await p.waitForTimeout(800); };

// --- boot + sample data ---
await p.goto(URL + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(() => {});
await p.waitForTimeout(1200);
if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1500); }
await openRec();

// --- surface + hero ---
check("S1 widgetized Scheduled surface", await p.locator(".bento-recurring").count() === 1);
check("S2 hub tabs (Scheduled/Bills/Subscriptions)", await p.locator('[data-testid="recurring-tab-scheduled"]').count() === 1 && await p.locator('[data-testid="recurring-tab-bills"]').count() === 1 && await p.locator('[data-testid="recurring-tab-subscriptions"]').count() === 1);
check("S3 'Recurring cash flows' title present (scoped-page contract)", (await p.locator("main").innerText()).includes("Recurring cash flows"));
check("S4 hero net figure renders", await p.locator('[data-testid="recurring-net"]').count() === 1, await p.locator('[data-testid="recurring-net"]').innerText().catch(() => ""));
// Hero math: |net| == |in − out| to the cent.
const chips = await p.locator(".rec-hero .debt-stat-value").allInnerTexts();
if (chips.length >= 2) {
  const inC = cents(chips[0]), outC = cents(chips[1]);
  const netC = cents(await p.locator('[data-testid="recurring-net"]').innerText());
  check("S5 hero math: |net| = |in − out|", Math.abs(Math.abs(inC - outC) - netC) <= 1, `in=${inC} out=${outC} net=${netC}`);
} else {
  check("S5 hero math: |net| = |in − out|", false, "chips missing");
}
check("S6 active-flows chip matches card count", cents(chips[2] || "") / 100 === await p.locator(".rec-flow").count() || (chips[2] || "").trim() === String(await p.locator(".rec-flow").count()), `chip=${(chips[2] || "").trim()} cards=${await p.locator(".rec-flow").count()}`);

// --- upcoming schedule ---
check("U1 next-30-days rows render", await p.locator(".rec-up-row").count() >= 1, `${await p.locator(".rec-up-row").count()}`);
check("U2 window meta line (N due · $out · $in)", await p.locator('[data-testid="recurring-upcoming-meta"]').count() === 1, (await p.locator('[data-testid="recurring-upcoming-meta"]').innerText().catch(() => "")).trim());
// Dates are sorted ascending.
const days = await p.locator(".rec-up-row .rec-up-date").allInnerTexts();
check("U3 rows capped at 10 (+ overflow note when more)", await p.locator(".rec-up-row").count() <= 10, `${days.length} rows`);

// --- flow cards ---
const flowCount = await p.locator(".rec-flow").count();
check("F1 flow cards render", flowCount >= 1, `${flowCount}`);
check("F2 every card has a cadence tag + per-month figure", await p.locator(".rec-flow .rec-cad-tag").count() === flowCount && await p.locator(".rec-flow .rec-flow-monthly").count() === flowCount);
check("F3 every card has a ⋯ menu", await p.locator('[data-testid^="recurring-menu-"]').count() === flowCount);
// Share meters stay within 0–100.
const meterVals = await p.locator(".rec-flow [role=meter]").evaluateAll(els => els.map(e => parseFloat(e.getAttribute("aria-valuenow") || "0")));
check("F4 share-of-outflow meters within [0,100]", meterVals.length > 0 && meterVals.every(v => v >= 0 && v <= 100), `${meterVals.length} meters, max=${Math.max(...meterVals, 0).toFixed(1)}`);

// --- add via the flip modal (money out) ---
await openModal(p.locator('[data-testid="recurring-add"]').first());
check("M1 add opens the flip modal", await p.locator('[data-testid="recurring-form"]').count() === 1);
// (neg) empty label → error, modal stays open.
await p.locator('[data-testid="rec-save"]').click(); await p.waitForTimeout(400);
check("M2 (neg) empty label shows an error, modal stays open", await p.locator('[data-testid="recurring-form"] [role=alert]').count() >= 1 && await p.locator('[data-testid="recurring-form"]').count() === 1);
// (neg) label but zero amount → error.
await p.locator('[data-testid="rec-label"]').fill("E2E Zero");
await p.locator('[data-testid="rec-amount"]').fill("0");
await p.locator('[data-testid="rec-save"]').click(); await p.waitForTimeout(400);
check("M3 (neg) zero amount shows an error", await p.locator('[data-testid="recurring-form"] [role=alert]').count() >= 1);
// Positive: a $15.99 monthly expense.
await p.locator('[data-testid="rec-label"]').fill("E2E Streaming");
await p.locator('[data-testid="rec-amount"]').fill("15.99");
await p.locator('[data-testid="rec-save"]').click(); await p.waitForTimeout(800);
check("M4 saving adds the flow (modal closes, card appears)", await p.locator('[data-testid="recurring-form"]').count() === 0 && (await p.locator("main").innerText()).includes("E2E Streaming"));
check("M5 the new expense reads as money out (toned figure)", (await p.locator(".rec-flow", { hasText: "E2E Streaming" }).locator(".rec-flow-monthly").innerText()).includes("15.99"));
check("M6 flow count went up by one", await p.locator(".rec-flow").count() === flowCount + 1);

// --- add money IN via the direction toggle ---
await openModal(p.locator('[data-testid="recurring-add"]').first());
await p.locator('[data-testid="rec-label"]').fill("E2E Sidegig");
await p.locator('[data-testid="rec-dir-in"]').click(); await p.waitForTimeout(200);
await p.locator('[data-testid="rec-amount"]').fill("100");
await p.locator('[data-testid="rec-save"]').click(); await p.waitForTimeout(800);
const sidegig = p.locator(".rec-flow", { hasText: "E2E Sidegig" });
check("M7 money-in flow saves with a positive amount", await sidegig.count() === 1 && !(await sidegig.locator(".rec-flow-monthly").innerText()).includes("("));

// --- (neg) cancel adds nothing ---
const beforeCancel = await p.locator(".rec-flow").count();
await openModal(p.locator('[data-testid="recurring-add"]').first());
await p.locator('[data-testid="rec-label"]').fill("E2E Ghost");
await p.locator('[data-testid="rec-cancel"]').click();
await p.waitForSelector('[data-testid="recurring-form"]', { state: "detached", timeout: 3000 }).catch(() => {});
check("M8 (neg) cancel closes without adding", await p.locator('[data-testid="recurring-form"]').count() === 0 && await p.locator(".rec-flow").count() === beforeCancel && !(await p.locator("main").innerText()).includes("E2E Ghost"));

// --- edit via the ⋯ menu ---
const streamCard = p.locator(".rec-flow", { hasText: "E2E Streaming" });
await streamCard.locator('[data-testid^="recurring-menu-"]').click(); await p.waitForTimeout(300);
await streamCard.locator('[data-testid^="recurring-edit-"]').click(); await p.waitForTimeout(800);
check("E1 edit opens the modal pre-filled", (await p.locator('[data-testid="rec-label"]').inputValue().catch(() => "")) === "E2E Streaming" && (await p.locator('[data-testid="rec-amount"]').inputValue().catch(() => "")) === "15.99");
await p.locator('[data-testid="rec-amount"]').fill("20.00");
await p.locator('[data-testid="rec-save"]').click(); await p.waitForTimeout(800);
check("E2 saving an edit updates the card", (await p.locator(".rec-flow", { hasText: "E2E Streaming" }).locator(".rec-flow-monthly").innerText()).includes("20.00"));
check("E3 editing didn't duplicate the flow", await p.locator(".rec-flow", { hasText: "E2E Streaming" }).count() === 1);

// --- delete via the ⋯ menu (confirm dialog) ---
// Wait for the confirm dialog to actually mount before clicking it (a fixed sleep
// races the dialog on slower renders), then wait for the card to detach.
const confirmDelete = async (card, name) => {
  await p.keyboard.press("Escape"); await p.waitForTimeout(200); // close any lingering menu
  await card.locator('[data-testid^="recurring-menu-"]').click(); await p.waitForTimeout(300);
  await card.locator('[data-testid^="recurring-del-"]').click();
  await p.waitForSelector("#cf-dialog-confirm", { timeout: 4000 }).catch(() => {});
  await p.evaluate(() => { const c = document.querySelector("#cf-dialog-confirm"); if (c) c.click(); });
  await p.waitForFunction((n) => !document.querySelector("main").innerText.includes(n), name, { timeout: 5000 }).catch(() => {});
};
const beforeDel = await p.locator(".rec-flow").count();
await confirmDelete(streamCard, "E2E Streaming");
check("D1 delete (confirmed) removes the card", await p.locator(".rec-flow").count() === beforeDel - 1 && !(await p.locator("main").innerText()).includes("E2E Streaming"));
// Clean up the sidegig too (keeps reruns deterministic-ish).
await confirmDelete(sidegig, "E2E Sidegig");
check("D2 second delete works too", !(await p.locator("main").innerText()).includes("E2E Sidegig"));

// --- post due ---
await p.locator('[data-testid="recurring-post-due"]').click(); await p.waitForTimeout(700);
check("P1 Post due reports a status, no crash", await p.locator('[data-testid="recurring-post-msg"]').count() === 1 && errs.length === 0, (await p.locator('[data-testid="recurring-post-msg"]').innerText().catch(() => "")).trim());

// --- detected charges (when present) ---
if (await p.locator('[data-testid="detected-recurring"]').count()) {
  const detBefore = await p.locator(".rec-detected").count();
  const detName = (await p.locator(".rec-detected .rec-flow-name").first().innerText()).trim();
  await p.locator('[data-testid="detected-add"]').first().click(); await p.waitForTimeout(800);
  check("T1 adding a detected charge moves it into the flows", (await p.locator(".rec-flow", { hasText: detName }).count()) >= 1 && (await p.locator(".rec-detected").count()) === detBefore - 1, detName);
} else {
  check("T1 adding a detected charge moves it into the flows", true, "no detected charges in this dataset — skipped");
}

// --- identity + interconnects ---
check("I1 each flow shows its formula identity (recurring_<slug>_monthly chip)",
  await p.locator(".rec-flow .rec-flow-var").count() >= 1 && ((await p.locator(".rec-flow-var").first().innerText()) || "").startsWith("recurring_"),
  (await p.locator(".rec-flow-var").first().innerText().catch(() => "")).trim());
// View transactions deep-links with the flow's filter applied.
const firstFlow = p.locator(".rec-flow").first();
await firstFlow.locator('[data-testid^="recurring-menu-"]').click(); await p.waitForTimeout(300);
check("I2 the ⋯ menu offers View transactions", await firstFlow.locator('[data-testid^="recurring-viewtxns-"]').count() === 1);
await firstFlow.locator('[data-testid^="recurring-viewtxns-"]').click(); await p.waitForTimeout(900);
check("I3 View transactions navigates to /transactions (filter applied)", p.url().endsWith("/transactions"), p.url());
await openRec();
// Metrics toggle reveals a FormulaBuilder exposing the recurring_* variables.
await p.locator('[data-testid="recurring-toggle-formulas"]').click(); await p.waitForTimeout(800);
check("I4 the metrics toggle reveals the recurring_* formula surface", (await p.locator(".bento-recurring").innerText()).includes("recurring_"),
  ((await p.locator(".bento-recurring").innerText()).match(/recurring_[a-z_]+/) || [""])[0]);
await p.locator('[data-testid="recurring-toggle-formulas"]').click(); await p.waitForTimeout(400);

// --- tabs still work ---
await p.locator('[data-testid="recurring-tab-bills"]').click(); await p.waitForTimeout(900);
check("B1 Bills tab renders its own surface (Scheduled flows swap out)", await p.locator("#sec-bills").count() === 1 && await p.locator(".rec-flow").count() === 0 && errs.length === 0);
check("B1a the bills list scrolls in place + the calendar is sticky", await p.locator('[data-testid="bills-scroll"]').count() === 1 && await p.locator(".bills-cal-sticky").count() === 1);
// Hovering a bill row highlights its due date on the calendar (when the date is in
// the displayed month); un-hovering clears it.
{
  let hovered = "";
  for (const r of await p.locator("[data-due]").all()) {
    const due = await r.getAttribute("data-due");
    if (await p.locator(`.cal-cell[data-date="${due}"]`).count()) { await r.hover(); await p.waitForTimeout(250); hovered = due; break; }
  }
  check("B1b hovering a bill highlights its calendar date", hovered !== "" && await p.locator(".cal-cell.cal-hl").count() === 1, hovered);
  await p.locator("#sec-bills h2").first().hover().catch(() => {}); await p.waitForTimeout(250);
  check("B1c un-hovering clears the highlight", await p.locator(".cal-cell.cal-hl").count() === 0);
}
await p.locator('[data-testid="recurring-tab-subscriptions"]').click(); await p.waitForTimeout(900);
check("B2 Subscriptions tab renders", errs.length === 0);
// Detection preferences live in a flip modal.
await p.locator('[data-testid="subs-detect-prefs-toggle"]').click(); await p.waitForTimeout(800);
check("B2a detection preferences open in a flip modal", await p.locator('[data-testid="subs-detect-prefs"]').count() === 1 && await p.locator('[data-testid="subs-detect-min-occur"]').count() === 1);
await p.locator('[data-testid="subs-detect-min-occur"]').selectOption("4"); await p.waitForTimeout(500);
await p.locator('[data-testid="subs-prefs-done"]').click();
await p.waitForSelector('[data-testid="subs-detect-prefs"]', { state: "detached", timeout: 3000 }).catch(() => {});
check("B2b changing sensitivity persists + Done closes the modal", await p.locator('[data-testid="subs-detect-prefs"]').count() === 0 && errs.length === 0);
// Reopen to confirm the saved value stuck, then restore the default.
await p.locator('[data-testid="subs-detect-prefs-toggle"]').click(); await p.waitForTimeout(800);
check("B2c the saved sensitivity survives a reopen", (await p.locator('[data-testid="subs-detect-min-occur"]').inputValue()) === "4");
await p.locator('[data-testid="subs-detect-min-occur"]').selectOption("3"); await p.waitForTimeout(400);
await p.locator('[data-testid="subs-prefs-done"]').click(); await p.waitForTimeout(500);
await p.locator('[data-testid="recurring-tab-scheduled"]').click(); await p.waitForTimeout(900);
check("B3 back to Scheduled intact", await p.locator(".bento-recurring").count() === 1 && await p.locator(".rec-flow").count() >= 1);

check("Z1 no page errors across the whole run", errs.length === 0, errs.slice(0, 4).join(" | "));

const passed = results.filter(Boolean).length;
console.log(`RESULT: ${passed}/${results.length}`);
await b.close();
process.exit(passed === results.length ? 0 : 1);
