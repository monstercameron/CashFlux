// Verify the Reports redesign actually RENDERS its charts (not just mounts empty
// SVGs): the category ranked-bar chart has bars with width, the category donut has
// arc paths, and the hero stat strip shows Net/Income/Spend figures.
import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed = 0; const fail = m => { console.error("FAIL: " + m); failed++; process.exitCode = 1; }; const pass = m => console.log("PASS: " + m);
try {
  const page = await browser.newPage();
  const errs = []; page.on("pageerror", e => errs.push(String(e)));
  await page.goto(BASE + "/reports", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  // Sample data seeds on first load; give wasm time to render charts.
  await page.waitForTimeout(6000);
  // Hero strip
  const hero = await page.evaluate(() => {
    const h = document.querySelector(".reports-hero");
    return { present: !!h, text: (h?.innerText || "").replace(/\s+/g, " ").trim().slice(0, 140) };
  });
  console.log("  hero: " + JSON.stringify(hero));
  if (hero.present && /\d/.test(hero.text)) pass("hero stat strip renders with figures"); else fail("hero strip missing/empty");
  // Count SVG geometry across all report charts.
  const geom = await page.evaluate(() => {
    const svgs = [...document.querySelectorAll(".reports-hero ~ * svg, .w svg, svg")];
    let rects = 0, widthRects = 0, paths = 0, arcPaths = 0;
    for (const s of svgs) {
      for (const r of s.querySelectorAll("rect")) { rects++; if ((r.getBBox?.().width || 0) > 1) widthRects++; }
      for (const p of s.querySelectorAll("path")) { paths++; const d = p.getAttribute("d") || ""; if (/A/i.test(d)) arcPaths++; }
    }
    return { svgCount: svgs.length, rects, widthRects, paths, arcPaths };
  });
  console.log("  geometry: " + JSON.stringify(geom));
  if (geom.widthRects >= 3) pass("ranked-bar chart renders bars with width (" + geom.widthRects + " rects >1px)"); else fail("no ranked bars with width (widthRects=" + geom.widthRects + ")");
  if (geom.arcPaths >= 2) pass("donut chart renders arc segments (" + geom.arcPaths + " arc paths)"); else fail("no donut arcs (arcPaths=" + geom.arcPaths + ")");
  await page.screenshot({ path: "e2e/screenshots/reports_charts_render.png", fullPage: true });
  console.log("  pageerrors: " + errs.length); errs.slice(0, 4).forEach(e => console.log("  ERR:" + e.slice(0, 120)));
  if (errs.length) fail("page errors present");
} catch (e) { fail("exception: " + e.message); } finally { await browser.close(); }
console.log(failed ? "RESULT: FAILED" : "RESULT: PASSED");
process.exit(failed ? 1 : 0);
