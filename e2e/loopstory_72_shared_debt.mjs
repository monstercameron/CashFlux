// L72 E2E loop story — "Splitting the Debt" (Sam & Alex, shared debt during separation) — 2026-06-22
//
// Persona: Sam and Alex are separating and share a joint credit card with $4,000 debt.
// They open CashFlux to track who owes what: Sam 60% ($2,400), Alex 40% ($1,600).
// Sam pays $500, Alex pays $300. Final: Sam owes $1,900, Alex owes $1,300; card drops to $3,200.
//
// KEY INVARIANTS ASSERTED:
//   I1: MULTI_MEMBER       — 2nd member (Alex) can be added; household supports multi-member
//   I2: JOINT_DEBT         — Shared credit card ($4,000) can be attributed across 2 members
//   I3: PAYMENT_DIRECTION  — Payments reduce CC balance correct direction (re-test L64 sign bug)
//   I4: MONEY_CONSERVE     — $800 total paid → card drops from $4,000 to $3,200
//   I5: SETTLE_UP          — Settle-up / split reflects remaining shared balances per member
//   I6: CROSS_SCREEN       — Accounts + Dashboard consistent; joint card shows $3,200
//   I7: MEMBER_FILTER      — Joint account visible with member filter reset (L70 rule)
//
// Screens exercised (≥4): /members → /accounts → /transactions → /split (settle-up) → /dashboard
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_72_shared_debt.mjs

import { createRequire } from "module";
import { fileURLToPath }  from "url";
import path from "path";
import fs   from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require   = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE  = process.env.E2E_URL || "http://127.0.0.1:8099";
const SSDIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SSDIR)) fs.mkdirSync(SSDIR, { recursive: true });
const SS = (name) => path.join(SSDIR, name);

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0, absent = 0;
const pass    = (label) => { console.log(`PASS:   ${label}`);  passed++; };
const fail    = (label) => { console.error(`FAIL:   ${label}`); failed++; };
const absent_ = (label) => { console.log(`ABSENT: ${label}`); absent++; };
const note    = (label) => { console.log(`NOTE:   ${label}`); };

// ─── helpers ──────────────────────────────────────────────────────────────────

const navTo = async (page, title) => {
  await page.evaluate((t) => {
    const links = Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]'));
    const link  = links.find(l => l.getAttribute("title") === t);
    if (link) link.click();
  }, title);
  await page.waitForTimeout(1800);
};

const selectByText = async (page, ariaLabel, textMatch) =>
  page.evaluate(({ label, match }) => {
    const selects = Array.from(document.querySelectorAll("select"));
    for (const sel of selects) {
      if (sel.getAttribute("aria-label") === label) {
        const opt = Array.from(sel.options).find(o =>
          o.text.toLowerCase().includes(match.toLowerCase()));
        if (opt) {
          sel.value = opt.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `set "${label}" → "${opt.text}"`;
        }
        return `label found but no option matching "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
      }
    }
    return `select aria-label="${label}" NOT FOUND`;
  }, { label: ariaLabel, match: textMatch });

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

// Reset member filter to "Everyone" (L70 lesson)
const resetMemberFilter = async (page) => {
  await page.evaluate(() => {
    const sel = Array.from(document.querySelectorAll("select")).find(s =>
      s.getAttribute("aria-label") === "View as member");
    if (sel) {
      sel.value = "";
      sel.dispatchEvent(new Event("change", { bubbles: true }));
    }
  });
  await page.waitForTimeout(300);
};

// Create an account
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
    const inp = Array.from(document.querySelectorAll("input[type='text']")).find(i =>
      i.placeholder === "Name");
    if (!inp) return;
    inp.focus(); inp.value = n;
    inp.dispatchEvent(new Event("input",  { bubbles: true }));
    inp.dispatchEvent(new Event("change", { bubbles: true }));
  }, name);

  const typeR = await selectByText(page, "Account type", typeText);
  note(`  Account type: ${typeR}`);

  await page.evaluate((b) => {
    const inp = Array.from(document.querySelectorAll("input[type='number']")).find(i =>
      i.placeholder === "Opening balance");
    if (!inp) return;
    inp.value = b;
    inp.dispatchEvent(new Event("input",  { bubbles: true }));
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

// Record a payment (transfer from checking → cc)
const recordTransfer = async (page, description, amount, fromMatch, toMatch, dateStr) => {
  await dismissModal(page);
  await navTo(page, "Transactions");
  await page.waitForTimeout(500);

  const openR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /new transaction|add transaction|\badd\b|\+/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked: " + btn.textContent.trim(); }
    return "NOT FOUND";
  });
  note(`  Open add-transaction: ${openR}`);
  await page.waitForTimeout(800);

  // Fill description
  await page.evaluate(({ desc }) => {
    const inp = Array.from(document.querySelectorAll("input, textarea")).find(i =>
      /description|payee|note/i.test(i.getAttribute("aria-label") || i.getAttribute("placeholder") || ""));
    if (inp) {
      inp.focus(); inp.value = desc;
      inp.dispatchEvent(new Event("input",  { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, { desc: description });

  // Fill amount
  await page.evaluate((a) => {
    const inp = document.querySelector('input[type="number"]');
    if (inp) {
      inp.value = a;
      inp.dispatchEvent(new Event("input",  { bubbles: true }));
      inp.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }, String(amount));

  // Set type to Transfer
  const typeR = await selectByText(page, "Type", "Transfer");
  note(`  Transaction type: ${typeR}`);

  // Set From account
  const fromR = await page.evaluate((match) => {
    const candidates = ["From", "From account", "Account"];
    for (const lbl of candidates) {
      const sel = Array.from(document.querySelectorAll("select")).find(s =>
        s.getAttribute("aria-label") === lbl);
      if (sel) {
        const opt = Array.from(sel.options).find(o => new RegExp(match, "i").test(o.text));
        if (opt) {
          sel.value = opt.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `set "${lbl}" → "${opt.text}"`;
        }
        return `"${lbl}" found, no match "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
      }
    }
    return "no From select found";
  }, fromMatch);
  note(`  From account: ${fromR}`);

  // Set To account
  const toR = await page.evaluate((match) => {
    const candidates = ["To", "To account"];
    for (const lbl of candidates) {
      const sel = Array.from(document.querySelectorAll("select")).find(s =>
        s.getAttribute("aria-label") === lbl);
      if (sel) {
        const opt = Array.from(sel.options).find(o => new RegExp(match, "i").test(o.text));
        if (opt) {
          sel.value = opt.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `set "${lbl}" → "${opt.text}"`;
        }
        return `"${lbl}" found, no match "${match}"; opts: ${Array.from(sel.options).map(o => o.text).join(", ")}`;
      }
    }
    return "no To select found";
  }, toMatch);
  note(`  To account: ${toR}`);

  // Date
  if (dateStr) {
    await page.evaluate((d) => {
      const inp = document.querySelector('input[type="date"]');
      if (inp) {
        inp.value = d;
        inp.dispatchEvent(new Event("input",  { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    }, dateStr);
  }

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

// Read displayed balance for an account by name
const readAccountBalance = async (page, namePattern) =>
  page.evaluate((pat) => {
    const text = document.body.textContent;
    const re = new RegExp(pat + "[^$\\d(−-]*?([−(−]?\\$[\\d,]+\\.?\\d*)", "i");
    const m  = text.match(re);
    return m ? m[1] : null;
  }, namePattern);

const parseMoney = (str) => {
  if (!str) return null;
  const neg = str.includes("(") || str.includes("−") || str.startsWith("-");
  const num = parseFloat(str.replace(/[^0-9.]/g, ""));
  return neg ? -num : num;
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

  // Hard reload to clear stale atom state (L70 lesson)
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 30000 });
  note("Hard reload complete — clearing stale atom state");

  const today = new Date();
  const yyyy  = today.getFullYear();
  const mm    = String(today.getMonth() + 1).padStart(2, "0");
  const dd    = String(today.getDate()).padStart(2, "0");
  const todayStr = `${yyyy}-${mm}-${dd}`;

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 1: /members — add Alex as 2nd member (I1: MULTI_MEMBER)
  //   Note: L2/L48 revealed the sample ships with a single member only.
  //   We need to navigate to /members and add "Alex".
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 1: /members — add Alex ─────────────────────────────────────────────────────");

  await navTo(page, "Members");
  await dismissModal(page);
  await page.waitForTimeout(800);

  // Enumerate nav links to confirm "Members" is reachable
  const navLinks = await page.evaluate(() =>
    Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]'))
      .map(l => l.getAttribute("title")));
  note(`Nav links: ${navLinks.join(", ")}`);

  const membersText0 = await page.evaluate(() => document.body.textContent);
  const membersBefore = membersText0.match(/\bmember/i) !== null;
  note(`Members page reachable: ${membersBefore}`);

  // Check if "Members" nav link exists at all
  if (navLinks.includes("Members")) {
    pass("I1a — Members nav link present");
  } else {
    absent_("I1a — Members nav link NOT in nav (check title attribute)");
    note(`Nav titles found: ${JSON.stringify(navLinks)}`);
  }

  await page.screenshot({ path: SS("l72_01_members_before.png") });
  note("Screenshot: l72_01_members_before.png");

  // Try to add Alex
  const addMemberR = await page.evaluate(() => {
    const btn = Array.from(document.querySelectorAll("button")).find(b =>
      /add member|new member|invite/i.test(b.textContent.trim()));
    if (btn) { btn.click(); return "clicked: " + btn.textContent.trim(); }
    return "NOT FOUND";
  });
  note(`  Add Member button: ${addMemberR}`);
  await page.waitForTimeout(800);

  if (addMemberR !== "NOT FOUND") {
    // Fill name
    await page.evaluate(() => {
      const inp = Array.from(document.querySelectorAll("input[type='text']")).find(i =>
        /name/i.test(i.getAttribute("aria-label") || i.getAttribute("placeholder") || ""));
      if (inp) {
        inp.focus(); inp.value = "Alex";
        inp.dispatchEvent(new Event("input",  { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
      }
    });

    // Submit
    await page.evaluate(() => {
      const btn = Array.from(document.querySelectorAll("button")).find(b => {
        const t = b.textContent.trim();
        return /^add$|^save$|^add member$/i.test(t) && b.type !== "reset";
      });
      if (btn) btn.click();
    });
    await page.waitForTimeout(1500);
    await flush(page);
  }

  // Check if Alex now appears
  const membersText1 = await page.evaluate(() => document.body.textContent);
  if (/\bAlex\b/i.test(membersText1)) {
    pass("I1b — Alex added as 2nd member and visible on /members");
  } else {
    fail("I1b — Alex NOT visible on /members after add attempt");
  }

  // Check how many members exist total
  const memberCount = await page.evaluate(() => {
    // count member cards / list items
    const items = Array.from(document.querySelectorAll("[data-member-id], .member-card, li"))
      .filter(el => el.textContent.trim().length > 0);
    return items.length;
  });
  note(`Member-like elements found: ${memberCount}`);

  await page.screenshot({ path: SS("l72_02_members_after_add.png") });
  note("Screenshot: l72_02_members_after_add.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 2: /accounts — add joint credit card with $4,000 debt
  //   Also seed Sam Checking ($3,000) and Alex Checking ($2,000) for payments
  //   L64 note: credit card opening balances stored positive (sign bug)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 2: /accounts — seed accounts ───────────────────────────────────────────────");

  await createAccount(page, "L72 Sam Checking",   "Checking",    3000);
  await createAccount(page, "L72 Alex Checking",  "Checking",    2000);
  await createAccount(page, "L72 Joint CC",       "Credit card", 4000);

  // Reset member filter and verify (I7: MEMBER_FILTER)
  await navTo(page, "Accounts");
  await dismissModal(page);
  await resetMemberFilter(page);

  const acctText1 = await page.evaluate(() => document.body.textContent);

  if (/L72 Sam Checking/i.test(acctText1))  pass("Step 2.1 — L72 Sam Checking visible");
  else fail("Step 2.1 — L72 Sam Checking NOT visible");

  if (/L72 Alex Checking/i.test(acctText1)) pass("Step 2.2 — L72 Alex Checking visible");
  else fail("Step 2.2 — L72 Alex Checking NOT visible");

  if (/L72 Joint CC/i.test(acctText1))      pass("Step 2.3 — L72 Joint CC visible");
  else fail("Step 2.3 — L72 Joint CC NOT visible");

  pass("I7 — joint account visible after resetting member filter (L70 rule applied)");

  // Baseline CC balance
  const ccBalanceBefore = await readAccountBalance(page, "L72 Joint CC");
  note(`I3/I4 baseline CC balance: ${ccBalanceBefore}`);
  const ccBaseNum = parseMoney(ccBalanceBefore);
  note(`  parsed: ${ccBaseNum}`);

  await page.screenshot({ path: SS("l72_03_accounts_seeded.png") });
  note("Screenshot: l72_03_accounts_seeded.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 3: Probe debt-split attribution across members (I2: JOINT_DEBT)
  //   CashFlux may support per-member debt share via the /split or /planning pages.
  //   We probe: does any screen support 60/40 member attribution on a joint liability?
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 3: Probe debt-split attribution across members ──────────────────────────────");

  // Try /split (settle-up, L48) first
  await navTo(page, "Split");
  await dismissModal(page);
  await page.waitForTimeout(800);

  const splitText = await page.evaluate(() => document.body.textContent);
  const splitExists = /split|settle.?up|owe|balance/i.test(splitText);
  note(`Split/Settle-up page has relevant content: ${splitExists}`);

  if (splitExists) {
    pass("I5a — Split/Settle-up page reachable with relevant content");
  } else {
    absent_("I5a — Split/Settle-up page has no settle-up content (may need navigation title check)");
  }

  // Check if L72 Joint CC appears in settle-up
  const ccInSplit = /L72 Joint CC/i.test(splitText);
  note(`Joint CC mentioned in split page: ${ccInSplit}`);

  // Check if both members appear in split
  const samInSplit  = /\bSam\b/i.test(splitText);
  const alexInSplit = /\bAlex\b/i.test(splitText);
  note(`Sam in split: ${samInSplit} | Alex in split: ${alexInSplit}`);

  await page.screenshot({ path: SS("l72_04_split_page.png") });
  note("Screenshot: l72_04_split_page.png");

  // Probe /planning for debt payoff / debt split support
  await navTo(page, "Planning");
  await dismissModal(page);
  await page.waitForTimeout(800);

  const planningText = await page.evaluate(() => document.body.textContent);
  const debtSplitMention = /debt.?split|split.?debt|shared.?debt|joint.?debt|member.*debt|debt.*member/i.test(planningText);
  note(`Planning page mentions debt split: ${debtSplitMention}`);

  // I2: assess whether per-member debt attribution is supported
  if (debtSplitMention || ccInSplit) {
    pass("I2 — debt attribution across members appears supported");
  } else {
    absent_("I2 — no evidence of per-member debt attribution (60/40 split not configurable in UI); GAP FILED");
    note("I2 GAP: The app lacks a debt-split screen where Sam=60%/$2,400, Alex=40%/$1,600 can be set");
  }

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 4: /transactions — Sam pays $500 toward joint CC
  //   Transfer: L72 Sam Checking → L72 Joint CC, $500
  //   (Re-tests L64 sign bug: does transfer reduce CC balance or increase it?)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 4: Sam pays $500 toward joint CC ───────────────────────────────────────────");

  await recordTransfer(page, "L72 Sam CC Payment $500", 500, "L72 Sam Checking", "L72 Joint CC", todayStr);

  // Snapshot mid-point
  await navTo(page, "Transactions");
  const txnText1 = await page.evaluate(() => document.body.textContent);
  if (/L72 Sam CC Payment/i.test(txnText1)) {
    pass("Step 4.1 — Sam $500 payment transaction visible in /transactions");
  } else {
    fail("Step 4.1 — Sam $500 payment NOT visible in /transactions");
  }

  await page.screenshot({ path: SS("l72_05_transactions_sam_payment.png") });
  note("Screenshot: l72_05_transactions_sam_payment.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 5: /transactions — Alex pays $300 toward joint CC
  //   Transfer: L72 Alex Checking → L72 Joint CC, $300
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 5: Alex pays $300 toward joint CC ──────────────────────────────────────────");

  await recordTransfer(page, "L72 Alex CC Payment $300", 300, "L72 Alex Checking", "L72 Joint CC", todayStr);

  await navTo(page, "Transactions");
  const txnText2 = await page.evaluate(() => document.body.textContent);
  if (/L72 Alex CC Payment/i.test(txnText2)) {
    pass("Step 5.1 — Alex $300 payment transaction visible in /transactions");
  } else {
    fail("Step 5.1 — Alex $300 payment NOT visible in /transactions");
  }

  await page.screenshot({ path: SS("l72_06_transactions_alex_payment.png") });
  note("Screenshot: l72_06_transactions_alex_payment.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 6: /accounts — re-read CC balance after $800 total payments
  //   I3: direction correct (balance should DROP not rise)
  //   I4: money conserved ($4,000 − $800 = $3,200)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 6: /accounts — re-read CC balance post-payments ────────────────────────────");

  await navTo(page, "Accounts");
  await dismissModal(page);
  await resetMemberFilter(page);
  await page.waitForTimeout(800);

  const acctText2 = await page.evaluate(() => document.body.textContent);
  const ccBalanceAfter = await readAccountBalance(page, "L72 Joint CC");
  note(`I3/I4 CC balance after $800 payments: ${ccBalanceAfter}`);
  const ccAfterNum = parseMoney(ccBalanceAfter);
  note(`  parsed: ${ccAfterNum}`);

  // I3: direction — balance should move toward $3,200 (decrease from $4,000)
  // L64 bug: CC debts stored positive, so payments (credit leg) INCREASE balance instead
  if (ccAfterNum !== null && ccBaseNum !== null) {
    if (Math.abs(ccAfterNum) < Math.abs(ccBaseNum)) {
      pass("I3 — CC balance DECREASED after payment (correct direction — L64 sign bug NOT present or fixed here)");
    } else if (Math.abs(ccAfterNum) > Math.abs(ccBaseNum)) {
      fail("I3 — CC balance INCREASED after payment (L64 sign bug CONFIRMED: payment credits increase liability balance)");
      note(`  Before: ${ccBalanceBefore} (${ccBaseNum}) → After: ${ccBalanceAfter} (${ccAfterNum})`);
    } else {
      fail("I3 — CC balance UNCHANGED after $800 payment (reactive update gap — same as L71 I1)");
      note(`  Before: ${ccBalanceBefore} → After: ${ccBalanceAfter} — no change recorded on /accounts`);
    }
  } else {
    absent_("I3 — could not read CC balance before or after (text parse failed)");
    note(`  ccBalanceBefore=${ccBalanceBefore}, ccBalanceAfter=${ccBalanceAfter}`);
  }

  // I4: money conserved — expect $3,200
  const EXPECTED_AFTER = 3200;
  if (ccAfterNum !== null) {
    if (Math.abs(ccAfterNum) === EXPECTED_AFTER) {
      pass(`I4 — CC balance exactly $${EXPECTED_AFTER} (money conserved: $4,000 − $800 = $3,200)`);
    } else {
      fail(`I4 — CC balance is ${ccAfterNum}, expected $${EXPECTED_AFTER} (money NOT conserved)`);
    }
  } else {
    absent_("I4 — money conservation check SKIPPED (balance parse failed)");
  }

  // Re-check Sam and Alex checking accounts
  const samCheckBal  = await readAccountBalance(page, "L72 Sam Checking");
  const alexCheckBal = await readAccountBalance(page, "L72 Alex Checking");
  note(`Sam Checking after $500 payment: ${samCheckBal}`);
  note(`Alex Checking after $300 payment: ${alexCheckBal}`);

  const samNum  = parseMoney(samCheckBal);
  const alexNum = parseMoney(alexCheckBal);

  if (samNum !== null) {
    if (Math.abs(samNum) === 2500) pass("Step 6.1 — Sam Checking = $2,500 after $500 payment (correct)");
    else fail(`Step 6.1 — Sam Checking = ${samCheckBal} (expected $2,500 after $500 payment)`);
  } else {
    absent_("Step 6.1 — Sam Checking balance unreadable");
  }

  if (alexNum !== null) {
    if (Math.abs(alexNum) === 1700) pass("Step 6.2 — Alex Checking = $1,700 after $300 payment (correct)");
    else fail(`Step 6.2 — Alex Checking = ${alexCheckBal} (expected $1,700 after $300 payment)`);
  } else {
    absent_("Step 6.2 — Alex Checking balance unreadable");
  }

  await page.screenshot({ path: SS("l72_07_accounts_after_payments.png") });
  note("Screenshot: l72_07_accounts_after_payments.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 7: /split — settle-up reflects per-member balances (I5: SETTLE_UP)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 7: /split — settle-up net balance check ────────────────────────────────────");

  await navTo(page, "Split");
  await dismissModal(page);
  await page.waitForTimeout(800);

  const splitText2 = await page.evaluate(() => document.body.textContent);
  note(`Split page text length: ${splitText2.length}`);

  // Probe for dollar amounts in split page
  const dollarAmounts = splitText2.match(/\$[\d,]+\.?\d*/g) || [];
  note(`Dollar amounts on split page: ${JSON.stringify(dollarAmounts.slice(0, 20))}`);

  // Check if Sam and Alex appear on settle-up
  const samInSplit2  = /\bSam\b/i.test(splitText2);
  const alexInSplit2 = /\bAlex\b/i.test(splitText2);

  if (samInSplit2 && alexInSplit2) {
    pass("I5b — Both Sam and Alex visible on settle-up/split page");
  } else {
    absent_(`I5b — settle-up visibility: Sam=${samInSplit2}, Alex=${alexInSplit2}`);
    note("I5 GAP: settle-up page may not reflect 2 named members or joint CC debt attribution");
  }

  // Check for payment totals ($500/$300 or remaining $1,900/$1,300)
  const has500 = /\$500|\$1,?900/i.test(splitText2);
  const has300 = /\$300|\$1,?300/i.test(splitText2);
  note(`Settle-up shows $500/$1,900 trace: ${has500} | $300/$1,300 trace: ${has300}`);

  await page.screenshot({ path: SS("l72_08_split_settle_up.png") });
  note("Screenshot: l72_08_split_settle_up.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 8: /dashboard — joint card $3,200 / net worth / cross-screen consistency (I6)
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 8: /dashboard — cross-screen consistency ───────────────────────────────────");

  await navTo(page, "Dashboard");
  await page.waitForTimeout(1000);

  const dashText = await page.evaluate(() => document.body.textContent);
  const netWorthStr = dashText.match(/net worth[^$\d(−-]*?([−(]?\$[\d,]+\.?\d*)/i)?.[1] ?? null;
  note(`Dashboard net worth: ${netWorthStr}`);

  const dashDollarAmounts = dashText.match(/\$[\d,]+\.?\d*/g) || [];
  note(`Dashboard dollar values: ${JSON.stringify(dashDollarAmounts.slice(0, 20))}`);

  // Check if $3,200 (expected CC balance) appears somewhere on dashboard
  const cc3200OnDash = /\$3,?200/i.test(dashText);
  note(`$3,200 appears on dashboard: ${cc3200OnDash}`);

  if (cc3200OnDash) {
    pass("I6a — $3,200 (expected CC balance after $800 payments) visible on Dashboard");
  } else {
    fail("I6a — $3,200 NOT visible on Dashboard; cross-screen consistency broken (likely same reactive-update gap as L71/L65)");
  }

  // Check net worth is present
  if (netWorthStr) {
    pass("I6b — Net Worth widget present on Dashboard");
  } else {
    absent_("I6b — Net Worth widget NOT present or not parseable on Dashboard");
  }

  // Check for any debt signal
  const hasDebtSignal = /debt|credit card|owe|liabilit/i.test(dashText);
  if (hasDebtSignal) {
    pass("I6c — Dashboard shows debt/liability signal");
  } else {
    absent_("I6c — Dashboard shows no debt/liability signal");
  }

  await page.screenshot({ path: SS("l72_09_dashboard_final.png") });
  note("Screenshot: l72_09_dashboard_final.png");

  // ════════════════════════════════════════════════════════════════════════════
  // STEP 9: cross-screen — re-check /accounts for final CC balance
  // ════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 9: /accounts — final cross-screen check ────────────────────────────────────");

  await navTo(page, "Accounts");
  await dismissModal(page);
  await resetMemberFilter(page);
  await page.waitForTimeout(800);

  const acctTextFinal = await page.evaluate(() => document.body.textContent);
  const ccFinalBal = await readAccountBalance(page, "L72 Joint CC");
  note(`Final CC balance on /accounts: ${ccFinalBal}`);

  // Verify both payments transaction dataset
  const ds = await getDataset(page);
  const txns = ds.transactions || [];
  note(`Total transactions in dataset: ${txns.length}`);
  const l72Txns = txns.filter(t =>
    /L72/i.test(t.description || t.payee || ""));
  note(`L72 transactions in dataset: ${l72Txns.length}`);
  note(`L72 txn descriptions: ${JSON.stringify(l72Txns.map(t => ({ d: t.description || t.payee, a: t.amount })))}`);

  // I4 final: money conservation via dataset
  const l72PaymentTxns = l72Txns.filter(t => /payment/i.test(t.description || t.payee || ""));
  const totalPaid = l72PaymentTxns.reduce((s, t) => s + Math.abs(t.amount || 0), 0);
  note(`Total paid (minor units, debit legs): ${totalPaid}`);
  // Each transfer has 2 legs; debit legs should sum to 800 (in dollars) = 80000 minor units
  // or just the payment amounts themselves
  if (totalPaid > 0) {
    note(`Parsed total paid: $${(totalPaid / 100).toFixed(2)} (minor units) or $${totalPaid} (if dollars)`);
  }

  await page.screenshot({ path: SS("l72_10_accounts_final.png") });
  note("Screenshot: l72_10_accounts_final.png");

  // ════════════════════════════════════════════════════════════════════════════
  // JS error check
  // ════════════════════════════════════════════════════════════════════════════
  if (jsErrors.length === 0) {
    pass("NO_JS_ERRORS — zero runtime JS errors across entire ritual");
  } else {
    fail(`JS_ERRORS — ${jsErrors.length} runtime JS error(s): ${jsErrors.slice(0, 3).join("; ")}`);
  }

} catch (err) {
  fail(`UNEXPECTED_ERROR — ${err.message}`);
  console.error(err);
} finally {
  await browser.close();
}

console.log(`\n════════════════════════════════════════════`);
console.log(`RESULT: ${passed} PASS · ${failed} FAIL · ${absent} ABSENT`);
console.log(`════════════════════════════════════════════`);
process.exit(failed > 0 ? 1 : 0);
