// L14 gate — "the command palette matches verbs via keyword aliases". Typing a
// verb like "add" surfaces the noun-labeled "New transaction" command (its label
// contains no "add", so this only works through the fuzzy keyword matcher), and
// "export" ranks the Export commands. Exits non-zero on any failure.
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

const rowsText = (page) =>
  page.evaluate(() =>
    Array.from(document.querySelectorAll("[data-cmd-row]")).map((e) => e.textContent || ""),
  );

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });

  // Open the palette (Ctrl+K) and confirm the input is up.
  await page.keyboard.press("Control+k");
  await page.waitForSelector("#cf-cmd-input", { timeout: 10000, state: "visible" });

  // "add" is not a substring of any command label, so a match for "New transaction"
  // can only come from the keyword alias matcher.
  await page.fill("#cf-cmd-input", "add");
  await page.waitForTimeout(150);
  let texts = await rowsText(page);
  if (!texts.some((t) => /New transaction/i.test(t))) {
    fail('typing "add" did not surface "New transaction" via keyword alias; rows = ' + JSON.stringify(texts));
  }

  // "export" should rank the Export commands.
  await page.fill("#cf-cmd-input", "export");
  await page.waitForTimeout(150);
  texts = await rowsText(page);
  if (!texts.some((t) => /Export/i.test(t))) {
    fail('typing "export" surfaced no Export command; rows = ' + JSON.stringify(texts));
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: command palette fuzzy keyword matching works (add → New transaction, export → Export).");
} finally {
  await browser.close();
}
