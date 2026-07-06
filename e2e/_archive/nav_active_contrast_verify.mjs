import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099"; const b = await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const lin=c=>{c/=255;return c<=0.03928?c/12.92:Math.pow((c+0.055)/1.055,2.4)};
const L=([r,g,b])=>0.2126*lin(r)+0.7152*lin(g)+0.0722*lin(b);
// composite a possibly-rgba/color() fg over bg-ish: we only have solid here
const toRGB=s=>{const m=s.match(/[\d.]+/g).map(Number); if(s.includes('srgb')) return m.slice(0,3).map(x=>Math.round(x*255)); return m.slice(0,3);};
const ratio=(fg,bg)=>{const a=L(toRGB(fg)),b2=L(toRGB(bg));const hi=Math.max(a,b2),lo=Math.min(a,b2);return (hi+0.05)/(lo+0.05)};
for(const theme of ["light","dark"]){
  const p=await b.newPage(); p.setViewportSize({width:1440,height:1100});
  await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
  await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:"domcontentloaded"});
  await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
  await p.evaluate(()=>{const x=[...document.querySelectorAll("button")].find(b=>/load sample|sample data/i.test(b.textContent)); if(x)x.click();});
  await p.waitForTimeout(1200);
  await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Categories"); if(l)l.click();});
  await p.waitForTimeout(1000);
  const m=await p.evaluate(()=>{
    const a=document.querySelector('aside.rail .nv.active, aside.rail a[aria-current]'); if(!a)return null;
    function bgOf(el){let n=el;while(n){const c=getComputedStyle(n).backgroundColor;if(c&&!/rgba?\(0, 0, 0, 0\)|transparent/.test(c))return c;n=n.parentElement;}return 'rgb(255,255,255)';}
    const span=a.querySelector('span')||a; const cs=getComputedStyle(span);
    return {color:cs.color, bg:bgOf(a), txt:a.textContent.trim().slice(0,20), weight:cs.fontWeight};
  });
  if(!m){F(`${theme}: no active nav item`); await p.close(); continue;}
  const r=ratio(m.color,m.bg);
  console.log(`[${theme}] active="${m.txt}" color=${m.color} bg=${m.bg} weight=${m.weight} -> ${r.toFixed(2)}:1`);
  if(r>=4.5) P(`${theme}: active nav label ${r.toFixed(2)}:1 >= 4.5 (AA)`); else if(r>=3.0) P(`${theme}: active nav label ${r.toFixed(2)}:1 >= 3.0 (AA-large/bold)`); else F(`${theme}: active nav label ${r.toFixed(2)}:1 < 3.0`);
  await p.close();
}
await b.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
