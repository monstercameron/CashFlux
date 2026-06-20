// C56 gate — "a detected subscription drills into its charges". Clicking a
// subscription's name opens Transactions searched for that payee, so the user can
// verify the detection. Exits non-zero on any failure.
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

  await page.goto(BASE + "/subscriptions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".sub-drill", { timeout: 60000 });

  const name = (await page.locator(".sub-drill").first().innerText()).trim();
  await page.locator(".sub-drill").first().click();

  await page.waitForFunction(() => location.pathname.endsWith("/transactions"), { timeout: 5000 }).catch(() => fail("did not navigate to /transactions"));
  await page.waitForSelector("#txn-add", { timeout: 60000 });

  const text = await page.evaluate(() => {
    try {
      return (JSON.parse(localStorage.getItem("cashflux:tx-filter") || "{}")).text || "";
    } catch {
      return "";
    }
  });
  if (!text) fail("the tx-filter search text was not set by the subscription drill-down");
  if (text !== name) fail(`tx-filter text = "${text}", expected the payee "${name}"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: subscription drill-down → /transactions searched for "${text}".`);
} finally {
  await browser.close();
}
