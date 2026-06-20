// C47 E2E — "transactions filter toolbar: popover + active-filter chips". Drives
// the compact toolbar that replaced the inline filter strip: a search term raises
// the active-filter badge and a removable chip; the Filters button opens the
// FlipPanel popover and a second filter raises the count; the chip ✕ clears just
// that filter; "Clear all filters" empties them. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const DESC = "ZZTOOLBAR-77";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#txn-add", { timeout: 60000 });

  // Seed a uniquely described transaction so the search filter has a match.
  await page.locator("#txn-add").fill(DESC);
  await page.locator('input[type="number"][aria-required="true"]').fill("4.21");
  await page.locator('button[type="submit"]').first().click();
  await page.waitForTimeout(600);

  // No filters yet → no badge, no chips.
  if ((await page.locator(".filter-badge").count()) !== 0) fail("badge should be absent with no active filters");
  if ((await page.locator(".filter-chip").count()) !== 0) fail("no chips should show with no active filters");

  // A search term → badge "1" and one chip naming the search.
  await page.locator(".filter-search").fill(DESC);
  await page.waitForTimeout(400);
  if ((await page.locator(".filter-badge").first().innerText()) !== "1") fail("badge should read 1 after a search filter");
  if ((await page.locator(".filter-chip").count()) !== 1) fail("one chip should show for the search filter");
  if (!(await page.locator(".filter-chip").first().innerText()).includes(DESC)) fail("the chip should name the search term");

  // Open the Filters popover and add a second filter (cleared = yes).
  await page.locator(".filters-trigger").click();
  await page.waitForSelector(".flip-wrap", { timeout: 5000 });
  await page.locator(".filter-fields select").last().selectOption("yes");
  await page.waitForTimeout(300);
  if ((await page.locator(".filter-badge").first().innerText()) !== "2") fail("badge should read 2 after adding the cleared filter");
  // Close the popover (Escape).
  await page.keyboard.press("Escape");
  await page.waitForTimeout(300);
  if ((await page.locator(".flip-wrap").count()) !== 0) fail("popover should close on Escape");
  if ((await page.locator(".filter-chip").count()) !== 2) fail("two chips should show (search + cleared)");

  // Remove the search chip via its ✕ → search clears, one filter remains.
  const searchChip = page.locator(".filter-chip", { hasText: DESC });
  await searchChip.locator(".chip-x").click();
  await page.waitForTimeout(300);
  if ((await page.locator(".filter-search").inputValue()) !== "") fail("removing the search chip should clear the search box");
  if ((await page.locator(".filter-badge").first().innerText()) !== "1") fail("badge should drop to 1 after removing the search chip");

  // Clear all filters → no chips, no badge.
  await page.locator(".chip-clear-all").click();
  await page.waitForTimeout(300);
  if ((await page.locator(".filter-chip").count()) !== 0) fail("Clear all filters should remove every chip");
  if ((await page.locator(".filter-badge").count()) !== 0) fail("badge should be gone after clearing all filters");

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: filter toolbar — search chip + badge, popover adds cleared (2), ✕ removes one, Clear all empties.");
} finally {
  await browser.close();
}
