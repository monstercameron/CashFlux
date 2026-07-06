// W-1 WONDER foundation verification.
// Checks card hover lift (W-1) in default, off, and reduced-motion modes.
// Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
let passed = 0;
let failed = 0;
const fail = (m) => { console.error("FAIL: " + m); failed++; process.exitCode = 1; };
const pass = (m) => { console.log("PASS: " + m); passed++; };

// Returns computed transform + wonder-on for the first .card.
const cardState = (page) =>
  page.evaluate(() => {
    const card = document.querySelector(".card");
    if (!card) return null;
    const cs = getComputedStyle(card);
    const rootCs = getComputedStyle(document.documentElement);
    return {
      transform: cs.transform,
      transitionDuration: cs.transitionDuration,
      wonderOn: rootCs.getPropertyValue("--wonder-on").trim(),
    };
  });

// Identity matrix = no transform.
const isIdentity = (t) =>
  !t || t === "none" || t === "matrix(1, 0, 0, 1, 0, 0)";

// Navigate to a page that has .card elements.
// The SPA requires following real link clicks after the WASM app boots.
const navigateToCards = async (page) => {
  await page.goto(BASE + "/", { waitUntil: "networkidle" });
  await page.waitForSelector("#app", { timeout: 60_000 });
  await page.waitForTimeout(4000); // WASM boot
  // Try budgets route (confirmed to have .card).
  await page.click('a[href="/budgets"]');
  await page.waitForTimeout(2000);
  const count = await page.evaluate(() => document.querySelectorAll(".card").length);
  if (count > 0) return;
  // Fallback: try dashboard.
  await page.click('a[href="/"]');
  await page.waitForTimeout(2000);
};

const consoleErrors = [];

try {
  const ctx = await browser.newContext();
  const p = await ctx.newPage();
  p.on("pageerror", (e) => consoleErrors.push(String(e)));

  await navigateToCards(p);

  const cardCount = await p.evaluate(() => document.querySelectorAll(".card").length);
  if (cardCount === 0) {
    fail("Setup: no .card elements found on page — cannot run W-1 tests");
    console.log(`Summary: ${passed} passed, ${failed} failed.`);
    await browser.close();
    process.exit(1);
  }
  console.log(`  Found ${cardCount} .card element(s) on ${await p.evaluate(() => location.href)}`);

  // ---- CASE 1: DEFAULT (full wonder) ----
  await p.evaluate(() => document.documentElement.removeAttribute("data-wonder"));

  const wonderOnDefault = await p.evaluate(() =>
    getComputedStyle(document.documentElement).getPropertyValue("--wonder-on").trim()
  );
  if (wonderOnDefault !== "1") {
    fail(`Case 1: --wonder-on in default mode = "${wonderOnDefault}", want "1"`);
  } else {
    pass(`Case 1: --wonder-on default = "${wonderOnDefault}"`);
  }

  // Move mouse away first, then hover the card.
  await p.mouse.move(0, 0);
  await p.waitForTimeout(50);
  await p.hover(".card");
  // Give transition time to fire (--wonder-dur = 170ms; wait longer for headless).
  await p.waitForTimeout(350);

  const defaultState = await cardState(p);
  if (!defaultState) {
    fail("Case 1: no .card found");
  } else {
    console.log(`  transform (default hover): ${defaultState.transform}`);
    console.log(`  transition-duration: ${defaultState.transitionDuration}`);
    if (isIdentity(defaultState.transform)) {
      fail(`Case 1: hover transform is identity/none — card lift not firing (transform="${defaultState.transform}")`);
    } else {
      pass(`Case 1: hover yields non-identity transform: ${defaultState.transform}`);
    }
    // transition-duration: CSS reports comma-separated values for the 3 transition props.
    // We expect the first value to be ~0.17s (170ms).
    if (defaultState.transitionDuration && defaultState.transitionDuration.includes("0.17")) {
      pass(`Case 1: transition-duration includes 0.17s (170ms) — ${defaultState.transitionDuration}`);
    } else if (defaultState.transitionDuration && defaultState.transitionDuration !== "0s") {
      pass(`Case 1: transition-duration is set (${defaultState.transitionDuration})`);
    } else {
      fail(`Case 1: unexpected transition-duration="${defaultState.transitionDuration}"`);
    }
  }

  // Screenshot with card hovered.
  await p.screenshot({ path: path.join(__dirname, "w1_verify_hover.png") });
  console.log("  Screenshot saved: e2e/w1_verify_hover.png");

  // Reset hover.
  await p.mouse.move(0, 0);
  await p.waitForTimeout(100);

  // ---- CASE 2: OFF mode ----
  await p.evaluate(() => document.documentElement.setAttribute("data-wonder", "off"));

  const wonderOnOff = await p.evaluate(() =>
    getComputedStyle(document.documentElement).getPropertyValue("--wonder-on").trim()
  );
  if (wonderOnOff !== "0") {
    fail(`Case 2: --wonder-on in off mode = "${wonderOnOff}", want "0"`);
  } else {
    pass(`Case 2: --wonder-on off = "${wonderOnOff}"`);
  }

  await p.hover(".card");
  await p.waitForTimeout(100);

  const offState = await cardState(p);
  if (!offState) {
    fail("Case 2: no .card found");
  } else {
    console.log(`  transform (off hover): ${offState.transform}`);
    if (isIdentity(offState.transform)) {
      pass("Case 2: off mode hover = identity/none (no lift)");
    } else {
      fail(`Case 2: off mode hover still has non-identity transform: ${offState.transform}`);
    }
  }

  await p.mouse.move(0, 0);
  await p.waitForTimeout(100);

  // ---- CASE 3: REDUCED-MOTION ----
  await p.evaluate(() => document.documentElement.removeAttribute("data-wonder"));
  await p.emulateMedia({ reducedMotion: "reduce" });

  const wonderOnReduced = await p.evaluate(() =>
    getComputedStyle(document.documentElement).getPropertyValue("--wonder-on").trim()
  );
  console.log(`  --wonder-on under reduced-motion: "${wonderOnReduced}"`);

  await p.hover(".card");
  await p.waitForTimeout(100);

  const reducedState = await cardState(p);
  if (!reducedState) {
    fail("Case 3: no .card found");
  } else {
    console.log(`  transform (reduced-motion hover): ${reducedState.transform}`);
    if (isIdentity(reducedState.transform)) {
      pass("Case 3: reduced-motion hover = identity/none (no lift)");
    } else {
      fail(`Case 3: reduced-motion hover has non-identity transform: ${reducedState.transform}`);
    }
  }

  // ---- CASE 4: No console errors ----
  if (consoleErrors.length) {
    fail(`Case 4: page errors detected: ${consoleErrors.join(" | ")}`);
  } else {
    pass("Case 4: no page/console errors");
  }

  console.log(`\nSummary: ${passed} passed, ${failed} failed.`);
} finally {
  await browser.close();
}
