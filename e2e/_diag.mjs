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
  const inBento = w?!!w.closest('.bento'):null;
  // check all .w and report first non-none shadow
  const ws=[...document.querySelectorAll('.w')];
  const shadows=ws.slice(0,4).map(e=>getComputedStyle(e).boxShadow.slice(0,30));
  return {wExists:!!w, inBento, parentClass: w?(typeof w.parentElement.className==='string'?w.parentElement.className:''):null, firstShadows:shadows};
});
console.log(JSON.stringify(m));
await b.close();
