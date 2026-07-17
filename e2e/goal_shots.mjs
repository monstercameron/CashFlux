// goal_shots.mjs — capture the nine primary pages at desktop + 390px into a
// named directory, for before/after UI-integrity comparison.
// Usage: node e2e/goal_shots.mjs <outDir> [port]
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";

const OUT = process.argv[2];
const PORT = process.argv[3] || "8097";
if (!OUT) { console.error("usage: node e2e/goal_shots.mjs <outDir> [port]"); process.exit(1); }
mkdirSync(OUT, { recursive: true });

const ROUTES = [
  ["dashboard", "/"], ["transactions", "/transactions"], ["accounts", "/accounts"],
  ["budgets", "/budgets"], ["goals", "/goals"], ["todo", "/todo"],
  ["notifications", "/notifications"], ["assistant", "/assistant"], ["reports", "/reports"],
];

const browser = await chromium.launch();
for (const [vpName, vp] of [["desktop", { width: 1440, height: 950 }], ["mobile", { width: 390, height: 844 }]]) {
  const ctx = await browser.newContext({ viewport: vp, reducedMotion: "reduce" });
  const page = await ctx.newPage();
  await page.goto(`http://127.0.0.1:${PORT}/`, { waitUntil: "load" });
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
  await page.waitForTimeout(1800);
  for (const [name, path] of ROUTES) {
    await page.evaluate((p) => { history.pushState({}, "", p); dispatchEvent(new PopStateEvent("popstate")); }, path);
    await page.waitForTimeout(1800);
    await page.screenshot({ path: `${OUT}/${name}-${vpName}.png`, fullPage: false });
  }
  await ctx.close();
}
await browser.close();
console.log("shots saved to", OUT);
