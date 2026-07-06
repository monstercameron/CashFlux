import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099"; const browser=await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const p=await browser.newPage(); p.setViewportSize({width:1280,height:1200});
await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
await p.goto(BASE+"/appearance",{waitUntil:"domcontentloaded"}); await p.waitForTimeout(1500);
const r=await p.evaluate(()=>{
  const body=document.body.textContent;
  return {
    hasBorderLeak: /border-top-width|border-top-style|border-color #/.test(body),
    hasRuleBraces: /\{\[\{border/.test(body),
    // the Hr divider exists and has a top border applied (not text)
    hr:(()=>{const h=[...document.querySelectorAll('hr')].pop(); if(!h)return null; const cs=getComputedStyle(h); return {borderTopWidth:cs.borderTopWidth, borderTopStyle:cs.borderTopStyle};})(),
  };
});
console.log("appearance:", JSON.stringify(r));
if(!r.hasBorderLeak && !r.hasRuleBraces) P("no raw CSS-rule text leaked on /appearance"); else F("CSS-rule text still leaking: borderLeak="+r.hasBorderLeak+" braces="+r.hasRuleBraces);
if(r.hr && parseFloat(r.hr.borderTopWidth)>=1 && r.hr.borderTopStyle==='solid') P(`divider <hr> renders its top border (${r.hr.borderTopWidth} ${r.hr.borderTopStyle}) — applied as style, not text`);
else F("hr border not applied: "+JSON.stringify(r.hr));
await p.screenshot({path:'e2e/screenshots/appearance_fixed.png', clip:{x:236,y:230,width:1044,height:200}}).catch(async()=>{await p.screenshot({path:'e2e/screenshots/appearance_fixed.png'});});
await p.close(); await browser.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
