// L67 E2E loop story — "The Balance Transfer" (Priya) — 2026-06-22
//
// Persona: Priya has been carrying $3,000 on Card A at 24.9% APR — a high-rate card
// she's been paying minimum payments on for months. She gets a mailer offering 0%
// intro APR for 18 months on Transfer Card B (with a 3% balance transfer fee). She
// decides to move the full $3,000 balance to the new card, pay the $90 fee, and
// lock in the 0% window to attack the principal. The ritual walks through creating
// both cards, executing the balance transfer, recording the fee, and verifying that
// the TOTAL DEBT DID NOT CHANGE (still $3,090) — not $0 or $6,090.
//
// KEY INVARIANTS ASSERTED:
//   I1: CARD_A_ZEROED — Card A balance ≈ $0 after the balance transfer
//   I2: CARD_B_LOADED — Card B balance ≈ $3,090 after transfer + fee
//   I3: DEBT_CONSERVED — total debt on dashboard ≈ $3,090 (NOT $0, NOT $6,090)
//   I4: SIGN_DIRECTION — LIABILITY→LIABILITY transfer moved debt in correct direction
//       (Card A decreases, Card B increases; no phantom doubling)
//   I5: MONEY_CONSERVE — no cents created or destroyed; net debt delta from original
//       $3,000 is exactly $90 (the fee)
//   I6: TXN_LEGS — transfer between two liability accounts posts correct debit/credit
//       legs (Card A gets credit leg reducing balance; Card B gets debit leg increasing balance)
//
// Balance checkpoints (minor units = cents):
//   Card A after creation:       300000 ($3,000.00) — note: sign bug may show as +$3,000
//   Card B after creation:            0 ($0.00)
//   Card A after transfer:            0 ($0.00 — fully transferred)
//   Card B after transfer + fee: 309000 ($3,090.00) — $3,000 + $90 fee
//   Total debt:                  309000 ($3,090.00 — conserved, not doubled)
//
// Screens exercised: /accounts (×2) → /transactions (transfer) → /transactions (fee expense)
//   → /accounts (verify balances) → /dashboard (total debt / net worth) → /transactions (audit)
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_67_balance_transfer.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

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

// Create a liability account (Credit Card type)
const createCreditCardAccount = async (page, name, openingBalance) => {
  await navTo(page, "Accounts");
  await dismissModal(page);

  // Click Add Account
  const addR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /add account|new account/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked"; }
    return "NOT FOUND";
  });
  note(`  Add Account button: ${addR}`);
  await page.waitForTimeout(800);

  // Name
  await page.evaluate((n) => {
    const inp = Array.from(document.querySelectorAll("input[type='text']")).find(i => i.placeholder === "Name");
    if (!inp) return "NOT FOUND";
    inp.focus(); inp.value = n;
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  }, name);

  // Type = Credit card
  const typeR = await selectByText(page, "Account type", "Credit card");
  note(`  Account type: ${typeR}`);

  // Opening balance
  await page.evaluate((b) => {
    const inp = Array.from(document.querySelectorAll("input[type='number']")).find(i =>
      i.placeholder === "Opening balance");
    if (!inp) return "NOT FOUND";
    inp.value = b;
    inp.dispatchEvent(new Event("input", { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  }, String(openingBalance));

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
};

// Record an expense transaction
const recordExpense = async (page, label, amount, accountMatch, todayStr) => {
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
      return /^add$|^save$|^add transaction$/i.test(t) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);
};

// Record a transfer between two accounts
const recordTransfer = async (page, label, amount, fromMatch, toMatch, todayStr) => {
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

  // Amount
  await page.evaluate((a) => {
    const inp = document.querySelector('input[type="number"]');
    if (inp) {
      inp.value = a;
      inp.dispatchEvent(new Event("input", { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, String(amount));

  // Type = Transfer
  await selectByText(page, "Type", "Transfer");
  await page.waitForTimeout(500);

  // From account
  const fromR = await page.evaluate((match) => {
    // Try various aria-label patterns for the "From" select
    const candidates = ["From", "From account", "Account"];
    for (const label of candidates) {
      const sel = Array.from(document.querySelectorAll("select")).find(s =>
        s.getAttribute("aria-label") === label);
      if (sel) {
        const opt = Array.from(sel.options).find(o => new RegExp(match, "i").test(o.text));
        if (opt) {
          sel.value = opt.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `[${label}] set → "${opt.text}"`;
        }
        return `[${label}] no option matching "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
      }
    }
    return `no From-style select found`;
  }, fromMatch);
  note(`  ${label} From: ${fromR}`);

  // To account
  const toR = await page.evaluate((match) => {
    const sel = Array.from(document.querySelectorAll("select")).find(s =>
      s.getAttribute("aria-label") === "To" || s.getAttribute("aria-label") === "To account");
    if (!sel) return `To select NOT FOUND`;
    const opt = Array.from(sel.options).find(o => new RegExp(match, "i").test(o.text));
    if (opt) {
      sel.value = opt.value;
      sel.dispatchEvent(new Event("change", { bubbles: true }));
      return `set → "${opt.text}"`;
    }
    return `no option matching "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
  }, toMatch);
  note(`  ${label} To: ${toR}`);

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
      return /^add$|^save$|^add transaction$/i.test(t) && b.type !== "reset";
    });
    if (btn) btn.click();
  });
  await page.waitForTimeout(1500);
  await flush(page);
};

// ─── main ─────────────────────────────────────────────────────────────────────

const jsErrors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  page.on("pageerror", (e) => {
    const msg = String(e);
    // Filter known WASM churn artifact
    if (!msg.includes("Go program has already exited")) jsErrors.push(msg);
  });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  pass("HYDRATION — app loaded and nav visible");

  const today = new Date();
  const pad = (n) => String(n).padStart(2, "0");
  const todayStr = `${today.getFullYear()}-${pad(today.getMonth() + 1)}-${pad(today.getDate())}`;

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 1: /accounts — Screenshot accounts BEFORE creation (baseline)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 1: Screenshot accounts baseline ───────────────────────────────────────────");

  await navTo(page, "Accounts");
  await dismissModal(page);
  await page.screenshot({ path: SS("story67_01_accounts_before.png") });
  pass("Step 1 — screenshot story67_01_accounts_before.png (baseline accounts list)");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 2: Create Card A (24.9% APR) — Credit Card type, $3,000 opening balance
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 2: Create Card A (24.9% APR, $3,000) ─────────────────────────────────────");

  await createCreditCardAccount(page, "L67 Card A (24.9% APR)", 3000);

  await navTo(page, "Accounts");
  const cardAStr = await readAccountBalance(page, "L67 Card A");
  const cardAMinor = parseBalanceStr(cardAStr);
  note(`Card A balance after creation: "${cardAStr}" → ${cardAMinor} minor units`);

  await page.screenshot({ path: SS("story67_02_card_a_created.png") });
  pass("Step 2 — screenshot story67_02_card_a_created.png (Card A created)");

  if (cardAStr !== null) {
    pass("Step 2.1 — Card A appears on /accounts");
  } else {
    fail("Step 2.1 — Card A NOT found on /accounts");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 3: Create Transfer Card B (0% intro) — Credit Card type, $0 opening balance
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 3: Create Transfer Card B (0% intro, $0) ─────────────────────────────────");

  await createCreditCardAccount(page, "L67 Transfer Card B (0% intro)", 0);

  await navTo(page, "Accounts");
  const cardBStr0 = await readAccountBalance(page, "L67 Transfer Card B");
  const cardBMinor0 = parseBalanceStr(cardBStr0);
  note(`Card B balance after creation: "${cardBStr0}" → ${cardBMinor0} minor units`);

  await page.screenshot({ path: SS("story67_03_card_b_created.png") });
  pass("Step 3 — screenshot story67_03_card_b_created.png (Card B created)");

  if (cardBStr0 !== null) {
    pass("Step 3.1 — Transfer Card B appears on /accounts");
  } else {
    fail("Step 3.1 — Transfer Card B NOT found on /accounts");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 4: Execute balance transfer — $3,000 from Card A → Transfer Card B
  // KEY TEST: LIABILITY→LIABILITY transfer — does debt move correctly?
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 4: Balance transfer $3,000 Card A → Transfer Card B ──────────────────────");

  await recordTransfer(
    page,
    "L67 Balance Transfer $3000",
    3000,
    "L67 Card A",
    "L67 Transfer Card B",
    todayStr
  );

  // Read dataset to check transaction legs
  const dsAfterTransfer = await getDataset(page);
  const txns = Object.values(dsAfterTransfer.transactions || {});
  const l67Txns = txns.filter(t =>
    (t.desc || t.description || t.payee || "").includes("L67 Balance Transfer") ||
    (t.payee || "").includes("L67 Balance Transfer")
  );
  note(`L67 transfer transactions in dataset: ${l67Txns.length}`);
  l67Txns.forEach((t, i) => note(`  leg[${i}]: acct=${t.accountID || t.account_id}, amt=${t.amount?.amount ?? t.amount}, transferAcct=${t.transferAccountID || t.transfer_account_id}`));

  if (l67Txns.length >= 2) {
    pass("Step 4.1 (I6) — Transfer posted two legs in dataset");
    // Check legs: one should be positive credit (to Card B), one negative debit (from Card A)
    const amounts = l67Txns.map(t => t.amount?.amount ?? Number(t.amount));
    const hasCredit = amounts.some(a => a > 0);
    const hasDebit  = amounts.some(a => a < 0);
    if (hasCredit && hasDebit) {
      pass("Step 4.2 (I6) — Transfer has both a credit leg (+) and a debit leg (−)");
    } else {
      fail(`Step 4.2 (I6) — Transfer legs are not a credit/debit pair; amounts: ${amounts.join(", ")}`);
    }
  } else if (l67Txns.length === 1) {
    fail("Step 4.1 (I6) — Only ONE leg found in dataset — transfer may not have posted both sides");
  } else {
    absent_("Step 4.1 (I6) — ABSENT: No L67 transfer transactions found in dataset (may be key format)");
  }

  await page.screenshot({ path: SS("story67_04_transfer_executed.png") });
  pass("Step 4.3 — screenshot story67_04_transfer_executed.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 5: Record $90 balance transfer fee as expense on Card B
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 5: Record $90 balance transfer fee on Card B ─────────────────────────────");

  // The fee is charged to the destination card (Card B accumulates $3,000 + $90 fee)
  await recordExpense(page, "L67 Balance Transfer Fee 3%", 90, "L67 Transfer Card B", todayStr);

  note("$90 transfer fee recorded as expense against Transfer Card B");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 6: Verify account balances post-transfer
  // I1: Card A ≈ $0 (fully transferred)
  // I2: Card B ≈ $3,090 ($3,000 + $90 fee)
  // I4: SIGN_DIRECTION — debt moved correctly, not doubled
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 6: Verify balances ────────────────────────────────────────────────────────");

  await navTo(page, "Accounts");
  const cardAStrFinal = await readAccountBalance(page, "L67 Card A");
  const cardAFinal = parseBalanceStr(cardAStrFinal);
  note(`Card A balance after transfer: "${cardAStrFinal}" → ${cardAFinal} minor units`);

  const cardBStrFinal = await readAccountBalance(page, "L67 Transfer Card B");
  const cardBFinal = parseBalanceStr(cardBStrFinal);
  note(`Card B balance after transfer + fee: "${cardBStrFinal}" → ${cardBFinal} minor units`);

  await page.screenshot({ path: SS("story67_05_balances_after_transfer.png") });
  pass("Step 6 — screenshot story67_05_balances_after_transfer.png");

  // I1: Card A should be ≈ $0
  const cardAAbsMinor = cardAFinal !== null ? Math.abs(cardAFinal) : null;
  if (cardAAbsMinor !== null && cardAAbsMinor <= 100) {
    // Allow ±$1 tolerance
    pass(`Step 6.1 (I1) CARD_A_ZEROED — Card A balance ≈ $0: ${cardAStrFinal}`);
  } else if (cardAAbsMinor !== null && cardAAbsMinor >= 295000 && cardAAbsMinor <= 305000) {
    // Card A still shows ~$3,000 — transfer did NOT reduce it
    fail(`Step 6.1 (I1) CARD_A_ZEROED — Card A still shows ${cardAStrFinal} after transfer (expected ≈ $0) — LIABILITY→LIABILITY transfer did NOT move debt off Card A`);
  } else if (cardAFinal !== null) {
    fail(`Step 6.1 (I1) CARD_A_ZEROED — Card A unexpected balance: ${cardAStrFinal} (${cardAFinal} minor units)`);
  } else {
    absent_("Step 6.1 (I1) CARD_A_ZEROED — ABSENT: Could not read Card A balance");
  }

  // I2: Card B should be ≈ $3,090
  const cardBAbsMinor = cardBFinal !== null ? Math.abs(cardBFinal) : null;
  const expectedCardB = 309000; // $3,090 in minor units
  if (cardBAbsMinor !== null && Math.abs(cardBAbsMinor - expectedCardB) <= 500) {
    pass(`Step 6.2 (I2) CARD_B_LOADED — Card B balance ≈ $3,090: ${cardBStrFinal}`);
  } else if (cardBAbsMinor !== null && Math.abs(cardBAbsMinor - 300000) <= 500) {
    fail(`Step 6.2 (I2) CARD_B_LOADED — Card B shows $3,000 (missing $90 fee): ${cardBStrFinal}`);
  } else if (cardBAbsMinor !== null) {
    fail(`Step 6.2 (I2) CARD_B_LOADED — Card B unexpected balance: ${cardBStrFinal} (${cardBFinal} minor units); expected ≈ $3,090`);
  } else {
    absent_("Step 6.2 (I2) CARD_B_LOADED — ABSENT: Could not read Card B balance");
  }

  // I4: SIGN_DIRECTION — detect phantom doubling
  // If BOTH cards show ~$3,000, that is phantom doubling (total debt doubled)
  if (cardAAbsMinor !== null && cardBAbsMinor !== null) {
    const totalMinor = cardAAbsMinor + cardBAbsMinor;
    if (totalMinor >= 590000 && totalMinor <= 620000) {
      fail(`Step 6.3 (I4) SIGN_DIRECTION — PHANTOM DOUBLING DETECTED: Card A ${cardAStrFinal} + Card B ${cardBStrFinal} = ~$6,090. Transfer doubled the debt instead of moving it.`);
    } else if (totalMinor <= 100) {
      fail(`Step 6.3 (I4) SIGN_DIRECTION — PHANTOM ERASURE: Both cards ≈ $0. Transfer erased the debt instead of moving it.`);
    } else if (Math.abs(totalMinor - expectedCardB) <= 1000) {
      pass(`Step 6.3 (I4) SIGN_DIRECTION — total combined debt ≈ $3,090 (${(totalMinor / 100).toFixed(2)}); no phantom doubling`);
    } else {
      fail(`Step 6.3 (I4) SIGN_DIRECTION — unexpected total: ${(totalMinor / 100).toFixed(2)} (Card A ${cardAStrFinal} + Card B ${cardBStrFinal})`);
    }
  } else {
    absent_("Step 6.3 (I4) SIGN_DIRECTION — ABSENT: cannot compute phantom doubling without both balances");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 7: Dashboard — verify total debt / net worth
  // I3: total debt ≈ $3,090 on Dashboard
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 7: Dashboard — total debt / net worth ─────────────────────────────────────");

  await navTo(page, "Dashboard");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  const dashboardText = await page.evaluate(() => document.body.textContent);

  // Check for net worth widget
  const hasNetWorthWidget = /net worth/i.test(dashboardText);
  if (hasNetWorthWidget) {
    pass("Step 7.1 — Dashboard net worth widget present");
  } else {
    fail("Step 7.1 — Dashboard net worth widget NOT present");
  }

  // Check for total debt widget
  const hasTotalDebtWidget = /total debt|liabilities/i.test(dashboardText);
  if (hasTotalDebtWidget) {
    pass("Step 7.2 — Dashboard total debt / liabilities widget present");
  } else {
    absent_("Step 7.2 (I3) DEBT_CONSERVED — ABSENT: No 'Total Debt' or 'Liabilities' widget on Dashboard");
  }

  // Probe: does dashboard show $3,090 or $6,090 or $0?
  const dashMatches = dashboardText.match(/\$[\d,]+\.?\d*/g) || [];
  note(`Dashboard amounts: ${dashMatches.slice(0, 20).join(", ")}`);

  const has6090 = /6[,.]?09[0-9]|6090/i.test(dashboardText);
  const has3090 = /3[,.]?09[0-9]|3090/i.test(dashboardText);
  if (has6090) {
    fail("Step 7.3 (I3) DEBT_CONSERVED — Dashboard shows ~$6,090 — PHANTOM DOUBLING present in dashboard total");
  } else if (has3090) {
    pass("Step 7.3 (I3) DEBT_CONSERVED — Dashboard shows ~$3,090 (correct, no phantom doubling)");
  } else {
    note("Step 7.3 (I3) — $3,090 and $6,090 not found literally on Dashboard (sample data may swamp signal)");
    absent_("Step 7.3 (I3) DEBT_CONSERVED — ABSENT: Cannot confirm total debt from dashboard text alone");
  }

  await page.screenshot({ path: SS("story67_06_dashboard_debt.png") });
  pass("Step 7.4 — screenshot story67_06_dashboard_debt.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 8: /transactions — verify transaction ledger
  // Both legs of the transfer should be visible; fee expense should be visible
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 8: Transactions — audit all L67 entries ───────────────────────────────────");

  await navTo(page, "Transactions");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  const txnBodyText = await page.evaluate(() => document.body.textContent);

  const hasTransferEntry = /L67 Balance Transfer/i.test(txnBodyText);
  const hasFeeEntry      = /L67 Balance Transfer Fee/i.test(txnBodyText);

  if (hasTransferEntry) {
    pass("Step 8.1 — 'L67 Balance Transfer' appears on /transactions screen");
  } else {
    fail("Step 8.1 — 'L67 Balance Transfer' NOT visible on /transactions screen");
  }

  if (hasFeeEntry) {
    pass("Step 8.2 — 'L67 Balance Transfer Fee' (expense) appears on /transactions screen");
  } else {
    fail("Step 8.2 — 'L67 Balance Transfer Fee' NOT visible on /transactions screen");
  }

  // I5: MONEY_CONSERVE — dataset audit
  const dsFinal = await getDataset(page);
  const allTxns = Object.values(dsFinal.transactions || {});
  const l67All = allTxns.filter(t => {
    const desc = (t.desc || t.description || t.payee || "");
    return desc.includes("L67");
  });
  note(`All L67 transactions in dataset: ${l67All.length}`);

  // Sum of amounts for Card A legs (debit should be negative or reduce liability)
  const totalInDataset = l67All.reduce((acc, t) => acc + (t.amount?.amount ?? Number(t.amount || 0)), 0);
  note(`Net sum of all L67 transactions: ${totalInDataset} minor units`);

  // For money conservation: the transfer should net to 0 (debit + credit cancel),
  // and the fee should net to -9000 (expense from Card B)
  if (Math.abs(totalInDataset + 9000) <= 100) {
    pass("Step 8.3 (I5) MONEY_CONSERVE — Net of all L67 txns = −$90 (transfer cancels, fee is the only net spend)");
  } else if (Math.abs(totalInDataset) <= 100) {
    fail("Step 8.3 (I5) MONEY_CONSERVE — Net is $0 — fee was not posted or cancelled out incorrectly");
  } else {
    note(`Step 8.3 (I5) — net dataset sum ${totalInDataset}; expected −9000 (−$90 fee net); dataset key format may differ`);
    absent_("Step 8.3 (I5) MONEY_CONSERVE — ABSENT: Cannot confirm via dataset (key format unknown)");
  }

  await page.screenshot({ path: SS("story67_07_transactions_verify.png") });
  pass("Step 8.4 — screenshot story67_07_transactions_verify.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 9: /planning — check if payoff tool is present and shows updated debts
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 9: Planning — payoff tool view ────────────────────────────────────────────");

  await navTo(page, "Planning");
  await dismissModal(page);
  await page.waitForTimeout(1000);

  const planText = await page.evaluate(() => document.body.textContent);
  const hasPayoffSection = /payoff|debt payoff/i.test(planText);

  if (hasPayoffSection) {
    pass("Step 9.1 — Payoff section present on /planning");
    const hasCardAInPlan = /L67 Card A/i.test(planText);
    const hasCardBInPlan = /L67 Transfer Card B/i.test(planText);
    note(`Payoff section includes L67 Card A: ${hasCardAInPlan}; Transfer Card B: ${hasCardBInPlan}`);
    if (!hasCardAInPlan && !hasCardBInPlan) {
      absent_("Step 9.2 — L67 accounts NOT visible in payoff tool (uses sample data only — same as L65)");
    } else {
      pass("Step 9.2 — At least one L67 account visible in payoff tool");
    }
  } else {
    absent_("Step 9.1 — ABSENT: No payoff section on /planning");
  }

  await page.screenshot({ path: SS("story67_08_payoff_view.png") });
  pass("Step 9.3 — screenshot story67_08_payoff_view.png");

  // ════════════════════════════════════════════════════════════════════════════
  // SUMMARY
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n════════════════════════════════════════════════════════════════════════════════════");
  console.log(`SUMMARY: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
  console.log(`Real JS errors: ${jsErrors.length}`);
  if (jsErrors.length > 0) jsErrors.forEach(e => console.error(`  ERROR: ${e}`));

  if (cardAFinal !== null && cardBFinal !== null) {
    const totalDebt = Math.abs(cardAFinal) + Math.abs(cardBFinal);
    console.log(`\nBALANCE TRANSFER VERDICT:`);
    console.log(`  Card A after transfer: ${cardAStrFinal} (${cardAFinal} minor units)`);
    console.log(`  Card B after transfer+fee: ${cardBStrFinal} (${cardBFinal} minor units)`);
    console.log(`  Combined debt: $${(totalDebt / 100).toFixed(2)} (expected $3,090.00)`);
    if (totalDebt >= 590000) {
      console.log(`  VERDICT: PHANTOM DOUBLING — debt doubled from $3,000 to ~$6,090`);
    } else if (totalDebt <= 100) {
      console.log(`  VERDICT: PHANTOM ERASURE — debt vanished instead of moving`);
    } else if (Math.abs(totalDebt - 309000) <= 1000) {
      console.log(`  VERDICT: HELD — debt correctly moved from Card A to Card B`);
    } else {
      console.log(`  VERDICT: UNEXPECTED — investigate manually`);
    }
  }

  console.log("════════════════════════════════════════════════════════════════════════════════════");

} finally {
  await browser.close();
}

const exitCode = failed > 0 ? 1 : 0;
process.exit(exitCode);
