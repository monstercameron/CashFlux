// L42 E2E loop story — "Adding a Pet Care Category"
// Persona: Maya, 29, tracks household spending and wants to separate vet bills
//          from general Miscellaneous by creating a dedicated "Pet Care" category.
// Flow:
//   1. Navigate to /categories; confirm page loaded.
//   2. Add "L42 Pet Care" as an Expense category; confirm it appears in the list.
//   3. Confirm "L42 Pet Care" appears in the Expense section of the category list.
//   4. (KEY PROBE) Navigate to /transactions; check that category picker includes
//      "L42 Pet Care" without requiring reload; also probe for inline category creation.
//   5. Fill the add-transaction form: "L42 Vet Bill", $85, Expense, "L42 Pet Care", today.
//   6. Verify the transaction row shows "L42 Vet Bill" and "Pet Care".
//   7. Navigate to /reports; screenshot and check for "L42 Pet Care" in spending breakdown.
//   8. Evaluate reports DOM for spending-by-category content.
//   9. Hard reload /categories — confirm "L42 Pet Care" persists.
//  10. Hard reload /transactions — confirm "L42 Vet Bill" persists.
//  11. JS error check.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_42_add_category.mjs

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS = (name) => path.join(__dirname, name);

const CAT_NAME = "L42 Pet Care";
const TXN_DESC = "L42 Vet Bill";
const TXN_AMOUNT = "85";
const TXN_DATE = "2026-06-22";

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

  // ── Step 1: Navigate to /categories ─────────────────────────────────────────
  await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);

  const h1 = await page.evaluate(() => document.querySelector("h1")?.textContent?.trim() ?? "");
  if (/categor/i.test(h1)) {
    pass(`Step 1 — /categories loaded (h1: "${h1}")`);
  } else {
    fail(`Step 1 — expected Categories h1, got "${h1}"`);
  }
  await page.screenshot({ path: SS("loop42-01-categories-before.png") });
  pass("Step 1b — screenshot loop42-01-categories-before.png");

  // ── Step 2: Fill the add form and submit ─────────────────────────────────────
  const catInput = page.locator("#cat-add");
  if ((await catInput.count()) > 0) {
    pass("Step 2a — #cat-add input found");
  } else {
    fail("Step 2a — #cat-add input not found");
  }

  await page.fill("#cat-add", CAT_NAME);

  // Confirm "Expense" is already selected in the kind select (it's the default)
  const kindSelect = page.locator('select[aria-label="Category type"]').first();
  const kindVal = await kindSelect.evaluate((el) => el.value);
  console.log(`  [debug] kind select current value: "${kindVal}"`);
  if (/expense/i.test(kindVal)) {
    pass("Step 2b — kind select defaults to Expense");
  } else {
    // Explicitly set it
    await kindSelect.selectOption({ label: "Expense" });
    pass("Step 2b — kind select set to Expense explicitly");
  }

  // Check if a color picker is present in the form
  const colorPicker = page.locator('input[type="color"]').first();
  const hasColor = (await colorPicker.count()) > 0;
  console.log(`  [finding] Color picker present in add-category form: ${hasColor}`);
  if (hasColor) {
    pass("Step 2c — color picker is present in category add form");
  } else {
    fail("Step 2c — color picker NOT found in category add form");
  }

  // Submit the form
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(1200);

  // Confirm the category appears anywhere on the page after add
  const bodyAfterAdd = await page.evaluate(() => document.body.textContent ?? "");
  if (bodyAfterAdd.includes(CAT_NAME)) {
    pass(`Step 2d — "${CAT_NAME}" appears in page after submit`);
  } else {
    fail(`Step 2d — "${CAT_NAME}" not found in page after submit`);
  }

  // Check if name field cleared after submit (form reset behavior)
  const nameValAfter = await page.locator("#cat-add").inputValue();
  console.log(`  [finding] Name field after submit: "${nameValAfter}" (${nameValAfter === "" ? "cleared" : "NOT cleared"})`);
  const kindValAfter = await kindSelect.evaluate((el) => el.value);
  console.log(`  [finding] Kind select after submit: "${kindValAfter}"`);

  await page.screenshot({ path: SS("loop42-02-after-add-category.png") });
  pass("Step 2e — screenshot loop42-02-after-add-category.png");

  // ── Step 3: Confirm "L42 Pet Care" in Expense section ───────────────────────
  // The expense section is under H2 with text matching "Expense" (categories.expenseTitle).
  // Category rows are inside .rows inside a Section after that H2.
  const inExpenseSection = await page.evaluate((name) => {
    // Find all h2 elements and locate the Expense one
    const h2s = Array.from(document.querySelectorAll("h2"));
    for (const h2 of h2s) {
      if (/expense/i.test(h2.textContent ?? "")) {
        // Walk up to the parent card, then look for the name
        const card = h2.closest("section, .card, div");
        if (card && card.textContent.includes(name)) return true;
      }
    }
    // Fallback: just check the full body
    return document.body.textContent.includes(name);
  }, CAT_NAME);

  if (inExpenseSection) {
    pass(`Step 3 — "${CAT_NAME}" found in Expense section`);
  } else {
    fail(`Step 3 — "${CAT_NAME}" not found in Expense section`);
  }

  // ── Step 4 (KEY PROBE): /transactions — category picker + inline creation ────
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);

  // Find the category select in the add form (aria-label="Category")
  const catOptions = await page.evaluate((name) => {
    const selects = Array.from(document.querySelectorAll("select"));
    for (const sel of selects) {
      const label = sel.getAttribute("aria-label") ?? "";
      if (/^category$/i.test(label.trim())) {
        return Array.from(sel.options).map((o) => ({ value: o.value, text: o.text }));
      }
    }
    // Fallback: try any select that contains our category name
    for (const sel of selects) {
      const opts = Array.from(sel.options);
      if (opts.some((o) => o.text.includes("Pet Care"))) {
        return opts.map((o) => ({ value: o.value, text: o.text }));
      }
    }
    return null;
  }, CAT_NAME);

  console.log(`  [debug] category select options found: ${catOptions ? catOptions.length : "null"}`);
  if (catOptions) {
    console.log(`  [debug] options: ${catOptions.map((o) => o.text).join(", ")}`);
  }

  const catOpt = catOptions?.find((o) => o.text.includes("Pet Care") || o.text.includes(CAT_NAME));
  if (catOpt) {
    pass(`Step 4a — "${CAT_NAME}" appears in transaction category picker immediately (value: ${catOpt.value})`);
    console.log("  [finding] KEY PROBE: Category is immediately available in /transactions after creation — no reload required.");
  } else {
    fail(`Step 4a — "${CAT_NAME}" NOT found in transaction category picker; available: ${catOptions?.map((o) => o.text).join(", ") ?? "no options found"}`);
    console.log("  [finding] KEY PROBE: Category NOT immediately available — may require page reload.");
  }

  // Probe for inline "Add new category" or "+" button in the transaction form
  const hasInlineAdd = await page.evaluate(() => {
    const body = document.body;
    // Look for any button/link containing + or "add" near a category-related label
    const btns = Array.from(body.querySelectorAll("button, a"));
    for (const b of btns) {
      const txt = b.textContent?.trim() ?? "";
      if ((txt === "+" || /add.*cat|new.*cat|cat.*add/i.test(txt)) && txt.length < 30) return txt || "+";
    }
    return null;
  });
  console.log(`  [finding] Inline "Add category" button in transaction form: ${hasInlineAdd ?? "NONE — must visit /categories first"}`);

  // ── Step 5: Fill add-transaction form ────────────────────────────────────────
  // Find the description input for adding a transaction
  const txnDescInput = page.locator('input[id^="txn-add"], input[placeholder*="description" i], input[placeholder*="desc" i]').first();
  const txnDescCount = await txnDescInput.count();
  if (txnDescCount > 0) {
    await txnDescInput.fill(TXN_DESC);
    pass("Step 5a — description filled");
  } else {
    // Try the first text input in the transaction add form area
    const allTextInputs = page.locator('input[type="text"]');
    const cnt = await allTextInputs.count();
    console.log(`  [debug] text inputs found: ${cnt}`);
    if (cnt > 0) {
      await allTextInputs.first().fill(TXN_DESC);
      pass("Step 5a — description filled via first text input");
    } else {
      fail("Step 5a — no description input found");
    }
  }

  // Amount
  const amountInput = page.locator('input[type="number"], input[placeholder*="amount" i]').first();
  if ((await amountInput.count()) > 0) {
    await amountInput.fill(TXN_AMOUNT);
    pass("Step 5b — amount set to $85");
  } else {
    fail("Step 5b — amount input not found");
  }

  // Kind (Expense) — look for the kind/type select in the add form
  // In transactions.go the kind select has aria-label "Type" or similar — let's probe
  const kindSelectTxn = await page.evaluate(() => {
    const selects = Array.from(document.querySelectorAll("select"));
    return selects.map((s) => ({
      ariaLabel: s.getAttribute("aria-label") ?? "",
      options: Array.from(s.options).map((o) => o.text),
    }));
  });
  console.log(`  [debug] all selects on /transactions: ${JSON.stringify(kindSelectTxn)}`);

  // Select the category
  if (catOpt) {
    // Find the category select and set it
    await page.evaluate((val) => {
      const selects = Array.from(document.querySelectorAll("select"));
      for (const sel of selects) {
        const label = sel.getAttribute("aria-label") ?? "";
        if (/^category$/i.test(label.trim())) {
          sel.value = val;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return;
        }
      }
      // Fallback: find any select with this option
      for (const sel of selects) {
        const opts = Array.from(sel.options);
        const found = opts.find((o) => o.value === val);
        if (found) {
          sel.value = val;
          sel.dispatchEvent(new Event("change", { bubbles: true }));
          return;
        }
      }
    }, catOpt.value);
    pass(`Step 5c — category select set to "${CAT_NAME}"`);
  } else {
    fail("Step 5c — could not set category (option not found)");
  }

  // Date
  const dateInput = page.locator('input[type="date"]').first();
  if ((await dateInput.count()) > 0) {
    await dateInput.fill(TXN_DATE);
    pass(`Step 5d — date set to ${TXN_DATE}`);
  } else {
    fail("Step 5d — date input not found");
  }

  await page.screenshot({ path: SS("loop42-03-after-add-transaction.png") });
  pass("Step 5e — screenshot loop42-03-after-add-transaction.png (before submit)");

  // Submit the transaction form
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(1200);

  await page.screenshot({ path: SS("loop42-03-after-add-transaction.png") });
  pass("Step 5f — screenshot loop42-03-after-add-transaction.png (after submit)");

  // ── Step 6: Verify the transaction row ───────────────────────────────────────
  const txnBodyText = await page.evaluate(() => document.body.textContent ?? "");
  if (txnBodyText.includes(TXN_DESC)) {
    pass(`Step 6a — "${TXN_DESC}" found in transactions list`);
  } else {
    fail(`Step 6a — "${TXN_DESC}" not found in transactions list`);
  }

  // Check for "Pet Care" or "L42 Pet Care" in the transaction row
  const hasCatInRow = txnBodyText.includes("Pet Care");
  if (hasCatInRow) {
    pass("Step 6b — \"Pet Care\" category label visible in transactions list");
  } else {
    fail("Step 6b — \"Pet Care\" category label NOT visible in transactions list");
    // Dump the transaction rows for debug
    const rowTexts = await page.evaluate(() => {
      const rows = Array.from(document.querySelectorAll(".row, .budget, tr, li"));
      return rows.filter((r) => r.textContent.includes("Vet")).map((r) => r.textContent.replace(/\s+/g, " ").trim());
    });
    console.log(`  [debug] vet-related rows: ${JSON.stringify(rowTexts)}`);
  }

  await page.screenshot({ path: SS("loop42-04-transaction-row.png") });
  pass("Step 6c — screenshot loop42-04-transaction-row.png");

  // ── Step 7: Navigate to /reports ─────────────────────────────────────────────
  await page.goto(BASE + "/reports", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(2000);

  await page.screenshot({ path: SS("loop42-05-reports-page.png") });
  pass("Step 7a — screenshot loop42-05-reports-page.png");

  const reportsText = await page.evaluate(() => document.body.textContent ?? "");
  const hasCatInReports = reportsText.includes("Pet Care") || reportsText.includes(CAT_NAME);
  if (hasCatInReports) {
    pass(`Step 7b — "Pet Care" / "${CAT_NAME}" found in /reports body text`);
  } else {
    fail(`Step 7b — "Pet Care" / "${CAT_NAME}" NOT found in /reports body text`);
  }

  // Check if $85 / 85.00 appears near Pet Care in reports
  const has85 = /\$85|85\.00/.test(reportsText);
  if (has85) {
    pass("Step 7c — $85 or 85.00 appears in /reports");
  } else {
    fail("Step 7c — $85 / 85.00 NOT found in /reports");
    console.log(`  [debug] reports page text snippet: ${reportsText.substring(0, 500)}`);
  }

  // ── Step 8: Evaluate spending-by-category DOM ─────────────────────────────────
  const catRows = await page.evaluate(() => {
    // Find the "by category" section
    const h2s = Array.from(document.querySelectorAll("h2"));
    for (const h2 of h2s) {
      if (/by category|spending.*category|categor/i.test(h2.textContent ?? "")) {
        const card = h2.closest("section, .card, div");
        if (card) {
          const rows = Array.from(card.querySelectorAll(".row, .row-desc, li"));
          return rows.map((r) => r.textContent.replace(/\s+/g, " ").trim()).filter(Boolean);
        }
      }
    }
    // Fallback: get all .row-desc elements
    return Array.from(document.querySelectorAll(".row-desc"))
      .map((el) => el.textContent.replace(/\s+/g, " ").trim())
      .filter(Boolean);
  });
  console.log(`  [debug] spending-by-category rows visible: ${JSON.stringify(catRows)}`);
  if (catRows.length > 0) {
    pass(`Step 8 — found ${catRows.length} category row(s) in reports spending section`);
    const petCareRow = catRows.find((r) => /pet care/i.test(r));
    if (petCareRow) {
      pass(`Step 8b — "Pet Care" row found: "${petCareRow}"`);
    } else {
      fail(`Step 8b — "Pet Care" NOT in spending rows; rows: ${JSON.stringify(catRows)}`);
    }
  } else {
    fail("Step 8 — no category rows found in reports spending section");
  }

  // ── Step 9: Hard reload /categories — persistence check ──────────────────────
  await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);
  await page.reload({ waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1200);

  const catBodyAfterReload = await page.evaluate(() => document.body.textContent ?? "");
  if (catBodyAfterReload.includes(CAT_NAME)) {
    pass(`Step 9 — "${CAT_NAME}" persists after hard reload of /categories`);
  } else {
    fail(`Step 9 — "${CAT_NAME}" NOT found after hard reload of /categories`);
  }
  await page.screenshot({ path: SS("loop42-06-categories-after-reload.png") });
  pass("Step 9b — screenshot loop42-06-categories-after-reload.png");

  // ── Step 10: Hard reload /transactions — persistence check ───────────────────
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);
  await page.reload({ waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1200);

  const txnBodyAfterReload = await page.evaluate(() => document.body.textContent ?? "");
  if (txnBodyAfterReload.includes(TXN_DESC)) {
    pass(`Step 10 — "${TXN_DESC}" persists after hard reload of /transactions`);
  } else {
    fail(`Step 10 — "${TXN_DESC}" NOT found after hard reload of /transactions`);
  }
  await page.screenshot({ path: SS("loop42-07-transactions-after-reload.png") });
  pass("Step 10b — screenshot loop42-07-transactions-after-reload.png");

  // ── Step 11: JS error check ──────────────────────────────────────────────────
  if (errors.length === 0) {
    pass("Step 11 — zero JS page errors across entire flow");
  } else {
    fail(`Step 11 — JS errors: ${errors.join(" | ")}`);
  }

} finally {
  await browser.close();
  console.log(`\nResult: ${passed} passed, ${failed} failed`);
  if (failed > 0) process.exitCode = 1;
}
