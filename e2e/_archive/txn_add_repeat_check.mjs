// L24 gate — "Repeat affordance on the add form creates a recurring schedule."
// Adds an Expense with a unique description and Repeat=Monthly, then asserts
// that localStorage contains both the one-off transaction and a recurring
// schedule with the correct cadence, autopost flag, and nextDue.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const getDS = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
async function waitDS(page, pred, timeoutMs = 10000) {
  let d = {};
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    d = await getDS(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return d;
}

const UNIQUE_DESC = `ZZREPEAT-${Date.now()}`;
const TXN_DATE = "2026-06-15";
const AMOUNT = "42.00";
// nextDue should be one month after the entered date: 2026-07-15
const EXPECTED_NEXT_DUE_PREFIX = "2026-07-15";

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });

  // Wait for the app to finish loading (accounts list should be present).
  await page.waitForSelector("form.form-grid", { timeout: 10000 });

  // Fill the add form.
  await page.fill('input[placeholder="Description"]', UNIQUE_DESC);
  await page.fill('input[placeholder="Amount"]', AMOUNT);

  // Set type to Expense (it's the default, but be explicit).
  await page.selectOption('select[aria-label="Type"]', { value: "Expense" });

  // Set the date.
  await page.fill('input[type="date"]', TXN_DATE);

  // Set the Repeat select to Monthly.
  await page.selectOption('[data-testid="txn-add-repeat"]', { value: "monthly" });

  // Submit.
  await page.click('button[type="submit"]');

  // Wait for autosave to persist both the transaction and the recurring schedule.
  const ds = await waitDS(page, (d) => {
    const hasTxn = (d.transactions || []).some((t) => t.desc === UNIQUE_DESC);
    const hasRec = (d.recurring || []).some((r) => r.label === UNIQUE_DESC);
    return hasTxn && hasRec;
  });

  // (a) One-off transaction must exist.
  const txn = (ds.transactions || []).find((t) => t.desc === UNIQUE_DESC);
  if (!txn) {
    fail(`transaction with desc "${UNIQUE_DESC}" not found in dataset`);
  } else {
    console.log(`  txn found: id=${txn.id} amount=${JSON.stringify(txn.amount)}`);
  }

  // (b) Recurring schedule must exist with cadence=="monthly", autopost==true,
  //     and nextDue one month after the entered date.
  const rec = (ds.recurring || []).find((r) => r.label === UNIQUE_DESC);
  if (!rec) {
    fail(`recurring with label "${UNIQUE_DESC}" not found in dataset`);
  } else {
    if (rec.cadence !== "monthly") {
      fail(`recurring cadence = "${rec.cadence}", want "monthly"`);
    }
    if (rec.autopost !== true) {
      fail(`recurring autopost = ${rec.autopost}, want true`);
    }
    const nd = rec.nextDue || "";
    if (!nd.startsWith(EXPECTED_NEXT_DUE_PREFIX)) {
      fail(`recurring nextDue = "${nd}", want date starting with "${EXPECTED_NEXT_DUE_PREFIX}"`);
    }
    console.log(`  recurring found: id=${rec.id} cadence=${rec.cadence} autopost=${rec.autopost} nextDue=${rec.nextDue}`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) {
    console.log(`PASS: transaction and monthly recurring schedule both created for "${UNIQUE_DESC}"; nextDue=${rec && rec.nextDue}.`);
  }
} finally {
  await browser.close();
}
