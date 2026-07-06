// L2 E2E story - "settle up across shared expenses". Adds three members, saves
// three shared expenses with different payers via the Split screen, asserts the
// running settle-up ledger shows the right net balances + the minimal payment,
// records that payment, and asserts everyone squares up and it survives a reload.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

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
const railTo = (page, title) => page.locator(`nav[aria-label="Main navigation"] a[title="${title}"]`).click();

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // 1. Add three members.
  await page.goto(BASE + "/members", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#member-add", { timeout: 60000 });
  for (const name of ["Priya", "Sam", "Lee"]) {
    await page.locator("#member-add").fill(name);
    await page.locator('button[type="submit"]').first().click();
    await page.waitForTimeout(400);
  }
  await waitForDataset(page, (d) => ["Priya", "Sam", "Lee"].every((n) => (d.members || []).some((m) => m.name === n)));

  // 2. Save three shared expenses, each split evenly three ways but paid by a
  //    different member: Priya $90, Sam $60, Lee $30.
  await railTo(page, "Split");
  await page.waitForSelector('input[type="number"][aria-label]', { timeout: 8000 });

  async function saveExpense(amount, payer) {
    await page.locator(".card input[type=number]").first().fill(amount);
    await page.locator(".card select").first().selectOption({ label: payer });
    for (const name of ["Priya", "Sam", "Lee"]) {
      await page.locator(`[role="switch"][aria-label="${name}"]`).click();
    }
    await page.locator('button:has-text("Save split")').click();
    await page.waitForTimeout(500);
  }
  await saveExpense("90", "Priya");
  await saveExpense("60", "Sam");
  await saveExpense("30", "Lee");

  // The seeded sample already includes a roommate-split demo (se-dinner/se-groceries
  // + settle-1), so assert by the test's own data rather than absolute counts.
  await waitForDataset(page, (d) => (d.sharedExpenses || []).length >= 3);

  // 3. The ledger nets out to: Priya +$30 owed, Lee owes $30, Sam settled. The
  //    single minimal payment is Lee -> Priya $30.
  const panel = page.locator(".card", { hasText: "Running balance across every saved split" });
  const panelText = (await panel.innerText()).replace(/\s+/g, " ");
  if (!panelText.includes("Priya is owed") || !panelText.includes("$30.00"))
    fail(`ledger should show Priya is owed $30.00: ${panelText}`);
  if (!panelText.includes("Lee owes")) fail(`ledger should show Lee owes: ${panelText}`);
  if (panelText.includes("Sam owes") || panelText.includes("Sam is owed"))
    fail(`Sam should be settled (no balance): ${panelText}`);
  if (!panelText.includes("Lee pays Priya")) fail(`minimal payment should be Lee pays Priya: ${panelText}`);
  await page.screenshot({ path: path.join(__dirname, "settle-up.png") });

  // 4. Record THIS scenario's payment (the Lee->Priya row, not the sample's demo
  //    transfer) and assert our three members square up — Priya/Lee drop out of the
  //    ledger. (The sample's roommate demo may still show its own balance.)
  const beforeN = (await dataset(page)).settlements?.length || 0;
  await page
    .locator(".row", { hasText: "Lee pays Priya" })
    .locator('button:has-text("Record settlement")')
    .first()
    .click();
  await waitForDataset(page, (d) => (d.settlements || []).length === beforeN + 1);
  await page.waitForTimeout(400);
  const after = (await panel.innerText()).replace(/\s+/g, " ");
  if (after.includes("Lee pays Priya")) fail(`payment should be gone after recording: ${after}`);
  if (after.includes("Priya is owed") || after.includes("Lee owes"))
    fail(`Priya/Lee should be settled after recording: ${after}`);

  // 5. Survives reload: the recorded settlement persists, Lee/Priya stay settled.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1000);
  const d = await dataset(page);
  if ((d.settlements || []).length !== beforeN + 1)
    fail(`after reload want ${beforeN + 1} settlements, got ${(d.settlements || []).length}`);
  const afterReload = (await panel.innerText()).replace(/\s+/g, " ");
  if (afterReload.includes("Lee pays Priya")) fail(`settlement did not persist: ${afterReload}`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: 3 shared expenses settle to Lee->Priya $30; recording it squares everyone up; survives reload.");
} finally {
  await browser.close();
}
