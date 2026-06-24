import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099"; const browser=await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const lum=s=>{const m=(s||'').match(/[\d.]+/g);if(!m)return null;const[r,g,b]=m.map(Number);return 0.299*r+0.587*g+0.114*b;};
// test in DARK theme (the harder case — must flip to light for print)
const p=await browser.newPage(); p.setViewportSize({width:1280,height:1000});
await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
await p.evaluate(()=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:'dark'})));
await p.reload({waitUntil:"domcontentloaded"}); await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Reports");if(l)l.click();});
await p.waitForTimeout(1500);
// SCREEN media baseline
const screenBg=await p.evaluate(()=>getComputedStyle(document.body).backgroundColor);
console.log("SCREEN body bg (dark):", screenBg);
// emulate print
await p.emulateMedia({media:"print"});
await p.waitForTimeout(300);
const r=await p.evaluate(()=>{
  const cs=getComputedStyle(document.documentElement);
  const rail=document.querySelector('aside.rail,.rail'); const top=document.querySelector('.topbar');
  const card=document.querySelector('.card,.reports-hero,.w');
  return {
    bodyBg:getComputedStyle(document.body).backgroundColor,
    textVar:cs.getPropertyValue('--text').trim(), bgVar:cs.getPropertyValue('--bg').trim(), colorScheme:cs.colorScheme,
    railHidden: rail?getComputedStyle(rail).display==='none':'no-rail',
    topHidden: top?getComputedStyle(top).display==='none':'no-top',
    cardBreak: card?getComputedStyle(card).breakInside:'no-card',
  };
});
console.log("PRINT:", JSON.stringify(r));
const bl=lum(r.bodyBg);
if(bl!==null && bl>240) P(`print body bg is white (lum ${Math.round(bl)}) even from dark theme`); else F(`print body bg not white: ${r.bodyBg}`);
if(r.textVar==='#111') P("print --text forced dark (#111) over the inline dark var"); else F(`--text not forced: ${r.textVar}`);
if(r.railHidden===true) P("nav rail hidden in print"); else F(`rail not hidden: ${r.railHidden}`);
if(r.topHidden===true) P("topbar hidden in print"); else F(`topbar not hidden: ${r.topHidden}`);
if(r.cardBreak==='avoid') P("cards avoid page-break-inside"); else F(`card break-inside: ${r.cardBreak}`);

await p.screenshot({path:'e2e/screenshots/print_reports.png', fullPage:false});
await p.close(); await browser.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
