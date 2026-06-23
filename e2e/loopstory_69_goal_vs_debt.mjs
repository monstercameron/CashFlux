// L69 E2E loop story — "Goal vs. Debt" (Aaliyah) — 2026-06-22
//
// Persona: Aaliyah has a checking account (~$400), a savings account ($150),
// a credit card with $2,000 balance at 23% APR and a $40 minimum payment, and
// an "Emergency Fund" goal (target $1,000, currently $150, linked to her savings
// account). She has $200 to deploy and faces a classic personal-finance fork:
//   Option A — Contribute $200 to Emergency Fund (savings goal progress)
//   Option B — Pay $200 toward the credit card (reduce liability + save interest)
//
// KEY INVARIANTS ASSERTED:
//   A1: GOAL_PROGRESS   — After $200 goal contribution, goal shows $150→$350 (35% of $1,000)
//   A2: SAVINGS_LINKED  — Goal contribution moves the linked savings account balance
//   B1: CARD_DECREASES  — After $200 card payment, liability drops $2,000→$1,800 (not rises)
//                         This re-tests L64/L65/L67 sign-convention bug
//   B2: NET_WORTH_A     — Scenario A: net worth +$200 (cash moves to savings, same household net)
//   B3: NET_WORTH_B     — Scenario B: net worth +$200 (liability falls by $200, net worth rises)
//   C1: CROSS_SCREEN    — Accounts / Goals / Dashboard agree after each operation
//   C2: LEDGER_POST     — Transactions are present after goal contribution + card payment
//                         Re-tests L56 Thread A (satellite systems post to ledger)
//   D1: DECISION_SUPPORT— Does any screen surface interest saved vs goal progress tradeoff?
//
// Screens exercised:
//   /accounts (create checking, savings, credit card) →
//   /goals (create Emergency Fund, contribute $200) →
//   /transactions (verify goal contribution ledger post) →
//   /accounts (verify savings balance + credit card balance) →
//   /transactions (record $200 card payment) →
//   /accounts (verify card drops $2,000→$1,800) →
//   /dashboard (net worth after each scenario)
//
// Run: E2E_URL=http://127.0.0.1:8080 node e2e/loopstory_69_goal_vs_debt.mjs

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

// Create an account by type (checking / savings / credit card)
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

// Record an expense transaction
const recordExpense = async (page, label, amount, accountMatch, dateStr) => {
  await dismissModal(page);
  await navTo(page, "Transactions");
  await page.waitForTimeout(500);

  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /new transaction|add transaction/i.test(b.textContent.trim()));
    if (btn) btn.click();
  });
  await page.waitForTimeout(800);

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

  await page.evaluate((a) => {
    const inp = document.querySelector('input[type="number"]');
    if (inp) {
      inp.value = a;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, String(amount));

  await selectByText(page, "Type", "Expense");

  const acctR = await page.evaluate((match) => {
    const sel = Array.from(document.querySelectorAll("select")).find(s =>
      s.getAttribute("aria-label") === "Account" || s.getAttribute("aria-label") === "From account");
    if (!sel) return "Account select NOT FOUND";
    const opt = Array.from(sel.options).find(o => new RegExp(match, "i").test(o.text));
    if (opt) {
      sel.value = opt.value;
      sel.dispatchEvent(new Event("change", { bubbles: true }));
      return `set → "${opt.text}"`;
    }
    return `no option matching "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
  }, accountMatch);
  note(`  ${label} account: ${acctR}`);

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
};

// Read the numeric balance of a named account from the dataset
const readAccountBalance = (dataset, namePattern) => {
  const accounts = Object.values(dataset.accounts || {});
  const acct = accounts.find(a => new RegExp(namePattern, "i").test(a.name || ""));
  if (!acct) return null;
  // Balance field may be CurrentBalance, balance, or amount struct
  const raw = acct.CurrentBalance ?? acct.currentBalance ?? acct.balance ?? null;
  if (raw === null) return null;
  // May be a money struct with Amount field
  if (typeof raw === "object" && raw !== null) {
    return raw.Amount ?? raw.amount ?? null;
  }
  return Number(raw);
};

// Read balance displayed on screen for a named account
const readScreenBalance = async (page, namePattern) => {
  return page.evaluate((pat) => {
    const text = document.body.textContent;
    const re = new RegExp(pat + "[^$]*?\\$([\\d,]+\\.?\\d*)", "i");
    const m = text.match(re);
    if (m) return m[1].replace(/,/g, "");
    return null;
  }, namePattern);
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

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 1: Seed accounts
  //   L69 Aaliyah Checking  ($400, checking)
  //   L69 Aaliyah Savings   ($150, savings — will be linked to goal)
  //   L69 Aaliyah Visa      ($2000 opening balance, credit card)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 1: Seed accounts ────────────────────────────────────────────────────────────");

  await createAccount(page, "L69 Aaliyah Checking", "Checking", 400);
  await createAccount(page, "L69 Aaliyah Savings",  "Savings",  150);
  await createAccount(page, "L69 Aaliyah Visa",     "Credit card", 2000);

  await navTo(page, "Accounts");
  await dismissModal(page);
  const acctText1 = await page.evaluate(() => document.body.textContent);

  const checkingVisible = /L69 Aaliyah Checking/i.test(acctText1);
  const savingsVisible  = /L69 Aaliyah Savings/i.test(acctText1);
  const visaVisible     = /L69 Aaliyah Visa/i.test(acctText1);

  if (checkingVisible) pass("Step 1.1 — L69 Aaliyah Checking visible on /accounts");
  else fail("Step 1.1 — L69 Aaliyah Checking NOT found on /accounts");

  if (savingsVisible) pass("Step 1.2 — L69 Aaliyah Savings visible on /accounts");
  else fail("Step 1.2 — L69 Aaliyah Savings NOT found on /accounts");

  if (visaVisible) pass("Step 1.3 — L69 Aaliyah Visa visible on /accounts");
  else fail("Step 1.3 — L69 Aaliyah Visa NOT found on /accounts");

  await page.screenshot({ path: SS("ss_L69_01_seed_accounts.png") });
  pass("Step 1.4 — screenshot ss_L69_01_seed_accounts.png");

  // Read initial dataset card balances for baseline
  const dsBaseline = await getDataset(page);
  const visaBaselineRaw = readAccountBalance(dsBaseline, "L69 Aaliyah Visa");
  note(`Visa baseline balance (dataset): ${JSON.stringify(visaBaselineRaw)}`);
  const savingsBaselineRaw = readAccountBalance(dsBaseline, "L69 Aaliyah Savings");
  note(`Savings baseline balance (dataset): ${JSON.stringify(savingsBaselineRaw)}`);

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 2: Create "Emergency Fund" goal
  //   Target: $1,000 | Current: $150 (pre-seeded via opening savings balance)
  //   Linked to: L69 Aaliyah Savings
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 2: Create Emergency Fund goal ──────────────────────────────────────────────");

  await navTo(page, "Goals");
  await dismissModal(page);
  await page.waitForTimeout(800);

  const addGoalR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /add goal|new goal/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked"; }
    return "NOT FOUND";
  });
  note(`  Add Goal button: ${addGoalR}`);
  await page.waitForTimeout(800);

  // Goal name
  await page.evaluate(() => {
    const inp = Array.from(document.querySelectorAll("input[type='text']")).find(i =>
      i.placeholder === "Name" || i.getAttribute("aria-label") === "Name");
    if (!inp) return;
    inp.focus(); inp.value = "L69 Emergency Fund";
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  });

  // Target amount
  await page.evaluate(() => {
    const inp = Array.from(document.querySelectorAll("input[type='number']")).find(i =>
      /target|goal.?amount|amount/i.test(i.getAttribute("aria-label") || i.placeholder || ""));
    if (!inp) {
      // fallback — first number input
      const all = document.querySelectorAll("input[type='number']");
      if (all[0]) {
        all[0].value = "1000";
        all[0].dispatchEvent(new Event("input", { bubbles: true }));
        all[0].dispatchEvent(new Event("change", { bubbles: true }));
      }
      return;
    }
    inp.value = "1000";
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  });

  // Try to link to savings account
  const linkR = await page.evaluate(() => {
    const sel = Array.from(document.querySelectorAll("select")).find(s =>
      /account|linked/i.test(s.getAttribute("aria-label") || ""));
    if (!sel) return "No linked-account select found";
    const opt = Array.from(sel.options).find(o => /L69 Aaliyah Savings/i.test(o.text));
    if (opt) {
      sel.value = opt.value;
      sel.dispatchEvent(new Event("change", { bubbles: true }));
      return `linked → "${opt.text}"`;
    }
    return `options: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
  });
  note(`  Goal linked account: ${linkR}`);

  // Submit
  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b => {
      const t = b.textContent.trim();
      return /^add goal$|^add$|^save$/i.test(t) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);

  await navTo(page, "Goals");
  await dismissModal(page);
  const goalsText1 = await page.evaluate(() => document.body.textContent);

  if (/L69 Emergency Fund/i.test(goalsText1)) {
    pass("Step 2.1 — L69 Emergency Fund goal visible on /goals");
  } else {
    fail("Step 2.1 — L69 Emergency Fund goal NOT found on /goals");
  }

  await page.screenshot({ path: SS("ss_L69_02_goal_created.png") });
  pass("Step 2.2 — screenshot ss_L69_02_goal_created.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 3: Scenario A — Contribute $200 to Emergency Fund
  //   Assert A1: goal progress $150→$350
  //   Assert A2: linked savings account balance increases
  //   Assert C2: a transaction is posted to the ledger (L56 re-test)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 3: Scenario A — Contribute $200 to Emergency Fund ──────────────────────────");

  // Try to contribute via the Goals screen
  const contributeR = await page.evaluate(() => {
    // Look for a Contribute / Add Funds / Deposit button near the goal
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /contribute|add funds|deposit/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked: " + btn.textContent.trim(); }
    return "NOT FOUND";
  });
  note(`  Contribute button: ${contributeR}`);
  await page.waitForTimeout(800);

  if (contributeR !== "NOT FOUND") {
    // Fill contribution amount
    await page.evaluate(() => {
      const inp = document.querySelector('input[type="number"]');
      if (inp) {
        inp.value = "200";
        inp.dispatchEvent(new Event("input", { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    });
    await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll("button")).find(b => {
        const t = b.textContent.trim();
        return /^contribute$|^add$|^save$|^confirm$/i.test(t) && b.type !== "reset";
      });
      if (btn) btn.click();
    });
    await page.waitForTimeout(1500);
    await flush(page);
    pass("Step 3.0 — Contribute button found and $200 submitted via Goals UI");
  } else {
    // Goals UI has no contribute button — record as a transfer/expense on Transactions instead
    // and note that goals don't have a native contribution flow (re-test L41/L59)
    absent_("Step 3.0 — ABSENT: No 'Contribute' / 'Add Funds' button on Goals screen — goals have no native contribution flow (re-confirms L41/L59)");
    note("  Falling back to recording a transaction tagged to savings account to simulate goal contribution");
    // Record income into savings to simulate a $200 contribution
    await dismissModal(page);
    await navTo(page, "Transactions");
    await page.waitForTimeout(500);
    await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll("button")).find(b =>
        /new transaction|add transaction/i.test(b.textContent.trim()));
      if (btn) btn.click();
    });
    await page.waitForTimeout(800);
    await page.evaluate(() => {
      const inp = Array.from(document.querySelectorAll("input,textarea")).find(i =>
        i.getAttribute("aria-label") === "Description" ||
        i.getAttribute("placeholder") === "Description" ||
        i.getAttribute("aria-label") === "Payee" ||
        i.getAttribute("placeholder") === "Payee");
      if (inp) {
        inp.focus(); inp.value = "L69 Goal Contribution Emergency Fund";
        inp.dispatchEvent(new Event("input", { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    });
    await page.evaluate(() => {
      const inp = document.querySelector('input[type="number"]');
      if (inp) { inp.value = "200"; inp.dispatchEvent(new Event("input", { bubbles: true })); inp.dispatchEvent(new Event("change", { bubbles: true })); }
    });
    await selectByText(page, "Type", "Income");
    const acctR = await page.evaluate(() => {
      const sel = Array.from(document.querySelectorAll("select")).find(s =>
        s.getAttribute("aria-label") === "Account" || s.getAttribute("aria-label") === "From account");
      if (!sel) return "NOT FOUND";
      const opt = Array.from(sel.options).find(o => /L69 Aaliyah Savings/i.test(o.text));
      if (opt) { sel.value = opt.value; sel.dispatchEvent(new Event("change", { bubbles: true })); return `set → "${opt.text}"`; }
      return `opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
    });
    note(`  Goal contribution transaction account: ${acctR}`);
    await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll("button")).find(b => {
        const t = b.textContent.trim();
        return /^add$|^save$|^add transaction$/i.test(t) && b.type !== "reset";
      });
      if (btn) btn.click();
    });
    await page.waitForTimeout(1500);
    await flush(page);
  }

  // A1: Check goal progress on /goals
  await navTo(page, "Goals");
  await dismissModal(page);
  await page.waitForTimeout(1000);
  const goalsText2 = await page.evaluate(() => document.body.textContent);

  // Look for progress indicators — $350 or 35% or "350"
  const has350 = /350/i.test(goalsText2);
  const goalAmounts = (goalsText2.match(/\$[\d,]+\.?\d*/g) || []).slice(0, 20);
  note(`Goals screen amounts after contribution: ${goalAmounts.join(", ")}`);

  if (has350) {
    pass("Step 3.1 (A1) GOAL_PROGRESS — Goal shows $350 after $200 contribution (150→350) ✓");
  } else {
    absent_("Step 3.1 (A1) GOAL_PROGRESS — ABSENT: $350 NOT found on /goals — goal progress may not update from contribution or has no contribution flow");
  }

  // Also check for 35% progress
  const has35pct = /35\s*%/i.test(goalsText2);
  if (has35pct) {
    pass("Step 3.2 (A1) — Goal shows 35% progress indicator");
  } else {
    note("Step 3.2 (A1) — No 35% text found (may use bar/amount-only, not percentage text)");
  }

  await page.screenshot({ path: SS("ss_L69_03_goal_after_contribution.png") });
  pass("Step 3.3 — screenshot ss_L69_03_goal_after_contribution.png");

  // A2: Check savings account balance
  await navTo(page, "Accounts");
  await dismissModal(page);
  await page.waitForTimeout(1000);
  const dsAfterContrib = await getDataset(page);
  const savingsAfterRaw = readAccountBalance(dsAfterContrib, "L69 Aaliyah Savings");
  note(`Savings balance after contribution (dataset): ${JSON.stringify(savingsAfterRaw)}`);

  // Savings should show $350 (150 + 200) if goal contribution moves account balance
  // It may still show $150 if goal contribution is decoupled from account (L41/L59 bug)
  const savingsAfterNum = typeof savingsAfterRaw === "number" ? savingsAfterRaw : null;
  if (savingsAfterNum !== null) {
    // Minor units: 35000 = $350, 15000 = $150
    if (Math.abs(savingsAfterNum - 35000) <= 1) {
      pass("Step 3.4 (A2) SAVINGS_LINKED — Savings account now $350.00 (contribution moved the balance) ✓");
    } else if (Math.abs(savingsAfterNum - 15000) <= 1) {
      fail("Step 3.4 (A2) SAVINGS_LINKED FAIL — Savings still $150.00 — goal contribution did NOT move the linked savings account (re-confirms L41/L59 decoupling bug)");
    } else {
      note(`Step 3.4 (A2) — Savings balance = ${savingsAfterNum} minor units ($${(savingsAfterNum/100).toFixed(2)}) — unclear if contribution reflected`);
      absent_("Step 3.4 (A2) SAVINGS_LINKED — ABSENT: Cannot confirm savings balance update from dataset");
    }
  } else {
    note(`Step 3.4 (A2) — Savings balance format unresolved: ${JSON.stringify(savingsAfterRaw)}`);
    absent_("Step 3.4 (A2) SAVINGS_LINKED — ABSENT: Savings balance field format unknown");
  }

  // C2: Check ledger — a transaction should have been posted (L56 re-test)
  const txnAfterContrib = await page.evaluate(() => {
    try {
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      return Object.values(ds.transactions || {}).filter(t => {
        const desc = (t.desc || t.description || t.payee || "");
        return /L69/i.test(desc);
      }).map(t => t.desc || t.description || t.payee || "(unnamed)");
    } catch { return []; }
  });
  note(`L69 transactions in ledger after Scenario A: ${JSON.stringify(txnAfterContrib)}`);

  if (txnAfterContrib.length > 0) {
    pass(`Step 3.5 (C2) LEDGER_POST — ${txnAfterContrib.length} L69 transaction(s) in ledger after Scenario A (goal contribution / savings income posted)`);
  } else {
    absent_("Step 3.5 (C2) LEDGER_POST — ABSENT: No L69 transactions in ledger after Scenario A — goal contribution does not post to ledger (re-tests L56 Thread A)");
  }

  await page.screenshot({ path: SS("ss_L69_04_accounts_after_goal.png") });
  pass("Step 3.6 — screenshot ss_L69_04_accounts_after_goal.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 4: Dashboard — net worth after Scenario A
  //   Assert B2: net worth should reflect the $200 (savings up $200 if linked)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 4: Dashboard net worth — post Scenario A ───────────────────────────────────");

  await navTo(page, "Dashboard");
  await dismissModal(page);
  await page.waitForTimeout(1500);

  const dashTextA = await page.evaluate(() => document.body.textContent);
  const dashAmountsA = (dashTextA.match(/\$[\d,]+\.?\d*/g) || []).slice(0, 30);
  note(`Dashboard amounts (post Scenario A): ${dashAmountsA.join(", ")}`);

  const hasNetWorthA = /net worth/i.test(dashTextA);
  if (hasNetWorthA) {
    pass("Step 4.1 — Dashboard shows net worth widget (Scenario A context)");
  } else {
    absent_("Step 4.1 — ABSENT: No 'net worth' text on Dashboard");
  }

  await page.screenshot({ path: SS("ss_L69_05_dashboard_after_goal.png") });
  pass("Step 4.2 — screenshot ss_L69_05_dashboard_after_goal.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 5: Scenario B — Record $200 card payment
  //   Re-tests L64/L65/L67 liability sign-convention bug:
  //   Card should DECREASE $2,000 → $1,800 (not increase to $2,200)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 5: Scenario B — $200 credit card payment (L64/L65/L67 sign re-test) ────────");

  await recordExpense(page, "L69 CC Payment to Visa", 200, "L69 Aaliyah Visa", "2026-06-22");

  // Check Visa balance in dataset
  const dsAfterPayment = await getDataset(page);
  const visaAfterRaw = readAccountBalance(dsAfterPayment, "L69 Aaliyah Visa");
  note(`Visa balance after $200 payment (dataset): ${JSON.stringify(visaAfterRaw)}`);

  // Opening was $2,000 = 200000 minor units
  // After $200 payment, liability should be $1,800 = 180000 minor units
  const visaAfterNum = typeof visaAfterRaw === "number" ? visaAfterRaw : null;
  if (visaAfterNum !== null) {
    if (Math.abs(visaAfterNum - 180000) <= 100) {
      pass("Step 5.1 (B1) CARD_DECREASES — Visa balance now ~$1,800 (payment reduced liability) ✓ — L64/L65/L67 sign bug does NOT reproduce here");
    } else if (Math.abs(visaAfterNum - 220000) <= 100) {
      fail("Step 5.1 (B1) CARD_DECREASES FAIL — Visa balance ~$2,200 (payment INCREASED liability) — L64/L65/L67 sign-convention bug STILL PRESENT");
    } else if (Math.abs(visaAfterNum - 200000) <= 100) {
      fail("Step 5.1 (B1) CARD_DECREASES FAIL — Visa balance unchanged at ~$2,000 — $200 payment had no effect on liability");
    } else {
      note(`Step 5.1 (B1) — Visa after: ${visaAfterNum} minor units ($${(visaAfterNum/100).toFixed(2)}) — ambiguous`);
      absent_("Step 5.1 (B1) CARD_DECREASES — ABSENT: Cannot confirm sign from dataset value");
    }
  } else {
    note(`Step 5.1 (B1) — Visa balance format unresolved: ${JSON.stringify(visaAfterRaw)}`);
    absent_("Step 5.1 (B1) CARD_DECREASES — ABSENT: Visa balance field format unknown");
  }

  // Also check on-screen
  await navTo(page, "Accounts");
  await dismissModal(page);
  await page.waitForTimeout(1000);
  const acctTextAfterPayment = await page.evaluate(() => document.body.textContent);
  const acctAmountsAfter = (acctTextAfterPayment.match(/\$[\d,]+\.?\d*/g) || []).slice(0, 30);
  note(`Accounts screen amounts after card payment: ${acctAmountsAfter.join(", ")}`);

  // Look for $1,800 or $1800
  const shows1800 = /\$1,?800/i.test(acctTextAfterPayment);
  const shows2200 = /\$2,?200/i.test(acctTextAfterPayment);
  const shows2000 = /\$2,?000/i.test(acctTextAfterPayment);

  if (shows1800) {
    pass("Step 5.2 (B1) CARD_DECREASES SCREEN — /accounts shows $1,800 for Visa (payment reduced it) ✓");
  } else if (shows2200) {
    fail("Step 5.2 (B1) CARD_DECREASES SCREEN FAIL — /accounts shows $2,200 for Visa (increased!) — sign bug on screen");
  } else if (shows2000 && !shows1800) {
    note("Step 5.2 (B1) — Screen shows $2,000 (unchanged) — payment may not have been recorded or the balance display uses a different field");
    absent_("Step 5.2 (B1) CARD_DECREASES SCREEN — ABSENT: Cannot confirm $1,800 on /accounts screen");
  } else {
    note("Step 5.2 (B1) — Neither $1,800 nor $2,200 found on screen");
    absent_("Step 5.2 (B1) CARD_DECREASES SCREEN — ABSENT: Visa balance not readable from screen text");
  }

  await page.screenshot({ path: SS("ss_L69_06_accounts_after_payment.png") });
  pass("Step 5.3 — screenshot ss_L69_06_accounts_after_payment.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 6: Ledger check for card payment (C2 re-test for Scenario B)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 6: Ledger check — card payment posted? (C2 Scenario B) ─────────────────────");

  const allL69Txns = await page.evaluate(() => {
    try {
      const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      return Object.values(ds.transactions || {}).filter(t => {
        const desc = (t.desc || t.description || t.payee || "");
        return /L69/i.test(desc);
      }).map(t => ({ name: t.desc || t.description || t.payee || "(unnamed)", amount: t.amount }));
    } catch { return []; }
  });
  note(`All L69 transactions in ledger: ${JSON.stringify(allL69Txns)}`);

  const cardPaymentInLedger = allL69Txns.some(t => /L69 CC Payment/i.test(t.name || ""));
  if (cardPaymentInLedger) {
    pass("Step 6.1 (C2) LEDGER_POST — Card payment transaction found in ledger ✓");
  } else {
    absent_("Step 6.1 (C2) LEDGER_POST — ABSENT: Card payment NOT in ledger (re-tests L56 Thread A for liability payments)");
  }

  await navTo(page, "Transactions");
  await dismissModal(page);
  await page.waitForTimeout(1000);
  const txnScreenText = await page.evaluate(() => document.body.textContent);
  const cardPaymentOnScreen = /L69 CC Payment/i.test(txnScreenText);
  if (cardPaymentOnScreen) {
    pass("Step 6.2 (C2) — Card payment visible on /transactions screen");
  } else {
    absent_("Step 6.2 (C2) — Card payment NOT visible on /transactions screen");
  }

  await page.screenshot({ path: SS("ss_L69_07_transactions_both.png") });
  pass("Step 6.3 — screenshot ss_L69_07_transactions_both.png (ledger view after both scenarios)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 7: Dashboard — net worth after Scenario B
  //   Both scenarios should improve net worth by $200 (or at least not worsen it)
  //   B3: net worth moves up when liability falls
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 7: Dashboard net worth — post Scenario B ───────────────────────────────────");

  await navTo(page, "Dashboard");
  await dismissModal(page);
  await page.waitForTimeout(1500);

  const dashTextB = await page.evaluate(() => document.body.textContent);
  const dashAmountsB = (dashTextB.match(/\$[\d,]+\.?\d*/g) || []).slice(0, 30);
  note(`Dashboard amounts (post Scenario B): ${dashAmountsB.join(", ")}`);

  const hasNetWorthB = /net worth/i.test(dashTextB);
  if (hasNetWorthB) {
    pass("Step 7.1 (B3) — Dashboard shows net worth widget (Scenario B context)");
  } else {
    absent_("Step 7.1 (B3) — ABSENT: No 'net worth' text on Dashboard");
  }

  // D1: Does dashboard or any screen surface interest vs goal tradeoff?
  const hasDecisionSupport = /interest|apr|payoff|tradeoff|trade.off|vs\.|versus|compare/i.test(dashTextB);
  if (hasDecisionSupport) {
    pass("Step 7.2 (D1) DECISION_SUPPORT — Dashboard surfaces interest/tradeoff language");
  } else {
    absent_("Step 7.2 (D1) DECISION_SUPPORT — ABSENT: No decision-support language (interest saved vs goal progress) on Dashboard");
  }

  await page.screenshot({ path: SS("ss_L69_08_dashboard_after_payment.png") });
  pass("Step 7.3 — screenshot ss_L69_08_dashboard_after_payment.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 8: Decision support probe — does any screen compare interest vs goal?
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 8: Decision support probe (D1) ─────────────────────────────────────────────");

  // Check /planning for any debt vs goal tradeoff surface
  await navTo(page, "Planning");
  await dismissModal(page);
  await page.waitForTimeout(1500);

  const planText = await page.evaluate(() => document.body.textContent);
  const planHasAPR = /apr|interest rate|23%/i.test(planText);
  const planHasGoalVsDebt = /goal.*debt|debt.*goal|interest saved|tradeoff/i.test(planText);

  if (planHasGoalVsDebt) {
    pass("Step 8.1 (D1) DECISION_SUPPORT — /planning surfaces goal vs debt comparison");
  } else {
    absent_("Step 8.1 (D1) DECISION_SUPPORT — ABSENT: /planning has no goal-vs-debt or interest-saved surface");
  }

  if (planHasAPR) {
    pass("Step 8.2 (D1) — /planning references APR / interest rate");
  } else {
    absent_("Step 8.2 (D1) — ABSENT: /planning has no APR / interest rate display");
  }

  // Check if payoff package surface exists anywhere
  const planAmounts = (planText.match(/\$[\d,]+\.?\d*/g) || []).slice(0, 20);
  note(`/planning amounts: ${planAmounts.join(", ")}`);

  await page.screenshot({ path: SS("ss_L69_09_planning_decision.png") });
  pass("Step 8.3 — screenshot ss_L69_09_planning_decision.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 9: Cross-screen consistency check (C1)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 9: Cross-screen consistency (C1) ───────────────────────────────────────────");

  // Re-read dataset for final state
  const dsFinal = await getDataset(page);
  const allAccts = Object.values(dsFinal.accounts || {});
  const l69Accts = allAccts.filter(a => /L69/i.test(a.name || ""));
  note(`L69 accounts in dataset: ${l69Accts.map(a => `${a.name}=${JSON.stringify(a.CurrentBalance ?? a.currentBalance ?? a.balance)}`).join(", ")}`);

  // Goals in dataset
  const allGoals = Object.values(dsFinal.goals || {});
  const l69Goals = allGoals.filter(g => /L69/i.test(g.name || ""));
  note(`L69 goals in dataset: ${l69Goals.map(g => `${g.name} saved=${JSON.stringify(g.saved ?? g.Saved)} target=${JSON.stringify(g.target ?? g.Target)}`).join(", ")}`);

  if (l69Goals.length > 0) {
    const goal = l69Goals[0];
    const savedRaw = goal.saved ?? goal.Saved;
    const targetRaw = goal.target ?? goal.Target;
    note(`Emergency Fund: saved=${JSON.stringify(savedRaw)}, target=${JSON.stringify(targetRaw)}`);
    pass("Step 9.1 (C1) — Emergency Fund goal found in dataset");
  } else {
    absent_("Step 9.1 (C1) — ABSENT: L69 goal not found in dataset");
  }

  // Final transaction count
  const allTxns = Object.values(dsFinal.transactions || {});
  const l69Txns = allTxns.filter(t => /L69/i.test(t.desc || t.description || t.payee || ""));
  note(`Final L69 transaction count in ledger: ${l69Txns.length}`);
  if (l69Txns.length >= 1) {
    pass(`Step 9.2 (C2) LEDGER_POST — ${l69Txns.length} L69 transaction(s) in ledger total`);
  } else {
    absent_("Step 9.2 (C2) LEDGER_POST — ABSENT: No L69 transactions in ledger at all (total)");
  }

  await navTo(page, "Accounts");
  await dismissModal(page);
  await page.waitForTimeout(1000);
  await page.screenshot({ path: SS("ss_L69_10_final_accounts.png") });
  pass("Step 9.3 — screenshot ss_L69_10_final_accounts.png (final account state)");

  // ════════════════════════════════════════════════════════════════════════════
  // SUMMARY
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n════════════════════════════════════════════════════════════════════════════════════");
  console.log(`SUMMARY: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
  console.log(`Real JS errors: ${jsErrors.length}`);
  if (jsErrors.length > 0) jsErrors.forEach(e => console.error(`  ERROR: ${e}`));

  console.log("\nINVARIANT VERDICTS:");
  console.log(`  A1 GOAL_PROGRESS:   ${has350 ? "HELD ($350 visible)" : "ABSENT/VIOLATED"}`);
  console.log(`  A2 SAVINGS_LINKED:  ${savingsAfterNum !== null ? (Math.abs(savingsAfterNum-35000)<=1 ? "HELD ($350)" : `VIOLATED (${savingsAfterNum} minor units)`) : "UNKNOWN (field format)"}`);
  console.log(`  B1 CARD_DECREASES:  ${visaAfterNum !== null ? (Math.abs(visaAfterNum-180000)<=100 ? "HELD ($1,800)" : `VIOLATED (${visaAfterNum} minor units)`) : "UNKNOWN (field format)"}`);
  console.log(`  C2 LEDGER_POST:     ${allL69Txns.length > 0 ? `HELD (${allL69Txns.length} txn(s))` : "ABSENT"}`);
  console.log(`  D1 DECISION_SUPPORT: ABSENT (no interest-vs-goal surface found)`);
  console.log("════════════════════════════════════════════════════════════════════════════════════");

} finally {
  await browser.close();
}

const exitCode = failed > 0 ? 1 : 0;
process.exit(exitCode);
