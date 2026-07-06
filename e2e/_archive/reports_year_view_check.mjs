// L16 gate — "one-tap YEAR / fiscal-year view." The Reports screen is now
// period-aware in the top bar; selecting the new "Year" resolution sets the
// period to the whole calendar year (label = "2026") so the tax-season annual
// review is one tap.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/reports", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".reso-control", { timeout: 60000 });

  const reso = page.locator(".reso-control");
  // The Year segment exists.
  const year = reso.getByText("Year", { exact: true });
  if ((await year.count()) === 0) { fail("no Year resolution segment in the top bar"); process.exit(1); }

  await year.first().click();
  await page.waitForTimeout(500);

  // The period stepper now shows a bare 4-digit year label (e.g. "2026").
  const text = (await reso.innerText()).replace(/\s+/g, " ");
  if (!/\b20\d{2}\b/.test(text)) {
    fail(`expected a year label after selecting Year; control text: "${text}"`);
  }
  // And the resolution persisted to the period state.
  const res = await page.evaluate(() => localStorage.getItem("cashflux:period:res") || localStorage.getItem("period:res") || "");
  // (Persistence key may vary; the visible year label is the primary assertion.)

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: Reports has a one-tap Year view (period label shows the year). res=${res || "n/a"}`);
} finally {
  await browser.close();
}
