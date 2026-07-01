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
  const d = await p.evaluate(() => {
    const rect = s => { const e = document.querySelector(s); if (!e) return null; const r = e.getBoundingClientRect(); return { top: Math.round(r.top), bottom: Math.round(r.bottom), left: Math.round(r.left), right: Math.round(r.right) }; };
    const strip = document.querySelector('[data-testid^="smart-strip-"]:not(button)');
    // What is the strip's previous and next sibling in the DOM?
    const sib = (el, dir) => { const n = dir === "prev" ? el?.previousElementSibling : el?.nextElementSibling; return n ? { tag: n.tagName, cls: (n.className || "").slice(0, 40), testid: n.getAttribute("data-testid") } : null; };
    return {
      pageView: rect('#cf-page-view'),
      strip: rect('[data-testid^="smart-strip-"]:not(button)'),
      stripParent: strip ? (strip.parentElement.className || strip.parentElement.id || strip.parentElement.tagName) : null,
      stripPrev: sib(strip, "prev"),
      stripNext: sib(strip, "next"),
      bento: rect('.bento'),
      hero: rect('[data-widget="hero"]'),
    };
  });
  console.log(JSON.stringify(d, null, 1));
  await browser.close();
} catch (e) { console.error("EXCEPTION:", e.message); process.exit(1); }
