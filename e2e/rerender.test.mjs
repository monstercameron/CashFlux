// Re-render stress E2E: a regression guard for the "page duplicates itself on
// rerender" symptom. Fires many re-render triggers — rail collapse toggle,
// add-menu open/close, rapid same-route re-clicks, cross navigation, and browser
// back/forward — and asserts the chrome never duplicates (exactly one rail, top
// bar, <h1>, and #app subtree throughout). Exits non-zero if duplication appears.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const RAIL = 'nav[aria-label="Main navigation"]';

const counts = (page) =>
  page.evaluate(() => ({
    app: document.querySelectorAll("#app").length,
    rails: document.querySelectorAll('nav[aria-label="Main navigation"]').length,
    asides: document.querySelectorAll("aside").length,
    topbars: document.querySelectorAll(".topbar").length,
    h1s: document.querySelectorAll("h1").length,
    shells: document.querySelectorAll("#app > div.flex.h-screen").length,
  }));

const failures = [];
async function assertSingle(page, label) {
  const c = await counts(page);
  for (const [k, v] of Object.entries(c)) {
    if (v !== 1) failures.push(`${label}: ${k} = ${v}, want 1 (${JSON.stringify(c)})`);
  }
}

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(`${RAIL} a[title]`, { timeout: 60000 });
  await assertSingle(page, "baseline");

  // Rail collapse/expand toggling.
  for (let i = 0; i < 6; i++) {
    await page.locator(".menu-btn").first().click().catch(() => {});
    await page.waitForTimeout(120);
    await assertSingle(page, `collapse#${i}`);
  }
  // Add-menu open/close.
  for (let i = 0; i < 4; i++) {
    await page.getByText("+ Add").first().click().catch(() => {});
    await page.waitForTimeout(100);
    await page.locator("body").click({ position: { x: 4, y: 4 } }).catch(() => {});
    await assertSingle(page, `addmenu#${i}`);
  }
  // Rapid same-route re-clicks + cross navigation.
  for (const name of ["Accounts", "Transactions", "Accounts", "Accounts", "Budgets", "Dashboard"]) {
    await page.locator(`${RAIL} a[title="${name}"]`).first().click().catch(() => {});
    await page.waitForTimeout(150);
    await assertSingle(page, `nav:${name}`);
  }
  // Browser history.
  for (let i = 0; i < 4; i++) {
    await page.goBack().catch(() => {});
    await page.waitForTimeout(150);
    await assertSingle(page, `back#${i}`);
  }
  for (let i = 0; i < 4; i++) {
    await page.goForward().catch(() => {});
    await page.waitForTimeout(150);
    await assertSingle(page, `forward#${i}`);
  }

  if (errors.length) failures.push(`page errors: ${errors.join(" | ")}`);
} finally {
  await browser.close();
}

if (failures.length) {
  console.error(`\nRERENDER E2E FAILED (${failures.length}):`);
  for (const f of failures) console.error("  - " + f);
  process.exit(1);
}
console.log("\nRERENDER E2E PASSED: chrome stays single across collapse/add-menu/nav/history stress.");
