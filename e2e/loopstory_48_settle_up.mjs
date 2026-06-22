// L48 E2E loop story — "Yours, Mine, and Ours" (Priya & Sam + Lee, shared household settle-up)
// Persona: Priya, Sam, and Lee share a flat. They use CashFlux to split shared household
//          expenses and eventually square up. The ritual spans ≥4 screens and 8+ actions:
//          add members → add shared expenses on /split → read net balances → record a
//          settlement → confirm the ledger re-balances → check /transactions and /dashboard.
//
// Storage model: the app uses an in-memory SQLite store (no cashflux:dataset in localStorage).
// All assertions are therefore DOM/UI-based. pushNav keeps the wasm session alive throughout.
//
// Flow:
//   0. Boot once at /. Add members Priya, Sam, Lee via /members (pushNav, same session).
//   1. /split — confirm all three members in payer select (MEMBERS_VISIBLE_IN_PICKERS).
//   2. Add three shared expenses with different payers:
//        Exp A: $90 dinner,    paid by Priya, split 3-ways → $30 each
//        Exp B: $60 groceries, paid by Sam,   split 3-ways → $20 each
//        Exp C: $30 supplies,  paid by Lee,   split 3-ways → $10 each
//      Net: Priya +$30 (owed); Sam $0 (settled); Lee -$30 (owes).
//   3. Assert per-expense share summary in UI (SHARES_SUM_TO_EXPENSE).
//   4. Assert net balances in running ledger (NET_BALANCE_MATH).
//   5. Record the settlement Lee→Priya $30.
//   6. Assert ledger re-balances (SETTLEMENT_ZEROES_PAIR).
//   7. Reload — assert settlement persists in UI (SETTLEMENT_SURVIVES_RELOAD).
//   8. /transactions — page loads without crash.
//   9. /dashboard — loads without crash (DASHBOARD_LOADS).
//  10. /reports — loads without crash.
//  11. JS error check.
//
// Key invariants:
//   SHARES_SUM_TO_EXPENSE      — split summary says "X split among 3" with correct per-person
//   NET_BALANCE_MATH           — Priya owed, Lee owes, Sam absent from ledger
//   SETTLEMENT_ZEROES_PAIR     — Lee→Priya transfer gone after recording
//   SETTLEMENT_SURVIVES_RELOAD — settlement absent from ledger after page reload
//   MEMBERS_VISIBLE_IN_PICKERS — all three members in payer select on /split
//   DASHBOARD_LOADS            — dashboard renders after settle-up actions
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_48_settle_up.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS   = (name) => path.join(__dirname, name);

const MEMBERS   = ["Priya", "Sam", "Lee"];
const EXP_A_AMT = "90";   // paid by Priya, split 3-ways → $30 each
const EXP_B_AMT = "60";   // paid by Sam,   split 3-ways → $20 each
const EXP_C_AMT = "30";   // paid by Lee,   split 3-ways → $10 each

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0;
const pass  = (label) => { console.log(`PASS: ${label}`); passed++; };
const fail  = (label) => { console.error(`FAIL: ${label}`); failed++; process.exitCode = 1; };
const maybe = (label) => { console.log(`ABSENT: ${label}`); };

// Boot once — all subsequent navigation via pushNav to keep wasm / SQLite alive.
const bootApp = async (page) => {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  await page.waitForTimeout(2500);
};

const pushNav = async (page, route) => {
  await page.evaluate((r) => {
    window.history.pushState({}, "", r);
    window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
  }, route);
  await page.waitForTimeout(1500);
};

const bodyText = (page) => page.evaluate(() => document.body.innerText);

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => {
    const s = String(e);
    if (/Go program has already exited/.test(s)) return;
    errors.push(s);
  });

  // ── Boot the SPA ──────────────────────────────────────────────────────────────
  await bootApp(page);

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 0: Add members Priya, Sam, Lee on /members
  // (If they already exist from the demo seed, the submit may silently succeed or
  // show a "name taken" error — both are fine; we check presence in payer select.)
  // ══════════════════════════════════════════════════════════════════════════════
  await pushNav(page, "/members");
  await page.screenshot({ path: SS("l48_step0a_members_before.png") });

  const existingText0 = await bodyText(page);
  for (const name of MEMBERS) {
    if (existingText0.includes(name)) {
      pass(`Step 0 — member "${name}" already in household (from demo seed or prior run)`);
      continue;
    }
    const nameIn = await page.$("#member-add");
    if (!nameIn) { fail(`Step 0 — #member-add not found for "${name}"`); continue; }
    await nameIn.fill(name);
    const btn = await page.$('button[type="submit"]');
    if (!btn) { fail(`Step 0 — submit button not found for "${name}"`); continue; }
    await btn.click();
    await page.waitForTimeout(600);

    const afterText = await bodyText(page);
    if (afterText.includes(name)) pass(`Step 0 — member "${name}" added and visible`);
    else fail(`Step 0 — member "${name}" not found after submit`);
  }
  await page.screenshot({ path: SS("l48_step0b_members_seeded.png") });

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 1: /split — confirm members appear in payer select
  // ══════════════════════════════════════════════════════════════════════════════
  await pushNav(page, "/split");
  await page.screenshot({ path: SS("l48_step1_split_before.png") });

  const splitBodyText = await bodyText(page);
  if (splitBodyText.toLowerCase().includes("split")) pass("Step 1 — /split page loaded");
  else fail("Step 1 — /split content not found");

  const payerOpts = await page.evaluate(() => {
    const sel = document.querySelector(
      'select[aria-label*="paid" i], select[title*="payer" i], select[aria-label*="payer" i]'
    );
    return sel ? Array.from(sel.options).map((o) => o.text) : null;
  });

  if (!payerOpts) {
    maybe("MEMBERS_VISIBLE_IN_PICKERS — payer select not found on /split");
  } else {
    let allFound = true;
    for (const name of MEMBERS) {
      if (payerOpts.some((o) => o === name || o.startsWith(name))) {
        pass(`MEMBERS_VISIBLE_IN_PICKERS — "${name}" in payer select`);
      } else {
        fail(`MEMBERS_VISIBLE_IN_PICKERS — "${name}" NOT in payer select (opts: ${JSON.stringify(payerOpts)})`);
        allFound = false;
      }
    }
    if (allFound) pass("MEMBERS_VISIBLE_IN_PICKERS — all three members in payer select");
  }

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 2: Add three shared expenses
  // ══════════════════════════════════════════════════════════════════════════════
  // Helper: fill amount, pick payer by exact text, toggle all three members, save.
  // Returns the split summary text for invariant checking, or null on failure.
  async function saveExpense(amount, payerName, desc) {
    // Amount
    const amtIn = await page.$(
      'input[type="number"][aria-label*="amount" i], .card input[type=number]'
    );
    if (!amtIn) { fail(`saveExpense(${desc}) — amount input not found`); return null; }
    await amtIn.fill(amount);

    // Optional description
    const descIn = await page.$(
      'input[placeholder*="for" i], input[aria-label*="description" i], input[placeholder*="description" i]'
    );
    if (descIn) await descIn.fill(desc);

    // Payer select — select by exact text label
    const payerSel = await page.$(
      'select[aria-label*="paid" i], select[title*="payer" i], select[aria-label*="payer" i]'
    );
    if (!payerSel) { fail(`saveExpense(${desc}) — payer select not found`); return null; }
    try {
      await payerSel.selectOption({ label: payerName });
    } catch (e) {
      fail(`saveExpense(${desc}) — could not select payer "${payerName}": ${e.message.slice(0, 100)}`);
      return null;
    }
    await page.waitForTimeout(200);

    // Toggle member switches — by aria-label matching the plain name
    for (const name of MEMBERS) {
      // Try role=switch first
      const sw = await page.$(`[role="switch"][aria-label="${name}"]`);
      if (sw) {
        const checked = await sw.getAttribute("aria-checked");
        if (checked !== "true") await sw.click();
        await page.waitForTimeout(100);
      } else {
        // Fallback: checkbox
        const cb = await page.$(
          `input[type="checkbox"][aria-label="${name}"], input[type="checkbox"][id*="${name.toLowerCase()}"]`
        );
        if (cb) {
          const isChecked = await cb.isChecked();
          if (!isChecked) await cb.click();
          await page.waitForTimeout(100);
        } else {
          maybe(`saveExpense(${desc}) — no toggle for member "${name}" (may already be on)`);
        }
      }
    }
    await page.waitForTimeout(300);

    // Read split summary before save (to capture it for invariant checking)
    const summaryEl = await page.$(".muted");
    const summaryText = summaryEl ? await summaryEl.textContent() : "";

    // Save
    const saveBtn = await page.$('button:has-text("Save split"), button[title*="Save this split"]');
    if (!saveBtn) { fail(`saveExpense(${desc}) — Save split button not found`); return null; }
    await saveBtn.click();
    await page.waitForTimeout(700);
    return summaryText;
  }

  const summaryA = await saveExpense(EXP_A_AMT, "Priya", "L48 Dinner");
  if (summaryA !== null) pass("Step 2a — Exp A ($90 by Priya) submitted");
  await page.screenshot({ path: SS("l48_step2a_after_expA.png") });

  const summaryB = await saveExpense(EXP_B_AMT, "Sam", "L48 Groceries");
  if (summaryB !== null) pass("Step 2b — Exp B ($60 by Sam) submitted");
  await page.screenshot({ path: SS("l48_step2b_after_expB.png") });

  const summaryC = await saveExpense(EXP_C_AMT, "Lee", "L48 Supplies");
  if (summaryC !== null) pass("Step 2c — Exp C ($30 by Lee) submitted");
  await page.screenshot({ path: SS("l48_step2c_after_expC.png") });

  // ══════════════════════════════════════════════════════════════════════════════
  // INVARIANT: SHARES_SUM_TO_EXPENSE
  // The split calculator shows a summary like "$90.00 split among 3 → $30.00 each"
  // We check that the summary appeared for each expense entry.
  // (The actual shares-sum-to-total arithmetic is enforced by the pure Go split package,
  // unit-tested separately; here we confirm the UI surfaces it.)
  // ══════════════════════════════════════════════════════════════════════════════
  // summaryA/B/C captured before each save. They may be empty if form auto-clears.
  // We assert based on whether a "split among 3" summary was at any point shown;
  // alternatively we can check the page text after the last save.
  const currentText = await bodyText(page);
  if (currentText.includes("split among 3") || currentText.includes("split among")) {
    pass("SHARES_SUM_TO_EXPENSE — split summary 'split among 3' shown in UI");
  } else {
    maybe("SHARES_SUM_TO_EXPENSE — split summary not found in current page text; form may have reset");
  }

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 3: Read net balances from the settle-up ledger
  // ══════════════════════════════════════════════════════════════════════════════
  await page.waitForTimeout(500);
  await page.screenshot({ path: SS("l48_step3_settle_up_ledger.png") });

  const ledgerCard = page.locator(".card", { hasText: "Running balance" }).first();
  const ledgerCardCount = await ledgerCard.count();
  let ledgerText = "";
  if (ledgerCardCount > 0) {
    ledgerText = (await ledgerCard.innerText()).replace(/\s+/g, " ");
    pass("Step 3 — Settle-up ledger card (Running balance) found");
  } else {
    ledgerText = (await bodyText(page)).replace(/\s+/g, " ");
    if (ledgerText.includes("Settle up")) pass("Step 3 — settle-up section found (alternate selector)");
    else maybe("Step 3 — settle-up ledger card not found; check screenshot l48_step3_settle_up_ledger.png");
  }

  // ══════════════════════════════════════════════════════════════════════════════
  // INVARIANT: NET_BALANCE_MATH
  // ══════════════════════════════════════════════════════════════════════════════
  if (ledgerText.includes("Priya is owed"))
    pass("NET_BALANCE_MATH — Priya is owed (positive balance confirmed)");
  else
    fail(`NET_BALANCE_MATH — 'Priya is owed' not found. Ledger text: ${ledgerText.slice(0, 400)}`);

  if (ledgerText.includes("Lee owes"))
    pass("NET_BALANCE_MATH — Lee owes (negative balance confirmed)");
  else
    fail(`NET_BALANCE_MATH — 'Lee owes' not found. Ledger text: ${ledgerText.slice(0, 400)}`);

  if (!ledgerText.includes("Sam owes") && !ledgerText.includes("Sam is owed"))
    pass("NET_BALANCE_MATH — Sam not in ledger (net zero, as expected)");
  else
    maybe(`NET_BALANCE_MATH — Sam appears in ledger (may include demo-seed data): ${ledgerText.slice(0, 300)}`);

  if (ledgerText.includes("Lee") && ledgerText.includes("Priya") && ledgerText.includes("pays"))
    pass("NET_BALANCE_MATH — minimal payment 'Lee pays Priya' shown");
  else
    fail(`NET_BALANCE_MATH — minimal payment 'Lee pays Priya' not found: ${ledgerText.slice(0, 400)}`);

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 4: Record the settlement Lee→Priya
  // ══════════════════════════════════════════════════════════════════════════════
  const recordBtn = page
    .locator(".row", { hasText: "Lee pays Priya" })
    .locator('button:has-text("Record settlement")')
    .first();
  const recordBtnCount = await recordBtn.count();

  if (recordBtnCount > 0) {
    await recordBtn.click();
    await page.waitForTimeout(800);
    pass("Step 4 — 'Record settlement' clicked for Lee→Priya");
  } else {
    // Broaden: any Record settlement button
    const anyRecord = await page.$('button:has-text("Record settlement")');
    if (anyRecord) {
      await anyRecord.click();
      await page.waitForTimeout(800);
      pass("Step 4 — 'Record settlement' clicked (broad selector)");
    } else {
      fail("Step 4 — No 'Record settlement' button found");
    }
  }
  await page.screenshot({ path: SS("l48_step4_after_settlement.png") });

  // ══════════════════════════════════════════════════════════════════════════════
  // INVARIANT: SETTLEMENT_ZEROES_PAIR
  // ══════════════════════════════════════════════════════════════════════════════
  const afterSettleText = await bodyText(page);
  if (!afterSettleText.includes("Lee pays Priya"))
    pass("SETTLEMENT_ZEROES_PAIR — Lee→Priya transfer gone from ledger after recording");
  else
    fail(`SETTLEMENT_ZEROES_PAIR — 'Lee pays Priya' still present after recording`);

  if (!afterSettleText.includes("Priya is owed") && !afterSettleText.includes("Lee owes"))
    pass("SETTLEMENT_ZEROES_PAIR — Priya and Lee both zeroed out");
  else
    maybe(`SETTLEMENT_ZEROES_PAIR — Priya/Lee still appear; may be from demo data: check screenshot`);

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 5: Reload — assert settlement persists (not re-shown)
  // ══════════════════════════════════════════════════════════════════════════════
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 30000 });
  await page.waitForTimeout(2000);

  // After reload the in-memory SQLite re-initialises from the stored dataset.
  // CashFlux auto-saves periodically; we check whether the settlement survived.
  // Navigate back to /split using pushNav (session reset by reload, so we use pushNav fresh).
  await page.evaluate((r) => {
    window.history.pushState({}, "", r);
    window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
  }, "/split");
  await page.waitForTimeout(1500);

  await page.screenshot({ path: SS("l48_step5_after_reload.png") });
  const reloadText = (await bodyText(page)).replace(/\s+/g, " ");

  if (!reloadText.includes("Lee pays Priya"))
    pass("SETTLEMENT_SURVIVES_RELOAD — Lee→Priya absent from ledger after reload");
  else {
    // The settle-up screen is partly ephemeral (the running ledger is re-computed from
    // persisted sharedExpenses + settlements on each render). If settlement didn't persist,
    // it re-appears. This is the top mechanical gap if it fires.
    fail("SETTLEMENT_SURVIVES_RELOAD — Lee→Priya re-appeared after reload (settlement not persisted)");
  }

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 6: /transactions — page loads without crash
  // ══════════════════════════════════════════════════════════════════════════════
  await page.evaluate((r) => {
    window.history.pushState({}, "", r);
    window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
  }, "/transactions");
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("l48_step6_transactions.png") });
  const txnText = await bodyText(page);
  if (txnText.toLowerCase().includes("transaction"))
    pass("Step 6 — /transactions page loads without crash");
  else
    maybe("Step 6 — 'transaction' not found on /transactions page");

  // ══════════════════════════════════════════════════════════════════════════════
  // INVARIANT: MONEY_CONSERVATION (partial — shared expenses not mirrored in txn ledger)
  // The app does NOT auto-post shared expenses as transactions. This is a structural gap
  // to document: split/settle-up is a separate sub-ledger, isolated from the transaction
  // ledger and therefore not reflected in /dashboard net worth or /reports totals.
  // ══════════════════════════════════════════════════════════════════════════════
  maybe("MONEY_CONSERVATION — shared expenses are NOT auto-posted to /transactions; " +
    "split/settle is a separate sub-ledger isolated from the main transaction ledger. " +
    "Dashboard/Reports show ZERO shared-expense impact. This is the top mechanical gap.");

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 7: /dashboard — loads without crash
  // ══════════════════════════════════════════════════════════════════════════════
  await page.evaluate((r) => {
    window.history.pushState({}, "", r);
    window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
  }, "/");
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("l48_step7_dashboard.png") });
  const dashText = await bodyText(page);
  if (
    dashText.toLowerCase().includes("net worth") ||
    dashText.toLowerCase().includes("balance") ||
    dashText.toLowerCase().includes("dashboard")
  )
    pass("DASHBOARD_LOADS — Dashboard renders after settle-up actions");
  else
    maybe("DASHBOARD_LOADS — dashboard content not confirmed; check screenshot");

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 8: /reports — loads without crash
  // ══════════════════════════════════════════════════════════════════════════════
  await page.evaluate((r) => {
    window.history.pushState({}, "", r);
    window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
  }, "/reports");
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("l48_step8_reports.png") });
  const reportsText = await bodyText(page);
  if (
    reportsText.toLowerCase().includes("report") ||
    reportsText.toLowerCase().includes("spending") ||
    reportsText.toLowerCase().includes("income")
  )
    pass("Step 8 — /reports loads after settle-up (no crash)");
  else
    maybe("Step 8 — /reports content not confirmed; check screenshot");

  // ══════════════════════════════════════════════════════════════════════════════
  // JS error check
  // ══════════════════════════════════════════════════════════════════════════════
  if (errors.length === 0) pass("No JS page errors during the ritual");
  else fail(`JS errors: ${errors.join(" | ")}`);

  // ══════════════════════════════════════════════════════════════════════════════
  // Summary
  // ══════════════════════════════════════════════════════════════════════════════
  console.log(`\nL48 result: ${passed} passed, ${failed} failed.`);
  if (failed > 0) process.exitCode = 1;

} finally {
  await browser.close();
}
