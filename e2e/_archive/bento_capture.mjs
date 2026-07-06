// Screenshot the dashboard .bento grid element (it scrolls inside main, so
// fullPage misses it) and report each tile's geometry + key classes.
import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const OUT = process.argv[2] || "bento";
try {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 2 });
  const p = await ctx.newPage();
  await p.goto(BASE + "/", { waitUntil: "networkidle" });
  await p.waitForSelector(".bento .w", { timeout: 60000 });
  await p.waitForTimeout(4000);
  const geo = await p.evaluate(() => {
    const out = [];
    document.querySelectorAll('.bento .w').forEach(w => {
      const r = w.getBoundingClientRect();
      const cs = getComputedStyle(w);
      out.push({
        id: w.getAttribute('data-widget'),
        gc: cs.gridColumn, gr: cs.gridRow,
        w: Math.round(r.width), h: Math.round(r.height),
        pad: cs.padding, border: cs.borderWidth, bg: cs.backgroundColor,
        hasWh: !!w.querySelector('.wh'), hasWbody: !!w.querySelector('.wbody'),
        cls: w.className,
      });
    });
    return out;
  });
  for (const g of geo) console.log(JSON.stringify(g));
  const bento = await p.$(".bento");
  if (bento) { await bento.screenshot({ path: `e2e/screenshots/${OUT}.png` }); console.log("shot: e2e/screenshots/" + OUT + ".png"); }
  await browser.close();
} catch (e) { console.error("EXCEPTION:", e.message); process.exit(1); }
