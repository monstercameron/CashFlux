// L6 gate — wipe via Settings → reload → assert zero accounts (no re-seed).
// After a Settings→Wipe, the seeded flag remains set; on reload the hydrate
// decision should be hydrateEmpty, not hydrateSeed, so the sample household
// never comes back.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const accounts = (page) =>
  page.evaluate(() => (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").accounts || []));
async function flush(page) {
  await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
  await page.waitForTimeout(400);
}
async function waitAccounts(page, pred, ms = 9000) {
  for (let w = 0; w < ms; w += 400) {
    const accs = await accounts(page);
    if (pred(accs)) return accs;
    await page.waitForTimeout(400);
  }
  return accounts(page);
}

try {
  const page = await browser.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // 1. Boot → sample seeds on first run.
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  const before = await waitAccounts(page, (a) => a.length > 0);
  if (before.length === 0) fail("first run should seed the sample (got 0 accounts)");

  // 2. Open Settings (household card) and wipe.
  await page.locator(".hh").click();
  await page.getByRole("button", { name: "Wipe data" }).first().scrollIntoViewIfNeeded();
  await page.getByRole("button", { name: "Wipe data" }).first().click();
  await page.locator("#cf-dialog-confirm").click();
  await page.waitForTimeout(400);

  // Flush the autosave so the empty dataset lands in localStorage before reload.
  await flush(page);

  // 3. Reload — hydrate must stay empty (seeded flag is still set).
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.waitForTimeout(2500); // let hydrate + autosave settle

  const after = await accounts(page);
  if (after.length !== 0) {
    fail(`wipe → reload re-seeded ${after.length} accounts — should stay empty`);
  }

  if (!process.exitCode)
    console.log("PASS: wipe via Settings → reload → 0 accounts (no re-seed).");
} finally {
  await browser.close();
}
