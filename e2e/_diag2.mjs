import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const b = await chromium.launch({headless:true}); const p = await b.newPage(); p.setViewportSize({width:1440,height:1100});
await p.goto("http://127.0.0.1:8099/",{waitUntil:"domcontentloaded",timeout:20000});
await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
await p.evaluate(()=>{const x=[...document.querySelectorAll("button")].find(b=>/load sample|sample data/i.test(b.textContent)); if(x)x.click();});
await p.waitForTimeout(1500);
const m=await p.evaluate(()=>{
  const out=[];
  for(const sheet of document.styleSheets){
    let rules; try{rules=sheet.cssRules;}catch(e){continue;}
    for(const r of rules){
      if(r.selectorText && /\.w\b|\.bento \.w|\.card\b/.test(r.selectorText) && r.style && r.style.boxShadow){
        out.push({sel:r.selectorText.slice(0,50), bs:r.style.boxShadow.slice(0,40)});
      }
    }
  }
  // also check inline animation pinning: is there an active animation on .w?
  const w=document.querySelector('.bento .w');
  const anim=w?getComputedStyle(w).animationName:null;
  return {rules:out, animName:anim, wComputed:w?getComputedStyle(w).boxShadow.slice(0,30):null};
});
console.log(JSON.stringify(m,null,1));
await b.close();
