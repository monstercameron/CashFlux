import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const lin=c=>{c/=255;return c<=0.03928?c/12.92:Math.pow((c+0.055)/1.055,2.4)};
const L=([r,g,b])=>0.2126*lin(r)+0.7152*lin(g)+0.0722*lin(b);
const toRGB=s=>{const m=s.match(/[\d.]+/g).map(Number); if(s.includes('srgb'))return m.slice(0,3).map(x=>Math.round(x*255)); return m.slice(0,3);};
const ratio=(f,b)=>{const a=L(toRGB(f)),c=L(toRGB(b));const hi=Math.max(a,c),lo=Math.min(a,c);return (hi+0.05)/(lo+0.05)};
const b = await chromium.launch({headless:true});
for(const theme of ["dark","light"]){
  const p=await b.newPage(); p.setViewportSize({width:1440,height:1100});
  await p.goto("http://127.0.0.1:8099/",{waitUntil:"domcontentloaded",timeout:20000});
  await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:"domcontentloaded"});
  await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
  await p.evaluate(()=>{const x=[...document.querySelectorAll("button")].find(b=>/load sample|sample data/i.test(b.textContent)); if(x)x.click();});
  await p.waitForTimeout(1300);
  await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Allocate"); if(l)l.click();});
  await p.waitForTimeout(1200);
  const m=await p.evaluate(()=>{const e=document.querySelector('.rank-badge'); if(!e)return null; const cs=getComputedStyle(e); return {color:cs.color,bg:cs.backgroundColor,txt:e.textContent.trim()};});
  if(!m){console.log(`[${theme}] no rank-badge`); await p.close(); continue;}
  console.log(`[${theme}] rank-badge "${m.txt}" color=${m.color} bg=${m.bg} -> ${ratio(m.color,m.bg).toFixed(2)}:1`);
  await p.close();
}
await b.close();
