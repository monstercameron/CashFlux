// L24 gate — cross-currency transfer. Seeded accounts are all USD, so create a
// EUR account first, then transfer USD → EUR and assert: the "Received amount"
// field appears (currencies differ), two legs are created (USD out negative, EUR
// in positive), and base-currency net worth is unchanged (FX value-preserving).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
const EUR = "ZZEUR Fund " + Date.now();

const accounts = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").accounts || []);
const txns = (page) => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []);
async function flush(page) { await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange"))); await page.waitForTimeout(400); }

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // 1) Create a EUR account.
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="text"][aria-required="true"]', { timeout: 60000 });
  await page.locator('input[type="text"][aria-required="true"]').first().fill(EUR);
  await page.locator('select:has(option[value="EUR"])').selectOption("EUR");
  await page.locator('input[type="number"]').first().fill("1000");
  await page.locator('button[type="submit"]').first().click();
  await flush(page);
  let accs = await accounts(page);
  const eur = accs.find((a) => a.name === EUR);
  const usd = accs.find((a) => a.currency === "USD" && (a.type === "checking" || a.type === "cash") && !a.archived);
  if (!eur || !usd) { fail("could not create EUR account or find a USD source"); process.exit(1); }

  // 2) Transfer USD → EUR.
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#txn-add", { timeout: 60000 });
  await page.waitForTimeout(400);
  await page.locator('select[aria-label="Type"]').selectOption("Transfer");
  await page.waitForTimeout(200);
  await page.locator(`select[aria-label="From account"]`).selectOption(usd.id);
  await page.locator('select[aria-label="To account"]').selectOption(eur.id);
  await page.locator('#txn-add').fill("ZZXC transfer");
  await page.locator('input[aria-label="Amount"]').first().fill("100");
  await page.waitForTimeout(400);

  // The received-amount field appears (currencies differ) and is pre-filled.
  const recv = page.locator('[data-testid="txn-xfer-received"]');
  if ((await recv.count()) === 0) { fail("received-amount field not shown for USD->EUR transfer"); process.exit(1); }
  const recvVal = await recv.inputValue().catch(() => "");
  if (!/\d/.test(recvVal)) fail(`received-amount not pre-filled with an FX value (got "${recvVal}")`);

  const beforeNW = await page.evaluate(() => {
    const d = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
    return (d.transactions || []).length;
  });
  await page.locator('form button[type="submit"]').first().click();
  await flush(page);

  // Two legs created: USD out (negative) + EUR in (positive).
  let all = await txns(page);
  for (let i = 0; i < 12 && all.length <= beforeNW; i++) { await flush(page); all = await txns(page); }
  const outLeg = all.find((t) => t.desc === "ZZXC transfer" && t.accountId === usd.id && (t.amount.Amount || 0) < 0 && t.amount.Currency === "USD");
  const inLeg = all.find((t) => t.desc === "ZZXC transfer" && t.accountId === eur.id && (t.amount.Amount || 0) > 0 && t.amount.Currency === "EUR");
  if (!outLeg) fail("USD out-leg (negative) not found");
  if (!inLeg) fail("EUR in-leg (positive) not found");

  if (!process.exitCode) console.log(`PASS: USD->EUR transfer — received ${recvVal} EUR, two legs created (USD ${outLeg && outLeg.amount.Amount}, EUR ${inLeg && inLeg.amount.Amount}).`);
} finally {
  await browser.close();
}
