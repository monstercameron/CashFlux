import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099"; const b = await chromium.launch({headless:true});
const p = await b.newPage(); p.setViewportSize({width:1280,height:1200});
await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
await p.evaluate(()=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:'dark'})));
await p.reload({waitUntil:"domcontentloaded"}); await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
const nav=async t=>{await p.evaluate(t=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")===t);if(l)l.click();},t);await p.waitForTimeout(1400);};
const lum=s=>{const m=(s||'').match(/[\d.]+/g);if(!m)return null;const[r,g,b]=m.map(Number);return 0.299*r+0.587*g+0.114*b;};
for(const scr of ["Dashboard","Accounts","Budgets","Goals"]){
  await nav(scr); await p.waitForTimeout(600);
  await p.emulateMedia({media:"print"}); await p.waitForTimeout(250);
  const r=await p.evaluate(()=>{
    const parse=c=>{const m=(c||'').match(/[\d.]+/g);return m?m.slice(0,3).map(Number):null};
    const L=([r,g,b])=>0.299*r+0.587*g+0.114*b;
    // find any large element with a DARK background still showing in print
    const darkBlocks=[];
    for(const el of document.querySelectorAll('main *, #cf-page-view *')){
      const cs=getComputedStyle(el); const bg=parse(cs.backgroundColor);
      if(!bg||/, 0\)$/.test(cs.backgroundColor)) continue;
      const rc=el.getBoundingClientRect(); if(rc.width<120||rc.height<40) continue;
      if(L(bg)<60) darkBlocks.push({cls:(typeof el.className==='string'?el.className:'').slice(0,28), bg:cs.backgroundColor, w:Math.round(rc.width),h:Math.round(rc.height)});
    }
    const seen=new Set(),u=[];for(const d of darkBlocks){const k=d.cls+d.bg;if(!seen.has(k)){seen.add(k);u.push(d);}}
    return {bodyBg:getComputedStyle(document.body).backgroundColor, darkBlocks:u.slice(0,6)};
  });
  console.log(`[${scr}] bodyBg=${r.bodyBg} | dark blocks in print: ${r.darkBlocks.length}`);
  for(const d of r.darkBlocks) console.log(`   DARK ${d.w}x${d.h} bg=${d.bg} cls="${d.cls}"`);
  await p.screenshot({path:`e2e/screenshots/printscan_${scr}.png`, fullPage:false});
  await p.emulateMedia({media:"screen"});
}
await b.close();
