import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const b = await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
for(const theme of ["dark","light"]){
  const p=await b.newPage(); p.setViewportSize({width:1440,height:1200});
  await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
  await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:"domcontentloaded"});
  await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
  await p.evaluate(()=>{const x=[...document.querySelectorAll("button")].find(b=>/load sample|sample data/i.test(b.textContent)); if(x)x.click();});
  await p.waitForTimeout(1500);
  await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Reports"); if(l)l.click();});
  await p.waitForTimeout(1800);
  const data=await p.evaluate(()=>{
    const out={};
    document.querySelectorAll('.hero-secondary .hero-stat').forEach(s=>{
      const label=s.querySelector('.hero-stat-label')?.textContent.trim();
      const v=s.querySelector('.hero-stat-value');
      out[label]={text:v?.textContent.trim(), cls:v?.className.toString(), color:v?getComputedStyle(v).color:null};
    });
    const accent=getComputedStyle(document.documentElement).getPropertyValue('--accent').trim();
    const danger=getComputedStyle(document.documentElement).getPropertyValue('--danger').trim();
    return {out, accent, danger};
  });
  console.log(`\n[${theme}] accent=${data.accent} danger=${data.danger}`);
  for(const [k,v] of Object.entries(data.out)) console.log(`  ${k}: "${v.text}" cls="${v.cls}" color=${v.color}`);
  const sr=data.out['Savings rate'];
  // -17% is negative -> should have neg class AND a reddish (non-white/non-neutral) color
  if(sr){
    if(/neg/.test(sr.cls)) P(`${theme}: savings rate ${sr.text} carries neg class`); else F(`${theme}: savings rate missing neg class (cls=${sr.cls})`);
    // color should NOT be the neutral white/dark; it should differ from a no-class value
    const isNeutral = /255, 255, 255/.test(sr.color) || /28, 28, 30/.test(sr.color);
    if(!isNeutral) P(`${theme}: savings rate renders a semantic (non-neutral) color ${sr.color}`); else F(`${theme}: savings rate still neutral ${sr.color}`);
  } else F(`${theme}: savings rate stat not found`);
  await p.screenshot({path:`e2e/screenshots/reports_hero_secondary_${theme}.png`, clip:await p.evaluate(()=>{const el=document.querySelector('.hero-secondary'); const r=el.getBoundingClientRect(); return {x:Math.max(0,r.left-4),y:Math.max(0,r.top-4),width:r.width+8,height:r.height+8};})});
  await p.close();
}
await b.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
