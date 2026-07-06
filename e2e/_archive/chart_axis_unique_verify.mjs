import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const b = await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const p=await b.newPage(); p.setViewportSize({width:1440,height:1200});
await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
await p.evaluate(()=>{const x=[...document.querySelectorAll("button")].find(b=>/load sample|sample data/i.test(b.textContent)); if(x)x.click();});
await p.waitForTimeout(1500);
const readCharts=()=>p.evaluate(()=>{
  const svgs=[...document.querySelectorAll('svg')].filter(s=>s.querySelector('.y-axis .tick'));
  return svgs.map(svg=>{
    let node=svg,head=null; for(let k=0;k<6&&node;k++){node=node.parentElement; if(node){const h=node.querySelector('h2,h3,.card-title'); if(h){head=h.textContent.trim().slice(0,32);break;}}}
    const ticks=[...svg.querySelectorAll('.y-axis .tick')].map(t=>({v:t.__data__, txt:(t.querySelector('text')||{}).textContent}));
    return {head, ticks};
  });
});
for(const [screen,nav] of [["Planning","Planning"],["Dashboard","Dashboard"]]){
  await p.evaluate((t)=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")===t); if(l)l.click();},nav);
  await p.waitForTimeout(2000);
  const charts=await readCharts();
  for(const c of charts){
    const labels=c.ticks.map(t=>t.txt);
    const uniq=new Set(labels);
    console.log(`[${screen}] "${c.head}" ticks=[${labels.join(", ")}]`);
    if(labels.length===0){ continue; }
    if(uniq.size===labels.length) P(`${screen}/${c.head}: all ${labels.length} y-labels distinct`);
    else F(`${screen}/${c.head}: DUPLICATE y-labels [${labels.join(", ")}]`);
  }
  await p.screenshot({path:`e2e/screenshots/axis_unique_${screen.toLowerCase()}.png`});
}
await b.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
