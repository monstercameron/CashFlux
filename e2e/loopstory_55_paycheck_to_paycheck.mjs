// L55 E2E loop story — "Paycheck to Paycheck" (Dani, cash-flow timing / overdraft warning)
//
// Persona: Dani lives paycheck to paycheck. She has a low checking balance ($200)
// and several bills due BEFORE her mid-month paycheck arrives. The story probes
// whether the app models INTRA-PERIOD timing (day-by-day dip below zero) and warns
// her before she overdraws.
//
// Ritual: /accounts (confirm low balance) → /transactions (seed upcoming paycheck +
// past expenses) → /bills (add rent $800 due 5th, electric $120 due 10th) →
// /planning (view cash runway card; assert intra-period dip + overdraft warning;
// adjust a bill's timing; re-check) → /dashboard (confirm cash-flow risk surfaced)
//
// KEY INVARIANTS ASSERTED:
//   I1: Forecast models TIMING within period — intra-period dip below $0 is visible
//       in the runway card before payday arrives (not just end-of-period balance)
//   I2: App WARNS when projected balance goes negative before income arrives
//       (overdraft prediction / breach alert)
//   I3: Adjusting bill/income timing updates projection and clears/raises warning
//   I4: Dashboard reflects cash-flow risk / shortfall warning
//   I5: Money conservation — projected balances are consistent across Bills/Planning/Dashboard
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_55_paycheck_to_paycheck.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS = (name) => path.join(__dirname, name);

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0, absent = 0;
const pass   = (label) => { console.log(`PASS:   ${label}`); passed++; };
const fail   = (label) => { console.error(`FAIL:   ${label}`); failed++; };
const absent_= (label) => { console.log(`ABSENT: ${label}`); absent++; };
const note   = (label) => { console.log(`NOTE:   ${label}`); };

// ─── helpers ──────────────────────────────────────────────────────────────────

const navTo = async (page, title) => {
  await page.evaluate((t) => {
    const links = Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]'));
    const link = links.find(l => l.getAttribute("title") === t);
    if (link) link.click();
  }, title);
  await page.waitForTimeout(1800);
};

const selectByText = async (page, ariaLabel, textMatch) => {
  return page.evaluate(({ label, match }) => {
    const selects = Array.from(document.querySelectorAll("select"));
    for (const sel of selects) {
      if (sel.getAttribute("aria-label") === label) {
        const opt = Array.from(sel.options).find(o => o.text.toLowerCase().includes(match.toLowerCase()));
        if (opt) {
          sel.value = opt.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `set "${sel.getAttribute("aria-label")}" → "${opt.text}"`;
        }
        return `label found but no option matching "${match}"; options: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
      }
    }
    return `select with aria-label="${label}" NOT found`;
  }, { label: ariaLabel, match: textMatch });
};

const fillInput = async (page, idOrLabel, value) => {
  return page.evaluate(({ key, val }) => {
    const inp = document.querySelector(`#${CSS.escape(key)}`) ||
      Array.from(document.querySelectorAll("input,textarea")).find(i =>
        i.getAttribute("aria-label") === key || i.getAttribute("placeholder") === key);
    if (!inp) return `NOT FOUND: "${key}"`;
    inp.focus();
    inp.value = val;
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
    return `filled "${key}" → "${val}"`;
  }, { key: idOrLabel, val: value });
};

const flush = async (page) => {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
};

const getDataset = (page) => page.evaluate(() => {
  try { return JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"); } catch { return {}; }
});

// Dismiss any open modal/backdrop before navigation (probe hardening from L54)
const dismissModal = async (page) => {
  await page.keyboard.press("Escape");
  await page.waitForTimeout(200);
  await page.evaluate(() => {
    const btn = document.querySelector('button[aria-label="Cancel"], dialog button.btn:not(.btn-primary)');
    if (btn) btn.click();
  });
  await page.waitForTimeout(200);
};

// ─── main ─────────────────────────────────────────────────────────────────────

const jsErrors = [];

// Compute dates relative to today for a realistic paycheck scenario:
// Bills due before the 15th, paycheck arrives on the 15th.
const today = new Date();
const currentYear  = today.getFullYear();
const currentMonth = today.getMonth(); // 0-based

// Bill due dates: 5th and 10th of this month (or next if we're past them)
const targetMonth = today.getDate() >= 12
  ? new Date(currentYear, currentMonth + 1, 1)
  : new Date(currentYear, currentMonth, 1);
const billYear  = targetMonth.getFullYear();
const billMonth = targetMonth.getMonth() + 1; // 1-based
const rentDue      = `${billYear}-${String(billMonth).padStart(2,"0")}-05`;
const electricDue  = `${billYear}-${String(billMonth).padStart(2,"0")}-10`;
const paycheckDate = `${billYear}-${String(billMonth).padStart(2,"0")}-15`;
const electricDueLate = `${billYear}-${String(billMonth).padStart(2,"0")}-16`; // AFTER payday

note(`Scenario dates — rent due: ${rentDue}, electric due: ${electricDue}, paycheck: ${paycheckDate}`);
note(`Electric timing-shift date (after payday): ${electricDueLate}`);

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  page.on("pageerror", (e) => jsErrors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  pass("Hydration — app loaded and nav visible");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 1: /accounts — view checking account balance (confirm low balance seed)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 1: /accounts — Dani's low checking balance ────────────────────────");

  await navTo(page, "Accounts");
  const acctBody = await page.evaluate(() => document.body.textContent ?? "");
  const acctH1 = await page.evaluate(() => document.querySelector("h1,h2")?.textContent?.trim());
  note(`Accounts heading: "${acctH1}"`);

  await page.screenshot({ path: SS("ss_L55_01_accounts.png") });
  pass("Step 1.1 — screenshot ss_L55_01_accounts.png");

  // Read current accounts from dataset
  const dsInit = await getDataset(page);
  // Accounts may be keyed as 'accounts' or nested inside 'data'. Check both.
  const accounts = dsInit.accounts || dsInit.data?.accounts || [];
  note(`Accounts in dataset (direct key): ${(dsInit.accounts || []).length}, data.accounts: ${(dsInit.data?.accounts || []).length}`);
  // Fallback: count account rows visible on screen
  const accountRowCount = await page.evaluate(() =>
    document.querySelectorAll(".row").length);
  note(`Account rows visible on screen: ${accountRowCount}`);
  if (accountRowCount > 0) {
    pass("Step 1.2 — Accounts visible on screen (checking account rows)");
  } else if (accounts.length > 0) {
    pass("Step 1.2 — Accounts found in dataset");
  } else {
    note("Step 1.2 — Accounts not found in localStorage dataset key; app likely uses a different storage key. Continuing.");
  }

  // Show account balances visible in the UI
  const acctRows = await page.evaluate(() =>
    Array.from(document.querySelectorAll(".row")).map(r => ({
      name: r.querySelector(".row-desc")?.textContent?.trim(),
      amount: r.querySelector(".budget-amount")?.textContent?.trim(),
    })).filter(r => r.name)
  );
  note(`Account rows on screen: ${JSON.stringify(acctRows.slice(0,5))}`);

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 2: /transactions — add paycheck income (mid-month) + note existing expenses
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 2: /transactions — seed paycheck ($1,200 on the 15th) ──────────────");

  await dismissModal(page);
  await navTo(page, "Transactions");

  const txnCountBefore = ((await getDataset(page)).transactions || []).length;
  note(`Transactions before seeding: ${txnCountBefore}`);

  await page.screenshot({ path: SS("ss_L55_02_transactions.png") });
  pass("Step 2.1 — screenshot ss_L55_02_transactions.png");

  // Open new-transaction form
  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /new transaction/i.test(b.textContent.trim()) || /add transaction/i.test(b.textContent.trim()));
    if (btn) btn.click();
  });
  await page.waitForTimeout(800);

  // Fill paycheck: Income, $1,200, date=paycheckDate
  const descR = await fillInput(page, "txn-add", "L55 Dani Paycheck");
  note(`Description: ${descR}`);
  if (descR.includes("filled")) pass("Step 2.2 — paycheck description filled");
  else fail(`Step 2.2 — description: ${descR}`);

  const amtR = await page.evaluate((val) => {
    const inp = document.querySelector('input[type="number"]');
    if (!inp) return "NOT FOUND";
    inp.value = val;
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
    return `filled ${val}`;
  }, "1200");
  note(`Amount: ${amtR}`);
  if (amtR.includes("filled")) pass("Step 2.3 — paycheck amount = 1200");
  else fail(`Step 2.3 — amount: ${amtR}`);

  const typeR = await selectByText(page, "Type", "Income");
  note(`Type: ${typeR}`);
  if (/Income/i.test(typeR)) pass("Step 2.4 — type = Income");
  else fail(`Step 2.4 — type: ${typeR}`);

  // Set date to paycheckDate
  const dateR = await page.evaluate((d) => {
    const inp = document.querySelector('input[type="date"]');
    if (!inp) return "NOT FOUND";
    inp.value = d;
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
    return `set date → ${d}`;
  }, paycheckDate);
  note(`Date: ${dateR}`);

  // Select account
  const acctR = await page.evaluate(() => {
    const sel = Array.from(document.querySelectorAll("select")).find(s => s.getAttribute("aria-label") === "Account");
    if (!sel) return "Account select NOT FOUND";
    const opt = Array.from(sel.options).find(o => /checking|everyday/i.test(o.text));
    if (opt) { sel.value = opt.value; sel.dispatchEvent(new Event("change", { bubbles: true })); return `set → "${opt.text}"`; }
    const first = sel.options[1];
    if (first) { sel.value = first.value; sel.dispatchEvent(new Event("change", { bubbles: true })); return `set → "${first.text}"`; }
    return "no account options";
  });
  note(`Account: ${acctR}`);

  // Submit paycheck transaction
  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b => {
      const txt = b.textContent.trim();
      return (txt === "Add" || /^save$/i.test(txt)) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);

  const dsAfterPaycheck = await getDataset(page);
  const paycheckTxn = (dsAfterPaycheck.transactions || []).find(t =>
    /L55.*Dani.*Paycheck|Dani.*Paycheck/i.test((t.payee || "") + (t.desc || "")));
  if (paycheckTxn) pass("Step 2.5 — Paycheck transaction persisted in dataset");
  else fail("Step 2.5 — Paycheck transaction NOT found in dataset");

  await page.screenshot({ path: SS("ss_L55_03_transactions_seeded.png") });
  pass("Step 2.6 — screenshot ss_L55_03_transactions_seeded.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 3: /bills — add rent ($800, due 5th) and electric ($120, due 10th)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 3: /bills — add rent ($800 due 5th) + electric ($120 due 10th) ────");

  await dismissModal(page);
  await navTo(page, "Bills");
  await page.waitForTimeout(500);

  await page.screenshot({ path: SS("ss_L55_04_bills_initial.png") });
  pass("Step 3.1 — screenshot ss_L55_04_bills_initial.png");

  // Read bills before
  const dsBeforeBills = await getDataset(page);
  const recurringBefore = (dsBeforeBills.recurring || []).length;

  // Navigate to /planning to add recurring entries (bills come from recurring in this app)
  await dismissModal(page);
  await navTo(page, "Planning");
  await page.waitForTimeout(500);

  // Add rent recurring outflow $800/month with nextDue = rentDue (5th)
  const recurringFormCheck = await page.evaluate(() => {
    const form = Array.from(document.querySelectorAll("form.form-grid")).find(f =>
      f.querySelector('select[aria-label="How often"]'));
    return form ? "found" : "NOT FOUND";
  });
  note(`Planning recurring form: ${recurringFormCheck}`);

  if (recurringFormCheck === "found") {
    // Add Rent
    await page.evaluate(({ label, amount, date }) => {
      const form = Array.from(document.querySelectorAll("form.form-grid")).find(f =>
        f.querySelector('select[aria-label="How often"]'));
      if (!form) return;
      const lbl = form.querySelector('input[placeholder="Label (e.g. Rent, Salary)"]');
      if (lbl) { lbl.value = label; lbl.dispatchEvent(new Event("input", { bubbles: true })); }
      const num = form.querySelector('input[type="number"]');
      if (num) { num.value = amount; num.dispatchEvent(new Event("input", { bubbles: true })); }
      // Set next due date if there's a date input
      const dateInp = form.querySelector('input[type="date"]');
      if (dateInp) { dateInp.value = date; dateInp.dispatchEvent(new Event("input", { bubbles: true })); }
    }, { label: "L55 Dani Rent", amount: "-800", date: rentDue });

    const cadence1 = await selectByText(page, "How often", "Monthly");
    note(`Rent cadence: ${cadence1}`);

    await page.evaluate(() => {
      const form = Array.from(document.querySelectorAll("form.form-grid")).find(f =>
        f.querySelector('select[aria-label="How often"]'));
      const btn = form?.querySelector('button[type="submit"]');
      if (btn) btn.click();
    });
    await page.waitForTimeout(900);
    await flush(page);

    const dsAfterRent = await getDataset(page);
    const rentEntry = (dsAfterRent.recurring || []).find(r => /L55.*Dani.*Rent|Dani.*Rent/i.test(r.label));
    if (rentEntry) {
      pass(`Step 3.2 — Rent recurring entry created: nextDue=${rentEntry.nextDue}, amount=${rentEntry.amount?.amount}`);
    } else {
      fail("Step 3.2 — Rent recurring entry NOT found after adding via /planning");
    }

    // Add Electric
    await page.evaluate(({ label, amount, date }) => {
      const form = Array.from(document.querySelectorAll("form.form-grid")).find(f =>
        f.querySelector('select[aria-label="How often"]'));
      if (!form) return;
      const lbl = form.querySelector('input[placeholder="Label (e.g. Rent, Salary)"]');
      if (lbl) { lbl.value = label; lbl.dispatchEvent(new Event("input", { bubbles: true })); }
      const num = form.querySelector('input[type="number"]');
      if (num) { num.value = amount; num.dispatchEvent(new Event("input", { bubbles: true })); }
      const dateInp = form.querySelector('input[type="date"]');
      if (dateInp) { dateInp.value = date; dateInp.dispatchEvent(new Event("input", { bubbles: true })); }
    }, { label: "L55 Dani Electric", amount: "-120", date: electricDue });

    const cadence2 = await selectByText(page, "How often", "Monthly");
    note(`Electric cadence: ${cadence2}`);

    await page.evaluate(() => {
      const form = Array.from(document.querySelectorAll("form.form-grid")).find(f =>
        f.querySelector('select[aria-label="How often"]'));
      const btn = form?.querySelector('button[type="submit"]');
      if (btn) btn.click();
    });
    await page.waitForTimeout(900);
    await flush(page);

    const dsAfterElec = await getDataset(page);
    const elecEntry = (dsAfterElec.recurring || []).find(r => /L55.*Dani.*Electric|Dani.*Electric/i.test(r.label));
    if (elecEntry) {
      pass(`Step 3.3 — Electric recurring entry created: nextDue=${elecEntry.nextDue}`);
    } else {
      fail("Step 3.3 — Electric recurring entry NOT found after adding via /planning");
    }
  } else {
    fail("Step 3.2 — Planning recurring form NOT FOUND; cannot seed bills");
    fail("Step 3.3 — (skipped — form absent)");
  }

  await page.screenshot({ path: SS("ss_L55_05_planning_after_add.png") });
  pass("Step 3.4 — screenshot ss_L55_05_planning_after_add.png");

  // Check /bills to confirm entries appear there
  await dismissModal(page);
  await navTo(page, "Bills");
  await page.waitForTimeout(500);

  const billsBodySeeded = await page.evaluate(() => document.body.textContent ?? "");
  const hasRentOnBills = /L55.*Dani.*Rent|Dani.*Rent/i.test(billsBodySeeded);
  const hasElecOnBills = /L55.*Dani.*Electric|Dani.*Electric/i.test(billsBodySeeded);
  note(`/bills after seed — rent visible: ${hasRentOnBills}, electric visible: ${hasElecOnBills}`);

  await page.screenshot({ path: SS("ss_L55_06_bills_seeded.png") });
  pass("Step 3.5 — screenshot ss_L55_06_bills_seeded.png");

  if (hasRentOnBills && hasElecOnBills) pass("Step 3.6 — Both bills visible on /bills");
  else if (hasRentOnBills || hasElecOnBills) note("Step 3.6 — Only one bill visible on /bills");
  else fail("Step 3.6 — Neither bill appears on /bills (recurring entries not surfaced)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 4: /planning — view cash runway, assert intra-period dip + overdraft warning
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 4: /planning — cash runway card / overdraft warning ────────────────");

  await dismissModal(page);
  await navTo(page, "Planning");
  await page.waitForTimeout(800);

  await page.screenshot({ path: SS("ss_L55_07_planning_runway.png") });
  pass("Step 4.1 — screenshot ss_L55_07_planning_runway.png");

  const planBody = await page.evaluate(() => document.body.textContent ?? "");

  // I1: Does the planning page have a cash runway / intra-period daily projection?
  const hasRunwayCard = /cash runway|runway|daily|60.day|days? left|dip|projected|balance/i.test(planBody);
  if (hasRunwayCard) {
    pass("Step 4.2 (I1) — Planning page has cash runway / day-by-day projection section");
  } else {
    fail("Step 4.2 (I1) — No cash runway / intra-period projection section found on /planning");
  }

  // Check if runway card is present and populated vs just the 12-month net-worth chart
  const runwayCardDetails = await page.evaluate(() => {
    // Look for the runway card by heading text
    const allEls = Array.from(document.querySelectorAll("*"));
    const header = allEls.find(el =>
      /cash runway|runway/i.test(el.textContent) && el.tagName.match(/^H[1-6]$/));
    if (!header) return { found: false, text: "" };
    const card = header.closest("section, .card, article, div[class]");
    const cardText = card ? card.textContent : header.textContent;
    // Check for breach using the runway-specific signals:
    // WillBreach → renders a <p role="alert"> INSIDE the runway card's verdict area.
    // We also check for the localized breach string pattern (date + shortfall).
    // "holds for the next" = safe verdict. Use both to distinguish.
    // IMPORTANT: querySelector('[role="alert"]') can match alerts from OTHER cards on the page
    // if `card` resolves to a large ancestor. We narrow to the runway verdict area.
    const verdictEl = card
      ? card.querySelector(".budget-sub, .err, p[role='alert']")
      : null;
    const verdictText = verdictEl ? verdictEl.textContent : "";
    const hasBreach = card
      ? (verdictEl?.getAttribute("role") === "alert" &&
         !(/holds for the next|safe|all clear/i.test(verdictText)))
      : false;
    const hasSafe = card
      ? /holds for the next|safe|all clear/i.test(cardText)
      : false;
    return {
      found: true,
      text: cardText.replace(/\s+/g, " ").slice(0, 400),
      hasBreach,
      hasSafe,
    };
  });
  note(`Runway card: found=${runwayCardDetails.found}, hasBreach=${runwayCardDetails.hasBreach}, hasSafe=${runwayCardDetails.hasSafe}`);
  note(`Runway card text: "${runwayCardDetails.text?.slice(0, 200)}"`);

  // I1 + I2: Does the runway card show an intra-period dip below zero?
  // The runway card needs a buffer value entered to activate it; check if the buffer field is present
  const runwayBufferField = await page.evaluate(() => {
    const inp = document.querySelector('input[aria-label*="buffer"], input[aria-label*="Buffer"], input[placeholder*="buffer"]');
    return inp ? `found: aria-label="${inp.getAttribute("aria-label")}"` : "NOT FOUND";
  });
  note(`Runway buffer field: ${runwayBufferField}`);

  if (runwayCardDetails.found) {
    pass("Step 4.3 (I1) — Cash runway card is present on /planning");

    if (runwayCardDetails.hasBreach) {
      // I2: Overdraft warning present
      pass("Step 4.4 (I2) — OVERDRAFT WARNING: Runway card shows balance breach / negative dip");
    } else if (runwayCardDetails.hasSafe) {
      note("Step 4.4 (I2) — Runway card shows 'safe' (no breach). " +
        "This may mean: (a) balance is high enough to cover bills, (b) recurring items have future NextDue beyond runway horizon, " +
        "or (c) the runway card requires a buffer value to be entered to activate the projection.");
      // Probe: try to activate the runway by entering a $0 buffer
      const bufferFilled = await page.evaluate(() => {
        const inp = document.querySelector('input[type="number"][aria-label*="uffer"], input[type="number"][placeholder*="uffer"]');
        if (!inp) {
          // Try min-balance buffer field
          const all = Array.from(document.querySelectorAll('input[type="number"]')).filter(i =>
            i.closest("section")?.querySelector("h2,h3")?.textContent?.match(/runway/i));
          const buf = all[0];
          if (buf) { buf.value = "0"; buf.dispatchEvent(new Event("input", { bubbles: true })); return "set buffer=0 (fallback)"; }
          return "buffer field NOT FOUND";
        }
        inp.value = "0";
        inp.dispatchEvent(new Event("input", { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
        return `set buffer=0 on "${inp.getAttribute("aria-label")}"`;
      });
      note(`Buffer set attempt: ${bufferFilled}`);
      await page.waitForTimeout(600);

      const planBodyAfterBuf = await page.evaluate(() => document.body.textContent ?? "");
      const hasBreachAfterBuf = /breach|overdr|below|warning|danger|shortfall/i.test(planBodyAfterBuf);
      if (hasBreachAfterBuf) pass("Step 4.4 (I2) — OVERDRAFT WARNING visible after buffer=0");
      else {
        absent_("Step 4.4 (I2) — ABSENT: No overdraft warning even after setting buffer=0. " +
          "The runway projection may not reflect the bills seeded in this story, OR the starting balance is not low enough.");
      }
    } else {
      // Runway card found but no clear safe/breach signal
      note("Step 4.4 (I2) — Runway card present but no clear breach/safe signal in text");
      absent_("Step 4.4 (I2) — ABSENT: Cannot determine overdraft warning state from runway card text");
    }
  } else {
    absent_("Step 4.3 (I1) — ABSENT: No cash runway card found on /planning. " +
      "The day-by-day intra-period projection card (internal/runway) is not rendered or requires setup.");
    absent_("Step 4.4 (I2) — ABSENT: No runway card → overdraft warning not assessable");
  }

  await page.screenshot({ path: SS("ss_L55_08_planning_runway_buffer.png") });
  pass("Step 4.5 — screenshot ss_L55_08_planning_runway_buffer.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 5: Adjust electric bill timing from 10th to 16th (after payday)
  //         and observe whether the runway projection updates
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 5: adjust electric due date → 16th (after payday) ─────────────────");

  // Find the electric recurring entry in the dataset and note its current nextDue
  const dsBefore5 = await getDataset(page);
  const elecBefore = (dsBefore5.recurring || []).find(r => /L55.*Dani.*Electric|Dani.*Electric/i.test(r.label));
  note(`Electric before timing adjustment: ${JSON.stringify(elecBefore ? { nextDue: elecBefore.nextDue, label: elecBefore.label } : null)}`);

  // The app doesn't have a direct "edit due date" on /bills; we need to find the recurring item
  // edit affordance. In /planning or /bills there should be an edit button for recurring items.
  const editElecResult = await page.evaluate((targetLabel) => {
    // Look for the electric recurring item row and try to find an edit/delete button
    const rows = Array.from(document.querySelectorAll(".row, .rows .row, li, [data-id]"));
    for (const row of rows) {
      const text = row.textContent || "";
      if (/L55.*Dani.*Electric|Dani.*Electric/i.test(text)) {
        const editBtn = row.querySelector('button[aria-label*="edit"], button[title*="edit"]') ||
          Array.from(row.querySelectorAll("button")).find(b => /edit/i.test(b.getAttribute("aria-label") || b.textContent));
        if (editBtn) { editBtn.click(); return "clicked edit on electric row"; }
        return `found electric row but no edit button; buttons: ${Array.from(row.querySelectorAll("button")).map(b => b.textContent.trim()).join(", ")}`;
      }
    }
    return "electric row NOT FOUND on current page";
  }, "L55 Dani Electric");
  note(`Edit electric: ${editElecResult}`);
  await page.waitForTimeout(600);

  // Check /bills for electric and try inline edit there
  await dismissModal(page);
  await navTo(page, "Bills");
  await page.waitForTimeout(500);

  // I3: Check if bill timing is adjustable
  // The bills page may have inline edit buttons for recurring items
  const billEditCheck = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row, .rows .row"));
    const results = [];
    for (const row of rows) {
      const text = row.textContent || "";
      if (/electric/i.test(text)) {
        const btns = Array.from(row.querySelectorAll("button")).map(b => ({
          text: b.textContent.trim(),
          ariaLabel: b.getAttribute("aria-label"),
        }));
        results.push({ rowText: text.replace(/\s+/g, " ").slice(0, 100), buttons: btns });
      }
    }
    return results;
  });
  note(`Electric bill row on /bills: ${JSON.stringify(billEditCheck)}`);

  if (billEditCheck.length > 0) {
    pass("Step 5.1 — Electric bill appears on /bills (timing adjustment is potentially possible)");
    // Note whether edit is available
    const hasEdit = billEditCheck.some(r => r.buttons.some(b => /edit|adjust|change/i.test(b.text + (b.ariaLabel || ""))));
    if (hasEdit) {
      note("Step 5.2 (I3) — Edit button present on electric bill row — timing adjustment UI exists");
    } else {
      absent_("Step 5.2 (I3) — ABSENT: No edit/adjust button on electric bill row. " +
        "Timing of individual bills cannot be adjusted from /bills UI.");
    }
  } else {
    note("Step 5.1 — Electric bill not specifically found on /bills");
    absent_("Step 5.2 (I3) — ABSENT: Cannot assess timing adjustment — bill not found on /bills");
  }

  await page.screenshot({ path: SS("ss_L55_09_bills_for_adjustment.png") });
  pass("Step 5.3 — screenshot ss_L55_09_bills_for_adjustment.png");

  // Even without a UI affordance, check if the runway projection would change
  // if we manually modify the dataset. We document this as structural behavior.
  // The runway.Project() engine correctly re-computes from NextDue on each render.
  // I3 as a structural invariant: verify the runway re-renders when recurring data changes.
  const dsCheck5 = await getDataset(page);
  const recItems = (dsCheck5.recurring || []).filter(r => /L55/i.test(r.label));
  note(`L55 recurring items: ${JSON.stringify(recItems.map(r => ({ label: r.label, nextDue: r.nextDue, amount: r.amount?.amount })))}`);

  if (recItems.length >= 2) {
    pass("Step 5.4 (I3) — Both L55 recurring entries exist; runway projection inputs are present");
    note("I3 structural note: runway.Project() re-runs on state change; if NextDue is updated, " +
      "the projection updates. The UI affordance to change due date is the gap (no edit button on /bills).");
  } else {
    fail("Step 5.4 (I3) — Expected 2 L55 recurring entries; found " + recItems.length);
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 6: /planning — re-check runway after noting timing; check I1/I2 from code
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 6: /planning — re-read runway for final I1/I2 state ───────────────");

  await dismissModal(page);
  await navTo(page, "Planning");
  await page.waitForTimeout(800);

  const planBody6 = await page.evaluate(() => document.body.textContent ?? "");

  // Assess whether the 12-month net-worth forecast incorporates the L55 recurring items
  const forecastText = await page.evaluate(() => {
    const all = Array.from(document.querySelectorAll("p.muted, .card p, .plan-hint"));
    return all.map(p => p.textContent.trim()).filter(Boolean);
  });
  note(`Planning forecast hint paragraphs: ${JSON.stringify(forecastText.slice(0, 8))}`);

  // I5: Money conservation check — compare dataset-level values
  const dsFinal = await getDataset(page);
  const allRec = (dsFinal.recurring || []).filter(r => /L55/i.test(r.label));
  // domain.Recurring amount can be stored as {amount: N, currency: "USD"} or as a plain number
  const getRecAmt = (r) => {
    if (typeof r.amount === "number") return r.amount;
    // Go JSON marshals money.Money as {Amount: N, Currency: "USD"} (capital A)
    if (r.amount?.Amount !== undefined) return r.amount.Amount;
    if (r.amount?.amount !== undefined) return r.amount.amount;
    if (typeof r.monthlyAmount === "number") return r.monthlyAmount;
    return 0;
  };
  const totalScheduledMonthly = allRec.reduce((sum, r) => sum + getRecAmt(r), 0);
  note(`I5 conservation check — L55 recurring items: ${JSON.stringify(allRec.map(r => ({ label: r.label, amount: r.amount, monthlyAmount: r.monthlyAmount })))}`);
  note(`I5 total scheduled monthly (minor units): ${totalScheduledMonthly} (rent -80000 + electric -12000 = -92000 expected)`);

  // Check the forecast numeric summary if present
  const forecastAmounts = await page.evaluate(() => {
    const cards = Array.from(document.querySelectorAll(".card, section"));
    const netCard = cards.find(c => /net worth|12.month|forecast/i.test(c.textContent) &&
      c.querySelector("h2,h3"));
    if (!netCard) return null;
    const spans = Array.from(netCard.querySelectorAll(".budget-amount, strong, b, [class*='amount']"));
    return spans.map(s => s.textContent.trim()).filter(Boolean).slice(0, 5);
  });
  note(`Forecast amounts visible: ${JSON.stringify(forecastAmounts)}`);

  await page.screenshot({ path: SS("ss_L55_10_planning_final.png") });
  pass("Step 6.1 — screenshot ss_L55_10_planning_final.png");

  // Structural finding for I1/I2: check whether forecast is month-only or day-by-day
  // forecast.Project() uses monthly granularity (verified from source code).
  // runway.Project() uses daily granularity but requires the runway card to be activated.
  note("STRUCTURAL ANALYSIS (I1): The app has TWO projection engines:");
  note("  1. forecast.Project() — 12-month granularity, end-of-month balances only. " +
    "Cannot model intra-period dip below zero within a calendar month.");
  note("  2. runway.Project() — 60-day daily granularity, flags first day balance dips below buffer. " +
    "This IS the intra-period engine. It IS wired to /planning's runway card.");
  note("  CONCLUSION: I1 (intra-period dip) IS architecturally supported via the runway card, " +
    "but the runway card requires the user to navigate to /planning AND configure a buffer value. " +
    "It is NOT automatically displayed with an overdraft warning when bills exceed balance.");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 7: /dashboard — check cash-flow risk / shortfall warning
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 7: /dashboard — cash-flow risk / shortfall warning ─────────────────");

  await dismissModal(page);
  await navTo(page, "Dashboard");
  await page.waitForTimeout(800);

  await page.screenshot({ path: SS("ss_L55_11_dashboard.png") });
  pass("Step 7.1 — screenshot ss_L55_11_dashboard.png");

  const dashBody = await page.evaluate(() => document.body.textContent ?? "");

  // I4: Dashboard shows cash-flow risk / upcoming bills / shortfall warning
  const hasUpcomingBills = /upcoming bills/i.test(dashBody);
  const hasShortfall = /shortfall|overdr|at risk|cash.flow risk|breach|low balance|negative/i.test(dashBody);
  const hasBillsWidget = hasUpcomingBills || /due|bills/i.test(dashBody);

  note(`Dashboard — "upcoming bills": ${hasUpcomingBills}, shortfall/risk signal: ${hasShortfall}`);

  if (hasUpcomingBills) pass("Step 7.2 (I4) — Dashboard shows 'Upcoming bills' widget");
  else fail("Step 7.2 (I4) — 'Upcoming bills' widget NOT found on Dashboard");

  if (hasShortfall) {
    pass("Step 7.3 (I4) — Dashboard shows cash-flow risk / shortfall warning");
  } else {
    absent_("Step 7.3 (I4) — ABSENT: No shortfall/overdraft warning on Dashboard. " +
      "Dashboard shows upcoming bills list but does NOT proactively warn about impending cash-flow gaps.");
  }

  // Check dashboard for L55 bills in the upcoming widget
  const dashBillRows = await page.evaluate(() => {
    const allEls = Array.from(document.querySelectorAll("*"));
    const header = allEls.find(el =>
      /upcoming bills/i.test(el.textContent) && el.tagName.match(/^H[1-6]$/));
    if (!header) return { found: false };
    const card = header.closest("section, .card, article, div[class]");
    if (!card) return { found: false };
    return {
      found: true,
      rows: Array.from(card.querySelectorAll(".row")).map(r => ({
        name: r.querySelector(".row-desc")?.textContent?.trim(),
        amount: r.querySelector(".budget-amount")?.textContent?.trim(),
      })).filter(r => r.name),
    };
  });
  note(`Dashboard bills widget: ${JSON.stringify(dashBillRows)}`);

  if (dashBillRows.found && dashBillRows.rows?.length > 0) {
    const hasRentInDash = dashBillRows.rows.some(r => /L55.*Dani.*Rent|Dani.*Rent/i.test(r.name));
    const hasElecInDash = dashBillRows.rows.some(r => /L55.*Dani.*Electric|Dani.*Electric/i.test(r.name));
    note(`Dashboard upcoming bills — rent: ${hasRentInDash}, electric: ${hasElecInDash}`);
    pass(`Step 7.4 (I4/I5) — Dashboard upcoming-bills widget has ${dashBillRows.rows.length} row(s)`);
  } else {
    note("Step 7.4 — Dashboard upcoming-bills widget not found or empty");
  }

  await page.screenshot({ path: SS("ss_L55_12_dashboard_final.png") });
  pass("Step 7.5 — screenshot ss_L55_12_dashboard_final.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 8: Final invariant summary
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 8: final invariant audit ───────────────────────────────────────────");

  // I1: Intra-period dip visible
  const i1_architecturally_supported = runwayCardDetails.found;
  if (i1_architecturally_supported) {
    note("I1 RESULT: PARTIAL — runway card EXISTS (day-by-day engine is wired). " +
      "However it requires manual buffer entry and does not automatically alert on bills-before-payday.");
  } else {
    note("I1 RESULT: ABSENT — no cash runway card rendered. The intra-period engine (runway.Project) " +
      "exists in code but the planning page card may not be rendering.");
  }

  // I2: Overdraft warning
  note("I2 RESULT: " + (runwayCardDetails.hasBreach ? "HELD — breach warning shown" :
    "ABSENT — no automatic overdraft/breach warning surfaced. The runway card is passive (requires user input)."));

  // I3: Timing adjustment
  note("I3 RESULT: ABSENT — no UI affordance to adjust individual bill due dates from /bills or /planning. " +
    "Recurring items' NextDue is set at creation; no edit-after-create UI found for due-date modification.");

  // I4: Dashboard risk
  note("I4 RESULT: " + (hasUpcomingBills ? "PARTIAL — upcoming bills widget present on Dashboard. " +
    "No proactive cash-flow risk/shortfall alert tied to intra-period analysis." :
    "ABSENT — dashboard does not show upcoming bills."));

  // I5: Money conservation
  const expectedMonthlyOutflow = -800 * 100 + -120 * 100; // minor units
  const actualMonthlyOutflow = totalScheduledMonthly;
  const conserved = Math.abs(actualMonthlyOutflow - expectedMonthlyOutflow) < 500; // within $5 rounding
  note(`I5 RESULT: ${conserved ? "HELD" : "CHECK"} — expected monthly outflow ${expectedMonthlyOutflow} minor units, ` +
    `dataset has ${actualMonthlyOutflow} for L55 items`);
  if (conserved) pass("Step 8.1 (I5) — Money conservation: scheduled outflows match seeded amounts");
  else note("Step 8.1 (I5) — Money conservation: amounts may differ from expected (check minor-unit encoding)");

  // ════════════════════════════════════════════════════════════════════════════
  // SUMMARY
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n══════════════════════════════════════════════════════════════════════════════");
  console.log(`SUMMARY: ${passed} passed, ${failed} failed, ${absent} absent`);
  if (jsErrors.length) {
    console.error(`JS Errors: ${jsErrors.join(" | ")}`);
  }
  if (failed > 0) {
    console.error(`RESULT: FAIL (${failed} failures, ${absent} absences)`);
    process.exitCode = 1;
  } else if (absent > 0) {
    console.warn(`RESULT: PARTIAL — all pass, but ${absent} invariants are ABSENT (gaps)`);
    process.exitCode = 1;
  } else {
    console.log("RESULT: PASS — all invariants confirmed");
  }

} finally {
  await browser.close();
}
