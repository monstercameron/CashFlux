// L6 E2E story - "a wiped store stays empty; only a true first run seeds". The app
// used to re-seed the sample household whenever the dataset key was empty/missing,
// so wiping data and reloading brought a stranger's finances back. Now a "seeded"
// flag distinguishes a genuine first run from an intentionally-empty store.
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
async function waitForDataset(page, pred, timeoutMs = 9000) {
  let d = {};
  for (let waited = 0; waited < timeoutMs; waited += 400) {
    d = await dataset(page);
    if (pred(d)) return d;
    await page.waitForTimeout(400);
  }
  return d;
}
const nAccounts = (d) => (d.accounts || []).length;

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // 1. First run seeds the sample and sets the seeded flag.
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  const d1 = await waitForDataset(page, (d) => nAccounts(d) > 0);
  if (nAccounts(d1) === 0) fail("first run should seed the sample household");
  const seeded = await page.evaluate(() => localStorage.getItem("cashflux:seeded"));
  if (!seeded) fail("the 'seeded' flag should be set after a first-run seed");

  // 2. Clear the dataset key but keep the flag (a wipe) -> reload -> STAYS EMPTY.
  await page.evaluate(() => localStorage.removeItem("cashflux:dataset"));
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  await page.waitForTimeout(5000); // let hydrate decide + autosave write the result
  const d2 = await dataset(page);
  if (nAccounts(d2) !== 0) fail(`a wiped store re-seeded ${nAccounts(d2)} accounts — should stay empty`);

  // 3. Clear the dataset AND the flag (a genuine fresh install) -> reload -> RE-SEEDS.
  await page.evaluate(() => {
    localStorage.removeItem("cashflux:dataset");
    localStorage.removeItem("cashflux:seeded");
  });
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app *", { timeout: 60000 });
  const d3 = await waitForDataset(page, (d) => nAccounts(d) > 0);
  if (nAccounts(d3) === 0) fail("a genuine first run (no flag) should seed the sample again");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: a wiped store stays empty on reload; only a true first run (no seeded flag) re-seeds the sample.");
} finally {
  await browser.close();
}
