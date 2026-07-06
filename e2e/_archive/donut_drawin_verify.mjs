import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099"; const b = await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
// run() navigates to Reports immediately and watches the donut for a d3 transition + final arc count
const run=(p)=>p.evaluate(()=>new Promise(res=>{
  let sawTransition=false, frames=0;
  const iv=setInterval(()=>{
    frames++;
    // donut arcs: paths inside a .cf-chart whose 'd' starts with M and contains 'A' (arc)
    const arcs=[...document.querySelectorAll('.cf-chart path')].filter(p=>{const d=p.getAttribute('d')||''; return /A/.test(d)&&/^M/.test(d)&&p.parentElement&&p.parentElement.querySelector('text');});
    for(const a of arcs) if(a.__transition) sawTransition=true;
    if(frames>120){clearInterval(iv); const finalArcs=[...document.querySelectorAll('.cf-chart path')].filter(p=>{const d=p.getAttribute('d')||''; return /A/.test(d);}).length; res({sawTransition, finalArcs});}
  },16);
  const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Reports"); if(l)l.click();
}));
for(const mode of ["full","off"]){
  const p=await b.newPage(); p.setViewportSize({width:1440,height:1200});
  await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
  await p.evaluate(m=>localStorage.setItem('cashflux:prefs',JSON.stringify({wonder:m})),mode);
  await p.reload({waitUntil:"domcontentloaded"});
  await p.evaluate(m=>document.documentElement.setAttribute('data-wonder',m),mode);
  await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
  await p.evaluate(()=>{const x=[...document.querySelectorAll("button")].find(b=>/load sample|sample data/i.test(b.textContent)); if(x)x.click();});
  await p.waitForTimeout(1200);
  await p.evaluate(m=>document.documentElement.setAttribute('data-wonder',m),mode);
  const r=await run(p);
  console.log(`[${mode}] sawTransition=${r.sawTransition} finalArcs=${r.finalArcs}`);
  if(r.finalArcs>=8) P(`${mode}: donut renders (${r.finalArcs} arcs final)`); else F(`${mode}: donut arcs missing (${r.finalArcs})`);
  if(mode==="full"){ r.sawTransition?P("full: donut animates (d3 transition active)"):F("full: donut did NOT animate"); }
  else { !r.sawTransition?P("off: donut static, no transition — WONDER off honored"):F("off: donut animated despite WONDER off"); }
  await p.close();
}
await b.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
