// Verifies the /debt de-alias (FEATURE_MAP §5.7a): /debt is now a real scoped
// "What you owe" page, NOT the old full Planning() screen. Also documents whether
// /planning still renders the debt widgets (single-theme completeness check).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const fails = [];
const ok = (cond, msg) => { if (!cond) fails.push(msg); else console.log("  ✓", msg); };
const info = (msg) => console.log("  •", msg);

// Planning-only sections that must NOT appear on a properly-scoped /debt page.
const PLANNING_ONLY = ["Net worth in 12 months", "Can I afford it?", "Cash runway"];

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1440, height: 900 });
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/debt", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(800);

  // /debt = scoped "What you owe" page.
  let body = await page.locator("main").innerText();
  ok(/What you owe|Total owed/.test(body), '/debt shows the "What you owe" / total-owed hero');
  ok(/Debt payoff strategy/.test(body), "/debt shows the debt-strategy panel");
  ok(/Debt payoff calculator/.test(body), "/debt shows the payoff calculator (moved from /planning)");
  const leakedToDebt = PLANNING_ONLY.filter((s) => body.includes(s));
  ok(leakedToDebt.length === 0,
     `/debt is scoped — no Planning-only sections leak in (${leakedToDebt.join(", ") || "none"})`);

  // /planning = forecasting screen.
  await page.locator('nav a[title="Planning"]').first().click();
  await page.waitForTimeout(800);
  body = await page.locator("main").innerText();
  ok(/Net worth in 12 months/.test(body), "/planning shows the 12-month forecast");
  ok(/Can I afford it\?/.test(body), "/planning shows the affordability tool");

  // Single-theme completeness: /planning must no longer render any debt widget.
  ok(!/Debt payoff strategy/.test(body), "/planning no longer shows the debt-strategy panel");
  ok(!/Debt payoff calculator/.test(body), "/planning no longer shows the payoff calculator");

  ok(errors.length === 0, `no page errors (${errors.length ? errors.join(" | ") : "none"})`);
} finally {
  await browser.close();
}

if (fails.length) {
  console.error("\nFAIL:\n - " + fails.join("\n - "));
  process.exit(1);
}
console.log("\nPASS: /debt is a real scoped page (see NOTE above re: /planning narrowing)");
