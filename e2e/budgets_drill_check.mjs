// C50 gate — "a budget drills into its transactions". Clicking a budget's title
// navigates to Transactions filtered to that budget's category (mirrors
// Accounts→Transactions / dashboard tile-click). Exits non-zero on any failure.
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

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".budget-drill", { timeout: 60000 });

  if ((await page.locator(".budget-drill").count()) === 0) fail("expected at least one drillable budget row");
  await page.locator(".budget-drill").first().click();

  // We should land on the Transactions screen…
  await page.waitForFunction(() => location.pathname.endsWith("/transactions"), { timeout: 5000 }).catch(() => fail("did not navigate to /transactions"));
  await page.waitForSelector("#txn-add", { timeout: 60000 });

  // …with the persisted tx-filter scoped to a category.
  const cat = await page.evaluate(() => {
    try {
      return (JSON.parse(localStorage.getItem("cashflux:tx-filter") || "{}")).category || "";
    } catch {
      return "";
    }
  });
  if (!cat) fail("the tx-filter category was not set by the drill-down");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: budget drill-down → /transactions filtered to category "${cat}".`);
} finally {
  await browser.close();
}
