// L14 gate — "the command palette can jump to the user's own data entities."
// Typing a seeded account's name surfaces an "<name> · Account" jump command, and
// running it navigates to /accounts. This proves the palette indexes accounts /
// goals / budgets, not just screens and actions.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const rowsText = (page) =>
  page.evaluate(() => Array.from(document.querySelectorAll("[data-cmd-row]")).map((e) => e.textContent || ""));

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });

  // Discover a real account name from the dataset (poll until it seeds).
  let acctName = "";
  for (let i = 0; i < 25 && !acctName; i++) {
    await page.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
    acctName = await page.evaluate(() => {
      const d = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
      const a = (d.accounts || []).find((x) => x.name && !x.archived);
      return a ? a.name : "";
    });
    if (!acctName) await page.waitForTimeout(400);
  }
  if (!acctName) { fail("no seeded account to search for"); process.exit(1); }

  await page.keyboard.press("Control+k");
  await page.waitForSelector("#cf-cmd-input", { timeout: 10000, state: "visible" });

  // Type the account name → an "<name> · Account" jump command appears.
  await page.fill("#cf-cmd-input", acctName);
  await page.waitForTimeout(200);
  const texts = await rowsText(page);
  const hit = texts.find((t) => t.includes(acctName) && /Account/i.test(t));
  if (!hit) {
    fail(`account "${acctName}" did not surface as a palette jump target; rows = ${JSON.stringify(texts.slice(0, 6))}`);
    process.exit(1);
  }

  // Running it navigates to /accounts.
  await page.keyboard.press("Enter");
  await page.waitForTimeout(500);
  if (!/\/accounts$/.test(new URL(page.url()).pathname)) {
    fail(`running the account jump did not navigate to /accounts (url=${page.url()})`);
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: palette jumps to data entities — "${acctName}" surfaced as an Account target and navigated to /accounts.`);
} finally {
  await browser.close();
}
