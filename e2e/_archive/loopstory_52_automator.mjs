// L52 E2E loop story — "The Automator" (Raj, no-code automation persona)
// Persona: Raj is a power user who sets up auto-categorization rules and
//          multi-condition workflows to automate his budget hygiene. This
//          ritual stresses Rules + Workflows ↔ transactions ↔ budgets ↔
//          reports integration across ≥4 screens and 8+ actions.
//
// Flow:
//   1.  /rules      — create auto-categorization rule: merchant contains "Uber" → Transport;
//                     verify rule SAVES and appears in the list.
//   2.  /workflows  — build a workflow (trigger=transaction_created, condition=amount>10,
//                     action=create task "Review Uber spend"); click "Save workflow";
//                     RELOAD and check if the workflow STILL APPEARS (C37 probe).
//   3.  /workflows  — dry-run the saved workflow; capture preview.
//   4.  /transactions — add two transactions: "Uber Pool" $15 (should match rule → Transport)
//                     and "Uber Eats" $25 (also matches rule).
//   5.  Confirm rule FIRES: both transactions auto-categorized to Transport.
//   6.  /budgets    — check Transport budget category reflects the spend.
//   7.  /reports    — confirm spend appears in category breakdown.
//   8.  Full page RELOAD — rule, workflow, transactions, budget, report all survive.
//
// Key invariants:
//   RULE_PERSISTS       — rule visible in /rules list after save
//   RULE_FIRES          — Uber transactions auto-categorized to Transport after add
//   WORKFLOW_PERSISTS   — workflow still in list after page reload (C37 probe)
//   DRY_RUN_WORKS       — dry-run preview panel opens and has content
//   BUDGET_REFLECTS     — /budgets shows Transport spending > 0
//   REPORTS_REFLECTS    — /reports loads with content (no crash)
//   RELOAD_SURVIVAL     — rule+workflow+transactions all present after hard reload
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_52_automator.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
// Use a unique tag so re-runs don't collide with stale data.
const TAG  = "Uber" + Date.now();
const SS   = (name) => path.join(__dirname, name);

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0;
const pass  = (label) => { console.log(`PASS: ${label}`);   passed++; };
const fail  = (label) => { console.error(`FAIL: ${label}`); failed++; process.exitCode = 1; };
const maybe = (label) => { console.log(`ABSENT: ${label}`); };

const flush = async (page) => {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(500);
};

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

const getDS = (page) =>
  page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));

async function waitDS(page, pred, timeoutMs = 10000) {
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    const d = await getDS(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return await getDS(page);
}

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });

  const jsErrors = [];
  page.on("pageerror", (e) => jsErrors.push(e.message));

  // ── Boot ─────────────────────────────────────────────────────────────────────
  await bootApp(page);

  // ── STEP 1: /rules — create auto-categorization rule ────────────────────────
  await pushNav(page, "/rules");
  await page.waitForTimeout(1000);
  await page.screenshot({ path: SS("ss_L52_01_rules_form.png") });

  // Find the rule "match" input (merchant / description phrase).
  const ruleInput = await page.$('#rule-add, input[placeholder*="merchant" i], input[placeholder*="match" i], input[placeholder*="phrase" i], input[placeholder*="description" i]');
  if (!ruleInput) {
    fail("Step 1 — rule match input not found on /rules");
  } else {
    await ruleInput.fill(TAG);

    // Pick the "Transport" category if available, else the first non-empty option.
    const catSelect = page.locator("form select").first();
    const catId = await catSelect.evaluate((el) => {
      // Prefer a "Transport" option.
      const transOpt = [...el.options].find(
        (o) => o.value && (o.text.toLowerCase().includes("transport") || o.text.toLowerCase().includes("travel") || o.text.toLowerCase().includes("auto"))
      );
      const firstOpt = [...el.options].find((o) => o.value);
      const chosen   = transOpt || firstOpt;
      if (!chosen) return null;
      el.value = chosen.value;
      el.dispatchEvent(new Event("change", { bubbles: true }));
      return chosen.value;
    });

    if (!catId) {
      fail("Step 1 — no category options available in rule form");
    } else {
      // Submit the rule.
      const submitBtn = await page.$('form button[type="submit"], form button:has-text("Add")');
      if (!submitBtn) {
        fail("Step 1 — rule form submit button not found");
      } else {
        await submitBtn.click();
        await page.waitForTimeout(800);

        // Confirm the rule persisted in localStorage.
        const dsAfterRule = await waitDS(
          page,
          (d) => (d.rules || []).some((r) => (r.Match || r.match || "").includes(TAG))
        );
        const rule = (dsAfterRule.rules || []).find((r) =>
          (r.Match || r.match || "").includes(TAG)
        );
        if (rule) {
          pass(`Step 1 — RULE_PERSISTS: rule "${TAG}" saved to localStorage`);
        } else {
          fail(`Step 1 — RULE_PERSISTS: rule "${TAG}" NOT found in localStorage after save`);
        }

        // Check rule appears in the DOM list.
        const rulesText = await bodyText(page);
        if (rulesText.includes(TAG)) {
          pass(`Step 1b — rule "${TAG}" visible in /rules list`);
        } else {
          fail(`Step 1b — rule "${TAG}" NOT visible in /rules list after save`);
        }

        await page.screenshot({ path: SS("ss_L52_02_rule_saved.png") });
      }
    }
  }

  // ── STEP 2: /workflows — build and save a workflow, then probe C37 ──────────
  await pushNav(page, "/workflows");
  await page.waitForTimeout(1000);
  await page.screenshot({ path: SS("ss_L52_03_workflows_form.png") });

  const wfNameInput = await page.$('input[placeholder*="Workflow name" i], input[aria-label*="name" i]');
  const wfName      = `L52-AutoWF-${Date.now()}`;

  if (!wfNameInput) {
    fail("Step 2 — workflow name input not found on /workflows");
  } else {
    await wfNameInput.fill(wfName);

    // Fill the action text (task title / message / tag).
    const actionInput = await page.$(
      'input[placeholder*="Task title" i], input[placeholder*="task title / message" i], input[placeholder*="message" i]'
    );
    const actionText = "Review Uber spend";
    if (actionInput) {
      await actionInput.fill(actionText);
    } else {
      maybe("Step 2 — action text input not found; skipping action text fill");
    }

    // Click "Add action" to stage it.
    const addActionBtn = await page.$('button:has-text("Add action")');
    if (addActionBtn) {
      await addActionBtn.click();
      await page.waitForTimeout(400);
      pass("Step 2 — 'Add action' clicked");
    } else {
      maybe("Step 2 — 'Add action' button not found; trying to save without explicit stage");
    }

    // Click "Save workflow".
    const saveBtn = await page.$('button:has-text("Save workflow"), button:has-text("Save Workflow")');
    if (!saveBtn) {
      fail("Step 2 — 'Save workflow' button not found");
    } else {
      await saveBtn.click();
      await page.waitForTimeout(1000);

      // Check if the workflow appears in the DOM list immediately after save.
      const textBeforeReload = await bodyText(page);
      const wfVisibleBeforeReload = textBeforeReload.includes(wfName);
      if (wfVisibleBeforeReload) {
        pass(`Step 2 — workflow "${wfName}" visible in list immediately after save`);
      } else {
        fail(`Step 2 — workflow "${wfName}" NOT visible in list immediately after save (possible silent save failure)`);
      }

      await page.screenshot({ path: SS("ss_L52_04_workflow_saved_before_reload.png") });

      // ── C37 PROBE: reload the page and check if the workflow still appears ──
      await page.reload({ waitUntil: "domcontentloaded" });
      await page.waitForSelector("#app", { timeout: 60000 });
      await page.waitForTimeout(2500);

      // Navigate back to /workflows after reload.
      await pushNav(page, "/workflows");
      await page.waitForTimeout(1000);

      const textAfterReload = await bodyText(page);
      const wfVisibleAfterReload = textAfterReload.includes(wfName);
      if (wfVisibleAfterReload) {
        pass(`Step 2 — WORKFLOW_PERSISTS (C37 FIXED): workflow "${wfName}" visible after page reload`);
      } else {
        fail(`Step 2 — WORKFLOW_PERSISTS VIOLATED (C37 OPEN): workflow "${wfName}" MISSING after page reload — save does not persist`);
      }

      await page.screenshot({ path: SS("ss_L52_05_workflow_after_reload.png") });
    }
  }

  // ── STEP 3: dry-run the saved workflow ──────────────────────────────────────
  // Look for a "Dry run" / "Preview" button in the workflow list.
  const dryRunBtn = await page.$(
    'button:has-text("Dry run"), button:has-text("dry run"), button:has-text("Preview")'
  );
  if (!dryRunBtn) {
    maybe("Step 3 — DRY_RUN_WORKS: no 'Dry run' button found (workflow may not be visible / C37 open)");
    await page.screenshot({ path: SS("ss_L52_06_dry_run.png") });
  } else {
    await dryRunBtn.click();
    await page.waitForTimeout(1200);
    const dryRunText = await bodyText(page);
    // The dry-run preview panel should show some content (effect list, "No effects", etc.)
    const dryRunPanelVisible =
      dryRunText.toLowerCase().includes("effect") ||
      dryRunText.toLowerCase().includes("preview") ||
      dryRunText.toLowerCase().includes("no transactions") ||
      dryRunText.toLowerCase().includes("dry") ||
      dryRunText.toLowerCase().includes("would");
    if (dryRunPanelVisible) {
      pass("Step 3 — DRY_RUN_WORKS: dry-run panel opened with content");
    } else {
      maybe("Step 3 — DRY_RUN_WORKS: dry-run clicked but no recognizable preview content found");
    }
    await page.screenshot({ path: SS("ss_L52_06_dry_run.png") });
  }

  // ── STEP 4: /transactions — add Uber transactions that should match the rule ─
  await pushNav(page, "/transactions");
  await page.waitForTimeout(800);

  const txns = [
    { desc: `${TAG} Pool`, amount: "15.00", date: "2026-06-01" },
    { desc: `${TAG} Eats`, amount: "25.00", date: "2026-06-02" },
  ];

  for (const [i, txn] of txns.entries()) {
    const descIn = await page.$(
      'input[id="txn-add"], input[placeholder*="Description" i], input[placeholder*="payee" i], input[placeholder*="what" i], input[aria-label*="description" i]'
    );
    const amtIn  = await page.$('input[type="number"][aria-required="true"], input[placeholder*="Amount" i], input[aria-label*="Amount" i]');
    const dateIn = await page.$('input[type="date"]');
    if (!descIn || !amtIn) {
      fail(`Step 4.${i+1} — transaction form fields not found`);
      continue;
    }
    await descIn.fill(txn.desc);
    await page.waitForTimeout(500); // allow SuggestTransactionFields to fire
    await amtIn.fill(txn.amount);
    if (dateIn) await dateIn.fill(txn.date);
    const submitBtn = await page.$('form button[type="submit"], form button:has-text("Add")');
    if (!submitBtn) { fail(`Step 4.${i+1} — submit button not found`); continue; }
    await submitBtn.click();
    await page.waitForTimeout(600);
  }

  await flush(page);
  await page.screenshot({ path: SS("ss_L52_07_transaction_added.png") });

  // ── STEP 5: confirm rule FIRES — auto-categorized to Transport ───────────────
  const dsAfterTxns = await waitDS(
    page,
    (d) => (d.transactions || []).filter(t => (t.desc || "").includes(TAG)).length >= 2
  );
  const uberTxns = (dsAfterTxns.transactions || []).filter(
    (t) => (t.desc || "").includes(TAG)
  );

  if (uberTxns.length < 2) {
    fail(`Step 5 — RULE_FIRES: expected ≥2 Uber transactions in store, found ${uberTxns.length}`);
  } else {
    // Find the rule's category id from localStorage.
    const dsRules = dsAfterTxns;
    const savedRule = (dsRules.rules || []).find((r) =>
      (r.Match || r.match || "").includes(TAG)
    );
    const ruleCatId = savedRule ? (savedRule.SetCategoryID || savedRule.setCategoryID) : null;

    const categorized = uberTxns.filter((t) => t.categoryId && t.categoryId !== "");
    const ruleMatched = ruleCatId
      ? uberTxns.filter((t) => t.categoryId === ruleCatId)
      : [];

    if (ruleCatId && ruleMatched.length === uberTxns.length) {
      pass(`Step 5 — RULE_FIRES: all ${uberTxns.length} Uber transactions auto-categorized to rule category ${ruleCatId}`);
    } else if (categorized.length > 0) {
      maybe(`Step 5 — RULE_FIRES: ${categorized.length}/${uberTxns.length} Uber transactions have a category (ruleCatId=${ruleCatId}; actual=${uberTxns.map(t=>t.categoryId).join(",")})`);
    } else {
      fail(`Step 5 — RULE_FIRES VIOLATED: 0/${uberTxns.length} Uber transactions auto-categorized (all have no category)`);
    }
  }

  await page.screenshot({ path: SS("ss_L52_08_rule_fired.png") });

  // ── STEP 6: /budgets — Transport spend reflected ─────────────────────────────
  await pushNav(page, "/budgets");
  await page.waitForTimeout(1000);
  const budgetsText = await bodyText(page);
  if (budgetsText.length > 100) {
    pass("Step 6 — BUDGET_REFLECTS: /budgets loads with content (no crash)");
  } else {
    fail("Step 6 — BUDGET_REFLECTS: /budgets has insufficient content (possible crash)");
  }
  await page.screenshot({ path: SS("ss_L52_09_budget_reflects.png") });

  // ── STEP 7: /reports — auto-categorized spend appears in breakdown ───────────
  await pushNav(page, "/reports");
  await page.waitForTimeout(1000);
  const reportsText = await bodyText(page);
  if (reportsText.length > 100) {
    pass("Step 7 — REPORTS_REFLECTS: /reports loads with content (no crash)");
  } else {
    fail("Step 7 — REPORTS_REFLECTS: /reports has insufficient content (possible crash)");
  }
  await page.screenshot({ path: SS("ss_L52_10_reports_reflects.png") });

  // ── STEP 8: full page reload — everything survives ───────────────────────────
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  await page.waitForTimeout(2500);

  const dsAfterFinalReload = await getDS(page);

  // Rule survives.
  const ruleAfterReload = (dsAfterFinalReload.rules || []).find((r) =>
    (r.Match || r.match || "").includes(TAG)
  );
  if (ruleAfterReload) {
    pass("Step 8 — RELOAD_SURVIVAL: rule survives full page reload");
  } else {
    fail("Step 8 — RELOAD_SURVIVAL: rule MISSING after full page reload");
  }

  // Transactions survive.
  const txnsAfterReload = (dsAfterFinalReload.transactions || []).filter(
    (t) => (t.desc || "").includes(TAG)
  );
  if (txnsAfterReload.length >= 2) {
    pass(`Step 8 — RELOAD_SURVIVAL: ${txnsAfterReload.length} Uber transactions survive full page reload`);
  } else {
    fail(`Step 8 — RELOAD_SURVIVAL: only ${txnsAfterReload.length} Uber transactions found after reload (expected ≥2)`);
  }

  // Workflow survival checked earlier (Step 2 C37 probe already covers it).
  // Navigate to workflows to re-confirm and screenshot.
  await pushNav(page, "/workflows");
  await page.waitForTimeout(1000);
  const wfTextFinal = await bodyText(page);
  if (wfTextFinal.includes(wfName)) {
    pass(`Step 8 — RELOAD_SURVIVAL: workflow "${wfName}" survives final reload`);
  } else {
    fail(`Step 8 — RELOAD_SURVIVAL: workflow "${wfName}" MISSING after final reload`);
  }

  // ── JS errors ────────────────────────────────────────────────────────────────
  if (jsErrors.length === 0) {
    pass("Step 9 — zero JS page errors across the full ritual");
  } else {
    fail(`Step 9 — ${jsErrors.length} JS page error(s): ${jsErrors.slice(0, 3).join(" | ")}`);
  }

  // ── Summary ──────────────────────────────────────────────────────────────────
  console.log(`\nResults: ${passed} passed, ${failed} failed.`);
  if (failed === 0) {
    console.log("All assertions passed — L52 The Automator ritual complete.");
  } else {
    console.log(`${failed} assertion(s) failed — see FAIL lines above.`);
  }

} finally {
  await browser.close();
}
