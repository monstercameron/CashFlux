import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099"; const browser=await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const run=async(label,w,minTarget)=>{
  const p=await browser.newPage(); p.setViewportSize({width:w,height:900});
  await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
  await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
  await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Transactions");if(l)l.click();});
  await p.waitForTimeout(1300);
  const m=await p.evaluate(()=>{
    const t=document.querySelector('.clr-toggle'); if(!t)return null;
    const rc=t.getBoundingClientRect(); const row=t.closest('tr,li,[role="row"]'); const rr=row?row.getBoundingClientRect():null;
    return {w:Math.round(rc.width),h:Math.round(rc.height),rowH:rr?Math.round(rr.height):null, glyph:t.textContent.trim()};
  });
  if(!m){F(label+": no toggle"); await p.close(); return null;}
  const min=Math.min(m.w,m.h);
  console.log(`[${label} @${w}px] toggle=${m.w}x${m.h} (min ${min}) rowH=${m.rowH} glyph="${m.glyph}"`);
  if(min>=minTarget) P(`${label}: min dimension ${min} >= ${minTarget}`); else F(`${label}: min ${min} < ${minTarget}`);
  await p.screenshot({path:`e2e/screenshots/clrtoggle_${label}.png`}); await p.close(); return m;
};
const d=await run("desktop",1280,24);
const mob=await run("mobile",390,44);
// row height sanity: desktop row should stay reasonable (was 55)
if(d && d.rowH && d.rowH<=64) P(`desktop row height ${d.rowH}px stayed compact (no blow-up)`); else if(d) F(`desktop row height ${d.rowH}px grew too much`);
await browser.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
