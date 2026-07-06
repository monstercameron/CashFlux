// C48 gate — "dashboard uses the semantic type scale". Asserts the dashboard
// renders the hero figure (.t-figure-lg) and primary figures (.t-figure) at the
// expected sizes, that the type tokens actually resolve to a font-size, and that
// no ad-hoc text-[Npx] arbitrary classes remain on the page. Exits non-zero on
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

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app .bento, #app", { timeout: 60000 });
  await page.waitForTimeout(800);

  // The hero figure (savings rate) renders and resolves to ~34px (2.125rem).
  const heroPx = await page.locator(".t-figure-lg").first().evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
  if (!(heroPx >= 32 && heroPx <= 36)) fail(`.t-figure-lg font-size = ${heroPx}px, want ~34px`);

  // At least one primary figure (.t-figure) renders at ~24px (1.5rem).
  const figCount = await page.locator(".t-figure").count();
  if (figCount < 1) fail("expected at least one .t-figure primary figure");
  const figPx = await page.locator(".t-figure").first().evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
  if (!(figPx >= 22 && figPx <= 26)) fail(`.t-figure font-size = ${figPx}px, want ~24px`);

  // Body + caption tokens resolve to a real size (not 0 / unset).
  for (const cls of [".t-body", ".t-caption"]) {
    const px = await page.locator(cls).first().evaluate((el) => parseFloat(getComputedStyle(el).fontSize));
    if (!(px > 8)) fail(`${cls} did not resolve to a font-size (got ${px})`);
  }

  // No leftover ad-hoc arbitrary px font classes within the dashboard bento grid
  // (the app shell is out of C48's scope and checked separately).
  const adhoc = await page.evaluate(() => {
    const hits = [];
    const root = document.querySelector(".bento");
    if (!root) return ["__no_bento__"];
    root.querySelectorAll("[class]").forEach((el) => {
      if (/text-\[\d+px\]/.test(el.getAttribute("class") || "")) hits.push(el.getAttribute("class"));
    });
    return hits;
  });
  if (adhoc.length) fail(`found ${adhoc.length} leftover text-[Npx] classes, e.g. "${adhoc[0]}"`);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: dashboard type scale — hero ${heroPx}px, primary ${figPx}px, ${figCount} primary figures, no ad-hoc px.`);
} finally {
  await browser.close();
}
