import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099"; const b = await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const alpha=s=>{const m=s.match(/[\d.]+/g)||[];return m.length>=4?parseFloat(m[3]):(/(rgb|#)/.test(s)?1:0)};
for(const theme of ["dark","light"]){
  const p=await b.newPage(); p.setViewportSize({width:1440,height:1100});
  await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
  await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:"domcontentloaded"});
  await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
  await p.evaluate(()=>{const x=[...document.querySelectorAll("button")].find(b=>/load sample|sample data/i.test(b.textContent)); if(x)x.click();});
  await p.waitForTimeout(1500);
  await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Categories"); if(l)l.click();});
  await p.waitForTimeout(1500);
  const m=await p.evaluate(()=>{const s=document.querySelector('.cat-map-sub'); if(!s)return null; const cs=getComputedStyle(s); return {bg:cs.backgroundColor,border:cs.borderTopColor,color:cs.color,txt:s.textContent.trim()};});
  if(!m){F(`${theme}: no sub-pill found`); await p.close(); continue;}
  console.log(`[${theme}] sub "${m.txt}" bg=${m.bg} border=${m.border} color=${m.color}`);
  if(alpha(m.bg)>0.02) P(`${theme}: sub-pill has a visible fill (${m.bg})`); else F(`${theme}: sub-pill fill transparent (${m.bg})`);
  if(alpha(m.border)>0.05) P(`${theme}: sub-pill has a visible border (${m.border})`); else F(`${theme}: sub-pill border invisible (${m.border})`);
  await p.screenshot({path:`e2e/screenshots/catmap_subpill_${theme}.png`, clip:await p.evaluate(()=>{const el=document.querySelector('.cat-map'); const r=el.getBoundingClientRect(); return {x:Math.max(0,r.left-8),y:Math.max(0,r.top-8),width:r.width+16,height:r.height+16};})});
  await p.close();
}
await b.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
