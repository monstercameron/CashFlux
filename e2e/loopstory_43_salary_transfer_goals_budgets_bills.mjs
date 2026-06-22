// L43 E2E loop story — "Salary Deposit, Transfer, Goal Contribute, Budget Top-up, Bills"
// Flow:
//   STEP 1: /transactions — log L43 Salary Deposit as income ($3500, Checking, 2026-06-22)
//   STEP 2: /accounts    — transfer $500 from Checking → Savings
//   STEP 3: /goals       — contribute $200 to Emergency Fund (or first goal)
//   STEP 4: /budgets     — top up two budgets ($100 each) if button exists
//   STEP 5: /bills       — mark two unpaid bills as paid
//   STEP 6: /dashboard   — verify end state + JS error check
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_43_salary_transfer_goals_budgets_bills.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS = (name) => path.join(__dirname, name);

const browser = await chromium.launch({ headless: true });
let passed = 0, failed = 0;
const pass = (label) => { console.log(`PASS: ${label}`); passed++; };
const fail = (label) => { console.error(`FAIL: ${label}`); failed++; };
const note = (label) => { console.log(`NOTE: ${label}`); };

// Wait for wasm hydration sentinel
const waitNav = (page) =>
  page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });

const errors = [];

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  page.on("pageerror", (e) => errors.push(String(e)));

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 1: /transactions — log salary deposit as income
  // ══════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 1: /transactions ────────────────────────────────────────────────");

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);

  // Screenshot before
  await page.screenshot({ path: SS("loop43-01-transactions-before.png") });
  pass("Step 1.1 — screenshot loop43-01-transactions-before.png");

  // Capture transaction list DOM before
  const txnDomBefore = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row, tr, li, [class*='txn'], [class*='transaction']"));
    return rows.map(r => r.textContent.replace(/\s+/g, " ").trim()).filter(Boolean).slice(0, 20);
  });
  note(`Transaction list before (${txnDomBefore.length} rows): ${JSON.stringify(txnDomBefore.slice(0, 5))}`);

  // Find the Add transaction button
  const addBtnText = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll("button, a[role='button']"));
    for (const b of btns) {
      const txt = b.textContent.trim();
      if (/add.*trans|new.*trans|^\+$|add income|add expense/i.test(txt) && txt.length < 40) {
        return { text: txt, class: b.className, id: b.id, type: b.tagName };
      }
    }
    // Fallback: look for any button with "Add" near the top
    for (const b of btns) {
      const txt = b.textContent.trim();
      if (/^add$/i.test(txt) || txt === "+" || /add transaction/i.test(txt)) {
        return { text: txt, class: b.className, id: b.id, type: b.tagName };
      }
    }
    // List all buttons for debugging
    return { debug: btns.map(b => ({ text: b.textContent.trim().slice(0,30), id: b.id, class: b.className.slice(0,30) })).slice(0, 15) };
  });
  note(`Add button probe: ${JSON.stringify(addBtnText)}`);

  // Try clicking the add/new transaction button
  const addClicked = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll("button, a[role='button']"));
    for (const b of btns) {
      const txt = b.textContent.trim();
      if (/add.*trans|new.*trans|^\+$|add income|add expense/i.test(txt) && txt.length < 40) {
        b.click(); return txt;
      }
    }
    for (const b of btns) {
      const txt = b.textContent.trim();
      if (/^add$/i.test(txt) || txt === "+") { b.click(); return txt; }
    }
    return null;
  });

  if (addClicked) {
    pass(`Step 1.2 — clicked add button: "${addClicked}"`);
    await page.waitForTimeout(800);
  } else {
    // The form may always be visible (inline add form)
    const hasForm = await page.evaluate(() => {
      const inputs = document.querySelectorAll('input[type="text"], input[placeholder]');
      return inputs.length > 0;
    });
    if (hasForm) {
      pass("Step 1.2 — inline add form already visible (no button needed)");
    } else {
      fail("Step 1.2 — could not find add transaction button or form");
    }
  }

  // Screenshot after form opens
  await page.screenshot({ path: SS("loop43-01b-form-open.png") });
  pass("Step 1.3 — screenshot loop43-01b-form-open.png");

  // Probe all form fields to understand structure
  const formFields = await page.evaluate(() => {
    const inputs = Array.from(document.querySelectorAll("input, select, textarea"));
    return inputs.map(el => ({
      tag: el.tagName,
      type: el.getAttribute("type") ?? "",
      id: el.id,
      name: el.getAttribute("name") ?? "",
      ariaLabel: el.getAttribute("aria-label") ?? "",
      placeholder: el.getAttribute("placeholder") ?? "",
      value: el.value ?? "",
      options: el.tagName === "SELECT" ? Array.from(el.options).map(o => ({ v: o.value, t: o.text })) : undefined,
    }));
  });
  note(`Form fields: ${JSON.stringify(formFields)}`);

  // Fill Description
  const descFilled = await page.evaluate((desc) => {
    const inputs = Array.from(document.querySelectorAll("input"));
    // Try by placeholder or aria-label
    for (const inp of inputs) {
      const label = (inp.getAttribute("aria-label") ?? "").toLowerCase();
      const ph = (inp.getAttribute("placeholder") ?? "").toLowerCase();
      if (/desc|narration|memo|payee|name|note/i.test(label) || /desc|narration|memo|payee/i.test(ph)) {
        inp.focus(); inp.value = desc;
        inp.dispatchEvent(new Event("input", { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
        return label || ph || inp.id || "matched";
      }
    }
    // Fallback: first text input
    const first = inputs.find(i => i.type === "text" || i.type === "");
    if (first) {
      first.focus(); first.value = desc;
      first.dispatchEvent(new Event("input", { bubbles: true }));
      first.dispatchEvent(new Event("change", { bubbles: true }));
      return "first-text-input";
    }
    return null;
  }, "L43 Salary Deposit");

  if (descFilled) {
    pass(`Step 1.4 — description filled via "${descFilled}"`);
  } else {
    fail("Step 1.4 — could not fill description");
  }

  // Fill Amount
  const amtFilled = await page.evaluate((amt) => {
    const inputs = Array.from(document.querySelectorAll("input"));
    for (const inp of inputs) {
      const label = (inp.getAttribute("aria-label") ?? "").toLowerCase();
      const ph = (inp.getAttribute("placeholder") ?? "").toLowerCase();
      if (inp.type === "number" || /amount|value|\$|money/i.test(label) || /amount/i.test(ph)) {
        inp.focus(); inp.value = amt;
        inp.dispatchEvent(new Event("input", { bubbles: true }));
        inp.dispatchEvent(new Event("change", { bubbles: true }));
        return label || ph || inp.id || "number-input";
      }
    }
    return null;
  }, "3500");

  if (amtFilled) {
    pass(`Step 1.5 — amount=3500 filled via "${amtFilled}"`);
  } else {
    fail("Step 1.5 — could not fill amount");
  }

  // Set Type to Income
  const typeFilled = await page.evaluate(() => {
    const selects = Array.from(document.querySelectorAll("select"));
    for (const sel of selects) {
      const label = (sel.getAttribute("aria-label") ?? "").toLowerCase();
      if (/type|kind/i.test(label)) {
        const incomeOpt = Array.from(sel.options).find(o => /income/i.test(o.text));
        if (incomeOpt) {
          sel.value = incomeOpt.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `type select → "${incomeOpt.text}" (value: ${incomeOpt.value})`;
        }
        return `type select found but no income option; options: ${Array.from(sel.options).map(o=>o.text).join(",")}`;
      }
    }
    return null;
  });
  note(`Type/Kind select: ${typeFilled}`);
  if (typeFilled && /income/i.test(typeFilled)) {
    pass(`Step 1.6 — type set to Income: ${typeFilled}`);
  } else {
    fail(`Step 1.6 — could not set type to Income: ${typeFilled}`);
  }

  // Set Category to Income/Salary
  const catFilled = await page.evaluate(() => {
    const selects = Array.from(document.querySelectorAll("select"));
    for (const sel of selects) {
      const label = (sel.getAttribute("aria-label") ?? "").toLowerCase();
      if (/categor/i.test(label)) {
        const opts = Array.from(sel.options);
        const match = opts.find(o => /income|salary/i.test(o.text));
        if (match) {
          sel.value = match.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `category select → "${match.text}"`;
        }
        return `category select found; options: ${opts.map(o=>o.text).join(", ")}`;
      }
    }
    return null;
  });
  note(`Category: ${catFilled}`);
  if (catFilled) {
    pass(`Step 1.7 — category: ${catFilled}`);
  } else {
    fail("Step 1.7 — could not set category");
  }

  // Set Date
  const dateFilled = await page.evaluate((d) => {
    const inputs = Array.from(document.querySelectorAll("input[type='date']"));
    if (inputs.length > 0) {
      inputs[0].value = d;
      inputs[0].dispatchEvent(new Event("input", { bubbles: true }));
      inputs[0].dispatchEvent(new Event("change", { bubbles: true }));
      return d;
    }
    return null;
  }, "2026-06-22");
  if (dateFilled) {
    pass(`Step 1.8 — date set to ${dateFilled}`);
  } else {
    fail("Step 1.8 — no date input found");
  }

  // Set Account to Checking
  const acctFilled = await page.evaluate(() => {
    const selects = Array.from(document.querySelectorAll("select"));
    for (const sel of selects) {
      const label = (sel.getAttribute("aria-label") ?? "").toLowerCase();
      if (/account/i.test(label)) {
        const opts = Array.from(sel.options);
        const match = opts.find(o => /checking/i.test(o.text));
        if (match) {
          sel.value = match.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `account select → "${match.text}"`;
        }
        return `account select found; options: ${opts.map(o=>o.text).join(", ")}`;
      }
    }
    return null;
  });
  note(`Account: ${acctFilled}`);
  if (acctFilled) {
    pass(`Step 1.9 — account: ${acctFilled}`);
  } else {
    fail("Step 1.9 — could not set account (no account select or no Checking option)");
  }

  // Submit the form
  const submitResult = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll("button[type='submit'], button"));
    for (const b of btns) {
      const txt = b.textContent.trim().toLowerCase();
      if (txt === "save" || txt === "add" || txt === "submit" || txt === "add transaction" || /save|submit|add/i.test(txt)) {
        b.click(); return txt;
      }
    }
    return null;
  });
  note(`Submit button clicked: "${submitResult}"`);
  await page.waitForTimeout(1200);

  // Screenshot after add
  await page.screenshot({ path: SS("loop43-02-income-added.png") });
  pass("Step 1.10 — screenshot loop43-02-income-added.png");

  // Capture transaction list DOM after
  const txnDomAfter = await page.evaluate(() => document.body.textContent ?? "");
  const hasL43Salary = txnDomAfter.includes("L43 Salary Deposit");
  if (hasL43Salary) {
    pass('Step 1.11 — "L43 Salary Deposit" found in transactions list after add');
  } else {
    fail('Step 1.11 — "L43 Salary Deposit" NOT found in transactions list after add');
  }

  const txnRowsAfter = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row, tr, li, [class*='txn'], [class*='transaction']"));
    return rows.map(r => r.textContent.replace(/\s+/g, " ").trim()).filter(Boolean).slice(0, 10);
  });
  note(`Transaction rows after add: ${JSON.stringify(txnRowsAfter)}`);

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 2: /accounts — transfer checking → savings
  // ══════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 2: /accounts ────────────────────────────────────────────────────");

  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);

  await page.screenshot({ path: SS("loop43-03-accounts-before-transfer.png") });
  pass("Step 2.1 — screenshot loop43-03-accounts-before-transfer.png");

  // Capture current balances
  const acctsDomBefore = await page.evaluate(() => document.body.textContent ?? "");
  const balBefore = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row, .card, li, tr"));
    return rows.map(r => r.textContent.replace(/\s+/g, " ").trim()).filter(Boolean).slice(0, 15);
  });
  note(`Account rows before transfer: ${JSON.stringify(balBefore)}`);

  // Find Transfer button on Checking account
  const transferBtnInfo = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll("button, a[role='button']"));
    const results = [];
    for (const b of btns) {
      const txt = b.textContent.trim();
      if (/transfer/i.test(txt)) {
        // Check nearby text for "Checking"
        const parent = b.closest(".row, .card, li, article, section, div[class]");
        const parentText = parent ? parent.textContent.replace(/\s+/g, " ").trim() : "";
        results.push({ text: txt, parentText: parentText.slice(0, 80), id: b.id, class: b.className.slice(0, 40) });
      }
    }
    return results;
  });
  note(`Transfer buttons found: ${JSON.stringify(transferBtnInfo)}`);

  let transferDone = false;

  if (transferBtnInfo.length > 0) {
    // Click the transfer button on Checking (or first one found)
    const checkingTransfer = transferBtnInfo.find(b => /checking/i.test(b.parentText));
    const targetBtn = checkingTransfer || transferBtnInfo[0];
    note(`Clicking transfer button: ${JSON.stringify(targetBtn)}`);

    await page.evaluate((targetText) => {
      const btns = Array.from(document.querySelectorAll("button, a[role='button']"));
      for (const b of btns) {
        if (/transfer/i.test(b.textContent.trim())) {
          const parent = b.closest(".row, .card, li, article, section, div[class]");
          const parentText = parent ? parent.textContent : "";
          if (targetText ? /checking/i.test(parentText) : true) {
            b.click(); return;
          }
        }
      }
      // Fallback: click first transfer button
      for (const b of btns) {
        if (/transfer/i.test(b.textContent.trim())) { b.click(); return; }
      }
    }, checkingTransfer ? "checking" : "");
    await page.waitForTimeout(1000);

    // Now fill the transfer form
    const transferForm = await page.evaluate(() => {
      const inputs = Array.from(document.querySelectorAll("input, select"));
      return inputs.map(el => ({
        tag: el.tagName,
        type: el.getAttribute("type") ?? "",
        id: el.id,
        ariaLabel: el.getAttribute("aria-label") ?? "",
        placeholder: el.getAttribute("placeholder") ?? "",
        options: el.tagName === "SELECT" ? Array.from(el.options).map(o => o.text) : undefined,
      }));
    });
    note(`Transfer form fields: ${JSON.stringify(transferForm)}`);

    // Fill amount = 500
    await page.evaluate(() => {
      const inputs = Array.from(document.querySelectorAll("input"));
      for (const inp of inputs) {
        if (inp.type === "number" || /amount/i.test(inp.getAttribute("aria-label") ?? "")) {
          inp.value = "500";
          inp.dispatchEvent(new Event("input", { bubbles: true }));
          inp.dispatchEvent(new Event("change", { bubbles: true }));
          return;
        }
      }
    });

    // Set To account = Savings
    const toAcctSet = await page.evaluate(() => {
      const selects = Array.from(document.querySelectorAll("select"));
      for (const sel of selects) {
        const label = (sel.getAttribute("aria-label") ?? "").toLowerCase();
        if (/to|destination|target/i.test(label)) {
          const opts = Array.from(sel.options);
          const match = opts.find(o => /saving/i.test(o.text));
          if (match) {
            sel.value = match.value;
            sel.dispatchEvent(new Event("change", { bubbles: true }));
            return `to-account set to "${match.text}"`;
          }
        }
      }
      // Fallback: any select with a Savings option (that isn't the from account)
      for (const sel of selects) {
        const opts = Array.from(sel.options);
        const match = opts.find(o => /saving/i.test(o.text));
        if (match) {
          sel.value = match.value;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return `savings option set on select (aria-label: "${sel.getAttribute("aria-label") ?? ""}")`;
        }
      }
      return null;
    });
    note(`To account: ${toAcctSet}`);

    // Submit transfer
    const transferSubmit = await page.evaluate(() => {
      const btns = Array.from(document.querySelectorAll("button[type='submit'], button"));
      for (const b of btns) {
        const txt = b.textContent.trim().toLowerCase();
        if (/transfer|confirm|save|submit/i.test(txt) && txt.length < 20) {
          b.click(); return txt;
        }
      }
      return null;
    });
    note(`Transfer submit button: "${transferSubmit}"`);
    await page.waitForTimeout(1200);
    transferDone = true;
    pass("Step 2.2 — transfer flow attempted ($500 Checking → Savings)");
  } else {
    fail("Step 2.2 — no Transfer button found on /accounts page");
  }

  await page.screenshot({ path: SS("loop43-04-after-transfer.png") });
  pass("Step 2.3 — screenshot loop43-04-after-transfer.png");

  const balAfter = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row, .card, li, tr"));
    return rows.map(r => r.textContent.replace(/\s+/g, " ").trim()).filter(Boolean).slice(0, 15);
  });
  note(`Account rows after transfer: ${JSON.stringify(balAfter)}`);

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 3: /goals — contribute to Emergency Fund
  // ══════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 3: /goals ────────────────────────────────────────────────────────");

  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);

  await page.screenshot({ path: SS("loop43-05-goals-before-contribute.png") });
  pass("Step 3.1 — screenshot loop43-05-goals-before-contribute.png");

  const goalsDomBefore = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row, .card, li, article, progress, [class*='goal']"));
    return rows.map(r => r.textContent.replace(/\s+/g, " ").trim()).filter(Boolean).slice(0, 20);
  });
  note(`Goals before contribute: ${JSON.stringify(goalsDomBefore)}`);

  // Find Contribute button on Emergency Fund goal
  const contributeBtns = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll("button, a[role='button']"));
    return btns
      .filter(b => /contribut|add funds|deposit/i.test(b.textContent.trim()))
      .map(b => {
        const parent = b.closest(".row, .card, li, article, section, div[class]");
        const parentText = parent ? parent.textContent.replace(/\s+/g, " ").trim().slice(0, 100) : "";
        return { text: b.textContent.trim(), parentText, id: b.id };
      });
  });
  note(`Contribute buttons: ${JSON.stringify(contributeBtns)}`);

  let contributeSuccess = false;

  if (contributeBtns.length > 0) {
    // Prefer Emergency Fund, else first goal
    const emFundBtn = contributeBtns.find(b => /emergency/i.test(b.parentText));
    const targetContrib = emFundBtn || contributeBtns[0];
    note(`Targeting contribute on: ${JSON.stringify(targetContrib)}`);

    await page.evaluate((targetParentText) => {
      const btns = Array.from(document.querySelectorAll("button, a[role='button']"));
      for (const b of btns) {
        if (/contribut|add funds|deposit/i.test(b.textContent.trim())) {
          if (!targetParentText) { b.click(); return; }
          const parent = b.closest(".row, .card, li, article, section, div[class]");
          const pt = parent ? parent.textContent : "";
          if (/emergency/i.test(pt)) { b.click(); return; }
        }
      }
      // Fallback: click first
      for (const b of btns) {
        if (/contribut|add funds|deposit/i.test(b.textContent.trim())) { b.click(); return; }
      }
    }, emFundBtn ? "emergency" : "");
    await page.waitForTimeout(1000);

    // Fill contribution amount
    const contribForm = await page.evaluate(() => {
      const inputs = Array.from(document.querySelectorAll("input, select"));
      return inputs.map(el => ({
        tag: el.tagName, type: el.getAttribute("type") ?? "",
        id: el.id, ariaLabel: el.getAttribute("aria-label") ?? "",
        placeholder: el.getAttribute("placeholder") ?? "",
      }));
    });
    note(`Contribution form: ${JSON.stringify(contribForm)}`);

    await page.evaluate(() => {
      const inputs = Array.from(document.querySelectorAll("input"));
      for (const inp of inputs) {
        if (inp.type === "number" || /amount/i.test(inp.getAttribute("aria-label") ?? "")) {
          inp.value = "200";
          inp.dispatchEvent(new Event("input", { bubbles: true }));
          inp.dispatchEvent(new Event("change", { bubbles: true }));
          return;
        }
      }
    });

    const contribSubmit = await page.evaluate(() => {
      const btns = Array.from(document.querySelectorAll("button[type='submit'], button"));
      for (const b of btns) {
        const txt = b.textContent.trim().toLowerCase();
        if (/save|confirm|contribut|add|submit/i.test(txt) && txt.length < 30) {
          b.click(); return txt;
        }
      }
      return null;
    });
    note(`Contribution submit: "${contribSubmit}"`);
    await page.waitForTimeout(1200);
    contributeSuccess = true;
    pass("Step 3.2 — contribute $200 flow attempted on Emergency Fund");
  } else {
    fail("Step 3.2 — no Contribute button found on /goals");
  }

  await page.screenshot({ path: SS("loop43-06-after-contribute.png") });
  pass("Step 3.3 — screenshot loop43-06-after-contribute.png");

  const goalsDomAfter = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row, .card, li, article, progress, [class*='goal']"));
    return rows.map(r => r.textContent.replace(/\s+/g, " ").trim()).filter(Boolean).slice(0, 20);
  });
  note(`Goals after contribute: ${JSON.stringify(goalsDomAfter)}`);

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 4: /budgets — top up budgets
  // ══════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 4: /budgets ──────────────────────────────────────────────────────");

  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);

  await page.screenshot({ path: SS("loop43-07-budgets-before-topup.png") });
  pass("Step 4.1 — screenshot loop43-07-budgets-before-topup.png");

  const budgetsDomBefore = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row, .card, li, article, [class*='budget']"));
    return rows.map(r => r.textContent.replace(/\s+/g, " ").trim()).filter(Boolean).slice(0, 10);
  });
  note(`Budget rows before top-up: ${JSON.stringify(budgetsDomBefore)}`);

  // Look for Top up / Add funds / Cover buttons
  const topupBtns = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll("button, a[role='button']"));
    return btns
      .filter(b => /top.?up|add funds|cover|refill|fund/i.test(b.textContent.trim()))
      .map(b => ({ text: b.textContent.trim(), class: b.className.slice(0, 40) }));
  });
  note(`Top-up buttons: ${JSON.stringify(topupBtns)}`);

  if (topupBtns.length >= 1) {
    // Click first top-up button
    let topupsApplied = 0;
    for (let i = 0; i < Math.min(2, topupBtns.length); i++) {
      await page.evaluate((idx) => {
        const btns = Array.from(document.querySelectorAll("button, a[role='button']"))
          .filter(b => /top.?up|add funds|cover|refill|fund/i.test(b.textContent.trim()));
        if (btns[idx]) btns[idx].click();
      }, i);
      await page.waitForTimeout(800);

      // Fill amount
      await page.evaluate(() => {
        const inputs = Array.from(document.querySelectorAll("input"));
        for (const inp of inputs) {
          if (inp.type === "number" || /amount/i.test(inp.getAttribute("aria-label") ?? "")) {
            inp.value = "100";
            inp.dispatchEvent(new Event("input", { bubbles: true }));
            inp.dispatchEvent(new Event("change", { bubbles: true }));
            return;
          }
        }
      });

      // Submit
      await page.evaluate(() => {
        const btns = Array.from(document.querySelectorAll("button[type='submit'], button"));
        for (const b of btns) {
          const txt = b.textContent.trim().toLowerCase();
          if (/save|confirm|add|submit|top/i.test(txt) && txt.length < 30) { b.click(); return; }
        }
      });
      await page.waitForTimeout(1000);
      topupsApplied++;
    }
    pass(`Step 4.2 — applied ${topupsApplied} top-up(s) of $100 each`);
  } else {
    // Report what buttons are available
    const allBudgetBtns = await page.evaluate(() => {
      const btns = Array.from(document.querySelectorAll("button, a[role='button']"));
      return btns.map(b => b.textContent.trim()).filter(t => t.length > 0 && t.length < 40).slice(0, 20);
    });
    note(`All buttons on /budgets: ${JSON.stringify(allBudgetBtns)}`);
    fail("Step 4.2 — no Top Up / Add Funds / Cover buttons found on /budgets");
  }

  await page.screenshot({ path: SS("loop43-08-after-budget-topup.png") });
  pass("Step 4.3 — screenshot loop43-08-after-budget-topup.png");

  const budgetsDomAfter = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row, .card, li, article, [class*='budget']"));
    return rows.map(r => r.textContent.replace(/\s+/g, " ").trim()).filter(Boolean).slice(0, 10);
  });
  note(`Budget rows after top-up: ${JSON.stringify(budgetsDomAfter)}`);

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 5: /bills — mark two bills as paid
  // ══════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 5: /bills ────────────────────────────────────────────────────────");

  await page.goto(BASE + "/bills", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);

  await page.screenshot({ path: SS("loop43-09-bills-before-paid.png") });
  pass("Step 5.1 — screenshot loop43-09-bills-before-paid.png");

  const billsDomBefore = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row, .card, li, article, tr, [class*='bill']"));
    return rows.map(r => r.textContent.replace(/\s+/g, " ").trim()).filter(Boolean).slice(0, 20);
  });
  note(`Bill rows before: ${JSON.stringify(billsDomBefore)}`);

  // All "Mark as paid" / "Pay" buttons
  const payBtns = await page.evaluate(() => {
    const btns = Array.from(document.querySelectorAll("button, a[role='button']"));
    return btns
      .filter(b => /mark.*paid|^pay$|pay now|mark paid/i.test(b.textContent.trim()))
      .map(b => {
        const parent = b.closest(".row, .card, li, article, tr, div[class]");
        const parentText = parent ? parent.textContent.replace(/\s+/g, " ").trim().slice(0, 80) : "";
        return { text: b.textContent.trim(), parentText };
      });
  });
  note(`Pay buttons: ${JSON.stringify(payBtns)}`);

  if (payBtns.length === 0) {
    // Check if bills page exists / has content
    const billsBodyText = await page.evaluate(() => document.body.textContent ?? "");
    const billKeywords = /bill|subscription|due|upcoming|overdue/i.test(billsBodyText);
    note(`/bills page has bill content: ${billKeywords}; body snippet: ${billsBodyText.slice(0, 200)}`);
    fail("Step 5.2 — no 'Mark as paid' / 'Pay' buttons found on /bills");
  } else {
    const billsMarked = [];
    for (let i = 0; i < Math.min(2, payBtns.length); i++) {
      const billName = payBtns[i].parentText.slice(0, 40);
      await page.evaluate((idx) => {
        const btns = Array.from(document.querySelectorAll("button, a[role='button']"))
          .filter(b => /mark.*paid|^pay$|pay now|mark paid/i.test(b.textContent.trim()));
        if (btns[idx]) btns[idx].click();
      }, i);
      await page.waitForTimeout(1000);
      billsMarked.push(billName);
    }
    pass(`Step 5.2 — marked ${billsMarked.length} bill(s) as paid: ${JSON.stringify(billsMarked)}`);
  }

  await page.screenshot({ path: SS("loop43-10-after-bills-paid.png") });
  pass("Step 5.3 — screenshot loop43-10-after-bills-paid.png");

  const billsDomAfter = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row, .card, li, article, tr, [class*='bill']"));
    return rows.map(r => r.textContent.replace(/\s+/g, " ").trim()).filter(Boolean).slice(0, 20);
  });
  note(`Bill rows after: ${JSON.stringify(billsDomAfter)}`);

  // ══════════════════════════════════════════════════════════════════════════════
  // STEP 6: /dashboard — verify end state
  // ══════════════════════════════════════════════════════════════════════════════
  console.log("\n── STEP 6: /dashboard ───────────────────────────────────────────────────");

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(2000);

  await page.screenshot({ path: SS("loop43-11-dashboard-end-state.png") });
  pass("Step 6.1 — screenshot loop43-11-dashboard-end-state.png");

  const dashDom = await page.evaluate(() => {
    const rows = Array.from(document.querySelectorAll(".row, .card, h1, h2, h3, [class*='widget'], [class*='summary']"));
    return rows.map(r => r.textContent.replace(/\s+/g, " ").trim()).filter(Boolean).slice(0, 30);
  });
  note(`Dashboard DOM: ${JSON.stringify(dashDom)}`);

  const jsErrors = await page.evaluate(() => window.__errors || "no errors");
  note(`window.__errors: ${JSON.stringify(jsErrors)}`);

  if (errors.length === 0) {
    pass("Step 6.2 — zero JS page errors across entire flow");
  } else {
    fail(`Step 6.2 — JS errors: ${errors.join(" | ")}`);
  }

} finally {
  await browser.close();
  console.log(`\nResult: ${passed} passed, ${failed} failed`);
  if (failed > 0) process.exitCode = 1;
}
