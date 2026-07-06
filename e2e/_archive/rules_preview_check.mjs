// L15 gate — "rules show a match-count preview". After adding a rule, the Rules
// screen shows a coverage line ("Your rules auto-file N of M transactions") and
// each rule row shows how many existing transactions it matches. Exits non-zero on
// any failure.
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

  await page.goto(BASE + "/rules", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#rule-add", { timeout: 60000 });

  // Add a rule whose phrase ("a") matches most sample transactions, so the preview
  // has something to count.
  await page.fill("#rule-add", "a");
  const cat = page.locator(".form-grid select").first();
  await cat.waitFor({ timeout: 5000 });
  await cat.selectOption({ index: 1 }); // first real category after the "choose…" placeholder
  await page.locator(".form-grid button[type=submit]").first().click();

  // A rule row should appear with a "Matches N transaction(s)" meta.
  await page.waitForSelector(".row-meta", { timeout: 60000 });
  const sawMatchMeta = await page.evaluate(() =>
    Array.from(document.querySelectorAll(".row-meta")).some((e) => /Matches\s+\d+/.test(e.textContent || "")),
  );
  if (!sawMatchMeta) fail('no rule row showed a "Matches N" preview');

  // The coverage line should report how many transactions auto-file.
  const sawCoverage = await page.evaluate(() =>
    Array.from(document.querySelectorAll(".muted")).some((e) => /auto-file \d+ of \d+/.test(e.textContent || "")),
  );
  if (!sawCoverage) fail("no rules coverage line was shown");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: rules screen shows per-rule match count + coverage preview.");
} finally {
  await browser.close();
}
