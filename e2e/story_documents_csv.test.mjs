// B16 E2E story — "documents: import transactions from CSV". Pastes a small CSV
// into the Documents importer and asserts the transaction it describes is created
// in the ledger (persisted to the dataset). Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const PAYEE = "ZZDOCIMPORT";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const dataset = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
async function waitForDataset(page, pred, timeoutMs = 8000) {
  let d = {};
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    d = await dataset(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return d;
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/documents", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("textarea", { timeout: 60000 });

  // Find a real account name (comma-free) to reference in the CSV's account column.
  const d0 = await waitForDataset(page, (d) => (d.accounts || []).some((a) => a.name && !a.name.includes(",")));
  const acct = (d0.accounts || []).find((a) => a.name && !a.name.includes(","));
  if (!acct) fail("no usable account found for the import");
  const acctName = acct && acct.name;

  // Paste a one-row CSV in the importer's DOCUMENTED shape (date,payee,amount,
  // account) and import it. The payee fills the required description (the C27 fix),
  // so this documented shape actually imports.
  const csv = `date,payee,amount,account\n2026-06-05,${PAYEE},12.34,${acctName}`;
  await page.locator("textarea").first().fill(csv);
  await page.locator("form", { has: page.locator("textarea") }).locator('button[type="submit"]').first().click();

  // The imported transaction shows in the dataset (payee filled the description).
  const d1 = await waitForDataset(page, (d) => (d.transactions || []).some((t) => t.desc === PAYEE));
  const txn = (d1.transactions || []).find((t) => t.desc === PAYEE);
  if (!txn) fail("CSV-imported transaction not found in the dataset");
  else if (txn.accountId !== acct.id) fail(`imported txn account = ${txn.accountId}, want ${acct.id} (${acctName})`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: imported "${PAYEE}" from CSV into "${acctName}" (persisted).`);
} finally {
  await browser.close();
}
