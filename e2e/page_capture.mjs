// Capture any app route for visual review.
// Usage: node e2e/page_capture.mjs <route> <outname>
import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const ROUTE = process.argv[2] || "/";
const OUT = process.argv[3] || "page";
const errs = [];
try {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 1100 }, deviceScaleFactor: 2 });
  const p = await ctx.newPage();
  p.on("pageerror", e => errs.push(String(e)));
  p.on("console", m => { if (m.type() === "error") errs.push(m.text()); });
  await p.goto(BASE + ROUTE, { waitUntil: "networkidle" });
  await p.waitForSelector("#app", { timeout: 60000 });
  await p.waitForTimeout(4000);
  const tiles = await p.evaluate(() => document.querySelectorAll('.bento .w, [data-testid="help-tile"]').length);
  const bento = await p.evaluate(() => !!document.querySelector('.bento'));
  const title = await p.evaluate(() => { const h = document.querySelector('main h1'); return h ? h.textContent : null; });
  console.log("route:", ROUTE, "title:", title, "bento:", bento, "tiles:", tiles);
  console.log("console-errors:", errs.length, errs.slice(0, 5));
  await p.screenshot({ path: `e2e/screenshots/${OUT}.png`, fullPage: true });
  console.log("screenshot: e2e/screenshots/" + OUT + ".png");
  await browser.close();
} catch (e) {
  console.error("EXCEPTION:", e.message);
  process.exit(1);
}
