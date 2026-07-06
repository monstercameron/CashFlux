// C70 gate — "the Split settle-up renders a who-owes-whom diagram". Drives an
// active split (amount + sharers + payer) and asserts the Mermaid digraph renders
// to SVG. Exits non-zero on any failure.
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

  // Sample seeds one member; add a second so a split has a debtor and a payer.
  await page.goto(BASE + "/members", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#member-add", { timeout: 60000 });
  await page.locator("#member-add").fill("ZZSPLITDIAG");
  await page.locator("form button[type='submit']").first().click();
  await page.waitForTimeout(500);

  await page.goto(BASE + "/split", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="number"]', { timeout: 60000 });
  await page.locator('input[type="number"]').first().fill("100");
  const selectAll = page.getByRole("button", { name: "Select all" });
  if (await selectAll.count()) await selectAll.click();
  // Pick a payer (the first select in the form is the payer picker).
  await page.locator("select").first().selectOption({ index: 1 });
  await page.waitForTimeout(400);

  await page.waitForSelector(".cf-mermaid svg", { timeout: 20000 }).catch(() => fail("settle-up diagram did not render to <svg>"));
  if ((await page.locator(".cf-mermaid svg").count()) === 0) fail("expected a rendered settle-up <svg>");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: Split settle-up renders a who-owes-whom diagram to SVG.");
} finally {
  await browser.close();
}
