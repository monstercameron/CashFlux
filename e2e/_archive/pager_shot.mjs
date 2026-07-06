// One-off visual check for the C47 reusable DataTable + pagination bar: boot the
// app at /transactions (seeded with sample data) and screenshot the table and its
// pager footer. Not pass/fail — writes e2e/txn-table.png for review.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1280, height: 1400 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".txn-table .row-desc", { timeout: 60000 });
  await page.waitForTimeout(400);

  await page.screenshot({ path: path.join(__dirname, "txn-table.png") });
  const pager = page.locator(".data-pager").first();
  if (await pager.count()) {
    await pager.scrollIntoViewIfNeeded();
    await pager.screenshot({ path: path.join(__dirname, "txn-pager.png") });
    console.log("pager text:", (await pager.innerText()).replace(/\s+/g, " ").trim());
  } else {
    console.log("NO .data-pager found");
  }
  console.log("page errors:", errors.length ? errors.join(" | ") : "none");
} finally {
  await browser.close();
}
