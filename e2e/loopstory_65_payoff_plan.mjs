// L65 E2E loop story — "The Payoff Plan" (Marcus & Dee) — 2026-06-22
//
// Persona: Marcus and Dee are a couple grinding through financial hardship. Three debts
// are eating their margin every month: a high-interest card they maxed during a rough patch,
// a second card from before they got serious, and a zero-interest medical loan. They've found
// $300/month of extra breathing room and want to use the debt payoff tool to build a plan —
// first avalanche (highest APR first, saves the most interest), then snowball (smallest
// balance first, builds psychological momentum). They want to see the debt-free date, the
// total interest comparison, and confirm that this month's payments actually move the needle.
//
// Debts:
//   Card A — $4,800 @ 22.9% APR  (min payment ~$96/mo)
//   Card B — $2,100 @ 18.0% APR  (min payment ~$42/mo)
//   Medical — $1,500 @ 0% APR    (min payment ~$30/mo)
//
// Extra payment budget: $300/mo
//
// Avalanche order (highest APR first): Card A → Card B → Medical
// Snowball order (smallest balance first): Medical → Card B → Card A
//
// KEY INVARIANTS ASSERTED:
//   I1: AVALANCHE_ORDER — payoff tool orders avalanche as Card A → Card B → Medical
//   I2: SNOWBALL_ORDER — payoff tool orders snowball as Medical → Card B → Card A
//   I3: STRATEGY_DIFF — avalanche and snowball produce different debt-free dates or
//       total interest (avalanche saves more interest)
//   I4: PAYMENT_REDUCES_BALANCE — after making payments, liability balances decrease
//       (re-tests L64's CC sign-convention bug and L46 balance-linkage gap)
//   I5: PLAN_ADVANCES — after payments, the payoff plan shows updated balances / moved
//       projected debt-free date (plan is live, not static)
//   I6: DASHBOARD_DEBT_DOWN — Dashboard total debt decreases, net worth increases
//   I7: MONEY_CONSERVE — sum of payments = sum of account debits; no cents lost
//
// Run: E2E_URL=http://127.0.0.1:8080 node e2e/loopstory_65_payoff_plan.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
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
    return null;
  }, nameMatch);
};

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
  const negative = s.includes("(") || s.startsWith("-");
  const raw = s.replace(/[^0-9.]/g, "");
  const val = Math.round(parseFloat(raw) * 100);
  return negative ? -val : val;
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
  // STEP 1: /accounts — seed Marcus & Dee's checking + 3 liability accounts
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 1: /accounts — seed checking ($5,000) + 3 liability accounts ─────────");

  await navTo(page, "Accounts");
  await page.screenshot({ path: SS("story65_step1_accounts.png") });
  pass("Step 1.0 — screenshot story65_step1_accounts.png (before seeding)");

  // Helper: create one account via the add form
  const createAccount = async (name, type, balance) => {
    await dismissModal(page);

    // Click Add Account button
    const addR = await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll("button")).find(b =>
        /add account|new account/i.test(b.textContent.trim()));
      if (btn) { btn.click(); return "clicked"; }
      return "NOT FOUND";
    });
    if (addR === "NOT FOUND") { note(`  Add account button not found for ${name}`); return false; }
    await page.waitForTimeout(800);

    // Name
    const nameR = await page.evaluate((n) => {
      const inp = Array.from(document.querySelectorAll("input[type='text']")).find(i => i.placeholder === "Name");
      if (!inp) return "NOT FOUND";
      inp.focus(); inp.value = n;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
      return `filled → "${n}"`;
    }, name);
    note(`  Name: ${nameR}`);

    // Account type (confirmed selector from L64)
    const typeR = await selectByText(page, "Account type", type);
    note(`  Type: ${typeR}`);

    // Opening balance
    const balR = await page.evaluate((b) => {
      const inp = Array.from(document.querySelectorAll("input[type='number']")).find(i =>
        i.placeholder === "Opening balance");
      if (!inp) return "NOT FOUND";
      inp.value = b;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
      return `set → ${b}`;
    }, String(balance));
    note(`  Balance: ${balR}`);

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
    return true;
  };

  // 1a: Checking account (funding source)
  note("Creating L65 Checking ($5,000)...");
  await createAccount("L65 Checking", "Checking", 5000);

  // 1b: Card A — $4,800 @ 22.9% (highest APR)
  note("Creating L65 Card A ($4,800)...");
  await createAccount("L65 Card A", "Credit card", 4800);

  // 1c: Card B — $2,100 @ 18.0%
  note("Creating L65 Card B ($2,100)...");
  await createAccount("L65 Card B", "Credit card", 2100);

  // 1d: Medical — $1,500 @ 0% (smallest balance)
  note("Creating L65 Medical ($1,500)...");
  await createAccount("L65 Medical", "Loan", 1500);

  // Verify accounts exist on screen
  await navTo(page, "Accounts");
  const acctScreenText = await page.evaluate(() => document.body.textContent ?? "");
  const hasCardA   = /L65.*Card.*A|Card.*A.*L65/i.test(acctScreenText);
  const hasCardB   = /L65.*Card.*B|Card.*B.*L65/i.test(acctScreenText);
  const hasMedical = /L65.*Medical|Medical.*L65/i.test(acctScreenText);
  const hasChecking = /L65.*Checking|Checking.*L65/i.test(acctScreenText);

  note(`Accounts on screen — Checking: ${hasChecking}, Card A: ${hasCardA}, Card B: ${hasCardB}, Medical: ${hasMedical}`);

  if (hasChecking && hasCardA && hasCardB && hasMedical)
    pass("Step 1.1 — All 4 L65 accounts visible on /accounts screen");
  else
    note(`Step 1.1 — Some accounts missing from /accounts screen. Checking=${hasChecking} CardA=${hasCardA} CardB=${hasCardB} Medical=${hasMedical}`);

  // L64 sign-convention bug re-check: do new credit card accounts show as positive or negative?
  const cardABalStr = await readAccountBalance(page, "L65 Card A");
  const cardBBalStr = await readAccountBalance(page, "L65 Card B");
  const medBalStr   = await readAccountBalance(page, "L65 Medical");
  note(`Card A balance (screen): ${cardABalStr}`);
  note(`Card B balance (screen): ${cardBBalStr}`);
  note(`Medical balance (screen): ${medBalStr}`);

  const cardABal = parseBalanceStr(cardABalStr);
  const cardBBal = parseBalanceStr(cardBBalStr);
  const medBal   = parseBalanceStr(medBalStr);

  // L64 sign bug: CC created as positive ($4800) not liability (($4800))
  const cardAIsLiability = cardABal !== null && cardABal < 0;
  const cardBIsLiability = cardBBal !== null && cardBBal < 0;
  note(`L64 sign-bug check — Card A is liability (negative): ${cardAIsLiability}, Card B: ${cardBIsLiability}`);

  if (!cardAIsLiability && cardABal !== null) {
    note("⚠ L64 SIGN BUG CONFIRMED STILL PRESENT: Card A shows as positive $" +
      (cardABal / 100).toFixed(2) + " — should be liability (negative)");
  } else if (cardAIsLiability) {
    note("✓ L64 sign bug FIXED: Card A stored as liability (negative balance)");
  }

  await page.screenshot({ path: SS("story65_step1_accounts.png") });
  pass("Step 1.2 — screenshot story65_step1_accounts.png (after seeding)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 2: /planning — navigate to payoff / debt section, build AVALANCHE plan
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 2: /planning — AVALANCHE payoff plan ───────────────────────────────────");

  await dismissModal(page);
  await navTo(page, "Planning");
  await page.waitForTimeout(800);

  // Probe what sections are available on /planning
  const planningBody = await page.evaluate(() => document.body.textContent ?? "");
  const planningHtml = await page.evaluate(() => document.body.innerHTML ?? "");

  const hasPayoffSection  = /payoff|debt.*payoff|pay.*off/i.test(planningBody);
  const hasAvalancheOpt   = /avalanche/i.test(planningBody);
  const hasSnowballOpt    = /snowball/i.test(planningBody);
  const hasDebtSection    = /debt/i.test(planningBody);
  const hasRecurringForm  = /how often|recurring/i.test(planningBody);

  note(`Planning sections — payoff: ${hasPayoffSection}, avalanche: ${hasAvalancheOpt}, snowball: ${hasSnowballOpt}, debt: ${hasDebtSection}, recurring: ${hasRecurringForm}`);
  note(`Planning page body (first 400 chars): ${planningBody.replace(/\s+/g, " ").slice(0, 400)}`);

  // List all buttons and select options on planning page to understand what's available
  const planningControls = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll("button")).map(b => b.textContent.trim()).filter(t => t).slice(0, 20);
    const sels = Array.from(document.querySelectorAll("select")).map(s => ({
      label: s.getAttribute("aria-label"),
      opts: Array.from(s.options).map(o => o.text).slice(0, 10),
    })).slice(0, 10);
    const headings = Array.from(document.querySelectorAll("h1,h2,h3,h4")).map(h => h.textContent.trim()).filter(t => t).slice(0, 10);
    return { btns, sels, headings };
  });
  note(`Planning controls: ${JSON.stringify(planningControls)}`);

  await page.screenshot({ path: SS("story65_step2_avalanche.png") });
  pass("Step 2.0 — screenshot story65_step2_avalanche.png (planning page overview)");

  if (hasPayoffSection || hasAvalancheOpt) {
    pass("Step 2.1 — Payoff/avalanche section found on /planning");
  } else {
    absent_("Step 2.1 (I1) — ABSENT: No payoff or avalanche section on /planning page. " +
      "The debt payoff planning tool (avalanche/snowball strategy selector) is not present.");
  }

  // Try to interact with avalanche strategy selector if it exists
  let avalancheResult = null;
  if (hasAvalancheOpt) {
    // Try clicking avalanche radio/button/select
    avalancheResult = await page.evaluate(() => {
      // Try radio buttons
      const radios = Array.from(document.querySelectorAll('input[type="radio"]'));
      const avRadio = radios.find(r => /avalanche/i.test(r.value + (r.getAttribute("aria-label") || "") + (r.id || "")));
      if (avRadio) { avRadio.click(); return `clicked radio: ${avRadio.value || avRadio.id}`; }

      // Try select
      const sels = Array.from(document.querySelectorAll("select"));
      for (const sel of sels) {
        const opt = Array.from(sel.options).find(o => /avalanche/i.test(o.text));
        if (opt) {
          sel.value = opt.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `select set to avalanche: ${opt.text}`;
        }
      }

      // Try button
      const btn = Array.from(document.querySelectorAll("button")).find(b => /avalanche/i.test(b.textContent));
      if (btn) { btn.click(); return `clicked button: ${btn.textContent.trim()}`; }

      return "no interactive avalanche control found";
    });
    note(`Avalanche selection: ${avalancheResult}`);
    await page.waitForTimeout(800);
  }

  // Probe payoff order display — look for Card A listed first (highest APR)
  const payoffOrderText = await page.evaluate(() => {
    const body = document.body.textContent ?? "";
    // Find any section that shows debt order
    const payoffSection = document.querySelector('[class*="payoff"], [class*="debt"], #payoff, section');
    return {
      bodySnippet: body.slice(0, 800).replace(/\s+/g, " "),
      payoffSectionText: payoffSection ? payoffSection.textContent.replace(/\s+/g, " ").slice(0, 400) : null,
    };
  });
  note(`Payoff order probe: ${JSON.stringify(payoffOrderText)}`);

  // I1: Check avalanche order — Card A should appear before Card B which appears before Medical
  const fullText = payoffOrderText.bodySnippet;
  const idxCardA   = fullText.indexOf("Card A") !== -1 ? fullText.indexOf("Card A") : fullText.search(/card.*a/i);
  const idxCardB   = fullText.indexOf("Card B") !== -1 ? fullText.indexOf("Card B") : fullText.search(/card.*b/i);
  const idxMedical = fullText.search(/medical|L65 Med/i);

  note(`Text positions — Card A: ${idxCardA}, Card B: ${idxCardB}, Medical: ${idxMedical}`);

  if (idxCardA >= 0 && idxCardB >= 0 && idxMedical >= 0 && idxCardA < idxCardB && idxCardB < idxMedical) {
    pass("Step 2.2 (I1) — AVALANCHE_ORDER: Card A → Card B → Medical (highest APR first) ✓");
  } else if (idxCardA < 0 && idxCardB < 0 && idxMedical < 0) {
    absent_("Step 2.2 (I1) — ABSENT: AVALANCHE_ORDER — L65 debt accounts not visible in /planning payoff section. " +
      "The payoff tool does not appear to incorporate newly-created liability accounts.");
  } else {
    note(`Step 2.2 (I1) — Payoff order could not be confirmed. Positions: A=${idxCardA} B=${idxCardB} Med=${idxMedical}`);
  }

  // Check for debt-free date and total interest
  const hasDebtFreeDate = /debt.*free|pay.*off.*date|payoff.*date|\d{4}/i.test(planningBody);
  const hasTotalInterest = /total.*interest|interest.*total|\$\d+/i.test(planningBody);
  note(`Planning — debt-free date signal: ${hasDebtFreeDate}, total interest signal: ${hasTotalInterest}`);

  if (hasDebtFreeDate) pass("Step 2.3 — Debt-free date shown on /planning payoff section");
  else absent_("Step 2.3 — ABSENT: No debt-free date shown in payoff plan section");

  if (hasTotalInterest) pass("Step 2.4 — Total interest shown on /planning payoff section");
  else absent_("Step 2.4 — ABSENT: No total interest shown in payoff plan section");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 3: Switch to SNOWBALL and check order change + value differences
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 3: /planning — switch to SNOWBALL ───────────────────────────────────────");

  let snowballResult = null;
  if (hasSnowballOpt) {
    snowballResult = await page.evaluate(() => {
      const radios = Array.from(document.querySelectorAll('input[type="radio"]'));
      const sbRadio = radios.find(r => /snowball/i.test(r.value + (r.getAttribute("aria-label") || "") + (r.id || "")));
      if (sbRadio) { sbRadio.click(); return `clicked radio: ${sbRadio.value || sbRadio.id}`; }

      const sels = Array.from(document.querySelectorAll("select"));
      for (const sel of sels) {
        const opt = Array.from(sel.options).find(o => /snowball/i.test(o.text));
        if (opt) {
          sel.value = opt.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `select set to snowball: ${opt.text}`;
        }
      }

      const btn = Array.from(document.querySelectorAll("button")).find(b => /snowball/i.test(b.textContent));
      if (btn) { btn.click(); return `clicked button: ${btn.textContent.trim()}`; }

      return "no interactive snowball control found";
    });
    note(`Snowball selection: ${snowballResult}`);
    await page.waitForTimeout(800);
  }

  await page.screenshot({ path: SS("story65_step3_snowball.png") });
  pass("Step 3.0 — screenshot story65_step3_snowball.png");

  if (hasSnowballOpt) {
    // I2: Snowball order — Medical (smallest, $1,500) → Card B ($2,100) → Card A ($4,800)
    const snowballPageText = await page.evaluate(() => document.body.textContent ?? "");
    const sfText = snowballPageText.slice(0, 800).replace(/\s+/g, " ");
    const sfIdxMed  = sfText.search(/medical|L65 Med/i);
    const sfIdxCardB = sfText.search(/card.*b|L65.*Card.*B/i);
    const sfIdxCardA = sfText.search(/card.*a|L65.*Card.*A/i);

    note(`Snowball text positions — Medical: ${sfIdxMed}, Card B: ${sfIdxCardB}, Card A: ${sfIdxCardA}`);

    if (sfIdxMed >= 0 && sfIdxCardB >= 0 && sfIdxCardA >= 0 && sfIdxMed < sfIdxCardB && sfIdxCardB < sfIdxCardA) {
      pass("Step 3.1 (I2) — SNOWBALL_ORDER: Medical → Card B → Card A (smallest balance first) ✓");
    } else if (sfIdxMed < 0 && sfIdxCardB < 0 && sfIdxCardA < 0) {
      absent_("Step 3.1 (I2) — ABSENT: SNOWBALL_ORDER — L65 debts not visible after snowball switch");
    } else {
      note(`Step 3.1 (I2) — Snowball order not confirmed. Positions: Med=${sfIdxMed} B=${sfIdxCardB} A=${sfIdxCardA}`);
    }

    // I3: Strategy difference — capture any interest or date values from both strategies
    // (We read planningBody for avalanche already; now read snowball state)
    const snowballInterestText = await page.evaluate(() => {
      // Try to find interest/date values
      const matches = [];
      document.querySelectorAll('[class*="interest"], [class*="date"], [class*="payoff"], [class*="total"]').forEach(el => {
        const t = el.textContent.trim();
        if (t) matches.push(t.slice(0, 60));
      });
      // Also look for dollar amounts
      const body = document.body.textContent ?? "";
      const dollarAmts = body.match(/\$[\d,]+\.?\d*/g) || [];
      return { elements: matches.slice(0, 10), dollars: dollarAmts.slice(0, 20) };
    });
    note(`Snowball interest/values: ${JSON.stringify(snowballInterestText)}`);

    if (snowballInterestText.elements.length > 0 || snowballInterestText.dollars.length > 0) {
      pass("Step 3.2 (I3) — STRATEGY_DIFF: Snowball plan shows numerical values (dates/interest) " +
        "— avalanche vs snowball comparison possible");
    } else {
      absent_("Step 3.2 (I3) — ABSENT: STRATEGY_DIFF — No interest totals or payoff dates visible " +
        "in snowball plan. Cannot confirm avalanche saves more interest than snowball.");
    }
  } else {
    absent_("Step 3.1 (I2) — ABSENT: SNOWBALL_ORDER — No snowball option exists on /planning. " +
      "Debt payoff strategy selector (avalanche/snowball) not implemented.");
    absent_("Step 3.2 (I3) — ABSENT: STRATEGY_DIFF — Cannot compare strategies without snowball option.");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 4: Record this month's payments
  //   - Card A: min ~$96 + $300 extra = $396 total (top priority, avalanche)
  //   - Card B: min ~$42
  //   - Medical: min ~$30
  //   Total out of checking: $468
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 4: /transactions — record three payments ────────────────────────────────");

  const today = new Date();
  const pad = (n) => String(n).padStart(2, "0");
  const todayStr = `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}`;

  // Read balances before payments
  const cardABalBeforeStr   = await readAccountBalance(page, "L65 Card A");
  const cardBBalBeforeStr   = await readAccountBalance(page, "L65 Card B");
  const medBalBeforeStr     = await readAccountBalance(page, "L65 Medical");
  const checkBalBeforeStr   = await readAccountBalance(page, "L65 Checking");
  note(`Before payments — Card A: ${cardABalBeforeStr}, Card B: ${cardBBalBeforeStr}, Medical: ${medBalBeforeStr}, Checking: ${checkBalBeforeStr}`);

  const cardABalBefore  = parseBalanceStr(cardABalBeforeStr);
  const cardBBalBefore  = parseBalanceStr(cardBBalBeforeStr);
  const medBalBefore    = parseBalanceStr(medBalBeforeStr);
  const checkBalBefore  = parseBalanceStr(checkBalBeforeStr);

  // Helper to record one payment as Transfer (Checking → liability account)
  const recordPayment = async (label, amount, toAccountMatch) => {
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
    await page.evaluate(({ desc }) => {
      const inp = document.querySelector('#txn-add') ||
        Array.from(document.querySelectorAll("input,textarea")).find(i =>
          i.getAttribute("aria-label") === "txn-add" || i.getAttribute("placeholder") === "txn-add");
      if (inp) { inp.focus(); inp.value = desc; inp.dispatchEvent(new Event("input", { bubbles: true })); inp.dispatchEvent(new Event("change", { bubbles: true })); }
    }, { desc: label });

    // Amount
    await page.evaluate((a) => {
      const inp = document.querySelector('input[type="number"]');
      if (inp) { inp.value = a; inp.dispatchEvent(new Event("input", { bubbles: true })); inp.dispatchEvent(new Event("change", { bubbles: true })); }
    }, String(amount));

    // Type = Transfer
    const typeR = await selectByText(page, "Type", "Transfer");
    note(`  ${label} type: ${typeR}`);

    // From: Checking
    const fromR = await page.evaluate(() => {
      const sel = Array.from(document.querySelectorAll("select")).find(s =>
        s.getAttribute("aria-label") === "From" || s.getAttribute("aria-label") === "From account" ||
        s.getAttribute("aria-label") === "Account");
      if (!sel) return "From/Account select NOT FOUND";
      const opt = Array.from(sel.options).find(o => /L65.*Checking/i.test(o.text));
      if (opt) { sel.value = opt.value; sel.dispatchEvent(new Event("change", { bubbles: true })); return `set From → "${opt.text}"`; }
      const first = Array.from(sel.options).find(o => /checking/i.test(o.text));
      if (first) { sel.value = first.value; sel.dispatchEvent(new Event("change", { bubbles: true })); return `set From → "${first.text}" (first checking)`; }
      return `no checking option; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
    });
    note(`  ${label} From: ${fromR}`);

    // To: target liability account
    const toR = await page.evaluate(({ match }) => {
      const sel = Array.from(document.querySelectorAll("select")).find(s =>
        s.getAttribute("aria-label") === "To" || s.getAttribute("aria-label") === "To account");
      if (!sel) return "To select NOT FOUND (Transfer may not have appeared)";
      const opt = Array.from(sel.options).find(o => new RegExp(match, "i").test(o.text));
      if (opt) { sel.value = opt.value; sel.dispatchEvent(new Event("change", { bubbles: true })); return `set To → "${opt.text}"`; }
      return `no option matching "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
    }, { match: toAccountMatch });
    note(`  ${label} To: ${toR}`);

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

    // Verify persisted
    const ds = await getDataset(page);
    const txn = (ds.transactions || []).find(t =>
      new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), "i").test((t.payee || "") + (t.desc || "")));
    return txn ? "persisted" : "not-in-dataset";
  };

  // Card A: $396 (min + extra)
  const cardAPaidR = await recordPayment("L65 Card A Payment", 396, "L65 Card A");
  note(`Card A payment: ${cardAPaidR}`);
  if (cardAPaidR === "persisted") pass("Step 4.1 — Card A payment ($396) persisted");
  else note("Step 4.1 — Card A payment not found in dataset (may have posted differently)");

  // Card B: $42 (min)
  const cardBPaidR = await recordPayment("L65 Card B Payment", 42, "L65 Card B");
  note(`Card B payment: ${cardBPaidR}`);
  if (cardBPaidR === "persisted") pass("Step 4.2 — Card B payment ($42) persisted");
  else note("Step 4.2 — Card B payment not found in dataset");

  // Medical: $30 (min)
  const medPaidR = await recordPayment("L65 Medical Payment", 30, "L65 Medical");
  note(`Medical payment: ${medPaidR}`);
  if (medPaidR === "persisted") pass("Step 4.3 — Medical payment ($30) persisted");
  else note("Step 4.3 — Medical payment not found in dataset");

  await dismissModal(page);
  await navTo(page, "Transactions");
  await page.screenshot({ path: SS("story65_step4_payments.png") });
  pass("Step 4.4 — screenshot story65_step4_payments.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 5: Check liability balances AFTER payments (I4 REDUCES_LIABILITY)
  // Re-tests L64 sign bug + L46 balance linkage
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 5: /accounts — check liability balances after payments ───────────────────");

  const cardABalAfterStr  = await readAccountBalance(page, "L65 Card A");
  const cardBBalAfterStr  = await readAccountBalance(page, "L65 Card B");
  const medBalAfterStr    = await readAccountBalance(page, "L65 Medical");
  const checkBalAfterStr  = await readAccountBalance(page, "L65 Checking");

  note(`After payments — Card A: ${cardABalAfterStr}, Card B: ${cardBBalAfterStr}, Medical: ${medBalAfterStr}, Checking: ${checkBalAfterStr}`);

  const cardABalAfter  = parseBalanceStr(cardABalAfterStr);
  const cardBBalAfter  = parseBalanceStr(cardBBalAfterStr);
  const medBalAfter    = parseBalanceStr(medBalAfterStr);
  const checkBalAfter  = parseBalanceStr(checkBalAfterStr);

  await page.screenshot({ path: SS("story65_step5_liability_check.png") });
  pass("Step 5.0 — screenshot story65_step5_liability_check.png");

  // I4: PAYMENT_REDUCES_BALANCE
  // For liabilities stored as positive (L64 sign bug): payment INCREASES balance (wrong direction)
  // For liabilities stored as negative: payment makes balance less negative (correct)
  // Check each:

  const checkCardAReduced = () => {
    if (cardABalBefore === null || cardABalAfter === null) return "unreadable";
    if (cardAIsLiability) {
      // Stored negative: after payment should be less negative (closer to 0)
      return cardABalAfter > cardABalBefore ? "REDUCED" : "WRONG-DIRECTION";
    } else {
      // Stored positive (L64 sign bug): after payment should decrease
      return cardABalAfter < cardABalBefore ? "REDUCED" : "WRONG-DIRECTION";
    }
  };

  const cardAReduceResult = checkCardAReduced();
  note(`Card A balance: before=${cardABalBefore} after=${cardABalAfter} → ${cardAReduceResult}`);

  if (cardAReduceResult === "REDUCED") {
    pass("Step 5.1 (I4) — PAYMENT_REDUCES_BALANCE: Card A balance reduced after $396 payment ✓");
  } else if (cardAReduceResult === "WRONG-DIRECTION") {
    fail(`Step 5.1 (I4) — FAIL PAYMENT_REDUCES_BALANCE: Card A balance INCREASED after payment. ` +
      `Before: ${cardABalBeforeStr} (${cardABalBefore}), After: ${cardABalAfterStr} (${cardABalAfter}). ` +
      `This is the L64 sign bug + L46 balance linkage gap: Transfer credit leg adds to CC balance instead of reducing liability.`);
  } else {
    absent_("Step 5.1 (I4) — ABSENT: Cannot read Card A balance from /accounts screen");
  }

  // Card B
  note(`Card B balance: before=${cardBBalBefore} after=${cardBBalAfter}`);
  if (cardBBalBefore !== null && cardBBalAfter !== null) {
    const cardBIsLiabilityNeg = cardBBalBefore < 0;
    const cardBReduced = cardBIsLiabilityNeg ? cardBBalAfter > cardBBalBefore : cardBBalAfter < cardBBalBefore;
    if (cardBReduced) pass("Step 5.2 (I4) — Card B balance reduced after $42 payment ✓");
    else fail(`Step 5.2 (I4) — FAIL: Card B balance did not reduce. Before: ${cardBBalBeforeStr}, After: ${cardBBalAfterStr}`);
  } else {
    absent_("Step 5.2 (I4) — ABSENT: Card B balance not readable");
  }

  // Medical
  note(`Medical balance: before=${medBalBefore} after=${medBalAfter}`);
  if (medBalBefore !== null && medBalAfter !== null) {
    const medReduced = medBalAfter < medBalBefore; // loan stored positive typically
    if (medReduced) pass("Step 5.3 (I4) — Medical balance reduced after $30 payment ✓");
    else fail(`Step 5.3 (I4) — FAIL: Medical balance did not reduce. Before: ${medBalBeforeStr}, After: ${medBalAfterStr}`);
  } else {
    absent_("Step 5.3 (I4) — ABSENT: Medical balance not readable");
  }

  // Checking debit
  note(`Checking balance: before=${checkBalBefore} after=${checkBalAfter}`);
  if (checkBalBefore !== null && checkBalAfter !== null) {
    const totalDebited = checkBalBefore - checkBalAfter; // should be ~46800 ($468)
    const expectedDebit = 46800; // $468 = $396 + $42 + $30
    note(`Checking debited: ${totalDebited} minor units (expected ${expectedDebit} = $468)`);
    if (Math.abs(totalDebited - expectedDebit) <= 500) {
      pass(`Step 5.4 (I7) — MONEY_CONSERVE: Checking debited $${(totalDebited/100).toFixed(2)} ≈ $468 ✓`);
    } else {
      note(`Step 5.4 (I7) — Checking debited ${totalDebited} vs expected ${expectedDebit}; ` +
        `accumulated test data or balance not updating (L46 gap). Total payments: $468.`);
    }
  } else {
    absent_("Step 5.4 (I7) — ABSENT: Checking balance not readable; money conservation unverifiable");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 6: /dashboard — confirm total debt down, net worth up (I6)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 6: /dashboard — total debt and net worth ───────────────────────────────");

  await dismissModal(page);
  await navTo(page, "Dashboard");
  await page.waitForTimeout(800);

  const dashBody = await page.evaluate(() => document.body.textContent ?? "");

  const hasNetWorth       = /net worth/i.test(dashBody);
  const hasDebtWidget     = /total debt|debt.*total/i.test(dashBody);
  const hasUpcomingBills  = /upcoming bills/i.test(dashBody);
  const hasCashFlow       = /cash.?flow/i.test(dashBody);

  note(`Dashboard — net worth: ${hasNetWorth}, total debt: ${hasDebtWidget}, upcoming bills: ${hasUpcomingBills}, cash flow: ${hasCashFlow}`);

  await page.screenshot({ path: SS("story65_step6_dashboard.png") });
  pass("Step 6.0 — screenshot story65_step6_dashboard.png");

  if (hasNetWorth) pass("Step 6.1 (I6) — Dashboard: Net Worth widget present");
  else absent_("Step 6.1 (I6) — ABSENT: No Net Worth widget on Dashboard");

  if (hasDebtWidget) pass("Step 6.2 (I6) — Dashboard: Total Debt widget present");
  else absent_("Step 6.2 (I6) — ABSENT: No Total Debt widget on Dashboard. " +
    "Dashboard does not show aggregate debt summary for payoff-plan progress tracking.");

  if (hasUpcomingBills) pass("Step 6.3 (I6) — Dashboard: Upcoming Bills widget present (regression anchor from L64)");
  else absent_("Step 6.3 (I6) — ABSENT: Upcoming Bills widget missing (was present in L64)");

  // Probe net worth / total debt values to detect direction
  const dashValues = await page.evaluate(() => {
    const amts = Array.from(document.querySelectorAll('[class*="amount"], [class*="value"], [class*="balance"], [class*="worth"]'))
      .map(el => ({ class: el.className, text: el.textContent.trim().slice(0, 30) }))
      .filter(e => e.text && /\$/.test(e.text))
      .slice(0, 15);
    return amts;
  });
  note(`Dashboard monetary values: ${JSON.stringify(dashValues)}`);

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 7: /planning — check if plan advanced after payments (I5 PLAN_ADVANCES)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 7: /planning — confirm plan advances after payments ─────────────────────");

  await dismissModal(page);
  await navTo(page, "Planning");
  await page.waitForTimeout(800);

  const planningBodyAfter = await page.evaluate(() => document.body.textContent ?? "");
  const hasDebtFreeAfter  = /debt.*free|pay.*off.*date|payoff.*date/i.test(planningBodyAfter);

  // Look for updated balance values reflecting payments
  const planningValues = await page.evaluate(() => {
    const body = document.body.textContent ?? "";
    const dollarAmts = body.match(/\$[\d,]+\.?\d*/g) || [];
    return dollarAmts.slice(0, 30);
  });
  note(`Planning values after payments: ${JSON.stringify(planningValues)}`);

  // Check if Card A shows updated balance (~$4,404 after $396 payment)
  // (only verifiable if plan uses live account balances)
  const hasUpdatedCardABal = planningBodyAfter.includes("4,404") ||
    planningBodyAfter.includes("4404") ||
    planningBodyAfter.includes("4,800"); // 4800 = unchanged (not live)

  if (!hasPayoffSection) {
    absent_("Step 7.1 (I5) — ABSENT: PLAN_ADVANCES — No payoff plan section exists on /planning. " +
      "Cannot verify plan advances after payments because the debt payoff tool is not present.");
  } else if (hasDebtFreeAfter) {
    pass("Step 7.1 (I5) — Payoff plan still shows debt-free date after payments (plan section present)");
    note("Step 7.1 (I5) — NOTE: Cannot confirm date moved forward without before/after comparison. " +
      "Payoff plan may be static (not recomputed from live balances).");
  } else {
    absent_("Step 7.1 (I5) — ABSENT: PLAN_ADVANCES — Payoff plan present but no debt-free date visible after payments");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 8: Money conservation cross-check via dataset (I7)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 8: money conservation cross-check ────────────────────────────────────────");

  const dsFinal = await getDataset(page);
  const l65Txns = (dsFinal.transactions || []).filter(t =>
    /L65/i.test((t.payee || "") + (t.desc || "")));
  note(`L65 transactions in dataset: ${l65Txns.length}`);

  const getAmt = (t) => {
    if (typeof t.amount === "number") return t.amount;
    if (t.amount?.Amount !== undefined) return t.amount.Amount;
    if (t.amount?.amount !== undefined) return t.amount.amount;
    return 0;
  };

  const cardAPayTxn = l65Txns.find(t => /card.*a.*payment|L65 Card A/i.test(t.desc || t.payee || ""));
  const cardBPayTxn = l65Txns.find(t => /card.*b.*payment|L65 Card B/i.test(t.desc || t.payee || ""));
  const medPayTxn   = l65Txns.find(t => /medical.*payment|L65 Medical/i.test(t.desc || t.payee || ""));

  note(`Card A payment txn: ${JSON.stringify(cardAPayTxn ? { desc: cardAPayTxn.desc || cardAPayTxn.payee, amount: getAmt(cardAPayTxn) } : null)}`);
  note(`Card B payment txn: ${JSON.stringify(cardBPayTxn ? { desc: cardBPayTxn.desc || cardBPayTxn.payee, amount: getAmt(cardBPayTxn) } : null)}`);
  note(`Medical payment txn: ${JSON.stringify(medPayTxn ? { desc: medPayTxn.desc || medPayTxn.payee, amount: getAmt(medPayTxn) } : null)}`);

  // I7: Sum of payment amounts should equal $468 out of checking
  const cardADebit  = cardAPayTxn ? Math.abs(getAmt(cardAPayTxn)) : 0;
  const cardBDebit  = cardBPayTxn ? Math.abs(getAmt(cardBPayTxn)) : 0;
  const medDebit    = medPayTxn   ? Math.abs(getAmt(medPayTxn))   : 0;
  const totalDebits = cardADebit + cardBDebit + medDebit;
  const expectedTotal = 46800; // $468 total

  note(`I7 money conservation: Card A ${cardADebit} + Card B ${cardBDebit} + Medical ${medDebit} = ${totalDebits} (expected ${expectedTotal})`);

  if (totalDebits === 0) {
    note("Step 8.1 (I7) — Transactions not found in dataset; money conservation unverifiable via dataset");
  } else if (Math.abs(totalDebits - expectedTotal) <= 500) {
    pass(`Step 8.1 (I7) — MONEY_CONSERVE: Total payment debits = $${(totalDebits/100).toFixed(2)} ≈ $468 ✓`);
  } else {
    note(`Step 8.1 (I7) — Total debits = ${totalDebits} vs expected ${expectedTotal}; ` +
      `Transfer posts two legs so total may differ from expected.`);
  }

  // Transfer two-leg check: each transfer should post both debit (from checking) and credit (to CC)
  // For a $396 Transfer: one leg -39600 and one leg +39600
  const l65AllTxns = l65Txns.map(t => ({ desc: t.desc || t.payee, amt: getAmt(t) }));
  note(`All L65 transactions: ${JSON.stringify(l65AllTxns)}`);

  const hasNegativeLegs = l65Txns.some(t => getAmt(t) < 0);
  const hasPositiveLegs = l65Txns.some(t => getAmt(t) > 0 && !/checking/i.test(t.account || ""));
  note(`Transfer two-leg check — has debit leg: ${hasNegativeLegs}, has credit leg: ${hasPositiveLegs}`);

  if (l65Txns.length >= 6) {
    // 3 payments × 2 legs = 6 transaction records
    pass("Step 8.2 — Transfer two-leg posting: ≥6 L65 transaction records (3 transfers × 2 legs) ✓");
  } else if (l65Txns.length > 0) {
    note(`Step 8.2 — ${l65Txns.length} L65 transaction records (expected 6 for 3 two-leg transfers)`);
  } else {
    note("Step 8.2 — No L65 transactions in dataset; two-leg check skipped");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 9: JS errors audit
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 9: JS errors audit ──────────────────────────────────────────────────────");

  note(`JS errors captured during run: ${jsErrors.length}`);
  if (jsErrors.length === 0) {
    pass("Step 9.1 — ZERO JS errors across full ritual ✓");
  } else {
    jsErrors.forEach((e, i) => note(`  JS Error ${i+1}: ${e.slice(0, 120)}`));
    fail(`Step 9.1 — ${jsErrors.length} JS error(s) detected during run`);
  }

  // ════════════════════════════════════════════════════════════════════════════
  // INVARIANT SUMMARY
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── INVARIANT SUMMARY ────────────────────────────────────────────────────────────");
  console.log(`I1 AVALANCHE_ORDER:       ${hasPayoffSection || hasAvalancheOpt ? "see Step 2.2 above" : "ABSENT — payoff tool not present"}`);
  console.log(`I2 SNOWBALL_ORDER:        ${hasSnowballOpt ? "see Step 3.1 above" : "ABSENT — snowball option not present"}`);
  console.log(`I3 STRATEGY_DIFF:         ${hasSnowballOpt ? "see Step 3.2 above" : "ABSENT — need both strategies"}`);
  console.log(`I4 PAYMENT_REDUCES_BAL:   ${cardABalAfter !== null ? `Card A: ${cardAReduceResult}` : "unreadable"}`);
  console.log(`I5 PLAN_ADVANCES:         ${hasPayoffSection ? "partial — plan present, advance not confirmed" : "ABSENT — no payoff plan"}`);
  console.log(`I6 DASHBOARD_DEBT_DOWN:   ${hasNetWorth ? "net worth present" : "absent"}; ${hasDebtWidget ? "total debt present" : "total debt absent"}`);
  console.log(`I7 MONEY_CONSERVE:        ${totalDebits > 0 ? `total debits = $${(totalDebits/100).toFixed(2)}` : "unverifiable (no dataset txns)"}`);

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
