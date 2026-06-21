// C74 — statement import engine end-to-end. Pastes a multi-format statement into
// the Documents "Import a bank or card statement" card, parses it (auto column
// mapping), reviews the drafts, imports them into an account, and asserts the
// transactions landed with the right signed minor-unit amounts. Re-imports to
// prove duplicate rows are skipped.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

// A statement with a non-obvious header order, a separate Debit/Credit pair, a
// parenthesised/sign-free layout, and one unparseable date row that must be
// skipped (not abort the import).
const CSV = [
  "Posting Date,Memo,Debit,Credit",
  "06/01/2026,STMT Paycheck,,1500.00",
  "06/02/2026,STMT Coffee,4.50,",
  "not-a-date,STMT Skip,9.99,",
].join("\n");

try {
  const page = await (await browser.newContext()).newPage();
  page.on("dialog", async (d) => { fail("native dialog opened: " + d.type()); await d.dismiss(); });
  page.on("pageerror", (e) => fail("page error: " + e.message));
  page.on("console", (m) => { if (/panic/i.test(m.text())) fail("console panic: " + m.text()); });

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: 60000 });
  await page.waitForTimeout(500);

  // Reach the Documents screen. It lives under the "Data & import" Tools
  // sub-section; expand that if its items are collapsed, then click through.
  if ((await page.locator('nav a[title="Documents"]').count()) === 0) {
    await page.locator('.rail-subhead', { hasText: "Data & import" }).first().click();
    await page.waitForTimeout(250);
  }
  await page.locator('nav a[title="Documents"]').first().click();
  await page.waitForTimeout(500);

  // The statement card's textarea is the first on the screen (it renders above the
  // CSV importer). Paste and parse.
  const stmt = page.locator("textarea").first();
  await stmt.waitFor({ timeout: 10000 });
  await stmt.fill(CSV);
  await page.getByRole("button", { name: "Parse statement", exact: true }).click();
  await page.waitForTimeout(500);

  // Review list should show the two good rows and not the bad-date row.
  const review = page.locator(".card", { hasText: "Review" });
  if ((await review.count()) === 0) fail("no review card appeared after parsing");
  if ((await page.locator("text=STMT Paycheck").count()) === 0) fail("Paycheck draft missing");
  if ((await page.locator("text=STMT Coffee").count()) === 0) fail("Coffee draft missing");
  if ((await page.locator("text=STMT Skip").count()) !== 0) fail("bad-date row was not skipped");

  // Import the reviewed rows into the default account.
  await page.getByRole("button", { name: "Import these", exact: true }).click();

  const dataset = () => page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
  const matching = (d) => (d.transactions || []).filter((t) => /STMT (Paycheck|Coffee)/.test(t.desc || t.payee || ""));

  // Flush autosave, then poll localStorage for the two imported transactions.
  let txns = [];
  for (let i = 0; i < 20; i++) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    txns = matching(await dataset());
    if (txns.length >= 2) break;
    await page.waitForTimeout(300);
  }
  if (txns.length !== 2) fail(`imported txn count = ${txns.length}, want 2`);

  const amt = (t) => (t.amount && (t.amount.amount ?? t.amount.Amount));
  const paycheck = txns.find((t) => /Paycheck/.test(t.desc));
  const coffee = txns.find((t) => /Coffee/.test(t.desc));
  if (paycheck && amt(paycheck) !== 150000) fail(`Paycheck amount = ${amt(paycheck)}, want 150000`);
  if (coffee && amt(coffee) !== -450) fail(`Coffee amount = ${amt(coffee)}, want -450`);

  // Dedupe: re-parse and re-import the same statement — the duplicate rows are
  // skipped, so the count stays at 2.
  await page.locator("textarea").first().fill(CSV);
  await page.getByRole("button", { name: "Parse statement", exact: true }).click();
  await page.waitForTimeout(400);
  await page.getByRole("button", { name: "Import these", exact: true }).click();
  await page.waitForTimeout(600);
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
  const after = matching(await dataset());
  if (after.length !== 2) fail(`dedupe failed: count after re-import = ${after.length}, want 2`);

  if (!process.exitCode) console.log("PASS: statement parsed (auto-mapped), bad row skipped, 2 txns imported, dedupe held.");
} finally {
  await browser.close();
}
