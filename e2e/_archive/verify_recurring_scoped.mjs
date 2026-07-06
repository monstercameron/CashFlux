// Verifies the /recurring de-alias (FEATURE_MAP §5.3): /recurring is now a scoped
// "Recurring cash flows" page, NOT the full Planning() screen, and /planning no
// longer renders the recurring manager.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const fails = [];
const ok = (cond, msg) => { if (!cond) fails.push(msg); else console.log("  ✓", msg); };

// Planning-only sections that must NOT appear on a properly-scoped /recurring page.
const PLANNING_ONLY = ["Net worth in 12 months", "Can I afford it?", "Debt payoff"];

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1440, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/recurring", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(800);

  let body = await page.locator("main").innerText();
  ok(/Recurring cash flows/.test(body), '/recurring shows the recurring cash-flow manager');
  const leaked = PLANNING_ONLY.filter((s) => body.includes(s));
  ok(leaked.length === 0,
     `/recurring is scoped — no Planning-only sections leak in (${leaked.join(", ") || "none"})`);

  await page.locator('nav a[title="Planning"]').first().click();
  await page.waitForTimeout(800);
  body = await page.locator("main").innerText();
  ok(/Net worth in 12 months/.test(body), "/planning still shows the forecast");
  ok(!/Recurring cash flows/.test(body), "/planning no longer shows the recurring manager");

  ok(errors.length === 0, `no page errors (${errors.length ? errors.join(" | ") : "none"})`);
} finally {
  await browser.close();
}

if (fails.length) {
  console.error("\nFAIL:\n - " + fails.join("\n - "));
  process.exit(1);
}
console.log("\nPASS: /recurring is a real scoped page; /planning narrowed");
