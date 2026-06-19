// Deep E2E for left-rail navigation (regression guard for the routing + active-
// highlight bug). Loads the built wasm app and, for EVERY rail item, clicks it
// and asserts: the URL pathname updates, the screen <h1> heading matches, the
// active highlight follows the click (exactly one active item, and it's the one
// we clicked), and the chrome never duplicates (exactly one rail + one top bar).
//
// Run via e2e/run.ps1 (builds wasm, serves web/, runs this). Uses the Playwright
// install under .tools/node_modules. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

// path -> expected <h1> heading (resolved English nav labels). Mirrors
// internal/screens.All() + internal/i18n nav.* values; if a screen is added,
// add it here so the rail stays fully covered.
const ROUTES = [
  ["/", "Dashboard"],
  ["/accounts", "Accounts"],
  ["/transactions", "Transactions"],
  ["/budgets", "Budgets"],
  ["/goals", "Goals"],
  ["/todo", "To-do"],
  ["/planning", "Planning"],
  ["/allocate", "Allocate"],
  ["/reports", "Reports"],
  ["/subscriptions", "Subscriptions"],
  ["/bills", "Bills"],
  ["/split", "Split"],
  ["/insights", "Insights"],
  ["/documents", "Documents"],
  ["/customize", "Customize"],
  ["/artifacts", "Artifacts"],
  ["/workflows", "Workflows"],
  ["/members", "Members"],
  ["/categories", "Categories"],
  ["/rules", "Rules"],
];

const RAIL = 'nav[aria-label="Main navigation"]';

const failures = [];
function check(cond, msg) {
  if (!cond) failures.push(msg);
}

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  // The 54MB wasm takes a moment to boot; wait for the rail to render.
  await page.waitForSelector(`${RAIL} a[title]`, { timeout: 60000 });

  for (const [routePath, heading] of ROUTES) {
    const btn = page.locator(`${RAIL} a[title="${heading}"]`).first();
    if ((await btn.count()) === 0) {
      failures.push(`${routePath}: rail item "${heading}" not found`);
      continue;
    }
    await btn.click();

    // URL must update to the route.
    await page
      .waitForFunction((p) => location.pathname === p, routePath, { timeout: 8000 })
      .catch(() => {});
    const loc = await page.evaluate(() => location.pathname);
    check(loc === routePath, `${routePath}: URL pathname = "${loc}"`);

    // Wait for the screen to actually render (heading updates), then assert it —
    // this is robust against the re-render being a tick behind the URL push.
    await page
      .waitForFunction((h) => document.querySelector("h1")?.textContent?.trim() === h, heading, { timeout: 8000 })
      .catch(() => {});
    const h1 = (await page.locator("h1").first().textContent())?.trim();
    check(h1 === heading, `${routePath}: <h1> = "${h1}", want "${heading}"`);

    // Active highlight must follow the click: exactly one active rail item, and
    // it is the one we navigated to (active class adds "font-medium").
    const actives = page.locator(`${RAIL} a.font-medium`);
    const activeCount = await actives.count();
    check(activeCount === 1, `${routePath}: ${activeCount} active rail items, want 1`);
    if (activeCount >= 1) {
      const activeTitle = await actives.first().getAttribute("title");
      check(
        activeTitle === heading,
        `${routePath}: active item is "${activeTitle}", want "${heading}"`
      );
    }

    // Chrome must never duplicate (the B3 outlet bug rendered two shells).
    const rails = await page.locator(RAIL).count();
    const topbars = await page.locator(".topbar").count();
    check(rails === 1, `${routePath}: ${rails} rails rendered, want 1`);
    check(topbars === 1, `${routePath}: ${topbars} top bars rendered, want 1`);
  }

  check(errors.length === 0, `page errors: ${errors.join(" | ")}`);
} finally {
  await browser.close();
}

if (failures.length) {
  console.error(`\nE2E FAILED (${failures.length}):`);
  for (const f of failures) console.error("  - " + f);
  process.exit(1);
}
console.log(`\nE2E PASSED: ${ROUTES.length} rail items navigate, highlight follows, chrome single.`);
