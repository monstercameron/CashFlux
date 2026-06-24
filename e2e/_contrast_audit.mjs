import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage(); page.setViewportSize({width:1280,height:1000});
await page.goto(BASE+"/",{waitUntil:"domcontentloaded"});
await page.evaluate(()=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:'light'})));
await page.reload({waitUntil:"domcontentloaded"});
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:60000});
await page.waitForFunction(()=>document.documentElement.getAttribute('data-theme')==='light',{timeout:8000}).catch(()=>{});
const navTo = async (t)=>{await page.evaluate((t)=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")===t); if(l)l.click();},t); await page.waitForTimeout(1300);};

const audit = async (label)=>{
  const bad = await page.evaluate(()=>{
    const parse=c=>{const m=(c||"").match(/[\d.]+/g);return m?m.slice(0,3).map(Number):null};
    const lum=([r,g,b])=>0.2126*r+0.7152*g+0.0722*b;
    const ratio=(a,b)=>{const L1=lum(a)/255,L2=lum(b)/255;const hi=Math.max(L1,L2),lo=Math.min(L1,L2);return (hi+0.05)/(lo+0.05)};
    const bgOf=el=>{let e=el;while(e&&e!==document.documentElement){const c=getComputedStyle(e).backgroundColor;const p=parse(c);if(p&&!/, 0\)$/.test(c))return p;e=e.parentElement;}return [247,246,243];};
    const out=[];
    const els=[...document.querySelectorAll('main *, #cf-page-view *')];
    for(const el of els){
      const txt=el.textContent&&el.textContent.trim();
      if(!txt||txt.length<2||txt.length>50) continue;
      // only leaf-ish text nodes
      if([...el.children].some(c=>c.textContent&&c.textContent.trim().length>0)) continue;
      const cs=getComputedStyle(el);
      const isSvgText = el.namespaceURI && el.namespaceURI.includes('svg');
      const colStr = isSvgText ? cs.fill : cs.color;
      const col=parse(colStr); if(!col) continue;
      if(cs.visibility==='hidden'||cs.display==='none'||parseFloat(cs.opacity)<0.1) continue;
      const bg=bgOf(el);
      const r=ratio(col,bg);
      if(r<2.0) out.push({t:txt.slice(0,36), col:colStr, bg:`rgb(${bg.join(',')})`, ratio:+r.toFixed(2), svg:isSvgText});
    }
    // dedupe by text+col
    const seen=new Set(); return out.filter(o=>{const k=o.t+o.col; if(seen.has(k))return false; seen.add(k); return true;}).slice(0,12);
  });
  console.log(`\n[${label}] contrast<2.0: ${bad.length}`);
  for(const b of bad) console.log(`   "${b.t}" ${b.svg?'fill':'color'}=${b.col} bg=${b.bg} ratio=${b.ratio}`);
  return bad.length;
};
let total=0;
for(const scr of ["Planning","Allocate","To-do","Subscriptions","Bills","Insights","Accounts"]) { await navTo(scr); total+=await audit(scr); }
console.log("\nTOTAL low-contrast findings:", total);
await browser.close();
