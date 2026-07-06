import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const b = await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const p=await b.newPage(); p.setViewportSize({width:1440,height:1100});
await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
await p.evaluate(()=>{const x=[...document.querySelectorAll("button")].find(b=>/load sample|sample data/i.test(b.textContent)); if(x)x.click();});
await p.waitForTimeout(1500);
await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Categories"); if(l)l.click();});
await p.waitForTimeout(1500);
const m=await p.evaluate(()=>{
  const map=document.querySelector('.cat-map'); if(!map)return{none:true};
  const cs=getComputedStyle(map);
  const groups=[...map.querySelectorAll('.cat-map-group')];
  const tops=groups.map(g=>g.getBoundingClientRect().top);
  const lefts=groups.map(g=>Math.round(g.getBoundingClientRect().left));
  const distinctRows=new Set(tops.map(t=>Math.round(t/10))).size;
  const distinctCols=new Set(lefts).size;
  const mermaid=!!document.querySelector('.cf-mermaid, .mermaid svg');
  // how wide does the map content span vs viewport?
  const maxRight=Math.max(...groups.map(g=>g.getBoundingClientRect().right));
  const mapW=map.getBoundingClientRect().width;
  const sub=map.querySelector('.cat-map-sub');
  return {display:cs.display, flexWrap:cs.flexWrap, nGroups:groups.length, distinctRows, distinctCols, maxRight:Math.round(maxRight), mapW:Math.round(mapW), mermaid, subColor: sub?getComputedStyle(sub).color:null, groupBg: groups[0]?getComputedStyle(groups[0]).backgroundColor:null};
});
console.log(JSON.stringify(m));
if(m.none){F("no .cat-map found");}
else{
  if(!m.mermaid) P("mermaid flowchart removed"); else F("mermaid still present");
  if(m.display==='flex'&&m.flexWrap==='wrap') P(`map is flex-wrap (${m.nGroups} groups)`); else F(`map not flex-wrap (${m.display}/${m.flexWrap})`);
  if(m.distinctCols>1) P(`groups use horizontal space: ${m.distinctCols} distinct columns across ${m.distinctRows} rows (was 1 column)`); else F(`still a single column (cols=${m.distinctCols})`);
  if(m.maxRight > m.mapW*0.5) P(`content fills width: rightmost group at ${m.maxRight}px of ${m.mapW}px container`); else F(`content narrow: ${m.maxRight}/${m.mapW}`);
}
await p.screenshot({path:'e2e/screenshots/category_map_grid.png'});
await b.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
