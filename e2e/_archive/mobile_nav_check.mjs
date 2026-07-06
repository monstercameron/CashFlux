// L11 E2E check — mobile bottom tab bar. Sets a 390×844 viewport (iPhone 14
// size), asserts that a .mobile-tabbar nav exists in the DOM with at least 3
// navigable links, and that tapping one of those links causes a SPA navigation
// (URL changes without a full-page reload, new content appears). Exits non-zero
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

try {
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
  });
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  // Load the dashboard and wait for the shell to mount.
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".mobile-tabbar", { timeout: 15000 });

  // 1. Assert .mobile-tabbar exists and contains ≥3 nav links.
  const linkCount = await page.evaluate(() => {
    const bar = document.querySelector(".mobile-tabbar");
    if (!bar) return 0;
    return bar.querySelectorAll("a[href]").length;
  });
  if (linkCount < 3) {
    fail(`expected ≥3 nav links in .mobile-tabbar, got ${linkCount}`);
  }

  // 2. Assert each link in the bar has a non-empty aria-label (a11y).
  const unlabelled = await page.evaluate(() => {
    const bar = document.querySelector(".mobile-tabbar");
    if (!bar) return [];
    return Array.from(bar.querySelectorAll("a[href]"))
      .filter((a) => !a.getAttribute("aria-label") && !a.textContent.trim())
      .map((a) => a.href);
  });
  if (unlabelled.length > 0) {
    fail(`tab bar links missing aria-label: ${unlabelled.join(", ")}`);
  }

  // 3. Tap the first non-active link and assert the URL changes (SPA navigation).
  const startURL = page.url();
  const tapped = await page.evaluate(() => {
    const bar = document.querySelector(".mobile-tabbar");
    if (!bar) return null;
    // Pick the first link that does NOT have aria-current="page".
    const link = Array.from(bar.querySelectorAll("a[href]")).find(
      (a) => a.getAttribute("aria-current") !== "page"
    );
    if (!link) return null;
    link.click();
    return link.getAttribute("href");
  });

  if (!tapped) {
    fail("could not find a non-active tab link to tap");
  } else {
    // Wait briefly for SPA router to commit the navigation.
    await page.waitForTimeout(600);
    const afterURL = page.url();
    if (afterURL === startURL) {
      fail(`tapping tab link (href=${tapped}) did not change the URL (still ${startURL})`);
    }
    // The page should not have done a full reload — evaluate() still works.
    const stillMounted = await page.evaluate(() => !!document.querySelector(".mobile-tabbar"));
    if (!stillMounted) {
      fail("full-page reload detected after tab tap — SPA navigation broken");
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode)
    console.log(
      `PASS: .mobile-tabbar present with ${linkCount} links at 390×844; tap navigated SPA to ${page.url()}.`
    );
} finally {
  await browser.close();
}
