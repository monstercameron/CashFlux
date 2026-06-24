import { createRequire } from "module";
import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base = "http://127.0.0.1:8099";
const browser = await chromium.launch();
let pass = true; const log=(c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
const posOf = (page,name)=>page.evaluate((n)=>{const e=[...document.querySelectorAll(".set-label")].find(x=>x.textContent.trim()===n);return e?Math.round(e.getBoundingClientRect().top):null;},name);
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 760 } });
  await page.goto(base + "/dashboard", { waitUntil: "networkidle" });
  await page.waitForSelector("aside.rail", { timeout: 15000 });
  await page.click(".hh");
  await page.waitForSelector(".flip-backdrop.show", { timeout: 5000 });
  const nav = await page.$(".set-section-nav");
  log(!!nav, "settings section-nav renders at top of panel");
  log(nav && (await nav.$$("button")).length >= 10, "section-nav has ≥10 jump buttons");
  const before = await posOf(page, "Languages");
  await page.click('.set-section-nav button:has-text("Languages")');
  await page.waitForTimeout(700);
  const after = await posOf(page, "Languages");
  log(before !== null && after !== null && after < before - 100, `jump scrolled Languages up: ${before} → ${after}`);
} catch(e){ log(false, "exception: "+String(e)); }
finally { await browser.close(); }
console.log(pass ? "\nRESULT: ALL PASS" : "\nRESULT: FAILURES");
process.exit(pass?0:1);
