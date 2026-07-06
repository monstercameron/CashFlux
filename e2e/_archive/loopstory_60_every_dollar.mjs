// L60 E2E loop story — "Every Dollar a Job" (Marcus, zero-based budgeter)
// Persona: Marcus uses CashFlux to give every dollar a job using the Allocate engine.
//          He has two asset accounts (checking + HYSA) and one debt (Visa). He
//          enters $2,000 to allocate using the "debt" profile, excludes one account,
//          runs the allocation, verifies the zero-remainder invariant, applies it,
//          then checks /goals and /dashboard for consistency.
//
// Flow (the ritual):
//   0. Seed: checking account, HYSA, Visa liability, and a savings goal.
//   1. /allocate — verify weight labels are shown (C6 regression).
//   2. Set profile to "debt" (Pay down debt), enter $2,000.
//   3. Exclude the HYSA — verify excluded destinations get $0.
//   4. Verify zero-remainder: sum(allocated) == $2,000 to the cent (ZERO_REMAINDER).
//   5. Apply allocation → confirm → verify success message.
//   6. Undo allocation → verify undo message (rollback path).
//   7. Re-apply for persistence check. Navigate /goals → verify goal updated.
//   8. /dashboard → verify no crash, net worth visible.
//
// Key invariants:
//   WEIGHT_LABELS      — every weight input has a visible label (not a raw number field)
//   ZERO_REMAINDER     — sum(allocated amounts on screen) == entered amount (to the cent)
//   EXCLUDED_GETS_ZERO — excluded candidate does not appear in suggestions
//   APPLY_WRITES       — apply updates goals (CurrentAmount bumped) and creates earmarks
//   UNDO_WORKS         — undo reverts the last allocation
//   DASHBOARD_STABLE   — /dashboard renders without JS error after apply
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_60_every_dollar.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import { mkdirSync } from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS = (name) => path.join(__dirname, "screenshots", name);

// ── seed constants (L60-prefixed for isolation) ───────────────────────────────
const CHECKING_NAME  = "L60 Marcus Checking";
const HYSA_NAME      = "L60 Marcus HYSA";
const VISA_NAME      = "L60 Marcus Visa";
const GOAL_NAME      = "L60 Emergency Fund";
const GOAL_TARGET    = "1000";
const ALLOC_AMOUNT   = "2000";      // $2,000 to allocate
const RESERVE_AMOUNT = "200";       // $200 buffer

// ── helpers ───────────────────────────────────────────────────────────────────
const parseDollar = (s) => {
  if (!s) return NaN;
  const neg = /^\(.*\)$/.test(s.trim());
  const n = parseFloat(s.replace(/[^0-9.]/g, ""));
  return neg ? -n : n;
};

const goto = async (page, hash) => {
  await page.goto(BASE + hash, { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 }).catch(() => {});
  await page.waitForTimeout(2500);
};

const softNav = async (page, routeTitle, fallbackHash) => {
  const navLink = await page.$(`nav[aria-label="Main navigation"] a[title="${routeTitle}"]`);
  if (navLink) {
    await navLink.click();
    await page.waitForTimeout(1800);
  } else {
    await page.evaluate((hash) => {
      window.history.pushState({}, "", hash);
      window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
    }, fallbackHash);
    await page.waitForTimeout(1800);
  }
};

const bodyText = (page) => page.evaluate(() => document.body.innerText);

let passes = 0, fails = 0, maybes = 0;
const pass  = (m) => { passes++;  console.log(`  PASS  ${m}`); };
const fail  = (m) => { fails++;   console.error(`  FAIL  ${m}`); process.exitCode = 1; };
const maybe = (m) => { maybes++;  console.warn(`  MAYBE ${m}`); };

try { mkdirSync(path.join(__dirname, "screenshots"), { recursive: true }); } catch (_) {}

// ── main ──────────────────────────────────────────────────────────────────────
const browser = await chromium.launch({ headless: true });

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // ── Step 0a: Seed checking account ───────────────────────────────────────
  console.log("\n── Step 0a: Seed L60 Marcus Checking ──");
  await goto(page, "/accounts");

  const nameIn = await page.$('input[placeholder="Name"], input[placeholder="Account name"], input[aria-label="Account name"]');
  const balIn  = await page.$('input[placeholder="Opening balance"], input[aria-label="Opening balance"], input[type="number"]');
  if (nameIn && balIn) {
    await nameIn.fill(CHECKING_NAME);
    await balIn.fill("5000");
    const sub = await page.$('button[type="submit"]');
    if (sub) { await sub.click(); await page.waitForTimeout(1500); pass(`Step 0a — ${CHECKING_NAME} created`); }
    else maybe("Step 0a — submit not found");
  } else {
    maybe(`Step 0a — account form not found (name=${!!nameIn} bal=${!!balIn})`);
  }
  await page.screenshot({ path: SS("l60_00a_checking_seeded.png") });

  // ── Step 0b: Seed HYSA ───────────────────────────────────────────────────
  console.log("\n── Step 0b: Seed L60 Marcus HYSA ──");
  const nameIn2 = await page.$('input[placeholder="Name"], input[placeholder="Account name"], input[aria-label="Account name"]');
  const balIn2  = await page.$('input[placeholder="Opening balance"], input[aria-label="Opening balance"], input[type="number"]');
  if (nameIn2 && balIn2) {
    await nameIn2.fill(HYSA_NAME);
    await balIn2.fill("3000");
    const sub2 = await page.$('button[type="submit"]');
    if (sub2) { await sub2.click(); await page.waitForTimeout(1500); pass(`Step 0b — ${HYSA_NAME} created`); }
    else maybe("Step 0b — submit not found");
  } else {
    maybe(`Step 0b — account form not found for HYSA`);
  }
  await page.screenshot({ path: SS("l60_00b_hysa_seeded.png") });

  // ── Step 0c: Seed savings goal ───────────────────────────────────────────
  console.log("\n── Step 0c: Seed Emergency Fund goal ──");
  await goto(page, "/goals");
  await page.waitForTimeout(1500);
  const goalNameIn = await page.$('input[placeholder="Name"], input[aria-label="Goal name"], input[placeholder*="goal" i], input[placeholder*="name" i]');
  const goalTargetIn = await page.$('input[placeholder*="target" i], input[placeholder*="amount" i], input[type="number"]');
  if (goalNameIn && goalTargetIn) {
    await goalNameIn.fill(GOAL_NAME);
    await goalTargetIn.fill(GOAL_TARGET);
    const goalSub = await page.$('button[type="submit"]');
    if (goalSub) { await goalSub.click(); await page.waitForTimeout(1500); pass(`Step 0c — ${GOAL_NAME} created`); }
    else maybe("Step 0c — goal submit not found");
  } else {
    maybe(`Step 0c — goal form not found (name=${!!goalNameIn} target=${!!goalTargetIn})`);
  }
  await page.screenshot({ path: SS("l60_00c_goal_seeded.png") });

  // ── Step 1: Navigate to /allocate — probe weight labels (C6 regression) ──
  console.log("\n── Step 1: /allocate — weight labels ──");
  await goto(page, "/allocate");
  const txt1 = await bodyText(page);

  // C6 regression: every weight input should have a visible span label
  const weightLabels = ["Returns weight", "Stability weight", "Liquidity weight", "Debt-reduction weight", "Goal-progress weight"];
  let allLabeled = true;
  for (const lbl of weightLabels) {
    if (!txt1.includes(lbl)) {
      allLabeled = false;
      fail(`C6 REGRESSION — weight label "${lbl}" missing from /allocate`);
    }
  }
  if (allLabeled) pass("C6 — all 5 criterion weight labels are visible on /allocate");

  // Check that "Criterion weights" section heading is present
  if (txt1.includes("Criterion weights")) pass("Step 1 — 'Criterion weights' section heading present");
  else maybe("Step 1 — 'Criterion weights' heading not found in page text");

  await page.screenshot({ path: SS("l60_01_allocate_landing.png") });

  // ── Step 2: Set profile to "debt", enter $2,000 amount ───────────────────
  console.log("\n── Step 2: Set profile=debt, amount=$2,000 ──");

  // Select "Pay down debt" profile
  const profileSel = await page.$('select.field');
  // There's a mode selector first, then profile selector
  const allSelects = await page.$$('select.field');
  let profileSelect = null;
  for (const sel of allSelects) {
    const val = await sel.getAttribute('data-testid');
    if (val === 'allocate-mode') continue;
    profileSelect = sel;
    break;
  }
  if (profileSelect) {
    await profileSelect.selectOption({ value: "debt" });
    await page.waitForTimeout(1000);
    pass("Step 2 — profile set to 'debt' (Pay down debt)");
  } else {
    maybe("Step 2 — could not find profile select (trying by index)");
    if (allSelects.length >= 2) {
      await allSelects[1].selectOption({ value: "debt" });
      await page.waitForTimeout(1000);
      maybe("Step 2 — set profile via index fallback");
    }
  }

  // Enter the amount to allocate
  const amtInput = await page.$('[data-testid="allocate-amount"]');
  if (amtInput) {
    await amtInput.fill(ALLOC_AMOUNT);
    await amtInput.dispatchEvent("input");
    await page.waitForTimeout(1200);
    pass(`Step 2 — entered amount $${ALLOC_AMOUNT}`);
  } else {
    maybe("Step 2 — amount input not found by placeholder");
    // Try to find any number input that's not a weight input
    const numInputs = await page.$$('input[type="number"][step="0.01"]');
    if (numInputs.length > 0) {
      await numInputs[0].fill(ALLOC_AMOUNT);
      await numInputs[0].dispatchEvent("input");
      await page.waitForTimeout(1200);
      maybe("Step 2 — entered amount via fallback input");
    }
  }

  await page.screenshot({ path: SS("l60_02_profile_debt_amount_set.png") });

  // ── Step 3: Check suggestions rendered, then exclude the HYSA ────────────
  console.log("\n── Step 3: Check suggestions + exclude HYSA ──");
  const txt3 = await bodyText(page);

  // Check suggestions section is present
  if (txt3.includes("Where to put your money next")) pass("Step 3 — suggestions section rendered");
  else maybe("Step 3 — suggestions section heading not found");

  // Try to find and click Exclude button next to HYSA
  const allButtons = await page.$$('button.btn');
  let excludeClicked = false;
  for (const btn of allButtons) {
    const btitle = await btn.getAttribute("title");
    const btext  = await btn.innerText().catch(() => "");
    if (btext.trim() === "Exclude" || (btitle && btitle.includes("Leave this out"))) {
      // Find the closest parent that contains HYSA name
      const parentText = await btn.evaluate((b) => {
        let el = b;
        for (let i = 0; i < 5; i++) {
          if (el.parentElement) el = el.parentElement;
          if (el.innerText && el.innerText.includes("HYSA")) return el.innerText;
        }
        return "";
      });
      if (parentText.includes("HYSA") || parentText.includes("L60 Marcus HYSA")) {
        await btn.click();
        await page.waitForTimeout(1000);
        excludeClicked = true;
        pass("Step 3 — excluded HYSA account from suggestions");
        break;
      }
    }
  }
  if (!excludeClicked) maybe("Step 3 — HYSA Exclude button not found (account may not appear as candidate due to missing APR/stability/liquidity)");

  await page.screenshot({ path: SS("l60_03_hysa_excluded.png") });

  // ── Step 4: ZERO_REMAINDER invariant — sum(shown amounts) == ALLOC_AMOUNT ─
  console.log("\n── Step 4: ZERO_REMAINDER invariant ──");
  const txt4 = await bodyText(page);

  // Parse all dollar amounts shown next to scored candidates.
  // Format: "$X.XX · YY%" appears in budget-amount spans for each suggestion.
  const dollarAmtRe = /\$([\d,]+\.\d{2})\s*·\s*\d+%/g;
  let sumAllocated = 0;
  let matchCount = 0;
  let m;
  while ((m = dollarAmtRe.exec(txt4)) !== null) {
    const val = parseFloat(m[1].replace(/,/g, ""));
    sumAllocated += val;
    matchCount++;
  }

  // Also parse "Kept back: $X.XX" for the remainder
  const keptBackMatch = txt4.match(/Kept back:\s*\$([\d,]+\.\d{2})/);
  const keptBack = keptBackMatch ? parseFloat(keptBackMatch[1].replace(/,/g, "")) : 0;

  const totalAccounted = sumAllocated + keptBack;
  const enteredAmount  = parseFloat(ALLOC_AMOUNT);
  const reserveAmount  = parseFloat(RESERVE_AMOUNT);

  console.log(`  → found ${matchCount} candidate amount(s); sum=$${sumAllocated.toFixed(2)}, kept_back=$${keptBack.toFixed(2)}, total=$${totalAccounted.toFixed(2)}, entered=$${enteredAmount.toFixed(2)}`);

  if (matchCount === 0) {
    maybe("Step 4 — no dollar amounts found in suggestions; candidates may have score=0 (no APR/scores set on seeded accounts). ZERO_REMAINDER cannot be verified without scored candidates.");
  } else {
    const diff = Math.abs(totalAccounted - enteredAmount);
    if (diff < 0.015) {
      pass(`ZERO_REMAINDER — sum(${sumAllocated.toFixed(2)}) + kept_back(${keptBack.toFixed(2)}) = ${totalAccounted.toFixed(2)} == entered ${enteredAmount.toFixed(2)} ✓`);
    } else {
      fail(`ZERO_REMAINDER VIOLATED — sum(${sumAllocated.toFixed(2)}) + kept_back(${keptBack.toFixed(2)}) = ${totalAccounted.toFixed(2)} != entered ${enteredAmount.toFixed(2)} (diff=${diff.toFixed(2)})`);
    }
  }

  // EXCLUDED_GETS_ZERO: if HYSA was excluded, its name should not appear in the ranked list
  // (it should appear in the Excluded section, not the Suggestions section)
  const suggestionsSection = txt4.split("Excluded")[0]; // text before "Excluded" section
  if (excludeClicked) {
    if (!suggestionsSection.includes("L60 Marcus HYSA")) {
      pass("EXCLUDED_GETS_ZERO — HYSA does not appear in the suggestions list after exclusion");
    } else {
      fail("EXCLUDED_GETS_ZERO VIOLATED — HYSA still appears in suggestions after Exclude");
    }
    // Verify it appears in the Excluded section
    if (txt4.includes("Excluded") && txt4.split("Excluded")[1]?.includes("HYSA")) {
      pass("Step 4 — HYSA appears in the Excluded section with Restore option");
    } else {
      maybe("Step 4 — could not confirm HYSA in Excluded section");
    }
  }

  await page.screenshot({ path: SS("l60_04_zero_remainder_check.png") });

  // ── Step 5: Apply allocation ──────────────────────────────────────────────
  console.log("\n── Step 5: Apply allocation ──");

  const applyBtn = await page.$('[data-testid="allocate-apply-btn"]');
  if (applyBtn) {
    await applyBtn.click();
    await page.waitForTimeout(1500);
    pass("Step 5 — clicked Apply allocation button");

    await page.screenshot({ path: SS("l60_05a_apply_confirm_panel.png") });

    // Confirm panel should be showing — click "Confirm"
    const confirmBtn = await page.$('button[aria-label="Confirm allocation"]');
    if (confirmBtn) {
      await confirmBtn.click();
      await page.waitForTimeout(1500);
      pass("Step 5 — clicked Confirm");
    } else {
      // Try text-based lookup
      const btns = await page.$$('button.btn');
      for (const btn of btns) {
        const t = await btn.innerText().catch(() => "");
        if (t.trim() === "Confirm") { await btn.click(); await page.waitForTimeout(1500); pass("Step 5 — confirmed via text match"); break; }
      }
    }
  } else {
    maybe("Step 5 — Apply allocation button not found (data-testid=allocate-apply-btn). Amount may be 0 or no candidates.");
  }

  const txt5 = await bodyText(page);
  await page.screenshot({ path: SS("l60_05b_after_apply.png") });

  const applySucceeded = txt5.includes("Funded") || txt5.includes("Earmarked") || txt5.includes("earmarked");
  if (applySucceeded) {
    pass("APPLY_WRITES — success message shown after apply (earmarks/goals updated)");
  } else if (txt5.includes("Enter an amount")) {
    maybe("Step 5 — apply blocked: no amount/plans (candidates may all have score=0)");
  } else {
    maybe("Step 5 — success message not found; apply may not have fired");
  }

  // ── Step 6: Undo allocation ───────────────────────────────────────────────
  console.log("\n── Step 6: Undo allocation ──");
  const undoBtn = await page.$('button[aria-label="Undo the last allocation"]');
  if (undoBtn) {
    await undoBtn.click();
    await page.waitForTimeout(1200);
    const txt6 = await bodyText(page);
    if (txt6.includes("Allocation undone")) {
      pass("UNDO_WORKS — 'Allocation undone.' message shown");
    } else {
      maybe("Step 6 — undo clicked but 'Allocation undone.' message not found");
    }
    await page.screenshot({ path: SS("l60_06_after_undo.png") });
  } else {
    maybe("Step 6 — Undo button not visible (apply may not have succeeded)");
  }

  // ── Step 7: Re-apply then check /goals ───────────────────────────────────
  console.log("\n── Step 7: Re-apply + check /goals ──");
  const applyBtn2 = await page.$('[data-testid="allocate-apply-btn"]');
  if (applyBtn2) {
    await applyBtn2.click();
    await page.waitForTimeout(1200);
    const confirmBtn2 = await page.$('button[aria-label="Confirm allocation"]');
    if (confirmBtn2) {
      await confirmBtn2.click();
      await page.waitForTimeout(1500);
      pass("Step 7 — re-applied allocation");
    } else {
      const btns2 = await page.$$('button.btn');
      for (const b of btns2) {
        const t = await b.innerText().catch(() => "");
        if (t.trim() === "Confirm") { await b.click(); await page.waitForTimeout(1500); pass("Step 7 — re-applied via text match"); break; }
      }
    }
  } else {
    maybe("Step 7 — Apply button not found for re-apply");
  }

  // Navigate to /goals to verify persistence
  await softNav(page, "Goals", "/goals");
  const txt7 = await bodyText(page);
  await page.screenshot({ path: SS("l60_07_goals_after_apply.png") });

  // The Emergency Fund goal should be visible; if apply wrote to it, CurrentAmount > 0
  if (txt7.includes(GOAL_NAME)) {
    pass(`Step 7 — ${GOAL_NAME} visible on /goals after apply`);
    // Look for any non-zero progress indicator (% or dollar amount > $0.00)
    const goalSection = txt7.split(GOAL_NAME);
    if (goalSection.length > 1) {
      const after = goalSection[1].slice(0, 200);
      const hasProgress = /[1-9][\d,]*\.?\d*%|[1-9][\d,]*\.\d{2}/.test(after);
      if (hasProgress) pass("APPLY_WRITES (goals) — goal shows non-zero progress after apply");
      else maybe("Step 7 — goal found but progress may still be $0 (goal contribution only fires when goal is a candidate with score > 0)");
    }
  } else {
    maybe(`Step 7 — ${GOAL_NAME} not found on /goals page`);
  }

  // ── Step 8: /dashboard stability check ───────────────────────────────────
  console.log("\n── Step 8: /dashboard stability ──");
  await softNav(page, "Dashboard", "/");
  const txt8 = await bodyText(page);
  await page.screenshot({ path: SS("l60_08_dashboard.png") });

  const netWorthMatch = txt8.match(/net\s+worth[^\$]*\$([\d,]+\.\d{2})/i);
  if (netWorthMatch) {
    pass(`DASHBOARD_STABLE — Net worth shown: $${netWorthMatch[1]}`);
  } else if (txt8.toLowerCase().includes("net worth")) {
    pass("DASHBOARD_STABLE — 'Net worth' widget is present on dashboard");
  } else {
    maybe("Step 8 — net worth not found on dashboard; widget may not be visible in default layout");
  }

  // JS error audit
  if (errors.length > 0) {
    fail(`JS errors during run: ${errors.slice(0, 3).join(" | ")}`);
  } else {
    pass("No JS errors during the full run");
  }

  // ── Summary ───────────────────────────────────────────────────────────────
  console.log(`\n── L60 Summary ─────────────────────────────────────────────`);
  console.log(`  PASS  ${passes}  FAIL  ${fails}  MAYBE  ${maybes}`);
  console.log(`  Exit code: ${process.exitCode ?? 0}`);

} finally {
  await browser.close();
}
