// Verifies D3 chart entrance animation honors the WONDER toggle (web/chart.js):
//   data-wonder="full" -> charts animate (d3 .transition() active on bars)
//   data-wonder="off"  -> charts render fully static (no transition)
// Detection: d3 sets node.__transition on elements with an active transition, so we poll
// for its presence/absence over the first ~2s of the chart's first draw. Reliable in headless
// (rAF height-sampling missed the 450ms in-flight grow under WONDER's frame load).
import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const b = await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const run=(p)=>p.evaluate(()=>new Promise(res=>{
  let sawTransition=false, maxH=0, frames=0;
  const iv=setInterval(()=>{
    frames++;
    for(const r of document.querySelectorAll('.cf-chart rect, svg.cf-chart rect')){
      if(r.__transition) sawTransition=true;
      const h=parseFloat(r.getAttribute('height'))||0; if(h>maxH)maxH=h;
    }
    if(frames>120){clearInterval(iv); res({sawTransition,maxH});}
  },16);
  const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Reports"); if(l)l.click();
}));
for(const mode of ["full","off"]){
  const p=await b.newPage(); p.setViewportSize({width:1440,height:1100});
  await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
  await p.evaluate(m=>localStorage.setItem('cashflux:prefs',JSON.stringify({wonder:m})),mode);
  await p.reload({waitUntil:"domcontentloaded"});
  await p.evaluate(m=>document.documentElement.setAttribute('data-wonder',m),mode);
  await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
  await p.evaluate(()=>{const x=[...document.querySelectorAll("button")].find(b=>/load sample|sample data/i.test(b.textContent)); if(x)x.click();});
  await p.waitForTimeout(1200);
  await p.evaluate(m=>document.documentElement.setAttribute('data-wonder',m),mode);
  const won=await p.evaluate(()=>parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--wonder-on')));
  const r=await run(p);
  console.log(`[${mode}] --wonder-on=${won} sawTransition=${r.sawTransition} maxBarH=${r.maxH.toFixed(0)}`);
  if(r.maxH<=0){F(`${mode}: no bars rendered`); await p.close(); continue;}
  if(mode==="full"){ r.sawTransition?P("full: charts animate (d3 transition active)"):F("full: charts did NOT animate"); }
  else { !r.sawTransition?P("off: charts static, no transition — WONDER off honored"):F("off: charts animated despite WONDER off"); }
  await p.close();
}
await b.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
