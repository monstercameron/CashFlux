// C70 gate — "the Reports money-flow Sankey renders". Exits non-zero on failure.
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

  await page.goto(BASE + "/reports", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".cf-mermaid", { timeout: 60000 });
  await page.waitForSelector(".cf-mermaid svg", { timeout: 20000 }).catch(() => fail("money-flow Sankey did not render to <svg>"));
  if ((await page.locator(".cf-mermaid svg").count()) === 0) fail("expected a rendered money-flow <svg>");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: Reports money-flow Sankey renders to SVG.");
} finally {
  await browser.close();
}
