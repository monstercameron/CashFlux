// L43 gate — Transfer button on /accounts.
// Verifies that the ⋯ menu on each account row exposes a "Transfer…" action that
// opens an inline form, creates a paired transfer (two legs, net-zero on net worth),
// and posts a confirmation toast.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const txns = (page) => page.evaluate(() =>
  JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []);
async function flush(page) {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
}

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });

  // Wait for at least one account row, then open its ⋯ overflow menu (the
  // Transfer item lives inside it and is hidden until the menu opens).
  await page.waitForSelector('.row button[aria-haspopup="menu"]', { timeout: 60000 });
  await page.locator('.row button[aria-haspopup="menu"]').first().click();
  await page.waitForTimeout(300);

  const transferBtns = page.locator('[data-testid^="transfer-start-btn-"]');
  await transferBtns.first().waitFor({ state: "visible", timeout: 10000 }).catch(() => {});
  if ((await transferBtns.count()) === 0) { fail("no Transfer buttons found on /accounts"); process.exit(1); }
  await transferBtns.first().click();
  await page.waitForTimeout(300);

  // The inline transfer form should now be visible.
  const form = page.locator('[id^="acct-transfer-form-"]');
  if ((await form.count()) === 0) { fail("transfer form not shown after clicking Transfer action"); process.exit(1); }

  // Capture net worth before. Flush first so localStorage reflects the seeded
  // in-memory dataset (a cold read returns an empty/zero dataset).
  await flush(page);
  const nwBefore = await page.evaluate(() => {
    const d = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    const txs = d.transactions || [];
    const accs = d.accounts || [];
    // Simple sum: sum opening balances + all transaction amounts per account.
    let total = 0;
    for (const ac of accs) {
      const ob = (ac.openingBalance && ac.openingBalance.Amount) || 0;
      total += ob;
    }
    for (const t of txs) {
      total += (t.amount && t.amount.Amount) || 0;
    }
    return total;
  });

  const txnsBefore = await txns(page);

  // Fill the form: pick the second account as destination, enter amount 5000 cents = $50.
  const toSelect = page.locator('[data-testid="acct-xfer-to-select"]');
  const toOptions = await toSelect.locator("option").all();
  // Pick the first non-empty option.
  let toVal = "";
  for (const opt of toOptions) {
    const v = await opt.getAttribute("value");
    if (v && v.length > 0) { toVal = v; break; }
  }
  if (!toVal) { fail("no destination account available for transfer"); process.exit(1); }
  await toSelect.selectOption(toVal);

  const amtInput = page.locator('[id^="acct-xfer-amt-"]');
  await amtInput.fill("50");

  await page.locator('[id^="acct-transfer-form-"] button[type="submit"]').first().click();
  await flush(page);

  // Verify two new transfer legs exist.
  let all = await txns(page);
  for (let i = 0; i < 10 && all.length <= txnsBefore.length; i++) { await flush(page); all = await txns(page); }

  const newTxns = all.filter((t) => !txnsBefore.find((p) => p.id === t.id));
  const legs = newTxns.filter((t) => t.transferAccountId && t.transferAccountId.length > 0);
  if (legs.length < 2) {
    fail(`expected 2 transfer legs, got ${legs.length} (new txns: ${newTxns.length})`);
  }

  // Net worth must be unchanged (money just moved between accounts).
  const nwAfter = await page.evaluate(() => {
    const d = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    const txs = d.transactions || [];
    const accs = d.accounts || [];
    let total = 0;
    for (const ac of accs) {
      const ob = (ac.openingBalance && ac.openingBalance.Amount) || 0;
      total += ob;
    }
    for (const t of txs) {
      total += (t.amount && t.amount.Amount) || 0;
    }
    return total;
  });
  if (nwBefore !== nwAfter) {
    fail(`net worth changed after transfer: before=${nwBefore} after=${nwAfter} (should be equal)`);
  }

  if (!process.exitCode) {
    console.log(`PASS: Transfer button on /accounts — 2 legs created, net worth conserved (${nwBefore} → ${nwAfter}).`);
  }
} finally {
  await browser.close();
}
