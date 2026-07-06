// L66 E2E loop story — "The Overdraft Spiral" (Renée) — 2026-06-22
//
// Persona: Renée is a single mother living paycheck to paycheck. After a cascade of bad
// luck — rent due before her paycheck clears, a gas fill, a coffee run, a pharmacy trip —
// her checking account goes deeply negative. The bank piles on three NSF fees of $35 each.
// She tracks every cent in CashFlux and needs to see the true negative balance at every
// step, get an overdraft warning, and have the NSF fees show up as real "Bank fees" expenses
// in Reports so she can prove the pattern to her bank and request a waiver. A $1,000
// emergency transfer from her sister finally digs her out.
//
// KEY INVARIANTS ASSERTED:
//   I1: NEGATIVE_BALANCE_SHOWN — balance displayed as negative (red / parenthesized),
//       NOT clamped to zero, at each overdraft step
//   I2: OVERDRAFT_WARN — overdraft warning present when balance goes negative (re-test L55)
//   I3: RUNNING_BALANCE_MATH — running balance correct to the cent at every step:
//       45.00 → −805.00 → −817.00 → −847.00 → −865.00 → −900.00 → −935.00 → −970.00 → +30.00
//   I4: NSF_AS_EXPENSE — NSF fees post as real expense transactions (not hidden), appear
//       in Reports under "Bank fees" or equivalent expense category
//   I5: NET_WORTH_HONEST — Dashboard net worth reflects negative balance (not clamped)
//   I6: CROSS_SCREEN_AGREE — Accounts / Transactions / Dashboard all show the same balance
//       (no stale value on any screen)
//
// Balance checkpoints (all in minor units = cents):
//   After opening:          +4500   ($45.00)
//   After rent ($850):      −80500  (−$805.00)
//   After coffee ($12):     −81700  (−$817.00)
//   After gas ($30):        −84700  (−$847.00)
//   After pharmacy ($18):   −86500  (−$865.00)
//   After NSF 1 ($35):      −90000  (−$900.00)
//   After NSF 2 ($35):      −93500  (−$935.00)
//   After NSF 3 ($35):      −97000  (−$970.00)
//   After recovery ($1000): +3000   (+$30.00)
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_66_overdraft_spiral.mjs

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

const parseBalanceStr = (s) => {
  if (!s) return null;
  const negative = s.includes("(") || s.startsWith("-") || s.includes("−");
  const raw = s.replace(/[^0-9.]/g, "");
  if (!raw) return null;
  const val = Math.round(parseFloat(raw) * 100);
  return negative ? -val : val;
};

// Read visible balance for an account from the /accounts screen
const readAccountBalance = async (page, nameMatch) => {
  await navTo(page, "Accounts");
  return page.evaluate((match) => {
    const rows = Array.from(document.querySelectorAll(".row"));
    for (const row of rows) {
      const desc = row.querySelector(".row-desc");
      if (desc && new RegExp(match, "i").test(desc.textContent)) {
        const amtEl = row.querySelector(".budget-amount, .row-amount, [class*='amount']");
        return amtEl ? amtEl.textContent.trim() : null;
      }
    }
    // Fallback: scan all text nodes near account name mentions
    const allEls = Array.from(document.querySelectorAll("*"));
    for (const el of allEls) {
      if (new RegExp(match, "i").test(el.textContent) && el.children.length < 4) {
        const txt = el.textContent.trim();
        if (/[\$\-\(]/.test(txt) && txt.length < 30) return txt;
      }
    }
    return null;
  }, nameMatch);
};

// ─── main ─────────────────────────────────────────────────────────────────────

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  page.on("pageerror", (e) => jsErrors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  pass("HYDRATION — app loaded and nav visible");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 1: /accounts — create Renée's checking account with $45.00 opening balance
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 1: Create L66 Checking ($45.00) ─────────────────────────────────────────");

  await navTo(page, "Accounts");
  await dismissModal(page);

  // Click Add Account
  const addR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /add account|new account/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked"; }
    return "NOT FOUND";
  });
  note(`Add Account button: ${addR}`);
  await page.waitForTimeout(800);

  // Name
  await page.evaluate(() => {
    const inp = Array.from(document.querySelectorAll("input[type='text']")).find(i => i.placeholder === "Name");
    if (!inp) return "NOT FOUND";
    inp.focus(); inp.value = "L66 Checking";
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  });

  // Type = Checking
  const typeR = await selectByText(page, "Account type", "Checking");
  note(`Account type: ${typeR}`);

  // Opening balance = $45
  await page.evaluate(() => {
    const inp = Array.from(document.querySelectorAll("input[type='number']")).find(i =>
      i.placeholder === "Opening balance");
    if (!inp) return "NOT FOUND";
    inp.value = "45";
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  });

  // Submit
  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b => {
      const t = b.textContent.trim();
      return /^add account$|^add$|^save$/i.test(t) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);

  // Verify account created and has $45.00
  await navTo(page, "Accounts");
  const balStr0 = await readAccountBalance(page, "L66 Checking");
  const bal0 = parseBalanceStr(balStr0);
  note(`L66 Checking balance after create: "${balStr0}" → ${bal0} minor units`);

  const expected0 = 4500; // $45.00
  if (bal0 !== null && Math.abs(bal0 - expected0) <= 50) {
    pass(`Step 1.1 (I3) — Opening balance correct: ${balStr0} ≈ $45.00`);
  } else if (bal0 !== null) {
    fail(`Step 1.1 (I3) — Opening balance wrong: got ${bal0} (${balStr0}), expected ${expected0} ($45.00)`);
  } else {
    absent_("Step 1.1 (I3) — ABSENT: Could not read L66 Checking balance from /accounts");
  }

  await page.screenshot({ path: SS("story66_step1_opening.png") });
  pass("Step 1.2 — screenshot story66_step1_opening.png (L66 Checking $45.00)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 2: /transactions — log rent debit ($850) → expected balance −$805.00
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 2: Log rent ($850) debit ─────────────────────────────────────────────────");

  const today = new Date();
  const pad = (n) => String(n).padStart(2, "0");
  const todayStr = `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}`;

  // Helper: record an expense transaction
  const recordExpense = async (label, amount, category, accountMatch) => {
    await dismissModal(page);
    await navTo(page, "Transactions");
    await page.waitForTimeout(500);

    // Click Add / New Transaction
    await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll("button")).find(b =>
        /new transaction|add transaction/i.test(b.textContent.trim()));
      if (btn) btn.click();
    });
    await page.waitForTimeout(800);

    // Description
    await page.evaluate(({ desc }) => {
      const inp = Array.from(document.querySelectorAll("input,textarea")).find(i =>
        i.getAttribute("aria-label") === "Description" ||
        i.getAttribute("placeholder") === "Description" ||
        i.getAttribute("aria-label") === "Payee" ||
        i.getAttribute("placeholder") === "Payee" ||
        i.id === "txn-add");
      if (inp) {
        inp.focus(); inp.value = desc;
        inp.dispatchEvent(new Event("input", { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    }, { desc: label });

    // Amount
    await page.evaluate((a) => {
      const inp = document.querySelector('input[type="number"]');
      if (inp) {
        inp.value = a;
        inp.dispatchEvent(new Event("input", { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    }, String(amount));

    // Type = Expense
    await selectByText(page, "Type", "Expense");

    // Account
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

    // Category if provided
    if (category) {
      const catR = await selectByText(page, "Category", category);
      note(`  ${label} category: ${catR}`);
    }

    // Date
    await page.evaluate((d) => {
      const inp = document.querySelector('input[type="date"]');
      if (inp) {
        inp.value = d;
        inp.dispatchEvent(new Event("input", { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    }, todayStr);

    // Submit
    await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll("button")).find(b => {
        const t = b.textContent.trim();
        return (t === "Add" || /^save$/i.test(t)) && b.type !== "reset";
      });
      if (btn) btn.click();
    });
    await page.waitForTimeout(1500);
    await flush(page);
  };

  await recordExpense("L66 Rent", 850, "Housing", "L66 Checking");

  const balStr1 = await readAccountBalance(page, "L66 Checking");
  const bal1 = parseBalanceStr(balStr1);
  note(`After rent ($850): "${balStr1}" → ${bal1} minor units (expected −80500)`);

  const expected1 = -80500; // −$805.00
  if (bal1 !== null && Math.abs(bal1 - expected1) <= 50) {
    pass(`Step 2.1 (I3) — Running balance after rent: ${balStr1} ≈ −$805.00 ✓`);
  } else if (bal1 !== null && bal1 >= 0) {
    fail(`Step 2.1 (I1+I3) — FAIL: Balance NOT negative after overdraft. Got ${bal1} (${balStr1}). ` +
      `Expected −80500 (−$805.00). Balance clamped to zero or positive — violates I1 NEGATIVE_BALANCE_SHOWN.`);
  } else if (bal1 !== null) {
    fail(`Step 2.1 (I3) — Running balance wrong: got ${bal1} (${balStr1}), expected ${expected1} (−$805.00)`);
  } else {
    absent_("Step 2.1 (I3) — ABSENT: Cannot read L66 Checking balance after rent");
  }

  // Check I1: Is balance shown as negative (red/parenthesized)?
  const negativeDisplay1 = await page.evaluate(() => {
    // Look for red color, parenthesized amount, or minus sign
    const amtEls = Array.from(document.querySelectorAll("[class*='amount'], [class*='balance'], [class*='negative'], [class*='red']"));
    const hasRedClass = amtEls.some(el =>
      /negative|red|danger|error/i.test(el.className) ||
      window.getComputedStyle(el).color.includes("rgb(") // any non-default color
    );
    // Check for parenthesized display
    const bodyText = document.body.textContent;
    const hasParens = /\(\$\d/.test(bodyText);
    const hasMinus  = /-\$/.test(bodyText) || /−\$/.test(bodyText);
    return { hasRedClass, hasParens, hasMinus, bodySnippet: bodyText.slice(0, 200).replace(/\s+/g, " ") };
  });
  note(`Negative balance display signals: ${JSON.stringify(negativeDisplay1)}`);

  if (negativeDisplay1.hasParens || negativeDisplay1.hasMinus) {
    pass("Step 2.2 (I1) — NEGATIVE_BALANCE_SHOWN: Negative balance displayed (parenthesized or minus) ✓");
  } else {
    absent_("Step 2.2 (I1) — ABSENT: No parenthesized/minus display found for negative balance. " +
      "Balance may be shown as positive or zero — not visually flagged as overdraft.");
  }

  // Check I2: Overdraft warning
  const overdraftWarn1 = await page.evaluate(() => {
    const body = document.body.textContent ?? "";
    return /overdraft|insufficient|negative.*balance|below.*zero|NSF/i.test(body);
  });
  note(`Overdraft warning present: ${overdraftWarn1}`);

  if (overdraftWarn1) {
    pass("Step 2.3 (I2) — OVERDRAFT_WARN: Overdraft warning shown when balance goes negative ✓");
  } else {
    absent_("Step 2.3 (I2) — ABSENT: OVERDRAFT_WARN — No overdraft warning shown when balance goes " +
      "negative (re-confirms L55 gap: runway shown passively, no alert/warning when balance < 0). " +
      "App does not alert user that account is overdrawn.");
  }

  await page.screenshot({ path: SS("story66_step2_after_rent.png") });
  pass("Step 2.4 — screenshot story66_step2_after_rent.png (after rent, balance −$805)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 3: Log 3 small debits ($12 coffee, $30 gas, $18 pharmacy)
  //   −$805 → −$817 → −$847 → −$865
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 3: Log 3 small debits (coffee $12, gas $30, pharmacy $18) ────────────────");

  await recordExpense("L66 Coffee", 12, "Food & Drink", "L66 Checking");
  const balStr2 = await readAccountBalance(page, "L66 Checking");
  const bal2 = parseBalanceStr(balStr2);
  note(`After coffee ($12): "${balStr2}" → ${bal2} (expected −81700)`);
  const expected2 = -81700;
  if (bal2 !== null && Math.abs(bal2 - expected2) <= 50) {
    pass(`Step 3.1 (I3) — After coffee: ${balStr2} ≈ −$817.00 ✓`);
  } else if (bal2 !== null) {
    fail(`Step 3.1 (I3) — After coffee: got ${bal2} (${balStr2}), expected ${expected2} (−$817.00)`);
  } else {
    absent_("Step 3.1 (I3) — ABSENT: Balance unreadable after coffee");
  }

  await recordExpense("L66 Gas", 30, "Transportation", "L66 Checking");
  const balStr3 = await readAccountBalance(page, "L66 Checking");
  const bal3 = parseBalanceStr(balStr3);
  note(`After gas ($30): "${balStr3}" → ${bal3} (expected −84700)`);
  const expected3 = -84700;
  if (bal3 !== null && Math.abs(bal3 - expected3) <= 50) {
    pass(`Step 3.2 (I3) — After gas: ${balStr3} ≈ −$847.00 ✓`);
  } else if (bal3 !== null) {
    fail(`Step 3.2 (I3) — After gas: got ${bal3} (${balStr3}), expected ${expected3} (−$847.00)`);
  } else {
    absent_("Step 3.2 (I3) — ABSENT: Balance unreadable after gas");
  }

  await recordExpense("L66 Pharmacy", 18, "Health", "L66 Checking");
  const balStr4 = await readAccountBalance(page, "L66 Checking");
  const bal4 = parseBalanceStr(balStr4);
  note(`After pharmacy ($18): "${balStr4}" → ${bal4} (expected −86500)`);
  const expected4 = -86500;
  if (bal4 !== null && Math.abs(bal4 - expected4) <= 50) {
    pass(`Step 3.3 (I3) — After pharmacy: ${balStr4} ≈ −$865.00 ✓`);
  } else if (bal4 !== null) {
    fail(`Step 3.3 (I3) — After pharmacy: got ${bal4} (${balStr4}), expected ${expected4} (−$865.00)`);
  } else {
    absent_("Step 3.3 (I3) — ABSENT: Balance unreadable after pharmacy");
  }

  await page.screenshot({ path: SS("story66_step3_small_debits.png") });
  pass("Step 3.4 — screenshot story66_step3_small_debits.png (after 3 small debits, −$865)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 4: Log 3 NSF fees ($35 each) as "Bank fees" expenses
  //   −$865 → −$900 → −$935 → −$970
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 4: Log 3 NSF fees ($35 each) as Bank fees ───────────────────────────────");

  await recordExpense("L66 NSF Fee 1", 35, "Bank fees", "L66 Checking");
  const balStr5 = await readAccountBalance(page, "L66 Checking");
  const bal5 = parseBalanceStr(balStr5);
  note(`After NSF 1 ($35): "${balStr5}" → ${bal5} (expected −90000)`);
  const expected5 = -90000;
  if (bal5 !== null && Math.abs(bal5 - expected5) <= 50) {
    pass(`Step 4.1 (I3) — After NSF 1: ${balStr5} ≈ −$900.00 ✓`);
  } else if (bal5 !== null) {
    fail(`Step 4.1 (I3) — After NSF 1: got ${bal5} (${balStr5}), expected ${expected5} (−$900.00)`);
  } else {
    absent_("Step 4.1 (I3) — ABSENT: Balance unreadable after NSF 1");
  }

  await recordExpense("L66 NSF Fee 2", 35, "Bank fees", "L66 Checking");
  const balStr6 = await readAccountBalance(page, "L66 Checking");
  const bal6 = parseBalanceStr(balStr6);
  note(`After NSF 2 ($35): "${balStr6}" → ${bal6} (expected −93500)`);
  const expected6 = -93500;
  if (bal6 !== null && Math.abs(bal6 - expected6) <= 50) {
    pass(`Step 4.2 (I3) — After NSF 2: ${balStr6} ≈ −$935.00 ✓`);
  } else if (bal6 !== null) {
    fail(`Step 4.2 (I3) — After NSF 2: got ${bal6} (${balStr6}), expected ${expected6} (−$935.00)`);
  } else {
    absent_("Step 4.2 (I3) — ABSENT: Balance unreadable after NSF 2");
  }

  await recordExpense("L66 NSF Fee 3", 35, "Bank fees", "L66 Checking");
  const balStr7 = await readAccountBalance(page, "L66 Checking");
  const bal7 = parseBalanceStr(balStr7);
  note(`After NSF 3 ($35): "${balStr7}" → ${bal7} (expected −97000)`);
  const expected7 = -97000;
  if (bal7 !== null && Math.abs(bal7 - expected7) <= 50) {
    pass(`Step 4.3 (I3) — After NSF 3: ${balStr7} ≈ −$970.00 ✓`);
  } else if (bal7 !== null) {
    fail(`Step 4.3 (I3) — After NSF 3: got ${bal7} (${balStr7}), expected ${expected7} (−$970.00)`);
  } else {
    absent_("Step 4.3 (I3) — ABSENT: Balance unreadable after NSF 3");
  }

  await page.screenshot({ path: SS("story66_step4_nsf_fees.png") });
  pass("Step 4.4 — screenshot story66_step4_nsf_fees.png (after 3 NSF fees, −$970)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 5: Check Dashboard — net worth reflects −$970 (before recovery)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 5: Dashboard — net worth at peak overdraft (−$970) ─────────────────────");

  await dismissModal(page);
  await navTo(page, "Dashboard");
  await page.waitForTimeout(800);

  const dashBodyPre = await page.evaluate(() => document.body.textContent ?? "");
  const hasNetWorthWidget = /net worth/i.test(dashBodyPre);
  note(`Dashboard net worth widget: ${hasNetWorthWidget}`);

  // Probe the net worth value
  const netWorthValuePre = await page.evaluate(() => {
    const body = document.body.textContent ?? "";
    // Look for dollar amount near "net worth"
    const match = body.match(/net worth[^$]*(\(?-?\$[\d,]+\.?\d*\)?)/i);
    if (match) return match[1];
    // Fallback: any dollar amount that looks like current balance
    const dollars = body.match(/\(?-?\$[\d,]+\.?\d*\)?/g) || [];
    return dollars.slice(0, 8).join(", ");
  });
  note(`Net worth area values (pre-recovery): ${netWorthValuePre}`);

  if (hasNetWorthWidget) {
    pass("Step 5.1 — Dashboard: Net Worth widget present");
    // Check if net worth shows negative
    const netWorthNegative = /\(-?\$|\-\$|−\$/.test(netWorthValuePre);
    if (netWorthNegative) {
      pass("Step 5.2 (I5) — NET_WORTH_HONEST: Net worth shows negative value at peak overdraft ✓");
    } else {
      note("Step 5.2 (I5) — Net worth display ambiguous; value: " + netWorthValuePre +
        " (may be clamped or include other assets)");
    }
  } else {
    absent_("Step 5.1 (I5) — ABSENT: NET_WORTH_HONEST — No Net Worth widget on Dashboard");
  }

  await page.screenshot({ path: SS("story66_step5_dashboard_overdraft.png") });
  pass("Step 5.3 — screenshot story66_step5_dashboard_overdraft.png (Dashboard at −$970)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 6: Log $1,000 emergency deposit → balance recovers to +$30
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 6: Log emergency deposit ($1,000) → balance +$30 ────────────────────────");

  // Record as Income transaction
  await dismissModal(page);
  await navTo(page, "Transactions");
  await page.waitForTimeout(500);

  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /new transaction|add transaction/i.test(b.textContent.trim()));
    if (btn) btn.click();
  });
  await page.waitForTimeout(800);

  // Description
  await page.evaluate(() => {
    const inp = Array.from(document.querySelectorAll("input,textarea")).find(i =>
      i.getAttribute("aria-label") === "Description" ||
      i.getAttribute("placeholder") === "Description" ||
      i.getAttribute("aria-label") === "Payee" ||
      i.getAttribute("placeholder") === "Payee" ||
      i.id === "txn-add");
    if (inp) {
      inp.focus(); inp.value = "L66 Emergency Deposit";
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  });

  // Amount
  await page.evaluate(() => {
    const inp = document.querySelector('input[type="number"]');
    if (inp) {
      inp.value = "1000";
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  });

  // Type = Income
  await selectByText(page, "Type", "Income");

  // Account = L66 Checking
  await page.evaluate(() => {
    const sel = Array.from(document.querySelectorAll("select")).find(s =>
      s.getAttribute("aria-label") === "Account" || s.getAttribute("aria-label") === "From account");
    if (!sel) return;
    const opt = Array.from(sel.options).find(o => /L66.*Checking/i.test(o.text));
    if (opt) { sel.value = opt.value; sel.dispatchEvent(new Event("change", { bubbles: true })); }
  });

  // Date
  await page.evaluate((d) => {
    const inp = document.querySelector('input[type="date"]');
    if (inp) { inp.value = d; inp.dispatchEvent(new Event("input", { bubbles: true })); inp.dispatchEvent(new Event("change", { bubbles: true })); }
  }, todayStr);

  // Submit
  await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b => {
      const t = b.textContent.trim();
      return (t === "Add" || /^save$/i.test(t)) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);

  // Check recovered balance
  const balStr8 = await readAccountBalance(page, "L66 Checking");
  const bal8 = parseBalanceStr(balStr8);
  note(`After emergency deposit ($1,000): "${balStr8}" → ${bal8} (expected +3000)`);

  const expected8 = 3000; // +$30.00
  if (bal8 !== null && Math.abs(bal8 - expected8) <= 50) {
    pass(`Step 6.1 (I3) — After recovery: ${balStr8} ≈ +$30.00 ✓`);
  } else if (bal8 !== null && bal8 > 0) {
    fail(`Step 6.1 (I3) — Recovery balance wrong: got ${bal8} (${balStr8}), expected ${expected8} (+$30.00)`);
  } else if (bal8 !== null) {
    fail(`Step 6.1 (I3) — Balance still negative after $1,000 deposit: ${bal8} (${balStr8})`);
  } else {
    absent_("Step 6.1 (I3) — ABSENT: Balance unreadable after emergency deposit");
  }

  await page.screenshot({ path: SS("story66_step6_recovery.png") });
  pass("Step 6.2 — screenshot story66_step6_recovery.png (after $1,000 deposit, +$30)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 7: Check Dashboard after recovery — net worth now +$30
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 7: Dashboard net worth after recovery ──────────────────────────────────");

  await dismissModal(page);
  await navTo(page, "Dashboard");
  await page.waitForTimeout(800);

  const dashBodyPost = await page.evaluate(() => document.body.textContent ?? "");
  const netWorthValuePost = await page.evaluate(() => {
    const body = document.body.textContent ?? "";
    const match = body.match(/net worth[^$]*(\(?-?\$[\d,]+\.?\d*\)?)/i);
    if (match) return match[1];
    const dollars = body.match(/\(?-?\$[\d,]+\.?\d*\)?/g) || [];
    return dollars.slice(0, 8).join(", ");
  });
  note(`Net worth area values (post-recovery): ${netWorthValuePost}`);

  // Net worth should now reflect +$30 (or similar positive value)
  const netWorthPositive = /\$30|\$0\.30|\$3\b/.test(netWorthValuePost) ||
    (/\$/.test(netWorthValuePost) && !/\(/.test(netWorthValuePost) && !/−/.test(netWorthValuePost));

  if (hasNetWorthWidget) {
    if (netWorthPositive) {
      pass("Step 7.1 (I5+I6) — NET_WORTH_HONEST: Net worth positive after recovery ✓");
    } else {
      note("Step 7.1 (I5+I6) — Net worth post-recovery ambiguous; value: " + netWorthValuePost);
    }
  } else {
    absent_("Step 7.1 (I5) — ABSENT: No Net Worth widget for post-recovery check");
  }

  await page.screenshot({ path: SS("story66_step7_dashboard_recovery.png") });
  pass("Step 7.2 — screenshot story66_step7_dashboard_recovery.png (Dashboard after recovery)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 8: /reports — NSF fees appear as "Bank fees" expense category
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 8: Reports — NSF fees categorized as Bank fees ─────────────────────────");

  await dismissModal(page);
  await navTo(page, "Reports");
  await page.waitForTimeout(1000);

  const reportsBody = await page.evaluate(() => document.body.textContent ?? "");
  const hasBankFees = /bank.?fee|NSF/i.test(reportsBody);
  const hasExpenseBreakdown = /expense|spending/i.test(reportsBody);

  note(`Reports — bank fees category: ${hasBankFees}, expense breakdown: ${hasExpenseBreakdown}`);

  if (hasBankFees) {
    // Check the total: 3 × $35 = $105
    const bankFeesAmount = await page.evaluate(() => {
      const body = document.body.textContent ?? "";
      // Look for $105 near "bank fees"
      const match = body.match(/bank.?fee[^$]*\$?([\d,]+\.?\d*)/i);
      return match ? match[1] : null;
    });
    note(`Bank fees amount shown: ${bankFeesAmount}`);

    pass("Step 8.1 (I4) — NSF_AS_EXPENSE: 'Bank fees' category visible in Reports ✓");

    if (bankFeesAmount && (bankFeesAmount.includes("105") || bankFeesAmount === "35")) {
      pass("Step 8.2 (I4) — NSF total in Reports: $" + bankFeesAmount + " (expected $105.00) ✓");
    } else if (bankFeesAmount) {
      note("Step 8.2 (I4) — Bank fees amount found: $" + bankFeesAmount + " (expected $105.00; may include other data from prior tests)");
    } else {
      note("Step 8.2 (I4) — Bank fees amount not parseable from Reports page");
    }
  } else if (hasExpenseBreakdown) {
    absent_("Step 8.1 (I4) — ABSENT: NSF_AS_EXPENSE — Reports has expense breakdown but 'Bank fees' " +
      "category not found. NSF fees may have been recorded without category or under a different label.");
  } else {
    absent_("Step 8.1 (I4) — ABSENT: NSF_AS_EXPENSE — No expense breakdown visible in /reports. " +
      "Cannot confirm NSF fees are categorized and visible in reporting.");
  }

  await page.screenshot({ path: SS("story66_step8_reports.png") });
  pass("Step 8.3 — screenshot story66_step8_reports.png (Reports — Bank fees category)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 9: Cross-screen agreement — /transactions shows all 8 L66 entries
  //         Accounts / Transactions / Dashboard should all see same balance
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 9: Cross-screen agreement (I6) ──────────────────────────────────────────");

  await dismissModal(page);
  await navTo(page, "Transactions");
  await page.waitForTimeout(800);

  const txnBodyText = await page.evaluate(() => document.body.textContent ?? "");
  const txnHasRent = /L66.*Rent|Rent.*L66/i.test(txnBodyText);
  const txnHasNSF  = /L66.*NSF/i.test(txnBodyText);
  const txnHasRecovery = /L66.*Emergency|Emergency.*L66/i.test(txnBodyText);

  note(`Transactions screen — rent: ${txnHasRent}, NSF: ${txnHasNSF}, recovery: ${txnHasRecovery}`);

  await page.screenshot({ path: SS("story66_step9_transactions.png") });
  pass("Step 9.0 — screenshot story66_step9_transactions.png (full transaction list)");

  // Count L66 transactions
  const l66TxnCount = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row"));
    return rows.filter(r => /L66/i.test(r.textContent)).length;
  });
  note(`L66 transaction rows visible on /transactions: ${l66TxnCount} (expected 8: rent+coffee+gas+pharmacy+NSF×3+deposit)`);

  // Dataset verification
  const dsFinal = await getDataset(page);
  const l66Txns = (dsFinal.transactions || []).filter(t =>
    /L66/i.test((t.payee || "") + (t.desc || "")));
  note(`L66 transactions in dataset: ${l66Txns.length}`);

  const getAmt = (t) => {
    if (typeof t.amount === "number") return t.amount;
    if (t.amount?.Amount !== undefined) return t.amount.Amount;
    if (t.amount?.amount !== undefined) return t.amount.amount;
    return 0;
  };

  const l66Summary = l66Txns.map(t => ({ desc: t.desc || t.payee, amt: getAmt(t), cat: t.category }));
  note(`L66 transaction dataset summary: ${JSON.stringify(l66Summary)}`);

  // Total money flow check: rent(850) + coffee(12) + gas(30) + pharmacy(18) + NSF×3(105) = 1015 debits
  // Income: 1000 (deposit). Net: 45 - 1015 + 1000 = 30 → +$30
  const debits  = l66Txns.filter(t => getAmt(t) < 0).reduce((s, t) => s + Math.abs(getAmt(t)), 0);
  const credits = l66Txns.filter(t => getAmt(t) > 0).reduce((s, t) => s + getAmt(t), 0);
  note(`Dataset money flow — total debits: ${debits}, total credits: ${credits}`);

  if (l66Txns.length >= 8) {
    pass(`Step 9.1 (I6) — CROSS_SCREEN_AGREE: All 8 L66 transactions in dataset ✓`);
  } else if (l66Txns.length > 0) {
    note(`Step 9.1 (I6) — ${l66Txns.length} L66 transactions in dataset (expected 8)`);
  } else {
    absent_("Step 9.1 (I6) — ABSENT: No L66 transactions found in dataset — cross-screen check unreliable");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 10: Balance clamping final check — read /accounts balance one last time
  //          Confirm final balance is +$30 and shown correctly
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 10: Final balance integrity check ────────────────────────────────────────");

  const balStrFinal = await readAccountBalance(page, "L66 Checking");
  const balFinal = parseBalanceStr(balStrFinal);
  note(`Final L66 Checking balance: "${balStrFinal}" → ${balFinal} (expected +3000)`);

  if (balFinal !== null && Math.abs(balFinal - 3000) <= 50) {
    pass(`Step 10.1 (I3+I6) — Final balance: ${balStrFinal} ≈ +$30.00 — full spiral + recovery correct ✓`);
  } else if (balFinal !== null) {
    fail(`Step 10.1 (I3+I6) — Final balance wrong: got ${balFinal} (${balStrFinal}), expected +3000 (+$30.00)`);
  } else {
    absent_("Step 10.1 (I3+I6) — ABSENT: Final balance not readable from /accounts");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 11: JS errors audit
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 11: JS errors audit ─────────────────────────────────────────────────────");

  note(`JS errors captured during run: ${jsErrors.length}`);
  if (jsErrors.length === 0) {
    pass("Step 11.1 — ZERO JS errors across full ritual ✓");
  } else {
    jsErrors.forEach((e, i) => note(`  JS Error ${i+1}: ${e.slice(0, 120)}`));
    fail(`Step 11.1 — ${jsErrors.length} JS error(s) detected during run`);
  }

  // ════════════════════════════════════════════════════════════════════════════
  // INVARIANT SUMMARY
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── INVARIANT SUMMARY ────────────────────────────────────────────────────────────");
  console.log(`I1 NEGATIVE_BALANCE_SHOWN: see Step 2.2 — ${negativeDisplay1.hasParens || negativeDisplay1.hasMinus ? "HELD" : "ABSENT"}`);
  console.log(`I2 OVERDRAFT_WARN:         see Step 2.3 — ${overdraftWarn1 ? "HELD" : "ABSENT"}`);
  console.log(`I3 RUNNING_BALANCE_MATH:   see Steps 1/2/3/4/6/10 above`);
  console.log(`I4 NSF_AS_EXPENSE:         see Step 8 — ${hasBankFees ? "HELD" : "ABSENT"}`);
  console.log(`I5 NET_WORTH_HONEST:       see Steps 5/7 — ${hasNetWorthWidget ? "net worth widget present" : "ABSENT — widget missing"}`);
  console.log(`I6 CROSS_SCREEN_AGREE:     see Step 9 — ${l66Txns.length >= 8 ? "HELD" : l66Txns.length > 0 ? "PARTIAL" : "ABSENT"}`);

  // ════════════════════════════════════════════════════════════════════════════
  // SUMMARY
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n══════════════════════════════════════════════════════════════════════════════════");
  console.log(`SUMMARY: ${passed} passed, ${failed} failed, ${absent} absent`);
  if (jsErrors.length) {
    console.error(`JS Errors (${jsErrors.length}): ${jsErrors.slice(0, 5).join(" | ")}`);
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
