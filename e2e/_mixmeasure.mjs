import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const lin=c=>{c/=255;return c<=0.03928?c/12.92:Math.pow((c+0.055)/1.055,2.4)};
const L=([r,g,b])=>0.2126*lin(r)+0.7152*lin(g)+0.0722*lin(b);
const parse=s=>s.match(/[\d.]+/g).slice(0,3).map(Number);
const ratio=(fg,bg)=>{const a=L(parse(fg)),b=L(parse(bg));const hi=Math.max(a,b),lo=Math.min(a,b);return (hi+0.05)/(lo+0.05)};
const b = await chromium.launch({headless:true}); const p=await b.newPage();
await p.goto("http://127.0.0.1:8099/",{waitUntil:"domcontentloaded",timeout:20000}); await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
const r=await p.evaluate(()=>{
  const out={};
  for(const pct of [85,88,90,92,95]){
    const el=document.createElement('div'); el.style.background=`color-mix(in srgb, var(--accent) ${pct}%, #000)`; document.body.appendChild(el);
    out[pct]=getComputedStyle(el).backgroundColor; el.remove();
  }
  return out;
});
for(const [pct,bg] of Object.entries(r)) console.log(`accent ${pct}% + black → ${bg} | white WCAG=${ratio('rgb(255,255,255)',bg).toFixed(2)}`);
await b.close();
