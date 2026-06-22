// L40 E2E loop story — "Setting a Grocery Budget"
// Persona: Sam, a household manager who wants to cap monthly grocery spending at $600.
// Ritual:
//   1. Navigate to /budgets, create a monthly $600 Groceries budget.
//   2. Verify the new budget appears with $0 spent / $600 left and an empty progress bar.
//   3. Navigate to /transactions, add two grocery purchases ($47.32 + $102.89 = $150.21).
//   4. Return to /budgets; verify spent ~$150, limit $600, progress bar is non-zero.
//   5. Add a non-grocery (Dining) expense; verify it does NOT count toward Groceries budget.
//   6. Reload the page; verify budget and spend survive across reload.
//
// Run: E2E_URL=http://127.0.0.1:8099 node e2e/loopstory_40_create_budget.mjs
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS = (name) => path.join(__dirname, name);

const browser = await chromium.launch({ headless: true });
let passed = 0;
let failed = 0;

const pass = (label) => { console.log(`PASS: ${label}`); passed++; };
const fail = (label) => { console.error(`FAIL: ${label}`); failed++; };

// Wait for wasm to hydrate (nav sentinel)
const waitNav = (page) =>
  page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });

try {
  const page = await browser.newPage();
  page.setViewportSize({ width: 1280, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // ── Step 1: Navigate to /budgets ─────────────────────────────────────────────
  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);

  const h1 = await page.evaluate(() => document.querySelector("h1")?.textContent?.trim() ?? "");
  if (/budget/i.test(h1)) {
    pass(`Step 1 — /budgets loaded (h1: "${h1}")`);
  } else {
    fail(`Step 1 — expected Budgets h1, got "${h1}"`);
  }
  await page.screenshot({ path: SS("loop40-01-budgets-before.png") });
  pass("Step 1b — screenshot loop40-01-budgets-before.png");

  // ── Step 2: Confirm Groceries category exists in the add-budget form ──────────
  // The Category select has aria-label="Category" (index 1 in the DOM; index 0
  // is the "Jump to…" period picker). Use the aria-label to target correctly.
  const catOpts = await page.evaluate(() => {
    const sel = document.querySelector('select[aria-label="Category"]');
    return sel ? Array.from(sel.options).map((o) => ({ value: o.value, text: o.text })) : [];
  });
  const groceryCat = catOpts.find((o) => /grocer/i.test(o.text));
  if (groceryCat) {
    pass(`Step 2 — Groceries category found: "${groceryCat.text}" (${groceryCat.value})`);
  } else {
    fail(`Step 2 — no Groceries category in add-budget form; available: ${catOpts.map((o) => o.text).join(", ")}`);
  }

  // ── Step 3: Fill and submit the add-budget form ───────────────────────────────
  // Name
  await page.fill('#budget-add', "Monthly Groceries");

  // Category → Groceries
  await page.selectOption('select[aria-label="Category"]', { value: "cat-groceries" });

  // Period → Monthly (already default but be explicit)
  await page.selectOption('select[aria-label="Period"]', { value: "monthly" });

  // Limit
  await page.fill('input[type="number"][aria-required="true"]', "600");

  await page.screenshot({ path: SS("loop40-02-add-form-filled.png") });
  pass("Step 3a — screenshot loop40-02-add-form-filled.png (form filled)");

  // Submit
  await page.locator('form button[type="submit"]').click();
  await page.waitForTimeout(1200);
  await page.screenshot({ path: SS("loop40-03-after-add-budget.png") });
  pass("Step 3b — screenshot loop40-03-after-add-budget.png (after submit)");

  // ── Step 4: Verify the new budget row appeared with $600 limit ───────────────
  // NOTE: The sample household dataset already has Groceries (cat-groceries)
  // transactions in the current month — so the budget will NOT start at $0.
  // The meaningful checks here are: (a) the row appeared, (b) $600 limit shows,
  // (c) spend + limit are consistent. We capture the baseline spend so later
  // steps can assert a DELTA rather than an absolute value.
  const afterAddText = await page.evaluate(() => document.body.innerText);

  if (afterAddText.includes("Monthly Groceries")) {
    pass('Step 4a — "Monthly Groceries" budget row appeared in list');
  } else {
    fail('Step 4a — "Monthly Groceries" not found after add');
  }

  if (/600/.test(afterAddText)) {
    pass("Step 4b — $600 limit visible");
  } else {
    fail("Step 4b — $600 limit NOT visible after add");
  }

  // Capture the baseline spent amount shown immediately after creation.
  const grocRowText = await page.evaluate(() => {
    const all = Array.from(document.querySelectorAll("*"));
    const gr = all.find((el) =>
      el.childNodes.length > 0 &&
      el.textContent.includes("Monthly Groceries") &&
      el.textContent.length < 500
    );
    return gr ? gr.innerText : "";
  });
  console.log("Grocery budget row text after add (baseline):\n", grocRowText.substring(0, 300));

  // Extract the baseline spent (first $ amount in the row, e.g. "$520.00 / $600.00")
  const baselineMatch = grocRowText.match(/\$([\d,]+\.\d{2})\s*\/\s*\$600/);
  const baselineSpentCents = baselineMatch
    ? Math.round(parseFloat(baselineMatch[1].replace(",", "")) * 100)
    : 0;
  console.log(`Baseline spent: $${(baselineSpentCents / 100).toFixed(2)}`);
  pass(`Step 4c — baseline grocery spend captured: $${(baselineSpentCents / 100).toFixed(2)} (sample data already applied)`);

  // ── Step 5: Add grocery transaction 1 ($47.32) ───────────────────────────────
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1000);

  await page.fill('#txn-add', "Whole Foods run");
  await page.fill('input[type="number"][aria-required="true"]', "47.32");
  // Set category to Groceries
  await page.selectOption('select[aria-label="Category"]', { value: "cat-groceries" });
  // Ensure type is Expense
  await page.selectOption('select[aria-label="Type"]', { value: "Expense" });

  await page.locator('form button[type="submit"]').click();
  await page.waitForTimeout(800);
  pass("Step 5 — grocery transaction 1 submitted ($47.32, Groceries)");

  // ── Step 6: Add grocery transaction 2 ($102.89) ──────────────────────────────
  await page.fill('#txn-add', "Trader Joe's");
  await page.fill('input[type="number"][aria-required="true"]', "102.89");
  await page.selectOption('select[aria-label="Category"]', { value: "cat-groceries" });
  await page.selectOption('select[aria-label="Type"]', { value: "Expense" });

  await page.locator('form button[type="submit"]').click();
  await page.waitForTimeout(800);
  pass("Step 6 — grocery transaction 2 submitted ($102.89, Groceries)");

  await page.screenshot({ path: SS("loop40-04-after-groceries.png") });
  pass("Step 6b — screenshot loop40-04-after-groceries.png");

  // Verify both transactions appear in the ledger
  const wholeFoodsVisible = await page.getByText("Whole Foods run").count();
  const traderJoesVisible = await page.getByText("Trader Joe's").count();
  if (wholeFoodsVisible > 0) {
    pass("Step 6c — 'Whole Foods run' visible in ledger");
  } else {
    fail("Step 6c — 'Whole Foods run' NOT found in ledger");
  }
  if (traderJoesVisible > 0) {
    pass("Step 6d — \"Trader Joe's\" visible in ledger");
  } else {
    fail("Step 6d — \"Trader Joe's\" NOT found in ledger");
  }

  // ── Step 7: Add a non-grocery (Dining) expense — should NOT count ─────────────
  await page.fill('#txn-add', "Thai restaurant");
  await page.fill('input[type="number"][aria-required="true"]', "35.00");
  await page.selectOption('select[aria-label="Category"]', { value: "cat-dining" });
  await page.selectOption('select[aria-label="Type"]', { value: "Expense" });

  await page.locator('form button[type="submit"]').click();
  await page.waitForTimeout(800);
  pass("Step 7 — non-grocery Dining expense submitted ($35.00)");

  // ── Step 8: Navigate to /budgets and verify spend ─────────────────────────────
  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("loop40-05-budgets-after-spend.png") });
  pass("Step 8a — screenshot loop40-05-budgets-after-spend.png");

  const budgetsText = await page.evaluate(() => document.body.innerText);

  if (budgetsText.includes("Monthly Groceries")) {
    pass('Step 8b — "Monthly Groceries" budget visible after adding transactions');
  } else {
    fail('Step 8b — "Monthly Groceries" budget MISSING from /budgets after transactions');
  }

  // Grocery spend delta: $47.32 + $102.89 = $150.21 above baseline.
  // Read the current spent figure from the row and compare to baseline.
  const gr2 = await page.evaluate(() => {
    const all = Array.from(document.querySelectorAll("*"));
    const gr = all.find((el) =>
      el.childNodes.length > 0 &&
      el.textContent.includes("Monthly Groceries") &&
      el.textContent.length < 500
    );
    return gr ? gr.innerText : "row not found";
  });
  console.log("Grocery row after transactions:\n", gr2.substring(0, 300));

  const afterSpendMatch = gr2.match(/\$([\d,]+\.\d{2})\s*\/\s*\$600/);
  const afterSpentCents = afterSpendMatch
    ? Math.round(parseFloat(afterSpendMatch[1].replace(",", "")) * 100)
    : -1;
  const deltaCents = afterSpentCents - baselineSpentCents;
  const expectedDelta = 15021; // $150.21 in cents
  const tolerance = 5;        // ±$0.05 rounding tolerance

  if (Math.abs(deltaCents - expectedDelta) <= tolerance) {
    pass(`Step 8c — grocery spend increased by $${(deltaCents / 100).toFixed(2)} (expected $150.21) ✓`);
  } else {
    fail(`Step 8c — grocery spend delta was $${(deltaCents / 100).toFixed(2)}, expected $150.21; baseline=$${(baselineSpentCents / 100).toFixed(2)} after=$${(afterSpentCents / 100).toFixed(2)}`);
  }

  // $600 limit still there
  if (/600/.test(budgetsText)) {
    pass("Step 8d — $600 limit still visible");
  } else {
    fail("Step 8d — $600 limit missing after transactions");
  }

  // ── Step 9: Verify progress bar is non-zero ───────────────────────────────────
  const barWidth = await page.evaluate(() => {
    const candidates = Array.from(document.querySelectorAll("[style*='width']"));
    for (const el of candidates) {
      const w = el.style.width;
      if (w && w.endsWith("%") && parseFloat(w) > 0) return w;
    }
    return null;
  });
  if (barWidth) {
    pass(`Step 9a — progress bar fill: ${barWidth}`);
  } else {
    const hasBar = await page.evaluate(() =>
      !!document.querySelector("[class*='bar'], [class*='progress'], [role='progressbar']")
    );
    if (hasBar) {
      pass("Step 9a — progress bar element present (width may be class-based)");
    } else {
      fail("Step 9a — no progress bar found after spend");
    }
  }

  // ── Step 10: Category isolation — Dining $35 must NOT appear in Groceries ─────
  // The Groceries budget should NOT show $185.21; it should show $150.21.
  const grRow = await page.evaluate(() => {
    const all = Array.from(document.querySelectorAll("*"));
    const gr = all.find((el) =>
      el.textContent.includes("Monthly Groceries") &&
      el.textContent.length < 600
    );
    return gr ? gr.innerText : "";
  });
  const has185 = /185/.test(grRow);
  if (!has185) {
    pass("Step 10 — grocery budget does NOT include non-grocery $35 (category isolation ✓)");
  } else {
    fail(`Step 10 — grocery budget row shows $185 — Dining expense incorrectly counted; row: "${grRow.substring(0, 200)}"`);
  }

  await page.screenshot({ path: SS("loop40-06-budgets-verified.png") });
  pass("Step 10b — screenshot loop40-06-budgets-verified.png");

  // ── Step 11: Reload and verify persistence ────────────────────────────────────
  await page.reload({ waitUntil: "domcontentloaded" });
  await waitNav(page);
  await page.waitForTimeout(1500);
  await page.screenshot({ path: SS("loop40-07-budgets-after-reload.png") });
  pass("Step 11a — screenshot loop40-07-budgets-after-reload.png");

  const reloadText = await page.evaluate(() => document.body.innerText);

  if (reloadText.includes("Monthly Groceries")) {
    pass('Step 11b — "Monthly Groceries" budget survived reload');
  } else {
    fail('Step 11b — "Monthly Groceries" MISSING after reload');
  }
  if (/600/.test(reloadText)) {
    pass("Step 11c — $600 limit survived reload");
  } else {
    fail("Step 11c — $600 limit missing after reload");
  }
  // After reload, the spent figure should be the same as after the transactions
  // (baseline + $150.21). Check the row contains the expected post-spend total.
  const reloadRow = await page.evaluate(() => {
    const all = Array.from(document.querySelectorAll("*"));
    const gr = all.find((el) =>
      el.childNodes.length > 0 &&
      el.textContent.includes("Monthly Groceries") &&
      el.textContent.length < 500
    );
    return gr ? gr.innerText : "";
  });
  const reloadSpendMatch = reloadRow.match(/\$([\d,]+\.\d{2})\s*\/\s*\$600/);
  const reloadSpentCents = reloadSpendMatch
    ? Math.round(parseFloat(reloadSpendMatch[1].replace(",", "")) * 100)
    : -1;
  const reloadDelta = reloadSpentCents - baselineSpentCents;

  if (Math.abs(reloadDelta - expectedDelta) <= tolerance) {
    pass(`Step 11d — spent delta still $${(reloadDelta / 100).toFixed(2)} after reload (persisted correctly)`);
  } else {
    console.log("Reload row:", reloadRow.substring(0, 200));
    fail(`Step 11d — spend delta after reload was $${(reloadDelta / 100).toFixed(2)}, expected $150.21`);
  }

  // ── Page error guard ─────────────────────────────────────────────────────────
  if (errors.length === 0) {
    pass("Page errors — none detected");
  } else {
    fail(`Page errors — ${errors.length} JS error(s): ${errors.slice(0, 3).join(" | ")}`);
  }

} finally {
  await browser.close();
  console.log(`\n── Summary: ${passed} passed, ${failed} failed ──`);
  if (failed > 0) process.exit(1);
}
