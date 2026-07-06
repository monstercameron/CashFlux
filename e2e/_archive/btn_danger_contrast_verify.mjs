import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const lin=c=>{c/=255;return c<=0.03928?c/12.92:Math.pow((c+0.055)/1.055,2.4)};
const L=([r,g,b])=>0.2126*lin(r)+0.7152*lin(g)+0.0722*lin(b);
const parse=s=>s.match(/[\d.]+/g).slice(0,3).map(Number);
const ratio=(fg,bg)=>{const a=L(parse(fg)),b=L(parse(bg));const hi=Math.max(a,b),lo=Math.min(a,b);return (hi+0.05)/(lo+0.05)};
const BASE="http://127.0.0.1:8099"; const browser=await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
for(const theme of ["dark","light"]){
  const p=await browser.newPage();
  await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
  await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:"domcontentloaded"}); await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
  const r=await p.evaluate(()=>{const el=document.createElement('button');el.className='btn btn-danger';document.querySelector('#cf-page-view,main,body').appendChild(el);const cs=getComputedStyle(el);const o={bg:cs.backgroundColor,color:cs.color};el.remove();return o;});
  const c=ratio(r.color,r.bg);
  console.log(`[${theme}] btn-danger bg=${r.bg} color=${r.color} WCAG=${c.toFixed(2)}`);
  if(c>=4.5) P(`${theme}: danger button >= 4.5 (AA normal text)`); else F(`${theme}: ${c.toFixed(2)} < 4.5`);
  await p.close();
}
await browser.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
