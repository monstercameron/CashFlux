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
  const w=document.querySelector('.bento .w');
  const inline=w?w.getAttribute('style'):null;
  // does .card on another mechanism work? check a .card element if any on dashboard
  const card=document.querySelector('.card');
  return {inlineStyle:inline?inline.slice(0,120):null, inlineBoxShadow:w?w.style.boxShadow:null, cardShadow:card?getComputedStyle(card).boxShadow.slice(0,40):'no .card', wTag:w?w.tagName:null, wClass:w?(typeof w.className==='string'?w.className:''):null};
});
console.log(JSON.stringify(m,null,1));
await b.close();
