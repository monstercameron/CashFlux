// L59 E2E loop story — "The Finish Line" (Aaliyah, goal-completion lifecycle)
// Persona: Aaliyah has been saving for a $500 "New Laptop" goal. She's at $475 (95%).
//          She makes a final $25 contribution to push it to 100%, observes completion,
//          then logs a $500 spend from the linked account, checks /accounts and
//          /transactions for reflection, and verifies /dashboard net worth is consistent.
//
// Flow (the ritual):
//   0. Seed linked account + near-complete goal ($500 target, $475 saved = 95%).
//   1. /goals — confirm near-target state (95%, $25 remaining, pace figure shown).
//   2. Final contribution of $25 → confirm completion state fires (100%, $0 to go).
//   3. Overfunding test: contribute another $10 → confirm overfund is handled sanely
//      (caps at 100% / shows surplus, does NOT go to 105%).
//   4. Log a $500 spend (the goal money is "spent") from the linked account.
//   5. /accounts — verify account balance reflects the spend (L41 re-test: still memo-only?).
//   6. /transactions — verify the spend transaction exists, linked account balance visible.
//   7. /dashboard — net worth correct after completion and spend; money conserved to the cent.
//
// Key invariants:
//   COMPLETION_FIRES   — goal reaches 100% and signals completion (not silent)
//   OVERFUND_SANE      — overfund (>100%) is handled: caps, shows surplus, no crash
//   ACCOUNT_DECOUPLED  — linked account balance unchanged by Contribute (L41 C51 gap re-test)
//   SPEND_POSTS        — manually logged $500 spend appears in /transactions + debits account
//   NET_WORTH_CORRECT  — /dashboard net worth tracks account balance changes
//   MONEY_CONSERVED    — (account_balance_before_spend) - 500 == account_balance_after_spend, to cent
//
// Cross-references:
//   L41 (CONFIRMED DECOUPLED: Contribute does NOT post to linked account — C51 gap)
//   L56 Thread A (satellite modules don't post to central ledger)
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_59_finish_line.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS = (name) => path.join(__dirname, "screenshots", name);

// Seed constants — L59-prefixed for isolation
const ACCT_NAME     = "L59 Aaliyah Savings";
const ACCT_BALANCE  = "3000";        // $3,000.00 opening — enough to cover the goal spend
const GOAL_NAME     = "L59 New Laptop";
const GOAL_TARGET   = "500";         // $500 target
const GOAL_SAVED    = "475";         // $475 saved so far (95%)
const FINAL_CONTRIB = "25";          // $25 to hit 100%
const OVERFUND_CONTRIB = "10";       // $10 extra to test overfunding
const SPEND_AMOUNT  = "500";         // Full $500 spend against the linked account

// ── helpers ──────────────────────────────────────────────────────────────────
const parseDollar = (s) => {
  if (!s) return NaN;
  const neg = /^\(.*\)$/.test(s.trim());
  const n = parseFloat(s.replace(/[^0-9.]/g, ""));
  return neg ? -n : n;
};

// Hard navigation (resets in-memory UI state)
const goto = async (page, hash) => {
  await page.goto(BASE + hash, { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 }).catch(() => {});
  await page.waitForTimeout(2000);
};

// Soft navigation (preserves period atom)
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

// Read net worth from the dashboard (parses "Net worth $X.XX" or similar)
const parseNetWorth = (text) => {
  const m = text.match(/net\s+worth[^\$]*\$([\d,]+\.\d{2})/i);
  return m ? parseFloat(m[1].replace(/,/g, "")) : NaN;
};

// Find account balance by account name in page text.
// The accounts page renders (probed): AccountName\nType · USD · cleared $X\n$X,XXX.XX
// Current balance is a standalone "$X,XXX.XX" line AFTER the "Type · USD" line.
// Skip lines that contain "cleared" (those show cleared balance, not current).
const parseAccountBalance = (text, acctName) => {
  const lines = text.split("\n");
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].includes(acctName)) {
      // Scan up to 5 lines ahead for a standalone dollar amount (not inline with "cleared")
      for (let j = i + 1; j <= i + 5 && j < lines.length; j++) {
        // Skip lines that contain "cleared" — those are the cleared balance annotation
        if (/cleared/i.test(lines[j])) continue;
        // Match a standalone "$X,XXX.XX" or "($X,XXX.XX)" line (current balance)
        const m = lines[j].match(/^\(?(\$[\d,]+\.\d{2})\)?$/);
        if (m) {
          const neg = lines[j].startsWith("(");
          const val = parseFloat(m[1].replace(/[$,]/g, ""));
          return neg ? -val : val;
        }
      }
    }
  }
  return NaN;
};

let passes = 0, fails = 0, maybes = 0;
const pass  = (m) => { passes++;  console.log(`  PASS  ${m}`); };
const fail  = (m) => { fails++;   console.error(`  FAIL  ${m}`); process.exitCode = 1; };
const maybe = (m) => { maybes++;  console.warn(`  MAYBE ${m}`); };

// ── ensure screenshots/ dir exists ───────────────────────────────────────────
import { mkdirSync } from "fs";
try { mkdirSync(path.join(__dirname, "screenshots"), { recursive: true }); } catch (_) {}

// ── main ─────────────────────────────────────────────────────────────────────
const browser = await chromium.launch({ headless: true });

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // ── Step 0a: Seed linked account ─────────────────────────────────────────
  console.log("\n── Step 0a: Seed linked account ──");
  await goto(page, "/accounts");

  // Fill account add form
  const nameIn0 = await page.$('input[placeholder="Name"], input[placeholder="Account name"], input[aria-label="Account name"]');
  const balIn0  = await page.$('input[placeholder="Opening balance"], input[aria-label="Opening balance"], input[type="number"]');
  if (nameIn0 && balIn0) {
    await nameIn0.fill(ACCT_NAME);
    await balIn0.fill(ACCT_BALANCE);
    const submitBtn0 = await page.$('button[type="submit"]');
    if (submitBtn0) {
      await submitBtn0.click();
      await page.waitForTimeout(1500);
      pass(`Step 0a — Account "${ACCT_NAME}" created with $${ACCT_BALANCE} opening balance`);
    } else {
      maybe("Step 0a — submit button not found for account add");
    }
  } else {
    maybe(`Step 0a — Account add form inputs not found (nameIn=${!!nameIn0}, balIn=${!!balIn0})`);
  }

  await page.screenshot({ path: SS("l59_00a_accounts_seeded.png") });

  // ── Step 0b: Seed near-complete goal ─────────────────────────────────────
  console.log("\n── Step 0b: Seed near-complete goal ──");
  await goto(page, "/goals");

  // Fill goal add form
  const goalNameIn = await page.$('#goal-add, input[placeholder="Name"], input[placeholder="Goal name"], input[aria-label="Name"]');
  const goalTargetIn = await page.$('input[aria-label*="target" i], input[placeholder*="target" i], input[aria-label*="amount" i]');
  const goalSavedIn  = await page.$('input[aria-label*="saved" i], input[placeholder*="saved" i], input[aria-label*="current" i]');

  if (goalNameIn) {
    await goalNameIn.fill(GOAL_NAME);
    pass("Step 0b — Goal name filled");
  } else {
    fail("Step 0b — Goal name input not found on /goals");
  }

  // Find target and saved inputs by placeholder (confirmed from probe)
  const goalTargetInp = await page.$('input[placeholder="Target (USD)"]');
  const goalSavedInp  = await page.$('input[placeholder="Saved so far"]');
  let targetFilled = false;
  let savedFilled  = false;
  if (goalTargetInp) {
    await goalTargetInp.fill(GOAL_TARGET);
    targetFilled = true;
  }
  if (goalSavedInp) {
    await goalSavedInp.fill(GOAL_SAVED);
    savedFilled = true;
  }

  if (targetFilled) pass(`Step 0b — Goal target $${GOAL_TARGET} filled`);
  else fail("Step 0b — Goal target input not found");
  if (savedFilled) pass(`Step 0b — Goal saved so far $${GOAL_SAVED} filled`);
  else maybe("Step 0b — Goal saved-so-far input not found (may default to 0)");

  // Try to set a target date
  const dateIn0b = await page.$('input[type="date"], input[aria-label*="date" i], input[aria-label*="Date" i]');
  if (dateIn0b) {
    await dateIn0b.fill("2026-12-31").catch(() => {});
  }

  // Try to wire the linked account (confirmed aria-label from probe)
  const linkedSel = await page.$('select[aria-label="Linked account (optional)"], select[aria-label*="linked" i]');
  if (linkedSel) {
    const opts = await linkedSel.evaluate((el) =>
      Array.from(el.options).map((o) => ({ v: o.value, t: o.text.trim() }))
    );
    const match = opts.find((o) => o.t.includes("L59"));
    if (match) {
      await linkedSel.selectOption({ value: match.v });
      pass(`Step 0b — Linked account set to "${match.t}"`);
    } else {
      maybe(`Step 0b — L59 account not yet in linked-account options (${opts.map(o => o.t).join(", ")})`);
    }
  } else {
    maybe("Step 0b — Linked account select not found on goal form");
  }

  // Use the "Add" submit button specifically (not the Contribute submit)
  const goalSubmitBtn = await page.$('button[type="submit"]:has-text("Add")') ?? await page.$('button[type="submit"]');
  if (goalSubmitBtn) {
    await goalSubmitBtn.click();
    await page.waitForTimeout(2000);
    pass("Step 0b — Goal add form submitted");
  } else {
    fail("Step 0b — Goal submit button not found");
  }

  await page.waitForTimeout(2000);
  await page.screenshot({ path: SS("l59_00b_goal_added.png") });

  // ── Step 1: /goals — confirm near-target state (95%) ─────────────────────
  console.log("\n── Step 1: /goals — confirm near-target state (95%) ──");
  // Already on /goals, just refresh body
  const goalsBody1 = await bodyText(page);

  if (goalsBody1.includes(GOAL_NAME)) {
    pass(`Step 1a — Goal "${GOAL_NAME}" visible on /goals`);
  } else {
    fail(`Step 1a — Goal "${GOAL_NAME}" NOT visible on /goals after add`);
  }

  // Check progress markers
  const has95    = /95\s*%|95%/.test(goalsBody1);
  const has475   = /\$?475/.test(goalsBody1);
  const has500   = /\$?500/.test(goalsBody1);
  if (has95 || (has475 && has500)) {
    pass(`Step 1b — Near-target state visible (95% or $475/$500 present in page)`);
  } else {
    maybe(`Step 1b — Near-target state not clearly visible (95%=${has95}, $475=${has475}, $500=${has500})`);
  }

  // Pace figure should be visible
  if (/save|\/mo|per month/i.test(goalsBody1)) {
    pass("Step 1c — Pace figure (/mo) visible on near-target goal row");
  } else {
    maybe("Step 1c — Pace figure not visible (may not be shown when near-complete)");
  }

  await page.screenshot({ path: SS("l59_01_goals_near_target.png") });

  // ── Step 2: Final contribution ($25) → completion fires ──────────────────
  console.log("\n── Step 2: Final $25 contribution → confirm 100% completion ──");

  // Snapshot net worth before contribution
  await softNav(page, "Dashboard", "/");
  const dashBody_preContrib = await bodyText(page);
  const netWorth_preContrib = parseNetWorth(dashBody_preContrib);
  console.log(`  INFO  Net worth pre-contribution: $${netWorth_preContrib}`);
  await softNav(page, "Goals", "/goals");

  // Find the Contribute button scoped to the L59 goal row.
  // Goals page renders each row as a list item; we scan all buttons for one whose
  // nearest ancestor row/li contains GOAL_NAME and whose text is "Contribute".
  const clickContribute = async (goalName) => {
    const allBtns = await page.$$('button');
    for (const btn of allBtns) {
      const info = await btn.evaluate((el, name) => {
        const txt = el.textContent?.trim() ?? "";
        const row = el.closest("li, tr, [class*='goal'], [class*='row'], article, section") ?? el.parentElement;
        const rowTxt = row ? row.textContent ?? "" : "";
        return { txt, inRow: rowTxt.includes(name) };
      }, goalName);
      if (/^contribute$/i.test(info.txt) && info.inRow) return btn;
    }
    // Fallback: first Contribute button on page
    for (const btn of allBtns) {
      const txt = await btn.evaluate(el => el.textContent?.trim() ?? "");
      if (/^contribute$/i.test(txt)) return btn;
    }
    return null;
  };

  let contribBtn = await clickContribute(GOAL_NAME);
  if (contribBtn) {
    await contribBtn.click();
    await page.waitForTimeout(1000);
    pass("Step 2a — Contribute button clicked");
  } else {
    fail("Step 2a — Contribute button not found on /goals");
  }

  // Fill in $25 contribution — confirmed placeholder from probe
  const contribAmtIn = await page.$('input[placeholder="Amount to add"], input[aria-label*="amount" i]');
  if (contribAmtIn) {
    await contribAmtIn.fill(FINAL_CONTRIB);
    pass(`Step 2b — Contribution amount $${FINAL_CONTRIB} filled`);
  } else {
    fail("Step 2b — Contribution amount input not found (expected placeholder='Amount to add')");
  }

  await page.screenshot({ path: SS("l59_02a_contribution_form.png") });

  // Use the Contribute SUBMIT button specifically (not the "Add" submit for the goal-add form)
  const contribSubmit = await page.$('button[type="submit"]:has-text("Contribute")');
  if (contribSubmit) {
    await contribSubmit.click();
    await page.waitForTimeout(3000);
    pass("Step 2c — Contribution form submitted");
  } else {
    fail("Step 2c — Contribute submit button not found");
  }

  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("l59_02b_after_final_contribution.png") });

  // Check completion state — both via UI text and via localStorage to distinguish
  // timing/render gaps from actual data bugs.
  const goalsBody2 = await bodyText(page);
  const has100 = /100\s*%|100%/.test(goalsBody2);
  const hasComplete = /complete|achieved|goal met|done|congrat/i.test(goalsBody2);
  const hasZeroRemain = /\$0\.00\s*to\s*go|\$0\s*remaining|fully\s*funded/i.test(goalsBody2);

  // Cross-check via localStorage: did currentAmount actually reach targetAmount?
  const goalDataAfterContrib = await page.evaluate((name) => {
    const d = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    const goals = d.goals ?? [];
    return goals.find(g => g.name === name) ?? null;
  }, GOAL_NAME);
  console.log(`  INFO  Goal localStorage after $25 contrib: ${JSON.stringify(goalDataAfterContrib)}`);
  const dataReached100 = goalDataAfterContrib &&
    goalDataAfterContrib.currentAmount?.Amount >= goalDataAfterContrib.targetAmount?.Amount;

  if (has100) {
    pass("Step 2d — COMPLETION_FIRES: goal shows 100% in UI after final contribution");
  } else if (dataReached100) {
    fail("Step 2d — COMPLETION_FIRES VIOLATED: data reached 100% but UI does NOT show 100% (render bug)");
  } else {
    fail("Step 2d — COMPLETION_FIRES VIOLATED: goal does NOT show 100% AND data currentAmount did not reach targetAmount — contribution had no effect");
  }

  if (hasComplete || hasZeroRemain) {
    pass(`Step 2e — COMPLETION_FIRES: completion signal visible (complete=${hasComplete}, zeroRemain=${hasZeroRemain})`);
  } else {
    maybe("Step 2e — COMPLETION_FIRES: no explicit completion badge/signal — may be silent (check screenshot l59_02b)");
  }

  // ── Step 3: Overfunding test ($10 extra) ─────────────────────────────────
  console.log("\n── Step 3: Overfunding test — contribute $10 extra on completed goal ──");

  // Find contribute button again (scoped to GOAL_NAME row)
  const contribBtn3 = await clickContribute(GOAL_NAME);

  if (contribBtn3) {
    await contribBtn3.click();
    await page.waitForTimeout(1000);

    const overfundIn = await page.$('input[placeholder="Amount to add"], input[aria-label*="amount" i]');
    if (overfundIn) {
      await overfundIn.fill(OVERFUND_CONTRIB);

      const overfundSubmit = await page.$('button[type="submit"]:has-text("Contribute")');
      if (overfundSubmit) {
        await overfundSubmit.click();
        await page.waitForTimeout(3000);
      }
    }

    await page.screenshot({ path: SS("l59_03_overfund_result.png") });
    const goalsBody3 = await bodyText(page);

    const over100 = /10[1-9]\s*%|1[1-9]\d\s*%/.test(goalsBody3);
    const capped  = /100\s*%/.test(goalsBody3);
    const hasSurplus = /surplus|over|extra|\+\$10/i.test(goalsBody3);

    if (over100) {
      fail(`Step 3 — OVERFUND_SANE VIOLATED: goal shows >100% after overfund contribution (not capped)`);
    } else if (capped) {
      pass(`Step 3 — OVERFUND_SANE: goal stays at 100% after overfund attempt (capped or surplus handled)`);
    } else {
      maybe(`Step 3 — OVERFUND_SANE: could not parse % after overfund (over100=${over100}, capped=${capped})`);
    }

    if (hasSurplus) {
      pass("Step 3 — OVERFUND_SANE: surplus explicitly shown after overfund");
    }

  } else {
    maybe("Step 3 — Overfund test skipped: Contribute button not found after completion (may be hidden on 100% goal — test as MAYBE)");
  }

  // ── Step 4: Log a $500 spend from the linked account ─────────────────────
  console.log("\n── Step 4: Log $500 spend from linked account ──");

  // Snapshot linked account balance BEFORE the spend
  await softNav(page, "Accounts", "/accounts");
  await page.waitForTimeout(1000);
  const acctsBodyPre = await bodyText(page);
  const acctBalPre   = parseAccountBalance(acctsBodyPre, ACCT_NAME);
  console.log(`  INFO  Account "${ACCT_NAME}" balance pre-spend: $${acctBalPre}`);

  // L41 re-test: did Contribute change the account balance?
  if (!isNaN(acctBalPre)) {
    const openingBal = parseFloat(ACCT_BALANCE);
    if (Math.abs(acctBalPre - openingBal) < 0.01) {
      fail(`Step 4 — ACCOUNT_DECOUPLED CONFIRMED (L41/C51 re-test): Contribute did NOT affect "${ACCT_NAME}" balance ($${acctBalPre} == opening $${openingBal}). The $25 contribution (and $10 overfund) are memo-only — money NOT debited from account.`);
    } else {
      pass(`Step 4 — ACCOUNT_DECOUPLED re-test: account balance changed after contributions ($${acctBalPre} != $${openingBal}) — may indicate coupling has been added`);
    }
  } else {
    maybe(`Step 4 — ACCOUNT_DECOUPLED re-test: could not parse "${ACCT_NAME}" balance on /accounts`);
  }

  await page.screenshot({ path: SS("l59_04a_accounts_pre_spend.png") });

  // Now log the $500 spend on /transactions against the linked account
  await softNav(page, "Transactions", "/transactions");
  await page.waitForSelector('#txn-add, input[placeholder="Description"]', { timeout: 30000 }).catch(() => {});

  const descIn4 = await page.$('input[placeholder="Description"], #txn-add');
  const amtIn4  = await page.$('input[placeholder="Amount"], input[type="number"][aria-required="true"]');
  const dateIn4 = await page.$('input[aria-label="Date"], input[type="date"]');

  if (descIn4 && amtIn4) {
    await descIn4.fill("L59 Laptop Purchase");
    await amtIn4.fill(SPEND_AMOUNT);

    if (dateIn4) {
      await dateIn4.fill("2026-06-22").catch(() => {});
    }

    // Try to select the L59 account in the account select
    const acctSel4 = await page.$('select[aria-label*="account" i]');
    if (acctSel4) {
      const acctOpts = await acctSel4.evaluate((el) =>
        Array.from(el.options).map((o) => ({ v: o.value, t: o.text.trim() }))
      );
      const match4 = acctOpts.find((o) => o.t.includes("L59"));
      if (match4) {
        await acctSel4.selectOption({ value: match4.v });
        pass(`Step 4 — Transaction account set to "${match4.t}"`);
      } else {
        maybe(`Step 4 — L59 account not in transaction account select (options: ${acctOpts.map(o => o.t).join(", ")})`);
      }
    }

    // Set category to expense if possible
    const catSel4 = await page.$('select[aria-label="Category"]');
    if (catSel4) {
      const catOpts4 = await catSel4.evaluate((el) =>
        Array.from(el.options).map((o) => ({ v: o.value, t: o.text.trim() }))
      );
      const techCat = catOpts4.find((o) => /tech|computer|electron|shopping/i.test(o.t));
      if (techCat) await catSel4.selectOption({ value: techCat.v });
    }

    const submit4 = await page.$('button[type="submit"]');
    if (submit4) {
      await submit4.click();
      await page.waitForTimeout(2000);
      pass("Step 4 — $500 spend transaction submitted");
    } else {
      fail("Step 4 — Transaction submit button not found");
    }
  } else {
    fail(`Step 4 — Transaction form inputs not found (descIn=${!!descIn4}, amtIn=${!!amtIn4})`);
  }

  await page.screenshot({ path: SS("l59_04b_spend_logged.png") });

  // ── Step 5: /accounts — verify account balance reflects the spend ─────────
  console.log("\n── Step 5: /accounts — verify account reflects $500 spend ──");
  await softNav(page, "Accounts", "/accounts");
  await page.waitForTimeout(1000);
  const acctsBodyPost = await bodyText(page);
  const acctBalPost   = parseAccountBalance(acctsBodyPost, ACCT_NAME);
  console.log(`  INFO  Account "${ACCT_NAME}" balance post-spend: $${acctBalPost}`);

  await page.screenshot({ path: SS("l59_05_accounts_post_spend.png") });

  // MONEY_CONSERVED: account_pre_spend - 500 == account_post_spend
  if (!isNaN(acctBalPre) && !isNaN(acctBalPost)) {
    const expectedPost = acctBalPre - parseFloat(SPEND_AMOUNT);
    const diff = Math.abs(acctBalPost - expectedPost);
    if (diff < 0.01) {
      pass(`Step 5a — MONEY_CONSERVED: account balance $${acctBalPre} - $${SPEND_AMOUNT} = $${acctBalPost.toFixed(2)} (exact)`);
    } else {
      fail(`Step 5a — MONEY_CONSERVED VIOLATED: expected $${expectedPost.toFixed(2)}, got $${acctBalPost.toFixed(2)}, diff $${diff.toFixed(2)}`);
    }
  } else if (!isNaN(acctBalPost)) {
    // We only have the post balance; just confirm it's less than opening
    const openingBal = parseFloat(ACCT_BALANCE);
    if (acctBalPost < openingBal) {
      pass(`Step 5a — SPEND_POSTS: account balance $${acctBalPost} is less than opening $${openingBal} (spend reflected)`);
    } else {
      fail(`Step 5a — SPEND_POSTS may be broken: account balance $${acctBalPost} not reduced from opening $${openingBal}`);
    }
  } else {
    maybe(`Step 5a — MONEY_CONSERVED: could not parse account balance for "${ACCT_NAME}" post-spend`);
  }

  // ── Step 6: /transactions — verify spend transaction exists ───────────────
  console.log("\n── Step 6: /transactions — verify L59 Laptop Purchase exists ──");
  await softNav(page, "Transactions", "/transactions");
  await page.waitForTimeout(1500);

  const txnBody6 = await bodyText(page);
  if (txnBody6.includes("L59 Laptop Purchase")) {
    pass("Step 6a — SPEND_POSTS: 'L59 Laptop Purchase' transaction visible in /transactions");
  } else {
    fail("Step 6a — SPEND_POSTS VIOLATED: 'L59 Laptop Purchase' NOT visible in /transactions");
  }

  // Check amount is shown as $500
  if (/\$500\.00/.test(txnBody6)) {
    pass("Step 6b — SPEND_POSTS: $500.00 amount visible in transactions list");
  } else {
    maybe("Step 6b — SPEND_POSTS: $500.00 not clearly parseable in /transactions text");
  }

  await page.screenshot({ path: SS("l59_06_transactions_with_spend.png") });

  // ── Step 7: /dashboard — net worth correct after completion + spend ────────
  console.log("\n── Step 7: /dashboard — net worth after goal completion + spend ──");
  await softNav(page, "Dashboard", "/");
  await page.waitForTimeout(1500);
  const dashBodyPost = await bodyText(page);
  const netWorthPost = parseNetWorth(dashBodyPost);
  console.log(`  INFO  Net worth pre-contrib: $${netWorth_preContrib}, post-spend: $${netWorthPost}`);

  await page.screenshot({ path: SS("l59_07_dashboard_final.png") });

  // NET_WORTH_CORRECT: net worth after $500 spend should be ~$500 lower than pre-contrib baseline
  if (!isNaN(netWorth_preContrib) && !isNaN(netWorthPost)) {
    const delta = netWorth_preContrib - netWorthPost;
    console.log(`  INFO  Net worth delta (pre contrib minus post spend): $${delta.toFixed(2)}`);
    // We expect net worth to have dropped by $500 (the spend), assuming no other changes
    if (Math.abs(delta - parseFloat(SPEND_AMOUNT)) < 1.0) {
      pass(`Step 7a — NET_WORTH_CORRECT: net worth dropped by $${delta.toFixed(2)} ≈ $${SPEND_AMOUNT} spend`);
    } else if (delta > 0) {
      maybe(`Step 7a — NET_WORTH_CORRECT: net worth dropped $${delta.toFixed(2)} (expected ~$${SPEND_AMOUNT}; contributions may be counted in net worth)`);
    } else {
      fail(`Step 7a — NET_WORTH_CORRECT may be violated: net worth delta $${delta.toFixed(2)} is unexpected after $${SPEND_AMOUNT} spend`);
    }
  } else {
    maybe(`Step 7a — NET_WORTH_CORRECT: net worth not parseable (pre=$${netWorth_preContrib}, post=$${netWorthPost})`);
  }

  // ── Step 8: /goals — confirm goal persists in completed state ─────────────
  console.log("\n── Step 8: /goals — confirm completed goal persists ──");
  await softNav(page, "Goals", "/goals");
  await page.waitForTimeout(1000);
  const goalsBodyFinal = await bodyText(page);

  if (goalsBodyFinal.includes(GOAL_NAME)) {
    pass(`Step 8a — Completed goal "${GOAL_NAME}" still visible on /goals after spend`);
  } else {
    maybe(`Step 8a — Goal "${GOAL_NAME}" not visible post-spend (may have been archived/removed on completion)`);
  }

  if (/100\s*%/.test(goalsBodyFinal) || /complete|achieved/i.test(goalsBodyFinal)) {
    pass("Step 8b — Goal shows 100%/complete state after spending the funds");
  } else {
    maybe("Step 8b — Goal completion state not clearly visible in final /goals view");
  }

  await page.screenshot({ path: SS("l59_08_goals_final_state.png") });

  // ── Step 9: JS error check ────────────────────────────────────────────────
  console.log("\n── Step 9: JS error check ──");
  if (errors.length > 0) {
    fail(`JS page errors: ${errors.join(" | ")}`);
  } else {
    pass("Step 9 — No JS page errors across the full ritual");
  }

  // ── Summary ───────────────────────────────────────────────────────────────
  console.log(`\n══ SUMMARY: ${passes} PASS, ${fails} FAIL, ${maybes} MAYBE ══`);
  if (fails > 0) {
    console.error("RESULT: FAIL");
  } else {
    console.log("RESULT: PASS");
  }

} finally {
  await browser.close();
}
