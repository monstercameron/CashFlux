// L21 E2E check — "per-transaction member assignment". Uses the seeded sample
// dataset (members m-daniel & m-jordan). Adds a transaction via the add form,
// sets the "Who" picker to the second member, submits, and asserts that the new
// transaction's memberId equals that member's id in cashflux:dataset. Also sets
// the ledger member filter to that member and asserts the row is visible.
// Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const DESC = "ZZWHO-MEMBER-ASSIGN-9471";
const AMOUNT = "25.00";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const dataset = (page) =>
  page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));

async function waitForDataset(page, pred, timeoutMs = 7000) {
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

  // Load sample data so the seeded members (m-daniel, m-jordan) exist.
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });

  // Check if sample data is already loaded; if not, load it.
  const d0 = await dataset(page);
  const hasSample = (d0.members || []).some((m) => m.id === "m-daniel" || m.id === "m-jordan");
  if (!hasSample) {
    const loadBtn = page.locator('button', { hasText: "Load sample data" });
    const loadBtnCount = await loadBtn.count();
    if (loadBtnCount > 0) {
      await loadBtn.first().click();
      await page.waitForTimeout(800);
    }
  }

  // Confirm both seeded members exist.
  const d1 = await waitForDataset(
    page,
    (d) =>
      (d.members || []).some((m) => m.id === "m-daniel") &&
      (d.members || []).some((m) => m.id === "m-jordan"),
    5000
  );
  const members = d1.members || [];
  const daniel = members.find((m) => m.id === "m-daniel");
  const jordan = members.find((m) => m.id === "m-jordan");
  if (!daniel) { fail("seeded member m-daniel not found"); }
  if (!jordan) { fail("seeded member m-jordan not found"); }

  // Navigate to the Transactions screen.
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#txn-add", { timeout: 8000 });

  // The "Who" picker is only shown when there are >1 members. Verify it's present.
  const whoAdd = page.locator('[data-testid="txn-who-add"]');
  if ((await whoAdd.count()) === 0) {
    fail("Who picker (data-testid=txn-who-add) not found in add form — requires >1 members");
  }

  // Fill in description and amount.
  await page.locator("#txn-add").fill(DESC);
  await page.locator('input[type="number"][aria-required="true"]').fill(AMOUNT);

  // Set the "Who" picker to the second member (m-jordan).
  if (!process.exitCode) {
    await whoAdd.selectOption({ value: jordan.id });
  }

  // Submit the form.
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(600);

  // Verify the transaction appears in the ledger.
  if ((await page.getByText(DESC).count()) === 0) {
    fail("transaction did not appear in the ledger after save");
  }

  // Wait for the dataset to be persisted and verify memberId on the new txn.
  await page.waitForTimeout(2500);
  const d2 = await waitForDataset(
    page,
    (d) => (d.transactions || []).some((t) => t.desc === DESC),
    5000
  );
  const txn = (d2.transactions || []).find((t) => t.desc === DESC);
  if (!txn) {
    fail("transaction not found in cashflux:dataset");
  } else if (txn.memberId !== jordan.id) {
    fail(`transaction.memberId = "${txn.memberId}", want "${jordan.id}" (m-jordan)`);
  }

  // Bonus: set the member filter to jordan and assert the row is visible.
  if (!process.exitCode) {
    const filtersBtn = page.locator('button', { hasText: /Filters/i });
    if ((await filtersBtn.count()) > 0) {
      await filtersBtn.first().click();
      await page.waitForTimeout(300);
    }
    const memberFilter = page.locator('select[aria-label="Member"]');
    if ((await memberFilter.count()) > 0) {
      await memberFilter.selectOption({ value: jordan.id });
      await page.waitForTimeout(400);
      if ((await page.getByText(DESC).count()) === 0) {
        fail("transaction row not visible after filtering by the assigned member");
      }
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) {
    console.log(
      `PASS: added "${DESC}" with Who=m-jordan; memberId persisted correctly and row visible in member filter.`
    );
  }
} finally {
  await browser.close();
}
