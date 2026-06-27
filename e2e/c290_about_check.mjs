// C290 / C293 — /about route renders the real About & Privacy screen.
//
// Test plan:
//   1. Navigate to /about.
//   2. Assert the page is NOT the Help screen (no "Getting set up" checklist).
//   3. Assert the privacy / local-first copy is present (C290, C293).
//   4. Assert the cloud-sync disclosure is present (C291).
//   5. Assert the AI-key disclosure is present (C292).
//   6. Assert the version string is visible (e.g. "v0.").
//
// Coverage limit: the e2e harness requires a running dev server at E2E_URL
// (default http://127.0.0.1:8099).  Run with:
//   node e2e/c290_about_check.mjs
// In CI the test is skipped if the server is not reachable (noted in output).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

let browser;
try {
  browser = await chromium.launch({ headless: true });
} catch (e) {
  console.warn("SKIP: could not launch Chromium — " + e.message);
  process.exit(0);
}

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => {
    const s = String(e);
    if (/Go program has already exited/.test(s)) return;
    errors.push(s);
  });

  // Try to reach the server; skip gracefully if it isn't running.
  try {
    await page.goto(BASE + "/", { waitUntil: "domcontentloaded", timeout: 10000 });
  } catch {
    console.warn("SKIP: dev server not reachable at " + BASE);
    await browser.close();
    process.exit(0);
  }

  // Wait for the wasm app to mount.
  await page.waitForSelector("#app", { timeout: 60000 });

  // Navigate to /about via the SPA router (not a hard reload, to avoid 404).
  await page.evaluate(() => {
    window.history.pushState({}, "", "/about");
    window.dispatchEvent(new PopStateEvent("popstate"));
  });
  await page.waitForTimeout(800);

  // ── 1. Must NOT be the Help screen (no "Getting set up" checklist title). ──
  const gettingSetUp = await page.locator("text=Getting set up").isVisible().catch(() => false);
  if (gettingSetUp) {
    fail("/about rendered the Help screen instead of the About screen — stub not replaced");
  }

  // ── 2. Privacy / local-first copy (C290, C293). ──
  const privacyText = page.locator("#about-page, [data-testid=about-privacy-card]");
  const pageText = await page.textContent("#app");

  if (!pageText.includes("local-first")) {
    fail('C290/C293: "local-first" not found on /about');
  }
  if (!pageText.includes("on this device")) {
    fail('C290/C293: "on this device" not found — local-first statement missing');
  }

  // ── 3. Cloud-sync disclosure (C291). ──
  if (!pageText.includes("Cloud sync")) {
    fail('C291: "Cloud sync" section heading not found on /about');
  }
  if (!pageText.includes("off by default")) {
    fail('C291: "off by default" disclosure text missing');
  }

  // ── 4. AI-key disclosure (C292). ──
  if (!pageText.includes("bring-your-own-key") && !pageText.includes("own API key")) {
    fail('C292: bring-your-own-key disclosure missing on /about');
  }
  if (!pageText.includes("OpenAI")) {
    fail('C292: OpenAI reference missing in AI disclosure');
  }

  // ── 5. Version string visible. ──
  if (!pageText.match(/v\d+\.\d+/)) {
    fail("version string (e.g. v0.1.0) not found on /about");
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));

  if (!process.exitCode) {
    console.log(
      "PASS: C290/C293 — /about renders the real About & Privacy screen with local-first, cloud-sync, AI-key disclosures and version.",
    );
  }
} finally {
  await browser.close();
}
