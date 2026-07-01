import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const id = process.argv[2] || "smart-digest";
try {
  const browser = await chromium.launch({ headless: true });
  const p = await (await browser.newContext({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 2 })).newPage();
  await p.goto(BASE + "/", { waitUntil: "networkidle" });
  await p.waitForSelector(".bento .w", { timeout: 60000 });
  await p.waitForTimeout(3500);
  const el = await p.$(`[data-widget="${id}"]`);
  if (!el) { console.log("missing", id); process.exit(0); }
  // Center the tile in the viewport, then screenshot the viewport (shows real
  // clipping/overflow as the user sees it, unlike an element screenshot).
  await el.evaluate(n => n.scrollIntoView({ block: "center" }));
  await p.waitForTimeout(400);
  const geo = await el.evaluate(n => {
    const r = n.getBoundingClientRect();
    const wb = n.querySelector('.wbody');
    return {
      tileH: Math.round(r.height),
      bodyScrollH: wb ? wb.scrollHeight : null,
      bodyClientH: wb ? wb.clientHeight : null,
      overflowHidden: getComputedStyle(n).overflow,
    };
  });
  console.log(id, JSON.stringify(geo));
  await p.screenshot({ path: `e2e/screenshots/ctx_${id}.png` });
  console.log("shot: e2e/screenshots/ctx_" + id + ".png");
  await browser.close();
} catch (e) { console.error("EXCEPTION:", e.message); process.exit(1); }
