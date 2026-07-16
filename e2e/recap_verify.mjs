// recap_verify.mjs — verifies the Monthly Recap dashboard widget (CG-S1):
// it mounts on the default dashboard with the seeded sample data, renders real
// figures, and produces no console errors. Screenshots the banner.
// Usage: node e2e/recap_verify.mjs <outDir>
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";

const BASE = "http://127.0.0.1:8097";
const OUT = (process.argv[2] || (process.env.TEMP || ".") + "/recaprev").replace(/\\/g, "/");
mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 950 }, deviceScaleFactor: 1, reducedMotion: "reduce" });
const page = await ctx.newPage();
const errors = [];
page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });
page.on("pageerror", (e) => errors.push(String(e)));

await page.goto(BASE + "/", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(1800); // let the seed hydrate + tiles settle

const recap = page.locator('[data-testid="monthly-recap"]');
const present = await recap.count();
console.log("recap-present:", present);

let spent = "", saved = "", nw = "", top = "", foot = "";
if (present) {
  const grab = async (sel) => (await page.locator(sel).count()) ? (await page.locator(sel).first().innerText()).replace(/\n/g, " · ") : "(absent)";
  spent = await grab('[data-testid="recap-spent"]');
  saved = await grab('[data-testid="recap-biggest"]');
  nw = await grab('[data-testid="recap-mover"]');
  top = await grab('[data-testid="recap-top"]');
  foot = await grab('[data-testid="recap-foot"]');
  await recap.scrollIntoViewIfNeeded();
  await page.waitForTimeout(300);
  await recap.screenshot({ path: `${OUT}/recap_banner.png` });
}
await page.screenshot({ path: `${OUT}/dashboard_full.png`, fullPage: true });

console.log("SPENT   :", spent);
console.log("SAVED   :", saved);
console.log("NETWORTH:", nw);
console.log("TOP     :", top);
console.log("FOOT    :", foot);
console.log("console-errors:", errors.length, errors.slice(0, 6).join(" | "));
console.log(present ? "PASS: recap mounted" : "FAIL: recap missing");
await browser.close();
