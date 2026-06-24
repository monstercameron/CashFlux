import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
// proper WCAG relative luminance + contrast ratio
const lin=c=>{c/=255; return c<=0.03928? c/12.92 : Math.pow((c+0.055)/1.055,2.4);};
const L=([r,g,b])=>0.2126*lin(r)+0.7152*lin(g)+0.0722*lin(b);
const parse=s=>s.match(/[\d.]+/g).slice(0,3).map(Number);
const ratio=(fg,bg)=>{const l1=L(parse(fg)),l2=L(parse(bg));const hi=Math.max(l1,l2),lo=Math.min(l1,l2);return (hi+0.05)/(lo+0.05);};
const run=async theme=>{
  const page=await browser.newPage(); page.setViewportSize({width:1280,height:1000});
  await page.goto(BASE+"/",{waitUntil:"domcontentloaded"});
  await page.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await page.reload({waitUntil:"domcontentloaded"});
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:60000});
  await page.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Allocate");if(l)l.click();});
  await page.waitForTimeout(1600);
  const m=await page.evaluate(()=>{const el=[...document.querySelectorAll('.rank-badge')][0]; if(!el)return null; const cs=getComputedStyle(el); return {color:cs.color,bg:cs.backgroundColor};});
  if(!m){F(theme+": no rank-badge"); await page.close(); return;}
  const r=ratio(m.color,m.bg);
  console.log(`[${theme}] badge color=${m.color} bg=${m.bg} WCAG_ratio=${r.toFixed(2)}`);
  if(r>=3.0) P(`${theme}: rank-badge ratio ${r.toFixed(2)} >= 3.0 (AA large/bold)`);
  else F(`${theme}: rank-badge ratio ${r.toFixed(2)} < 3.0`);
  await page.screenshot({path:`e2e/screenshots/rankbadge_${theme}.png`});
  await page.close();
};
await run("light"); await run("dark");
await browser.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`);
process.exit(fail>0?1:0);
