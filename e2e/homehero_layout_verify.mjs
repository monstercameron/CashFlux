import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099"; const browser=await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const run=async theme=>{
  const p=await browser.newPage(); p.setViewportSize({width:1280,height:1000});
  await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
  await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:"domcontentloaded"}); await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
  await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Dashboard");if(l)l.click();});
  await p.waitForTimeout(1500);
  const m=await p.evaluate(()=>{
    const stats=document.querySelector('.home-hero-stats'); if(!stats)return null;
    const csStats=getComputedStyle(stats);
    const blocks=[...stats.querySelectorAll('.home-hero-stat')];
    // are the 4 stat blocks laid out in a ROW (different x) and each label ABOVE value (different y)?
    const xs=blocks.map(b=>Math.round(b.getBoundingClientRect().left));
    const inRow = new Set(xs).size>=Math.min(blocks.length,2); // distinct left positions => row
    const first=blocks[0]; let labelAboveValue=false, gapOk=false;
    if(first){const lab=first.querySelector('.home-hero-stat-label'),val=first.querySelector('.home-hero-stat-value');
      if(lab&&val){const lr=lab.getBoundingClientRect(),vr=val.getBoundingClientRect(); labelAboveValue=vr.top>=lr.bottom-2; gapOk=vr.top>lr.top;}}
    return {display:csStats.display, gap:csStats.gap, blockCount:blocks.length, xs, inRow, labelAboveValue};
  });
  if(!m){F(theme+": no .home-hero-stats"); await p.close(); return;}
  console.log(`[${theme}]`, JSON.stringify(m));
  if(m.display==="flex" && m.blockCount===4) P(`${theme}: stats row is flex with 4 blocks`); else F(`${theme}: stats layout wrong: ${JSON.stringify(m)}`);
  if(m.inRow) P(`${theme}: stat blocks laid out in a row (distinct x: ${m.xs.join(",")})`); else F(`${theme}: stat blocks not in a row (${m.xs.join(",")})`);
  if(m.labelAboveValue) P(`${theme}: label sits above value (no jam)`); else F(`${theme}: label/value not stacked`);
  await p.screenshot({path:`e2e/screenshots/homehero_fixed_${theme}.png`, clip:{x:236,y:60,width:780,height:360}}).catch(async()=>{await p.screenshot({path:`e2e/screenshots/homehero_fixed_${theme}.png`});});
  await p.close();
};
await run("dark"); await run("light"); await browser.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
