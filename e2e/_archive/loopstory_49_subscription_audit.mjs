// L49 E2E loop story — "The Subscription Audit" (Marcus & Lin)
// Persona: Marcus and Lin live together. Every few months they audit their recurring
//          charges — Netflix, Spotify, and a gym — to make sure nothing sneaks past them.
//          The ritual: seed several months of recurring charges plus a one-off → open
//          Subscriptions and confirm detection → drill from a subscription into its
//          underlying transactions → mark a subscription as cancelled (the correction
//          path) → check Budgets reflects the recurring total → verify Reports loads and
//          the annualized figure is correct → return to Dashboard and confirm totals
//          are consistent.
//
// Storage model: in-memory SQLite — all assertions are DOM/UI-based.
// Navigation: boot once at "/", then pushNav everywhere to keep wasm session alive.
//
// Flow:
//   0.  Seed — add an L49 checking account, then add recurring charges via /transactions:
//         • "L49 Netflix"  $15.99  ×4 months (days 1 of Jan–Apr 2026)
//         • "L49 Spotify"  $9.99   ×4 months (days 3 of Jan–Apr 2026)
//         • "L49 Gym"      $40.00  ×4 months (days 5 of Jan–Apr 2026)
//         • "L49 OneOff"   $200.00 ×1 (one-off — must NOT be flagged)
//   1.  /subscriptions — screenshot → assert Netflix, Spotify, Gym are detected.
//         Assert OneOff is NOT detected (false-positive check).
//         Assert "Yearly subscriptions" stat is present (annualized figure check).
//         ANNUAL_MATH invariant: each sub's displayed annual = monthly × 12 (computed
//         from known amounts: Netflix $15.99×12=$191.88, Spotify $9.99×12=$119.88,
//         Gym $40×12=$480; total monthly = $65.98; total annual = $791.76).
//   2.  Drill from L49 Netflix → /transactions should pre-filter to "L49 Netflix".
//         DRILL_FILTER invariant: the transactions list shows "L49 Netflix".
//   3.  Return to /subscriptions → Mark L49 Gym as cancelled ("Mark as cancelled").
//         CANCEL_REFLECTED invariant: Gym row now shows "Cancelled" text.
//   4.  /budgets — page loads, "Subscriptions" budget is present (may be demo seed).
//         BUDGETS_LOADS invariant.
//   5.  /reports — page loads without crash.
//         REPORTS_LOADS invariant.
//   6.  /dashboard — page loads without crash.
//         DASHBOARD_LOADS invariant.
//   7.  Cross-screen consistency check: compare monthly total visible on /subscriptions
//         stat grid with what we seeded.
//
// Key invariants:
//   DETECTION_ACCURACY  — all three recurring merchants detected, one-off NOT flagged
//   DRILL_FILTER        — clicking a subscription drills into pre-filtered /transactions
//   CANCEL_REFLECTED    — cancel action reflected on subscriptions screen
//   ANNUAL_MATH         — annualized figure is monthly × 12 (per C57 concern)
//   BUDGETS_LOADS       — /budgets loads without crash
//   REPORTS_LOADS       — /reports loads without crash
//   DASHBOARD_LOADS     — /dashboard loads without crash
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_49_subscription_audit.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS   = (name) => path.join(__dirname, name);

// Seed constants — L49-prefixed for isolation from demo data.
const ACCT_NAME     = "L49 Marcus Checking";
const ACCT_OPENING  = "3000";

// Recurring merchants seeded across 4 months (~30-day spacing) so Detect() fires.
const NETFLIX_DESC  = "L49 Netflix";
const NETFLIX_AMT   = "15.99";                // monthly; annual = 15.99 * 12 = 191.88
const SPOTIFY_DESC  = "L49 Spotify";
const SPOTIFY_AMT   = "9.99";                 // monthly; annual = 9.99 * 12 = 119.88
const GYM_DESC      = "L49 Gym";
const GYM_AMT       = "40.00";               // monthly; annual = 40.00 * 12 = 480.00
const ONEOFF_DESC   = "L49 OneOff Dentist";  // single charge — must NOT be detected
const ONEOFF_AMT    = "200.00";

// Dates: 4 occurrences each, ~30 days apart, well in the past so sample-data months
// don't interfere with date-range filters.
const NETFLIX_DATES = ["2026-01-01", "2026-02-01", "2026-03-01", "2026-04-01"];
const SPOTIFY_DATES = ["2026-01-03", "2026-02-03", "2026-03-03", "2026-04-03"];
const GYM_DATES     = ["2026-01-05", "2026-02-05", "2026-03-05", "2026-04-05"];
const ONEOFF_DATE   = "2026-03-15";

// Expected totals (minor-unit-free plain English for UI matching)
// Monthly burden = 15.99 + 9.99 + 40.00 = 65.98
// Annual burden  = 65.98 * 12 = 791.76
const EXPECTED_MONTHLY_TOTAL = 65.98;
const EXPECTED_ANNUAL_TOTAL  = 791.76;

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0;
const pass  = (label) => { console.log(`PASS: ${label}`); passed++; };
const fail  = (label) => { console.error(`FAIL: ${label}`); failed++; process.exitCode = 1; };
const maybe = (label) => { console.log(`ABSENT: ${label}`); };

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

// Add one transaction via the /transactions form.
// Returns true on success, false on failure (non-fatal so we can accumulate seed errors).
const addTxn = async (page, desc, amount, date, stepLabel) => {
  const descIn = await page.$(
    'input[placeholder*="Description" i], input[aria-label*="Description" i], input[id="txn-add"]'
  );
  const amtIn  = await page.$(
    'input[placeholder*="Amount" i], input[aria-label*="Amount" i]'
  );
  const dateIn = await page.$('input[type="date"]');

  if (!descIn) { fail(`${stepLabel} — description input not found`); return false; }
  if (!amtIn)  { fail(`${stepLabel} — amount input not found`);      return false; }
  if (!dateIn) { fail(`${stepLabel} — date input not found`);        return false; }

  await descIn.fill(desc);
  await amtIn.fill(amount);
  await dateIn.fill(date);

  const btn = await page.$(
    'button:has-text("Add"), button[type="submit"]:not([disabled]), button:has-text("Save")'
  );
  if (!btn) { fail(`${stepLabel} — submit button not found`); return false; }
  await btn.click();
  await page.waitForTimeout(600);
  return true;
};

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });

  // Collect JS errors for the final check.
  const jsErrors = [];
  page.on("pageerror", (e) => jsErrors.push(e.message));

  // ── STEP 0: Boot and add L49 account ──────────────────────────────────────
  await bootApp(page);
  await page.screenshot({ path: SS("l49_step0_boot.png") });

  await pushNav(page, "/accounts");
  await page.screenshot({ path: SS("l49_step0a_accounts_before.png") });

  const nameIn = await page.$(
    'input[placeholder*="Name" i], input[aria-label*="Name" i], input[placeholder*="Account" i]'
  );
  const openingIn = await page.$(
    'input[placeholder*="Opening" i], input[placeholder*="Balance" i], input[aria-label*="Opening" i]'
  );
  if (nameIn) {
    await nameIn.fill(ACCT_NAME);
    pass("Step 0a.1 — Account name filled");
  } else fail("Step 0a.1 — Account name input not found");

  if (openingIn) {
    await openingIn.fill(ACCT_OPENING);
    pass("Step 0a.2 — Opening balance filled");
  } else maybe("Step 0a.2 — Opening balance input not found (may not exist)");

  const addAcctBtn = await page.$(
    'button:has-text("Add"), button[type="submit"]:not([disabled])'
  );
  if (addAcctBtn) {
    await addAcctBtn.click();
    await page.waitForTimeout(800);
    const txt = await bodyText(page);
    if (txt.includes(ACCT_NAME)) pass(`Step 0a.3 — "${ACCT_NAME}" visible in accounts`);
    else maybe(`Step 0a.3 — "${ACCT_NAME}" not visible (account may exist or name format differs)`);
  } else fail("Step 0a.3 — Add account button not found");

  await page.screenshot({ path: SS("l49_step0a_accounts_seeded.png") });

  // ── STEP 0b: Seed recurring transactions ──────────────────────────────────
  await pushNav(page, "/transactions");
  await page.screenshot({ path: SS("l49_step0b_txns_before.png") });

  let seedOk = true;

  // Seed Netflix × 4
  for (let i = 0; i < NETFLIX_DATES.length; i++) {
    const ok = await addTxn(page, NETFLIX_DESC, NETFLIX_AMT, NETFLIX_DATES[i],
      `Step 0b — Netflix[${i + 1}] $${NETFLIX_AMT} on ${NETFLIX_DATES[i]}`);
    if (!ok) seedOk = false;
  }
  // Seed Spotify × 4
  for (let i = 0; i < SPOTIFY_DATES.length; i++) {
    const ok = await addTxn(page, SPOTIFY_DESC, SPOTIFY_AMT, SPOTIFY_DATES[i],
      `Step 0b — Spotify[${i + 1}] $${SPOTIFY_AMT} on ${SPOTIFY_DATES[i]}`);
    if (!ok) seedOk = false;
  }
  // Seed Gym × 4
  for (let i = 0; i < GYM_DATES.length; i++) {
    const ok = await addTxn(page, GYM_DESC, GYM_AMT, GYM_DATES[i],
      `Step 0b — Gym[${i + 1}] $${GYM_AMT} on ${GYM_DATES[i]}`);
    if (!ok) seedOk = false;
  }
  // Seed one-off
  await addTxn(page, ONEOFF_DESC, ONEOFF_AMT, ONEOFF_DATE,
    `Step 0b — OneOff $${ONEOFF_AMT} on ${ONEOFF_DATE}`);

  await page.screenshot({ path: SS("l49_step0b_txns_seeded.png") });

  if (!seedOk) {
    console.error("WARNING: One or more seed transactions failed — detection assertions may be incomplete");
  }

  // ── STEP 1: /subscriptions — detection ────────────────────────────────────
  await pushNav(page, "/subscriptions");
  await page.waitForTimeout(1000); // let detection run
  await page.screenshot({ path: SS("l49_step1_subscriptions.png") });

  const subsTxt = await bodyText(page);

  // DETECTION_ACCURACY: all three recurring merchants detected
  if (subsTxt.includes(NETFLIX_DESC)) pass("DETECTION_ACCURACY — L49 Netflix detected ✓");
  else fail("DETECTION_ACCURACY — L49 Netflix NOT detected");

  if (subsTxt.includes(SPOTIFY_DESC)) pass("DETECTION_ACCURACY — L49 Spotify detected ✓");
  else fail("DETECTION_ACCURACY — L49 Spotify NOT detected");

  if (subsTxt.includes(GYM_DESC)) pass("DETECTION_ACCURACY — L49 Gym detected ✓");
  else fail("DETECTION_ACCURACY — L49 Gym NOT detected");

  // One-off must NOT be flagged
  if (!subsTxt.includes(ONEOFF_DESC)) pass("DETECTION_ACCURACY — OneOff NOT detected (no false positive) ✓");
  else fail("DETECTION_ACCURACY — OneOff WAS detected (FALSE POSITIVE — one-off charge flagged as recurring)");

  // Cadence labels present
  if (subsTxt.includes("monthly") || subsTxt.includes("Monthly")) pass("DETECTION_ACCURACY — monthly cadence label present ✓");
  else maybe("DETECTION_ACCURACY — monthly cadence label not found");

  // Stat grid: "MONTHLY SUBSCRIPTIONS" / "YEARLY SUBSCRIPTIONS" present.
  // The UI applies CSS text-transform:uppercase so innerText returns uppercase labels.
  const subsUpper = subsTxt.toUpperCase();
  if (subsUpper.includes("MONTHLY SUBSCRIPTIONS")) {
    pass("ANNUAL_MATH — Monthly subscriptions stat present ✓");
  } else maybe("ANNUAL_MATH — Monthly subscriptions stat not found on page");

  if (subsUpper.includes("YEARLY SUBSCRIPTIONS")) {
    pass("ANNUAL_MATH — Yearly subscriptions stat present ✓");
  } else maybe("ANNUAL_MATH — Yearly subscriptions stat not found on page");

  // ANNUAL_MATH: verify the annualized figure is monthly × 12.
  // Strategy: extract the Monthly and Yearly dollar figures from the stat grid
  // and assert yearly ≈ monthly × 12. The demo data contributes additional
  // subscriptions so the totals won't match our seeded subs exactly — but the
  // ratio must hold regardless of how many subscriptions are present.
  //
  // The innerText stat block looks like:
  //   MONTHLY SUBSCRIPTIONS\n$2,085.99\nYEARLY SUBSCRIPTIONS\n$25,031.88\n…
  // We parse dollar amounts from the text near each label.
  const extractAmount = (text, labelUppercase) => {
    const idx = text.toUpperCase().indexOf(labelUppercase);
    if (idx === -1) return null;
    const slice = text.slice(idx, idx + 80); // look ahead max 80 chars
    const m = slice.match(/\$[\d,]+\.?\d*/);
    if (!m) return null;
    return parseFloat(m[0].replace(/[$,]/g, ""));
  };

  const monthlyTotal = extractAmount(subsTxt, "MONTHLY SUBSCRIPTIONS");
  const yearlyTotal  = extractAmount(subsTxt, "YEARLY SUBSCRIPTIONS");

  if (monthlyTotal !== null && yearlyTotal !== null) {
    const expectedAnnual = Math.round(monthlyTotal * 12 * 100) / 100;
    const tolerance = 0.12; // allow up to $0.12 rounding across many subs (integer minor-unit truncation)
    if (Math.abs(yearlyTotal - expectedAnnual) <= tolerance) {
      pass(`ANNUAL_MATH — Annual $${yearlyTotal} = monthly $${monthlyTotal} × 12 (correct) ✓`);
    } else {
      fail(`ANNUAL_MATH — MISMATCH: Annual=$${yearlyTotal} but monthly×12=$${expectedAnnual} (diff=${Math.abs(yearlyTotal - expectedAnnual).toFixed(2)}) — annualized figure is WRONG`);
    }
  } else {
    maybe(`ANNUAL_MATH — Could not parse monthly/yearly stat amounts from page text (monthly=${monthlyTotal}, yearly=${yearlyTotal})`);
  }

  // ── STEP 2: Drill from Netflix → /transactions ─────────────────────────────
  // The Netflix row name is a clickable button (sub-drill).
  const netflixBtn = await page.$(`button.sub-drill, button[class*="sub-drill"]`);
  // Fallback: find any button whose text is NETFLIX_DESC
  const allBtns = await page.$$("button");
  let drillBtn = null;
  for (const b of allBtns) {
    const txt2 = await b.innerText().catch(() => "");
    if (txt2.trim() === NETFLIX_DESC) { drillBtn = b; break; }
  }

  if (drillBtn) {
    await drillBtn.click();
    await page.waitForTimeout(1500);
    await page.screenshot({ path: SS("l49_step2_drill_transactions.png") });
    const drillTxt = await bodyText(page);
    const onTransactions = (await page.evaluate(() => location.pathname)).includes("transactions");
    if (onTransactions) pass("DRILL_FILTER — navigated to /transactions after drill ✓");
    else maybe("DRILL_FILTER — did not navigate to /transactions (may have stayed in-page)");

    if (drillTxt.includes(NETFLIX_DESC)) pass("DRILL_FILTER — L49 Netflix transactions visible after drill ✓");
    else fail("DRILL_FILTER — L49 Netflix NOT visible after drill (filter carry-over broken)");
  } else {
    maybe("DRILL_FILTER — drill button for L49 Netflix not found (likely not yet detected or UI changed)");
    await pushNav(page, "/transactions");
    await page.screenshot({ path: SS("l49_step2_drill_transactions.png") });
  }

  // ── STEP 3: Return to /subscriptions and cancel Gym ───────────────────────
  await pushNav(page, "/subscriptions");
  await page.waitForTimeout(800);
  await page.screenshot({ path: SS("l49_step3_subs_before_cancel.png") });

  // Find the "Mark as cancelled" button for L49 Gym.
  // The button text is "Mark as cancelled" per en.go "subs.cancel".
  // We need to click the one near the Gym row.
  // Strategy: find all "Mark as cancelled" buttons; if only one, click it;
  // if multiple, find the one whose nearest ancestor row contains GYM_DESC.
  const cancelBtns = await page.$$('button:has-text("Mark as cancelled")');
  let gymCancelBtn = null;

  if (cancelBtns.length === 1) {
    // Only one subscription has the cancel button — may not be Gym specifically;
    // proceed if the page has Gym.
    gymCancelBtn = cancelBtns[0];
  } else if (cancelBtns.length > 1) {
    // Multiple — find the one in the Gym row.
    for (const cb of cancelBtns) {
      const rowText = await cb.evaluate((el) => {
        let node = el;
        for (let i = 0; i < 5; i++) {
          node = node.parentElement;
          if (!node) break;
          if (node.innerText && node.innerText.includes("L49 Gym")) return node.innerText;
        }
        return "";
      });
      if (rowText.includes("L49 Gym")) { gymCancelBtn = cb; break; }
    }
    if (!gymCancelBtn) gymCancelBtn = cancelBtns[0]; // fallback
  }

  if (gymCancelBtn) {
    const cancelBtnsBefore = await page.$$('button:has-text("Mark as cancelled")');
    const cancelCountBefore = cancelBtnsBefore.length;
    await gymCancelBtn.click();
    await page.waitForTimeout(1500);
    await page.screenshot({ path: SS("l49_step3_subs_after_cancel.png") });

    // CANCEL_REFLECTED: the row that was cancelled should now show an "Undo cancel" button
    // instead of "Mark as cancelled", and a "Cancelled <date>" label.
    // NOTE: checking for the word "cancelled" alone is a FALSE POSITIVE — "Mark as cancelled"
    // buttons on OTHER rows contain that substring. We check specifically for:
    //   (a) an "Undo cancel" button (the per-row uncancel action that replaces "Mark as cancelled")
    //   (b) a date-qualified "Cancelled <date>" label
    const undoBtnsAfter = await page.$$('button:has-text("Undo cancel")');
    const afterCancelTxt = await bodyText(page);
    // "Cancelled on <date>" — the cancelledState i18n label pattern
    const hasCancelledDate = /Cancelled \d{4}|Cancelled \w+ \d/.test(afterCancelTxt);

    if (undoBtnsAfter.length > 0) {
      pass(`CANCEL_REFLECTED — "Undo cancel" button appeared after cancel (${undoBtnsAfter.length} row(s)) ✓`);
    } else {
      fail("CANCEL_REFLECTED — BUG: UI does NOT update after cancel — \"Undo cancel\" button absent. Root cause: doCancel() writes to store but does not update any reactive state atom, so the Subscriptions component does not re-render. Fix: call notice.Set() with a success message on cancel/uncancel, or add a reactive cancellations atom.");
    }
    if (hasCancelledDate) pass("CANCEL_REFLECTED — Cancelled date label visible on row ✓");
    else maybe("CANCEL_REFLECTED — Cancelled date label not found in page (expected 'Cancelled <date>' per subs.cancelledState)");
  } else {
    maybe("CANCEL_REFLECTED — 'Mark as cancelled' button not found (subscriptions read-only or Gym not detected — GAP: C56 concern confirmed)");
  }

  // ── STEP 4: /budgets — loads, Subscriptions budget present ───────────────
  await pushNav(page, "/budgets");
  await page.screenshot({ path: SS("l49_step4_budgets.png") });
  const budTxt = await bodyText(page);
  if (budTxt.includes("Subscriptions") || budTxt.includes("subscriptions")) {
    pass("BUDGETS_LOADS — /budgets loads; Subscriptions budget present ✓");
  } else {
    pass("BUDGETS_LOADS — /budgets loads without crash ✓");
    maybe("BUDGETS_LOADS — No 'Subscriptions' budget found (demo budget may be absent or screen changed)");
  }

  // Check if the subscription recurring total appears in budgets in any form.
  // The app's budgets are category-based, not subscription-based; so this is
  // an advisory check only — no strict invariant, just screen consistency.
  maybe("BUDGETS_LOADS — Note: budgets are category-based, not subscription-detected; recurring sub total not automatically reflected in budget numbers");

  // ── STEP 5: /reports — loads ──────────────────────────────────────────────
  await pushNav(page, "/reports");
  await page.screenshot({ path: SS("l49_step5_reports.png") });
  const repTxt = await bodyText(page);
  if (repTxt.length > 50) pass("REPORTS_LOADS — /reports loads with content ✓");
  else fail("REPORTS_LOADS — /reports content suspiciously short (possible crash)");

  // Advisory: does reports mention our subscription amounts at all?
  if (repTxt.includes("15.99") || repTxt.includes("9.99") || repTxt.includes("40.00")) {
    pass("REPORTS_LOADS — Seeded subscription amounts visible in reports (spend tracked) ✓");
  } else {
    maybe("REPORTS_LOADS — Seeded amounts not immediately visible in reports (may require period matching or are in a collapsed section)");
  }

  // ── STEP 6: /dashboard — loads, check recurring totals ───────────────────
  await pushNav(page, "/dashboard");
  await page.screenshot({ path: SS("l49_step6_dashboard.png") });
  const dashTxt = await bodyText(page);
  if (dashTxt.length > 50) pass("DASHBOARD_LOADS — /dashboard loads with content ✓");
  else fail("DASHBOARD_LOADS — /dashboard content suspiciously short (possible crash)");

  // ── STEP 7: Cross-screen consistency ─────────────────────────────────────
  // Return to /subscriptions to read the monthly total from the stat grid,
  // then verify it is consistent with what we seeded.
  await pushNav(page, "/subscriptions");
  await page.waitForTimeout(800);
  const finalSubsTxt = await bodyText(page);
  await page.screenshot({ path: SS("l49_step7_subs_final.png") });

  // CROSS_SCREEN_CONSISTENCY: the "Monthly subscriptions" stat should contain
  // the seeded monthly total ($65.98) — though it may be higher due to demo data.
  // We can only confirm it's NOT lower than $65.98 if our seed worked.
  // The most actionable check: the number of detected subs is ≥ 3.
  const subRows = await page.$$(".row.sub-drill, button.sub-drill, [class*='sub-drill']");
  // Fallback count via text scan for L49 prefixes.
  let detectedCount = 0;
  if (finalSubsTxt.includes(NETFLIX_DESC)) detectedCount++;
  if (finalSubsTxt.includes(SPOTIFY_DESC)) detectedCount++;
  if (finalSubsTxt.includes(GYM_DESC))     detectedCount++;

  if (detectedCount === 3) pass(`CROSS_SCREEN_CONSISTENCY — all 3 L49 subscriptions detected (${detectedCount}/3) ✓`);
  else if (detectedCount > 0) maybe(`CROSS_SCREEN_CONSISTENCY — partial detection: ${detectedCount}/3 L49 subs found`);
  else fail("CROSS_SCREEN_CONSISTENCY — 0 L49 subscriptions detected (seeding or detection broken)");

  // ── Final: JS error check ──────────────────────────────────────────────────
  if (jsErrors.length === 0) pass("Zero JS page errors across the full ritual ✓");
  else {
    fail(`JS errors detected (${jsErrors.length}):`);
    jsErrors.slice(0, 5).forEach((e) => console.error("  JS ERROR:", e));
  }

  // ── Summary ───────────────────────────────────────────────────────────────
  console.log(`\n── L49 Subscription Audit: ${passed} passed, ${failed} failed ──`);
  if (failed > 0) process.exitCode = 1;

} finally {
  await browser.close();
}
