// C70/C64 gate — "the Rules screen shows a precedence-chain diagram". Adds two
// rules, then asserts the rule-order Mermaid chain renders to SVG. Exits non-zero
// on any failure.
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

const addRule = async (page, match) => {
  await page.locator("#rule-add").fill(match);
  await page.locator("form select").first().selectOption({ index: 1 });
  await page.getByRole("button", { name: "Add", exact: true }).first().click();
  await page.waitForTimeout(400);
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/rules", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#rule-add", { timeout: 60000 });

  await addRule(page, "ZZDIAGRULE-A");
  await addRule(page, "ZZDIAGRULE-B");

  await page.waitForSelector(".cf-mermaid svg", { timeout: 20000 }).catch(() => fail("rule-order chain did not render to <svg>"));
  if ((await page.locator(".cf-mermaid svg").count()) === 0) fail("expected a rendered rule-order <svg>");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: Rules screen renders the precedence-chain diagram to SVG.");
} finally {
  await browser.close();
}
