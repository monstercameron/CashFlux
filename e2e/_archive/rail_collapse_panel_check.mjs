// C20 gate — "on-panel rail collapse toggle is present and functional".
// Verifies that clicking the in-rail collapse button (data-testid="rail-collapse-btn")
// collapses the rail (adds the "collapsed" class to the <aside.rail>) and that
// clicking it again expands it. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import { ready } from "./_ready.mjs";

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
  await ready(page);

  // The on-panel collapse button must exist.
  const btn = page.locator("[data-testid='rail-collapse-btn']");
  if (!(await btn.count())) {
    fail("rail-collapse-btn not found in DOM");
  } else {
    const rail = page.locator("aside.rail");

    // Record initial collapsed state.
    const wasBefore = await rail.evaluate((el) => el.classList.contains("collapsed"));

    // Click the on-panel toggle once — rail state should flip.
    await btn.click();
    await page.waitForTimeout(150); // allow state + re-render
    const afterFirst = await rail.evaluate((el) => el.classList.contains("collapsed"));
    if (afterFirst === wasBefore) {
      fail(`rail collapsed state did not change after first click (before=${wasBefore}, after=${afterFirst})`);
    }

    // Click again — should revert.
    await btn.click();
    await page.waitForTimeout(150);
    const afterSecond = await rail.evaluate((el) => el.classList.contains("collapsed"));
    if (afterSecond !== wasBefore) {
      fail(`rail collapsed state did not revert after second click (expected=${wasBefore}, got=${afterSecond})`);
    }
  }

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: on-panel rail collapse toggle is present and toggles the rail correctly.");
} finally {
  await browser.close();
}
