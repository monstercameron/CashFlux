// C70/C63 gate — "the Categories screen shows a rendered category-map diagram".
// Exits non-zero on any failure.
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

  await page.goto(BASE + "/categories", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".cf-mermaid", { timeout: 60000 });
  await page.waitForSelector(".cf-mermaid svg", { timeout: 20000 }).catch(() => fail("category map did not render to <svg>"));

  if ((await page.locator(".cf-mermaid svg").count()) === 0) fail("expected a rendered category-map <svg>");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: Categories screen renders the category-map diagram to SVG.");
} finally {
  await browser.close();
}
