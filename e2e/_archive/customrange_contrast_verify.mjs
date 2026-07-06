import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099"; const browser=await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const lin=c=>{c/=255;return c<=0.03928?c/12.92:Math.pow((c+0.055)/1.055,2.4)};
const L=([r,g,b])=>0.2126*lin(r)+0.7152*lin(g)+0.0722*lin(b);
const parse=s=>s.match(/[\d.]+/g).slice(0,3).map(Number);
const ratio=(fg,bg)=>{const a=L(parse(fg)),b=L(parse(bg));const hi=Math.max(a,b),lo=Math.min(a,b);return (hi+0.05)/(lo+0.05)};
const run=async theme=>{
  const p=await browser.newPage(); p.setViewportSize({width:1280,height:1000});
  await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
  await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:"domcontentloaded"}); await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
  await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Planning");if(l)l.click();});
  await p.waitForTimeout(1500);
  const m=await p.evaluate(()=>{
    const el=[...document.querySelectorAll('.reso-control button, button')].find(b=>b.textContent.trim()==="Custom range");
    if(!el)return null; const cs=getComputedStyle(el);
    let e=el,bg='rgb(247,246,243)'; while(e){const c=getComputedStyle(e).backgroundColor; if(c&&!/, 0\)$/.test(c)&&c!=='transparent'){bg=c;break;} e=e.parentElement;}
    return {color:cs.color,bg};
  });
  if(!m){F(theme+": Custom range not found"); await p.close(); return;}
  const r=ratio(m.color,m.bg);
  console.log(`[${theme}] Custom range color=${m.color} bg=${m.bg} WCAG=${r.toFixed(2)}`);
  if(r>=4.5) P(`${theme}: ratio ${r.toFixed(2)} >= 4.5 (AA)`); else if(r>=3.0) P(`${theme}: ratio ${r.toFixed(2)} >= 3.0 (AA large)`); else F(`${theme}: ratio ${r.toFixed(2)} < 3.0`);
  await p.screenshot({path:`e2e/screenshots/customrange_${theme}.png`}); await p.close();
};
await run("light"); await run("dark"); await browser.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
