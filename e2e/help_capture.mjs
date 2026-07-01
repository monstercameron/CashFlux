// Capture the /help page (widgetized) for visual review.
// Usage: node e2e/help_capture.mjs [outname]
import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const OUT = process.argv[2] || "help";
const errs = [];
try {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 1100 }, deviceScaleFactor: 2 });
  const p = await ctx.newPage();
  p.on("pageerror", e => errs.push(String(e)));
  p.on("console", m => { if (m.type() === "error") errs.push(m.text()); });
  await p.goto(BASE + "/help", { waitUntil: "networkidle" });
  await p.waitForSelector("#app", { timeout: 60000 });
  // Wait for the help tiles to mount.
  await p.waitForSelector('[data-testid="help-tile"]', { timeout: 60000 }).catch(() => {});
  await p.waitForTimeout(3500);
  const tiles = await p.evaluate(() => document.querySelectorAll('[data-testid="help-tile"]').length);
  const bento = await p.evaluate(() => !!document.querySelector('.bento'));
  const title = await p.evaluate(() => { const h = document.querySelector('main h1'); return h ? h.textContent : null; });
  // Detect clipped tiles: scrollHeight > clientHeight on .wbody (content overflow).
  const clipped = await p.evaluate(() => {
    const out = [];
    document.querySelectorAll('[data-testid="help-tile"]').forEach(t => {
      const b = t.querySelector('.wbody');
      const id = t.getAttribute('data-widget');
      if (b && b.scrollHeight - b.clientHeight > 4) out.push({ id, over: b.scrollHeight - b.clientHeight });
    });
    return out;
  });
  console.log("help-title:", title);
  console.log("bento-present:", bento, "tiles:", tiles);
  console.log("overflowing-tiles:", JSON.stringify(clipped));
  console.log("console-errors:", errs.length, errs.slice(0, 5));
  await p.screenshot({ path: `e2e/screenshots/${OUT}.png`, fullPage: true });
  console.log("screenshot: e2e/screenshots/" + OUT + ".png");
  await browser.close();
} catch (e) {
  console.error("EXCEPTION:", e.message);
  process.exit(1);
}
