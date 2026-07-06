import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
try {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 2 });
  const p = await ctx.newPage();
  await p.goto(BASE + "/", { waitUntil: "networkidle" });
  await p.waitForSelector(".bento .w", { timeout: 60000 });
  await p.waitForTimeout(4000);
  for (const id of ["smart-digest", "anomaly-hub", "highlight", "freshness"]) {
    const el = await p.$(`[data-widget="${id}"]`);
    if (!el) { console.log(id, "MISSING"); continue; }
    await el.scrollIntoViewIfNeeded();
    await p.waitForTimeout(300);
    await el.screenshot({ path: `e2e/screenshots/tile_${id}.png` });
    // Dump the header + body inner structure for comparison.
    const info = await el.evaluate(n => {
      const wh = n.querySelector('.wh');
      const wb = n.querySelector('.wbody');
      return {
        whHTML: wh ? wh.innerHTML.slice(0, 300) : null,
        whText: wh ? wh.textContent.trim() : null,
        wbClass: wb ? wb.className : null,
        wbChildCount: wb ? wb.children.length : 0,
        wbFirstClass: wb && wb.firstElementChild ? wb.firstElementChild.className : null,
        bodyPadTop: wb ? getComputedStyle(wb).paddingTop : null,
      };
    });
    console.log(id, JSON.stringify(info));
  }
  await browser.close();
} catch (e) { console.error("EXCEPTION:", e.message); process.exit(1); }
