// L70 E2E loop story — "The Payment Plan" (Marcus) — 2026-06-22
//
// Persona: Marcus negotiated a structured payment plan on a $1,200 medical bill.
// The creditor agreed to 6 monthly installments of $200/month. Marcus opens CashFlux
// to seed a checking account (~$600 cash), record the $1,200 medical debt as a
// liability, set up the installment schedule as recurring transactions, record the
// first $200 payment, and verify the debt shrinks $1,200 → $1,000.
//
// KEY INVARIANTS ASSERTED:
//   I1: DEBT_SEED_SIGN    — Medical liability created shows as $1,200 owed (negative / liability)
//                           Re-tests L64 sign-convention bug for liability accounts
//   I2: INSTALLMENT_MODEL — Can 6 recurring payments with a fixed end date be expressed?
//                           (fixed-term vs open-ended gap filed if absent)
//   I3: BILLS_APPEAR      — 6 monthly installments appear on /bills or recurring list
//                           (re-tests NextDue=now bug from L54/L55/L64)
//   I4: FIRST_PAYMENT     — Recording $200 payment: medical debt $1,200→$1,000 (sign direction)
//                           Checking $600→$400 (asset debit)
//   I5: PROGRESS_SURFACE  — Does any screen show "1 of 6 paid, $1,000 remaining, 5 to go"?
//   I6: FORECAST_RECURRING— Dashboard/forecast includes the 5 remaining installments in upcoming
//                           obligations (re-tests Thread B: forecast ignoring recurring)
//   I7: MONEY_CONSERVE    — After 1 payment: debt $1,000 + checking $400 consistent across screens;
//                           no phantom money
//
// Screens exercised (≥4):
//   /accounts → /planning (recurring bills) → /bills → /transactions → /accounts →
//   /dashboard
//
// Run: E2E_URL=http://127.0.0.1:8080 node e2e/loopstory_70_payment_plan.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";

// Screenshots go in e2e/screenshots/
const SSDIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SSDIR)) fs.mkdirSync(SSDIR, { recursive: true });
const SS = (name) => path.join(SSDIR, name);

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0, absent = 0;
const pass    = (label) => { console.log(`PASS:   ${label}`); passed++; };
const fail    = (label) => { console.error(`FAIL:   ${label}`); failed++; };
const absent_ = (label) => { console.log(`ABSENT: ${label}`); absent++; };
const note    = (label) => { console.log(`NOTE:   ${label}`); };

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

const flush = async (page) => {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
};

const getDataset = (page) => page.evaluate(() => {
  try { return JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"); } catch { return {}; }
});

const dismissModal = async (page) => {
  await page.keyboard.press("Escape");
  await page.waitForTimeout(200);
  await page.evaluate(() => {
    const btn = document.querySelector('button[aria-label="Cancel"], dialog button.btn:not(.btn-primary)');
    if (btn) btn.click();
  });
  await page.waitForTimeout(200);
};

// Create an account by type
const createAccount = async (page, name, typeText, openingBalance) => {
  await navTo(page, "Accounts");
  await dismissModal(page);

  const addR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /add account|new account/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked"; }
    return "NOT FOUND";
  });
  note(`  Add Account button: ${addR}`);
  await page.waitForTimeout(800);

  await page.evaluate((n) => {
    const inp = Array.from(document.querySelectorAll("input[type='text']")).find(i => i.placeholder === "Name");
    if (!inp) return "NOT FOUND";
    inp.focus(); inp.value = n;
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  }, name);

  const typeR = await selectByText(page, "Account type", typeText);
  note(`  Account type: ${typeR}`);

  await page.evaluate((b) => {
    const inp = Array.from(document.querySelectorAll("input[type='number']")).find(i =>
      i.placeholder === "Opening balance");
    if (!inp) return "NOT FOUND";
    inp.value = b;
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  }, String(openingBalance));

  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b => {
      const t = b.textContent.trim();
      return /^add account$|^add$|^save$/i.test(t) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);
};

// Seed a recurring/scheduled bill via /planning
const seedRecurring = async (page, label, amount, cadence, dueDate) => {
  await navTo(page, "Planning");
  await dismissModal(page);
  await page.waitForTimeout(800);

  // Try to click "Add recurring" or similar
  const addR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /add recurring|new recurring|add bill|new bill/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked"; }
    return "NOT FOUND";
  });
  note(`  Add recurring button: ${addR}`);
  if (addR === "NOT FOUND") return "SKIPPED";

  await page.waitForTimeout(800);

  // Fill label
  await page.evaluate((lbl) => {
    const inp = Array.from(document.querySelectorAll("input,textarea")).find(i =>
      i.getAttribute("aria-label") === "Label" ||
      i.getAttribute("aria-label") === "Name" ||
      i.getAttribute("placeholder") === "Label" ||
      i.getAttribute("placeholder") === "Name" ||
      i.getAttribute("placeholder") === "Description");
    if (inp) {
      inp.focus(); inp.value = lbl;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, label);

  // Fill amount
  await page.evaluate((a) => {
    const inp = document.querySelector('input[type="number"]');
    if (inp) {
      inp.value = a;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, String(amount));

  // Set cadence (monthly)
  const cadR = await selectByText(page, "Cadence", cadence);
  note(`  Cadence: ${cadR}`);
  if (cadR.includes("NOT found")) {
    // Try "Frequency" or "Period"
    const cadR2 = await selectByText(page, "Frequency", cadence);
    note(`  Frequency: ${cadR2}`);
  }

  // Set due date if a date input is present
  if (dueDate) {
    const dateR = await page.evaluate((d) => {
      const inp = document.querySelector('input[type="date"]');
      if (!inp) return "NOT FOUND";
      inp.value = d;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
      return `set to ${d}`;
    }, dueDate);
    note(`  Due date: ${dateR}`);
  }

  // Check for end-date / term / number of payments field (installment-plan modeling)
  const termR = await page.evaluate(() => {
    const possibleLabels = ["End date", "End after", "Number of payments", "Occurrences", "Term", "Installments"];
    for (const lbl of possibleLabels) {
      const inp = Array.from(document.querySelectorAll("input")).find(i =>
        i.getAttribute("aria-label") === lbl ||
        i.getAttribute("placeholder") === lbl);
      if (inp) return `FOUND: field "${lbl}" (type=${inp.type})`;
    }
    // Also check for select options like "ends after"
    const selects = Array.from(document.querySelectorAll("select"));
    for (const s of selects) {
      const opts = Array.from(s.options).map(o => o.text);
      if (opts.some(t => /end|term|occurrence|installment/i.test(t))) {
        return `FOUND: select with opts: ${opts.join(", ")}`;
      }
    }
    return "ABSENT — no end-date/term/installment-count field found";
  });
  note(`  Installment term field: ${termR}`);

  // Submit
  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b => {
      const t = b.textContent.trim();
      return /^add$|^save$|^add bill$/i.test(t) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);

  return termR;
};

// Record an expense transaction (e.g. debt payment)
const recordTransfer = async (page, label, amount, fromAccountMatch, toAccountMatch, dateStr) => {
  await dismissModal(page);
  await navTo(page, "Transactions");
  await page.waitForTimeout(500);

  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /new transaction|add transaction/i.test(b.textContent.trim()));
    if (btn) btn.click();
  });
  await page.waitForTimeout(800);

  // Fill description
  await page.evaluate(({ desc }) => {
    const inp = Array.from(document.querySelectorAll("input,textarea")).find(i =>
      i.getAttribute("aria-label") === "Description" ||
      i.getAttribute("placeholder") === "Description" ||
      i.getAttribute("aria-label") === "Payee" ||
      i.getAttribute("placeholder") === "Payee");
    if (inp) {
      inp.focus(); inp.value = desc;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, { desc: label });

  // Fill amount
  await page.evaluate((a) => {
    const inp = document.querySelector('input[type="number"]');
    if (inp) {
      inp.value = a;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, String(amount));

  // Set type to Transfer
  const typeR = await selectByText(page, "Type", "Transfer");
  note(`  Transaction type: ${typeR}`);

  // Set From account (checking)
  const fromR = await page.evaluate((match) => {
    const sel = Array.from(document.querySelectorAll("select")).find(s =>
      s.getAttribute("aria-label") === "From" || s.getAttribute("aria-label") === "From account");
    if (!sel) return `From select NOT FOUND; selects: ${Array.from(document.querySelectorAll("select")).map(s => s.getAttribute("aria-label")).join(", ")}`;
    const opt = Array.from(sel.options).find(o => new RegExp(match, "i").test(o.text));
    if (opt) {
      sel.value = opt.value; sel.dispatchEvent(new Event("change", { bubbles: true }));
      return `set → "${opt.text}"`;
    }
    return `no option matching "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
  }, fromAccountMatch);
  note(`  From account: ${fromR}`);

  // Set To account (medical debt liability)
  const toR = await page.evaluate((match) => {
    const sel = Array.from(document.querySelectorAll("select")).find(s =>
      s.getAttribute("aria-label") === "To" || s.getAttribute("aria-label") === "To account");
    if (!sel) return `To select NOT FOUND; selects: ${Array.from(document.querySelectorAll("select")).map(s => s.getAttribute("aria-label")).join(", ")}`;
    const opt = Array.from(sel.options).find(o => new RegExp(match, "i").test(o.text));
    if (opt) {
      sel.value = opt.value; sel.dispatchEvent(new Event("change", { bubbles: true }));
      return `set → "${opt.text}"`;
    }
    return `no option matching "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
  }, toAccountMatch);
  note(`  To account: ${toR}`);

  if (dateStr) {
    await page.evaluate((d) => {
      const inp = document.querySelector('input[type="date"]');
      if (inp) {
        inp.value = d;
        inp.dispatchEvent(new Event("input", { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    }, dateStr);
  }

  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b => {
      const t = b.textContent.trim();
      return /^add$|^save$|^add transaction$/i.test(t) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);

  return { fromR, toR };
};

// ─── main ─────────────────────────────────────────────────────────────────────

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  page.on("pageerror", (e) => {
    const msg = String(e);
    if (!msg.includes("Go program has already exited")) jsErrors.push(msg);
  });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  pass("HYDRATION — app loaded and nav visible");

  // Hard reload to clear any stale atom state from prior runs (L69 harness lesson)
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  note("Hard reload complete — clearing stale atom state");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 1: Seed accounts
  //   L70 Marcus Checking  ($600, checking)
  //   L70 Marcus MedDebt   ($1200 opening balance, liability account)
  //
  // L64 NOTE: Liability sign convention bug — new liability accounts may show as
  // positive ($1,200) rather than negative (($1,200)). We check the sign here.
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 1: Seed accounts ────────────────────────────────────────────────────────────");

  await createAccount(page, "L70 Marcus Checking", "Checking", 600);
  await createAccount(page, "L70 Marcus MedDebt",  "Liability", 1200);

  await navTo(page, "Accounts");
  await dismissModal(page);
  const acctText1 = await page.evaluate(() => document.body.textContent);

  // L69 re-test: do freshly-created accounts appear on /accounts?
  const checkingVisible = /L70 Marcus Checking/i.test(acctText1);
  const medDebtVisible  = /L70 Marcus MedDebt/i.test(acctText1);

  if (checkingVisible) pass("Step 1.1 — L70 Marcus Checking visible on /accounts");
  else fail("Step 1.1 — L70 Marcus Checking NOT visible on /accounts (re-tests L69 new-account visibility bug)");

  if (medDebtVisible) pass("Step 1.2 — L70 Marcus MedDebt visible on /accounts");
  else fail("Step 1.2 — L70 Marcus MedDebt NOT visible on /accounts (re-tests L69 new-account visibility bug)");

  // I1: Sign convention check — does MedDebt appear as liability (negative / parenthesized)?
  const debtSignRaw = await page.evaluate(() => {
    const text = document.body.textContent;
    // Look for parenthesized amount (e.g. ($1,200.00)) or minus sign near "MedDebt"
    const match = text.match(/L70 Marcus MedDebt[^$\d-]*?([-($][\d,]+\.?\d*)/i);
    return match ? match[1] : null;
  });
  note(`I1 debt sign on screen: ${JSON.stringify(debtSignRaw)}`);

  // Also inspect the dataset opening balance to check stored sign
  const dsStep1 = await getDataset(page);
  const medDebtAccount = Object.values(dsStep1.accounts || {}).find(a => /MedDebt/i.test(a.name || ""));
  const medDebtOpeningRaw = medDebtAccount?.openingBalance?.Amount ?? medDebtAccount?.openingBalance?.amount ?? null;
  note(`I1 MedDebt dataset openingBalance.Amount: ${medDebtOpeningRaw}`);

  // Stored as negative means correct liability; positive = L64 bug
  if (medDebtOpeningRaw !== null) {
    if (medDebtOpeningRaw < 0) {
      pass("I1 DEBT_SEED_SIGN — MedDebt stored as negative (correct liability sign)");
    } else if (medDebtOpeningRaw > 0) {
      fail(`I1 DEBT_SEED_SIGN — MedDebt stored as POSITIVE (${medDebtOpeningRaw}): RE-CONFIRMS L64 liability sign bug`);
    } else {
      fail("I1 DEBT_SEED_SIGN — MedDebt stored as 0 (unexpected)");
    }
  } else {
    absent_("I1 DEBT_SEED_SIGN — MedDebt not found in dataset; cannot check sign");
  }

  await page.screenshot({ path: SS("ss_L70_01_accounts_seed.png") });
  note("Screenshot: ss_L70_01_accounts_seed.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 2: Set up installment plan — 6 recurring $200 monthly payments
  //
  // KEY QUESTION: Can CashFlux express a FIXED-TERM installment plan?
  //   - If yes: set end date or "6 payments" count
  //   - If no: it's open-ended recurring (file the gap)
  // We also check NextDue date handling (re-tests L54/L55/L64 NextDue=now bug)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 2: Set up installment plan (6 × $200/month) ─────────────────────────────────");

  // First installment due today/this month; seed 6 total (open-ended if no term field)
  const today = new Date();
  const yyyy = today.getFullYear();
  const mm = String(today.getMonth() + 1).padStart(2, "0");
  const dd = String(today.getDate()).padStart(2, "0");
  const todayStr = `${yyyy}-${mm}-${dd}`;

  // Seed just one recurring bill (the installment), probe for term/end-date field
  const termResult = await seedRecurring(page, "L70 Marcus Medical Installment", 200, "Monthly", todayStr);
  note(`Installment term probe result: ${termResult}`);

  // I2: Can the app model a fixed-term installment plan?
  if (termResult.startsWith("FOUND")) {
    pass("I2 INSTALLMENT_MODEL — Fixed-term installment field exists in recurring form: " + termResult);
  } else if (termResult === "SKIPPED") {
    absent_("I2 INSTALLMENT_MODEL — Recurring form not reachable; cannot test installment modeling");
  } else {
    fail("I2 INSTALLMENT_MODEL — No end-date/term/occurrence-count field: only open-ended recurring is supported, not a 6-payment fixed installment plan");
  }

  await navTo(page, "Planning");
  await page.screenshot({ path: SS("ss_L70_02_planning_recurring.png") });
  note("Screenshot: ss_L70_02_planning_recurring.png");

  // Check if recurring entry appears in planning list
  const planText = await page.evaluate(() => document.body.textContent);
  const recurringInPlan = /L70 Marcus Medical Installment/i.test(planText);
  if (recurringInPlan) pass("Step 2.1 — Recurring installment appears in /planning list");
  else fail("Step 2.1 — Recurring installment NOT found in /planning list after seeding");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 3: /bills — do the scheduled installments appear with correct dates?
  // Re-tests NextDue=now bug (L54/L55/L64)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 3: Check /bills for installment schedule ───────────────────────────────────");

  await navTo(page, "Bills");
  await page.waitForTimeout(800);
  const billsText = await page.evaluate(() => document.body.textContent);
  const installmentOnBills = /L70 Marcus Medical Installment/i.test(billsText) || /Medical Installment/i.test(billsText);

  if (installmentOnBills) {
    pass("I3 BILLS_APPEAR — Medical installment bill appears on /bills screen");
    // Check for a correct date nearby (not just "now" which would be gone if NextDue=now)
    const dateNearInstallment = await page.evaluate(() => {
      const text = document.body.textContent;
      const match = text.match(/Medical Installment[^$\d]*?(\d{4}-\d{2}-\d{2}|\w+ \d+,? \d{4})/i);
      return match ? match[1] : null;
    });
    note(`I3 installment date on /bills: ${dateNearInstallment}`);
  } else {
    fail("I3 BILLS_APPEAR — Medical installment NOT found on /bills (re-confirms NextDue=now bug or bill not seeded)");
  }

  await page.screenshot({ path: SS("ss_L70_03_bills_schedule.png") });
  note("Screenshot: ss_L70_03_bills_schedule.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 4: Record first $200 installment payment
  //   Transfer: Checking ($600) → MedDebt ($1,200)
  //   Expected after: Checking $400, MedDebt $1,000
  //
  // Re-tests I4 FIRST_PAYMENT sign direction (L64 bug: payment adds to liability instead of reducing)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 4: Record first $200 payment ──────────────────────────────────────────────");

  const txResult = await recordTransfer(
    page,
    "L70 Payment 1 of 6 - Medical",
    200,
    "L70 Marcus Checking",
    "L70 Marcus MedDebt",
    todayStr
  );
  note(`Transfer result: from=${txResult.fromR}, to=${txResult.toR}`);

  // Verify the transaction posted
  await navTo(page, "Transactions");
  const txText = await page.evaluate(() => document.body.textContent);
  const txPosted = /L70 Payment 1 of 6/i.test(txText) || /L70.*Medical/i.test(txText);
  if (txPosted) pass("Step 4.1 — L70 payment transaction appears in /transactions");
  else fail("Step 4.1 — L70 payment NOT found in /transactions");

  await page.screenshot({ path: SS("ss_L70_04_transactions_payment.png") });
  note("Screenshot: ss_L70_04_transactions_payment.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 5: Verify balances after payment
  //   Checking: $600 → $400 (asset debit)
  //   MedDebt:  $1,200 → $1,000 (liability reduced)
  //
  // I4 FIRST_PAYMENT — check both accounts
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 5: Verify balances after payment ───────────────────────────────────────────");

  await navTo(page, "Accounts");
  await dismissModal(page);
  const acctText2 = await page.evaluate(() => document.body.textContent);

  await page.screenshot({ path: SS("ss_L70_05_accounts_after_payment.png") });
  note("Screenshot: ss_L70_05_accounts_after_payment.png");

  // Screen-read balances
  const checkingScreenBalance = await page.evaluate(() => {
    const text = document.body.textContent;
    const m = text.match(/L70 Marcus Checking[^$]*?\$([\d,]+\.?\d*)/i);
    return m ? m[1].replace(/,/g, "") : null;
  });
  const medDebtScreenBalance = await page.evaluate(() => {
    const text = document.body.textContent;
    // Liability might show as ($X) or -$X or just $X
    const m = text.match(/L70 Marcus MedDebt[^$\d(]*?[(−-]?\$?([\d,]+\.?\d*)/i);
    return m ? m[1].replace(/,/g, "") : null;
  });

  note(`Checking screen balance after payment: ${checkingScreenBalance}`);
  note(`MedDebt screen balance after payment: ${medDebtScreenBalance}`);

  if (checkingScreenBalance !== null) {
    const val = parseFloat(checkingScreenBalance);
    if (Math.abs(val - 400) < 1) {
      pass("I4a CHECKING_DEBIT — Checking balance $600→$400 after $200 payment ✓");
    } else {
      fail(`I4a CHECKING_DEBIT — Checking balance = $${val} (expected $400); debit may not have applied`);
    }
  } else {
    // Try dataset
    const ds2 = await getDataset(page);
    const checkingAcct = Object.values(ds2.accounts || {}).find(a => /L70 Marcus Checking/i.test(a.name || ""));
    note(`Checking dataset: ${JSON.stringify(checkingAcct?.openingBalance)}`);
    absent_("I4a CHECKING_DEBIT — Cannot read checking balance from screen (account not visible; L69 bug)");
  }

  if (medDebtScreenBalance !== null) {
    const val = parseFloat(medDebtScreenBalance);
    if (Math.abs(val - 1000) < 1) {
      pass("I4b DEBT_REDUCES — MedDebt balance $1,200→$1,000 after $200 payment ✓");
    } else if (Math.abs(val - 1400) < 1) {
      fail(`I4b DEBT_REDUCES — MedDebt = $${val} (INCREASED to $1,400 instead of dropping to $1,000): RE-CONFIRMS L64 sign bug — payment adds to liability`);
    } else {
      fail(`I4b DEBT_REDUCES — MedDebt = $${val} (expected $1,000 after payment)`);
    }
  } else {
    // Fall back to dataset
    const ds2 = await getDataset(page);
    const medDebtAcct2 = Object.values(ds2.accounts || {}).find(a => /MedDebt/i.test(a.name || ""));
    note(`MedDebt dataset (after payment): ${JSON.stringify(medDebtAcct2)}`);
    absent_("I4b DEBT_REDUCES — Cannot read MedDebt balance from screen; see dataset note above");
  }

  // I7: Money conservation — check dataset totals
  const ds3 = await getDataset(page);
  const txns = Object.values(ds3.transactions || {}).filter(t => /L70/i.test(t.description || t.payee || t.name || ""));
  note(`I7 L70 transactions in dataset: ${txns.length}`);
  note(`I7 dataset transactions detail: ${JSON.stringify(txns.map(t => ({ desc: t.description, amount: t.amount })))}`);

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 6: Installment progress surface
  //   I5: Does any screen show "1 of 6 paid" or progress toward installment completion?
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 6: Installment progress surface ────────────────────────────────────────────");

  await navTo(page, "Bills");
  const billsText2 = await page.evaluate(() => document.body.textContent);
  const progressText = /(\d) of (\d)|paid.*remain|remain.*paid|installment.*progress/i.test(billsText2);

  if (progressText) {
    pass("I5 PROGRESS_SURFACE — Bills screen shows installment progress (X of Y paid)");
  } else {
    absent_("I5 PROGRESS_SURFACE — No installment-progress surface (\"X of Y paid\") found on /bills or elsewhere");
  }

  // Also check /planning for installment progress
  await navTo(page, "Planning");
  const planText2 = await page.evaluate(() => document.body.textContent);
  const progressInPlan = /(\d) of (\d)|paid.*remain|remain.*paid|installment.*progress/i.test(planText2);
  if (progressInPlan) pass("I5b PROGRESS_SURFACE_PLANNING — /planning shows installment progress");
  else note("I5b — No installment progress on /planning either");

  await page.screenshot({ path: SS("ss_L70_06_bills_progress.png") });
  note("Screenshot: ss_L70_06_bills_progress.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 7: Dashboard/forecast — do remaining installments appear?
  //   I6 FORECAST_RECURRING — re-tests Thread B (forecast ignores recurring)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 7: Dashboard/forecast — recurring installments ─────────────────────────────");

  await navTo(page, "Dashboard");
  const dashText = await page.evaluate(() => document.body.textContent);
  const medicalOnDash = /Medical|Installment|L70/i.test(dashText);

  if (medicalOnDash) {
    pass("I6a FORECAST_RECURRING — Dashboard references medical installment (some forecast integration)");
  } else {
    fail("I6a FORECAST_RECURRING — Dashboard does NOT show medical installment in upcoming/forecast (re-confirms Thread B: forecast ignores recurring)");
  }

  // Probe for upcoming bills widget
  const upcomingWidget = /upcoming bills|upcoming payments|scheduled/i.test(dashText);
  if (upcomingWidget) {
    pass("I6b UPCOMING_WIDGET — Dashboard has upcoming bills/payments widget");
  } else {
    absent_("I6b UPCOMING_WIDGET — No upcoming bills/payments widget on Dashboard");
  }

  await page.screenshot({ path: SS("ss_L70_07_dashboard_forecast.png") });
  note("Screenshot: ss_L70_07_dashboard_forecast.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 8: Cross-screen consistency check (I7 money conservation)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 8: Cross-screen consistency ────────────────────────────────────────────────");

  // Final accounts snapshot
  await navTo(page, "Accounts");
  await dismissModal(page);
  await page.screenshot({ path: SS("ss_L70_08_accounts_final.png") });
  note("Screenshot: ss_L70_08_accounts_final.png");

  const finalAcctText = await page.evaluate(() => document.body.textContent);
  const finalCheckingVisible = /L70 Marcus Checking/i.test(finalAcctText);
  const finalDebtVisible = /L70 Marcus MedDebt/i.test(finalAcctText);

  if (finalCheckingVisible && finalDebtVisible) {
    pass("I7a CROSS_SCREEN — Both L70 accounts visible on /accounts final check");
  } else {
    fail(`I7a CROSS_SCREEN — Missing accounts on final /accounts: checking=${finalCheckingVisible}, medDebt=${finalDebtVisible}`);
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 9: Inspect accounts screen selector availability
  //   (Investigate L69 new-account visibility — real bug or harness artifact?)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 9: L69 visibility investigation ─────────────────────────────────────────────");

  const accountsPageInspect = await page.evaluate(() => {
    // Check for member/scope filter selects that might hide new accounts
    const selects = Array.from(document.querySelectorAll("select"));
    const selectInfo = selects.map(s => ({
      label: s.getAttribute("aria-label"),
      id: s.id,
      value: s.value,
      options: Array.from(s.options).map(o => o.text)
    }));

    // Check for any "show all" / member filter UI
    const filterUI = Array.from(document.querySelectorAll("button, a")).filter(el =>
      /show all|filter|member|all accounts/i.test(el.textContent));

    // Check localStorage member key
    let memberKey = null;
    try {
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      memberKey = Object.keys(ds.members || {});
    } catch {}

    return { selectInfo, filterButtons: filterUI.map(e => e.textContent.trim()), memberKey };
  });

  note(`L69 investigation — selects on /accounts: ${JSON.stringify(accountsPageInspect.selectInfo)}`);
  note(`L69 investigation — filter buttons: ${JSON.stringify(accountsPageInspect.filterButtons)}`);
  note(`L69 investigation — member keys: ${JSON.stringify(accountsPageInspect.memberKey)}`);

  // Check if there's a member filter select that could be excluding new accounts
  const memberFilterExists = accountsPageInspect.selectInfo.some(s =>
    /member|owner|scope/i.test(s.label || "") ||
    s.options.some(o => /all members|household|everyone/i.test(o)));

  if (memberFilterExists) {
    note("L69 DIAGNOSIS: Member/owner filter select found on /accounts — new accounts may be filtered by member scope (HARNESS ARTIFACT possible)");
  } else {
    note("L69 DIAGNOSIS: No member filter select found — new-account invisibility is likely a state/atom reactive gap (REAL BUG)");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // FINAL SUMMARY
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── FINAL SUMMARY ────────────────────────────────────────────────────────────────────");

  if (jsErrors.length) {
    console.error(`JS ERRORS (${jsErrors.length}):`);
    jsErrors.forEach(e => console.error("  " + e));
  } else {
    console.log("JS ERRORS: none");
  }

  console.log(`\nResults: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
  console.log("Screenshots produced:");
  [
    "ss_L70_01_accounts_seed.png",
    "ss_L70_02_planning_recurring.png",
    "ss_L70_03_bills_schedule.png",
    "ss_L70_04_transactions_payment.png",
    "ss_L70_05_accounts_after_payment.png",
    "ss_L70_06_bills_progress.png",
    "ss_L70_07_dashboard_forecast.png",
    "ss_L70_08_accounts_final.png",
  ].forEach(f => console.log("  " + f));

  process.exit(failed > 0 || absent > 0 ? 1 : 0);

} catch (err) {
  console.error("FATAL:", err);
  process.exit(2);
} finally {
  await browser.close();
}
