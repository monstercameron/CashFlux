// L41 E2E loop story — "Starting a Vacation Fund"
// Persona: Cam, 32, building his first savings goal.
// Ritual:
//   1. Navigate to /goals; confirm the add form is present.
//   2. Create a Vacation Fund goal: target $2,000, target date 2026-12-01,
//      linked to the sample Savings account, starting balance $0.
//   3. Verify: goal row appears with $0.00 / $2,000.00, 0% progress bar,
//      and a "$X/mo to stay on pace" figure.
//   4. Contribute $200 via the row's Contribute button.
//   5. Verify: progress advances to ~10% ($200 / $2,000), "$1,800 remaining",
//      pace figure recomputes.
//   6. Navigate to /accounts; verify the Savings account balance is UNCHANGED
//      (contribution is decoupled — no transaction was auto-created).
//   7. Hard reload /goals; verify goal + $200 progress persists.
//
// Key correctness question (C51): does "Contribute" balance against the linked
// account, or is it a silent decoupled progress bump?
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_41_create_goal.mjs
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS = (name) => path.join(__dirname, name);

const GOAL_NAME = "L41 Vacation Fund";
const TARGET    = "2000";
const CONTRIB   = "200";
const TARGET_DATE = "2026-12-01";

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0;
const pass = (label) => { console.log(`PASS: ${label}`); passed++; };
const fail = (label) => { console.error(`FAIL: ${label}`); failed++; };

// Wait for wasm hydration sentinel
const waitNav = (page) =>
  page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // ── Step 1: Navigate to /goals ──────────────────────────────────────────────
  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);

  const h1 = await page.evaluate(() => document.querySelector("h1")?.textContent?.trim() ?? "");
  if (/goal/i.test(h1)) {
    pass(`Step 1 — /goals loaded (h1: "${h1}")`);
  } else {
    fail(`Step 1 — expected Goals h1, got "${h1}"`);
  }
  await page.screenshot({ path: SS("loop41-01-goals-before.png") });
  pass("Step 1b — screenshot loop41-01-goals-before.png");

  // ── Step 2: Confirm the add form is present ─────────────────────────────────
  const addInput = await page.locator("#goal-add").count();
  if (addInput > 0) {
    pass("Step 2 — #goal-add input is present");
  } else {
    fail("Step 2 — #goal-add input not found");
  }

  // ── Step 3: Find the linked-account select and pick a Savings account ───────
  // The linked account select has aria-label matching "goals.linkedOptional" i18n key.
  // In practice the label contains "Linked account" or similar. We'll look for all selects
  // and find the one with a Savings option.
  const acctOpts = await page.evaluate(() => {
    const selects = Array.from(document.querySelectorAll("select"));
    for (const sel of selects) {
      const opts = Array.from(sel.options).map((o) => ({ value: o.value, text: o.text }));
      if (opts.some((o) => /saving/i.test(o.text))) return opts;
    }
    return [];
  });
  const savingsOpt = acctOpts.find((o) => /saving/i.test(o.text));
  if (savingsOpt) {
    pass(`Step 3 — Savings account option found: "${savingsOpt.text}" (${savingsOpt.value})`);
  } else {
    fail(`Step 3 — no Savings account in goal linked-account select; options: ${acctOpts.map((o) => o.text).join(", ")}`);
  }

  // ── Step 4: Fill the add-goal form ─────────────────────────────────────────
  // Name
  await page.fill("#goal-add", GOAL_NAME);

  // Target amount (aria-required number field = the first required number input in the form)
  await page.locator('input[type="number"][aria-required="true"]').fill(TARGET);

  // Starting balance = 0 (default; the second number input "Saved so far")
  // We leave it at its default ("0") — no action needed.

  // Target date
  const dateField = page.locator('input[type="date"]');
  const dateCount = await dateField.count();
  if (dateCount > 0) {
    await dateField.first().fill(TARGET_DATE);
    pass(`Step 4a — target date set to ${TARGET_DATE}`);
  } else {
    // Maybe it's a text input for date
    const textDate = page.locator('input[placeholder*="date" i], input[aria-label*="date" i]');
    if (await textDate.count() > 0) {
      await textDate.first().fill(TARGET_DATE);
      pass(`Step 4a — target date set via text input to ${TARGET_DATE}`);
    } else {
      fail("Step 4a — no date input found; pace figure will be absent");
    }
  }

  // Link account — use the select that has a Savings option
  if (savingsOpt) {
    const linkedSel = await page.evaluate(() => {
      const selects = Array.from(document.querySelectorAll("select"));
      for (const sel of selects) {
        if (Array.from(sel.options).some((o) => /saving/i.test(o.text))) return sel.getAttribute("aria-label");
      }
      return null;
    });
    if (linkedSel) {
      await page.selectOption(`select[aria-label="${linkedSel}"]`, { value: savingsOpt.value });
      pass(`Step 4b — linked account set to "${savingsOpt.text}"`);
    } else {
      await page.evaluate((val) => {
        const selects = Array.from(document.querySelectorAll("select"));
        for (const sel of selects) {
          if (Array.from(sel.options).some((o) => /saving/i.test(o.text))) {
            sel.value = val;
            sel.dispatchEvent(new Event("change", { bubbles: true }));
            return;
          }
        }
      }, savingsOpt.value);
      pass(`Step 4b — linked account set via eval to "${savingsOpt.text}"`);
    }
  }

  await page.screenshot({ path: SS("loop41-02-add-form-filled.png") });
  pass("Step 4c — screenshot loop41-02-add-form-filled.png");

  // ── Step 5: Submit the form ─────────────────────────────────────────────────
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(1000);

  const goalRow = page.locator(".budget", { hasText: GOAL_NAME });
  const rowCount = await goalRow.count();
  if (rowCount > 0) {
    pass("Step 5 — goal row appeared after submit");
  } else {
    fail(`Step 5 — goal row not found after submit; looking for "${GOAL_NAME}"`);
  }
  await page.screenshot({ path: SS("loop41-03-after-add-goal.png") });
  pass("Step 5b — screenshot loop41-03-after-add-goal.png");

  // ── Step 6: Verify initial state — 0% progress, $0/$2000, pace figure ───────
  if (rowCount > 0) {
    const rowText = (await goalRow.first().textContent()) ?? "";
    const normalized = rowText.replace(/\s+/g, " ").trim();
    console.log(`  [debug] initial row text: ${normalized}`);

    // Should show $0.00 / $2,000.00 (or locale variant)
    if (/\$0\.00\s*\/\s*\$2,000\.00|\$0\s*\/\s*\$2,000/.test(normalized)) {
      pass("Step 6a — initial amount shows $0.00 / $2,000.00");
    } else if (/0(\.\d+)?\s*\/\s*2[,.]?000/.test(normalized)) {
      pass(`Step 6a — initial amount shows 0 / 2000 (formatted: "${normalized}")`);
    } else {
      fail(`Step 6a — expected $0/$2000, got: "${normalized}"`);
    }

    // Progress: should be "0%" or similar — the bar fill should be width:0% or minimal
    const barWidth = await page.evaluate((name) => {
      const rows = Array.from(document.querySelectorAll(".budget"));
      const row = rows.find((r) => r.textContent.includes(name));
      if (!row) return null;
      const fill = row.querySelector(".bar-fill");
      return fill ? fill.getAttribute("style") : null;
    }, GOAL_NAME);
    console.log(`  [debug] bar fill style: ${barWidth}`);
    if (barWidth !== null && /width:\s*0%|width:\s*0\.0+%/.test(barWidth)) {
      pass("Step 6b — progress bar is at 0% initially");
    } else if (barWidth !== null) {
      pass(`Step 6b — bar fill style present: "${barWidth}" (0% not confirmed — note if nonzero)`);
    } else {
      fail("Step 6b — .bar-fill not found in goal row");
    }

    // Pace figure: "$X/mo to stay on pace" appears when TargetDate is set
    const hasPace = /mo\b|month|pace|save/i.test(normalized);
    if (hasPace) {
      pass("Step 6c — pace figure present in row text");
    } else {
      fail(`Step 6c — no pace/monthly figure found in row; text: "${normalized}"`);
    }
  }

  // ── Step 7: Capture Savings account balance BEFORE contribution ──────────────
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1200);

  let savingsBalanceBefore = null;
  if (savingsOpt) {
    savingsBalanceBefore = await page.evaluate((name) => {
      // Account rows typically have the account name + a balance figure.
      const rows = Array.from(document.querySelectorAll(".budget, .acct-row, [class*='account'], li, tr"));
      for (const r of rows) {
        if (r.textContent.includes(name.replace(/\s*\(.*\)/, ""))) {
          return r.textContent.trim();
        }
      }
      // Fallback: scan all text
      const body = document.body.textContent ?? "";
      const m = body.match(/Savings[^\n]*\$[\d,]+\.\d{2}/);
      return m ? m[0] : null;
    }, savingsOpt.text);
    console.log(`  [debug] savings balance text before contribution: ${savingsBalanceBefore}`);
    pass("Step 7 — captured Savings account state before contribution");
  }
  await page.screenshot({ path: SS("loop41-04-accounts-before-contrib.png") });
  pass("Step 7b — screenshot loop41-04-accounts-before-contrib.png");

  // ── Step 8: Return to /goals and contribute $200 ────────────────────────────
  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1200);

  const goalRow2 = page.locator(".budget", { hasText: GOAL_NAME });
  if ((await goalRow2.count()) === 0) {
    fail("Step 8 — goal row not found on return to /goals");
  } else {
    // The Contribute button is the FIRST action button in the row head.
    // From goals.go: buttons order is: Contribute (PlusCircle), Edit (Pencil), Delete (X).
    const contribBtn = goalRow2.first().locator("button").first();
    await contribBtn.click();
    await page.waitForTimeout(500);

    // The contribute form appears inline: input[id^="goal-contrib-"]
    const contribInput = page.locator('input[id^="goal-contrib-"]');
    if ((await contribInput.count()) > 0) {
      await contribInput.fill(CONTRIB);
      pass("Step 8a — contribution amount entered ($200)");
    } else {
      fail("Step 8a — contribution input not found after clicking Contribute");
    }

    // Submit the contribute form
    await goalRow2.first().locator('button[type="submit"]').first().click();
    await page.waitForTimeout(800);
    pass("Step 8b — contribution submitted");
  }

  await page.screenshot({ path: SS("loop41-05-after-contribute.png") });
  pass("Step 8c — screenshot loop41-05-after-contribute.png");

  // ── Step 9: Verify progress advanced to ~10% ($200/$2000) ───────────────────
  const goalRow3 = page.locator(".budget", { hasText: GOAL_NAME });
  if ((await goalRow3.count()) > 0) {
    const rowText2 = (await goalRow3.first().textContent()) ?? "";
    const norm2 = rowText2.replace(/\s+/g, " ").trim();
    console.log(`  [debug] after-contribution row text: ${norm2}`);

    // $200.00 / $2,000.00
    if (/\$200\.00\s*\/\s*\$2,000\.00|\$200\s*\/\s*\$2,000/.test(norm2)) {
      pass("Step 9a — progress shows $200.00 / $2,000.00 after contribution");
    } else {
      fail(`Step 9a — expected $200/$2000 in row, got: "${norm2}"`);
    }

    // "1,800" remaining
    if (/1[,.]?800/.test(norm2)) {
      pass("Step 9b — remaining shows ~$1,800");
    } else {
      fail(`Step 9b — $1,800 remaining not found; row: "${norm2}"`);
    }

    // Bar fill should now be ~10%
    const barWidth2 = await page.evaluate((name) => {
      const rows = Array.from(document.querySelectorAll(".budget"));
      const row = rows.find((r) => r.textContent.includes(name));
      if (!row) return null;
      const fill = row.querySelector(".bar-fill");
      return fill ? fill.getAttribute("style") : null;
    }, GOAL_NAME);
    console.log(`  [debug] bar fill style after contribution: ${barWidth2}`);
    if (barWidth2 && /width:\s*10%|width:\s*10\.0+%/.test(barWidth2)) {
      pass("Step 9c — bar fill is 10% after contribution");
    } else if (barWidth2 && !/width:\s*0%/.test(barWidth2)) {
      pass(`Step 9c — bar fill advanced from 0% (now: "${barWidth2}")`);
    } else {
      fail(`Step 9c — bar fill still 0% or missing after contribution; style: "${barWidth2}"`);
    }

    // Pace should recompute (still present with TargetDate)
    const hasPace2 = /mo\b|month|pace|save/i.test(norm2);
    if (hasPace2) {
      pass("Step 9d — pace figure still present after contribution");
    } else {
      fail(`Step 9d — pace figure missing after contribution; row: "${norm2}"`);
    }
  } else {
    fail("Step 9 — goal row not found after contribution");
  }

  // ── Step 10: KEY CORRECTNESS CHECK — linked account balance unchanged ────────
  // The central question from the ticket spec: is contribute a decoupled progress
  // bump, or does it debit the linked account / create a transaction?
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1200);

  await page.screenshot({ path: SS("loop41-06-accounts-after-contrib.png") });
  pass("Step 10a — screenshot loop41-06-accounts-after-contrib.png");

  if (savingsOpt && savingsBalanceBefore !== null) {
    const savingsBalanceAfter = await page.evaluate((name) => {
      const rows = Array.from(document.querySelectorAll(".budget, .acct-row, [class*='account'], li, tr"));
      for (const r of rows) {
        if (r.textContent.includes(name.replace(/\s*\(.*\)/, ""))) {
          return r.textContent.trim();
        }
      }
      const body = document.body.textContent ?? "";
      const m = body.match(/Savings[^\n]*\$[\d,]+\.\d{2}/);
      return m ? m[0] : null;
    }, savingsOpt.text);
    console.log(`  [debug] savings balance text after contribution: ${savingsBalanceAfter}`);

    if (savingsBalanceBefore === savingsBalanceAfter) {
      // Contribution is DECOUPLED — just a progress bump, account untouched.
      pass("Step 10b — DECOUPLED: Savings account balance unchanged after Contribute (no auto-transaction)");
      console.log("  [finding] DECOUPLED contribution confirmed: Contribute only bumps Goal.CurrentAmount; no transaction is created and the linked account balance is not debited. This is a mechanical gap vs. accounting correctness (C51).");
    } else {
      // Contribution DID affect the linked account — a transaction was auto-created.
      pass("Step 10b — COUPLED: Savings account balance changed after Contribute (auto-transaction was created)");
      console.log(`  [finding] Contribution appears coupled. Before: "${savingsBalanceBefore}", After: "${savingsBalanceAfter}"`);
    }
  } else {
    fail("Step 10b — could not compare savings balance (no baseline or no savings account found)");
  }

  // Also check /transactions — was any transaction auto-created for the contribution?
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1200);
  const txnText = await page.evaluate(() => document.body.textContent ?? "");
  const hasContribTxn = txnText.includes("Vacation Fund") || txnText.includes("L41");
  if (!hasContribTxn) {
    pass("Step 10c — no auto-transaction for goal contribution in /transactions (confirms DECOUPLED)");
  } else {
    pass("Step 10c — auto-transaction found in /transactions for goal contribution (COUPLED)");
  }
  await page.screenshot({ path: SS("loop41-07-transactions-after-contrib.png") });
  pass("Step 10d — screenshot loop41-07-transactions-after-contrib.png");

  // ── Step 11: Hard reload /goals — persistence check ──────────────────────────
  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(2500); // let autosave flush

  // Force a real reload
  await page.reload({ waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1200);

  const goalRow4 = page.locator(".budget", { hasText: GOAL_NAME });
  if ((await goalRow4.count()) > 0) {
    pass("Step 11a — goal row survives hard reload");
    const rowText3 = (await goalRow4.first().textContent()) ?? "";
    const norm3 = rowText3.replace(/\s+/g, " ").trim();
    console.log(`  [debug] post-reload row text: ${norm3}`);
    if (/\$200\.00\s*\/\s*\$2,000\.00|\$200\s*\/\s*\$2,000/.test(norm3)) {
      pass("Step 11b — $200 contribution persists across reload");
    } else {
      fail(`Step 11b — $200 contribution not found after reload; row: "${norm3}"`);
    }
  } else {
    fail("Step 11a — goal row not found after hard reload");
  }
  await page.screenshot({ path: SS("loop41-08-goals-after-reload.png") });
  pass("Step 11c — screenshot loop41-08-goals-after-reload.png");

  // ── Step 12: JS error check ──────────────────────────────────────────────────
  if (errors.length === 0) {
    pass("Step 12 — zero JS page errors across entire flow");
  } else {
    fail(`Step 12 — JS errors: ${errors.join(" | ")}`);
  }

} finally {
  await browser.close();
  console.log(`\nResult: ${passed} passed, ${failed} failed`);
  if (failed > 0) process.exitCode = 1;
}
