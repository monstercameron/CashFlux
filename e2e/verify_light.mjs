import { createRequire } from "module";
import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base = "http://127.0.0.1:8099";
const out = []; let allPass = true;
const ok = (c, m) => { out.push((c ? "PASS " : "FAIL ") + m); if (!c) allPass = false; };
const dark = (rgb) => { const m = rgb.match(/\d+/g); return m && (+m[0] + +m[1] + +m[2]) < 360; }; // avg < 120
const light = (rgb) => { const m = rgb.match(/\d+/g); return m && (+m[0] + +m[1] + +m[2]) > 600; }; // avg > 200
const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 860 } });
  await page.addInitScript(() => localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" })));
  await page.goto(base + "/rules", { waitUntil: "networkidle" });
  await page.waitForSelector("aside.rail", { timeout: 15000 });
  ok((await page.getAttribute("html", "data-theme")) === "light", "data-theme=light applied");
  const topbar = await page.evaluate(() => getComputedStyle(document.querySelector(".topbar")).backgroundColor);
  ok(light(topbar), "topbar is a light surface (" + topbar + ")");
  const card = await page.evaluate(() => { const c = document.querySelector(".card,.w"); return c ? getComputedStyle(c).backgroundColor : null; });
  ok(card && light(card), "card/tile is a light surface (" + card + ")");
  // §12 #3 — settings header h3 is dark (not near-white) in light theme
  await page.click(".hh");
  await page.waitForSelector(".flip-backdrop.show", { timeout: 5000 });
  const h3 = await page.evaluate(() => { const h = document.querySelector(".set-h h3"); return h ? getComputedStyle(h).color : null; });
  ok(h3 && dark(h3), "#12.3 settings header h3 is dark in light theme, not near-white (" + h3 + ")");
} catch (e) { ok(false, "exception: " + String(e)); }
finally { await browser.close(); }
console.log(out.join("\n"));
console.log(allPass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(allPass ? 0 : 1);
