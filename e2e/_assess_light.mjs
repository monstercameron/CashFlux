import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage(); page.setViewportSize({width:1280,height:1000});
const errs=[]; page.on("pageerror",e=>errs.push(String(e)));
await page.goto(BASE+"/",{waitUntil:"domcontentloaded"});
await page.evaluate(()=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:'light'})));
await page.reload({waitUntil:"domcontentloaded"});
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:60000});
await page.waitForFunction(()=>document.documentElement.getAttribute('data-theme')==='light',{timeout:10000}).catch(()=>{});
const navTo = async (t)=>{await page.evaluate((t)=>{const l=Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')).find(x=>x.getAttribute("title")===t); if(l)l.click();},t); await page.waitForTimeout(1500);};

// contrast auditor: find text elements whose color is near-white AND background near-white
const audit = async (label) => {
  const bad = await page.evaluate(() => {
    const parse = (c) => { const m=c.match(/[\d.]+/g); return m?m.map(Number):[0,0,0]; };
    const lum = ([r,g,b]) => 0.299*r+0.587*g+0.114*b;
    const bgOf = (el) => { let e=el; while(e){const c=getComputedStyle(e).backgroundColor; if(c&&c!=="rgba(0, 0, 0, 0)"&&c!=="transparent")return parse(c); e=e.parentElement;} return [255,255,255]; };
    const out=[];
    for (const el of document.querySelectorAll('#cf-page-view *')) {
      const t=el.textContent?.trim(); if(!t||t.length>60||el.children.length>0) continue;
      const cs=getComputedStyle(el); const col=parse(cs.color); const bg=bgOf(el);
      const cl=lum(col), bl=lum(bg);
      // low contrast: both light OR both dark, and a real delta threshold
      const contrast=Math.abs(cl-bl);
      if (contrast<60 && t.length>1) out.push({t:t.slice(0,40), color:cs.color, bgLum:Math.round(bl), colLum:Math.round(cl), contrast:Math.round(contrast)});
    }
    return out.slice(0,15);
  });
  console.log(`\n[${label}] low-contrast text (contrast<60):`, bad.length);
  for (const b of bad) console.log(`   "${b.t}" color=${b.color} colLum=${b.colLum} bgLum=${b.bgLum} Δ=${b.contrast}`);
};
for (const scr of ["Reports","Budgets","Goals","Dashboard"]) {
  await navTo(scr); await page.waitForTimeout(900);
  await page.screenshot({path:`e2e/screenshots/light_${scr}.png`, fullPage:true});
  await audit(scr);
}
console.log("\nERRS:", errs.slice(0,5));
await browser.close();
