// L15 gate — "rule → matching transaction auto-categorizes → survives reload."
// Creates a rule (match a unique phrase → a real category), then adds a
// transaction whose description contains that phrase and asserts the new
// transaction is auto-filed into the rule's category (the SuggestTransactionFields
// path) and that it persists across a reload. This is the core "set it and
// forget it" round-trip, which no prior e2e covered.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const PHRASE = "ZZRULEMATCH" + Date.now();
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

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // 1) Create a rule: match PHRASE → the first real category.
  await page.goto(BASE + "/rules", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#rule-add", { timeout: 60000 });
  await page.fill("#rule-add", PHRASE);
  // The rule form's category <select> is the form-grid select; pick the first
  // non-empty option and remember its id + label.
  const catSelect = page.locator("form .field").locator("xpath=self::select").first();
  // Fall back to the first <select> in the add form.
  const select = (await catSelect.count()) ? catSelect : page.locator("form select").first();
  const catId = await select.evaluate((el) => {
    const opt = [...el.options].find((o) => o.value);
    el.value = opt.value;
    el.dispatchEvent(new Event("change", { bubbles: true }));
    return opt.value;
  });
  if (!catId) { fail("no real category option to build the rule with"); process.exit(1); }
  // Submit the rule.
  await page.locator('form button[type="submit"]').first().click();
  // Confirm the rule persisted.
  const dRule = await waitDS(page, (d) => (d.rules || []).some((r) => r.Match === PHRASE || r.match === PHRASE));
  const rule = (dRule.rules || []).find((r) => (r.Match || r.match) === PHRASE);
  if (!rule) { fail("rule did not persist"); process.exit(1); }
  const ruleCat = rule.SetCategoryID || rule.setCategoryID;
  if (ruleCat !== catId) fail(`rule category = ${ruleCat}, want ${catId}`);

  // 2) Add a transaction whose description contains PHRASE — leave the category
  // for the rule to fill.
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[aria-label]', { timeout: 60000 });
  // Description field, then amount; the add form's first text input is the desc.
  const descInput = page.getByPlaceholder(/description|payee|what/i).first();
  await descInput.fill(`Coffee at ${PHRASE} downtown`);
  await page.waitForTimeout(400); // let SuggestTransactionFields run
  const amountInput = page.locator('input[type="number"]').first();
  await amountInput.fill("4.50");
  await page.locator('form button[type="submit"]').first().click();

  // 3) The new transaction persists with the rule's category.
  const d2 = await waitDS(page, (d) => (d.transactions || []).some((t) => (t.desc || "").includes(PHRASE)));
  const txn = (d2.transactions || []).find((t) => (t.desc || "").includes(PHRASE));
  if (!txn) { fail("matching transaction not found"); process.exit(1); }
  if (txn.categoryId !== catId) fail(`transaction category = ${txn.categoryId || "(none)"}, want ${catId} (rule auto-categorize)`);

  // 4) Survives reload.
  await page.reload({ waitUntil: "domcontentloaded" });
  const d3 = await waitDS(page, (d) => (d.transactions || []).some((t) => (t.desc || "").includes(PHRASE)));
  const txn3 = (d3.transactions || []).find((t) => (t.desc || "").includes(PHRASE));
  if (!txn3 || txn3.categoryId !== catId) fail("auto-categorization did not survive reload");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: rule matched "${PHRASE}" → transaction auto-filed to category ${catId}; survived reload.`);
} finally {
  await browser.close();
}
