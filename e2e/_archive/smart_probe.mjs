import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
try {
  const browser = await chromium.launch({ headless: true });
  const p = await (await browser.newContext({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 2 })).newPage();
  await p.goto(BASE + "/", { waitUntil: "networkidle" });
  await p.waitForSelector(".bento .w", { timeout: 60000 });
  await p.waitForTimeout(3500);
  const data = await p.evaluate(() => {
    const dump = (label, el) => {
      if (!el) return { label, missing: true };
      const cs = getComputedStyle(el);
      const r = el.getBoundingClientRect();
      return {
        label, cls: el.className,
        radius: cs.borderTopLeftRadius, border: cs.borderTopWidth + " " + cs.borderTopColor,
        margin: cs.margin, x: Math.round(r.x), right: Math.round(r.right), w: Math.round(r.width),
      };
    };
    const bento = document.querySelector('.bento');
    const br = bento ? bento.getBoundingClientRect() : null;
    return {
      bentoX: br ? Math.round(br.x) : null, bentoRight: br ? Math.round(br.right) : null, bentoGap: bento ? getComputedStyle(bento).gap : null,
      tileNormal: dump("recent-tile", document.querySelector('[data-widget="recent"]')),
      tileDigest: dump("smart-digest-tile", document.querySelector('[data-widget="smart-digest"]')),
      stripCard: dump("smart-strip-card", document.querySelector('[data-testid^="smart-strip-"]:not(button)')),
    };
  });
  console.log(JSON.stringify(data, null, 1));
  await browser.close();
} catch (e) { console.error("EXCEPTION:", e.message); process.exit(1); }
