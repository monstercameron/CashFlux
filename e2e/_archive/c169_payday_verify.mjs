import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed=0; const fail=m=>{console.error("FAIL: "+m);failed++;process.exitCode=1;}; const pass=m=>console.log("PASS: "+m);
const errs=[];
const paydayStat=p=>p.evaluate(()=>{
  const l=[...document.querySelectorAll('.stat-label')].find(e=>/balance on payday/i.test(e.textContent||""));
  if(!l) return null;
  const s=l.closest('.stat')||l.parentElement; return s?s.textContent.trim():l.textContent.trim();
});
try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000});
  await p.waitForTimeout(4500);
  // 1) No anchor yet → payday stat absent on Planning.
  await p.click('a[href="/planning"]'); await p.waitForTimeout(1400);
  const before=await paydayStat(p);
  if(!before) pass("payday stat absent when no pay-cycle anchor is set"); else fail("unexpected payday stat before anchor: "+before);
  // 2) Set the anchor via Settings (day-of-month = 1).
  await p.evaluate(()=>{const b=document.querySelector('button.hh'); if(b)b.click();});
  await p.waitForTimeout(800);
  const sel='input[type="date"][aria-label="Pay cycle anchor"]';
  if(await p.$(sel)){ await p.fill(sel,"2026-06-01"); pass("set pay-cycle anchor via Settings"); }
  else fail("pay-cycle anchor input not found in Settings");
  await p.waitForTimeout(1600); // let async pref persistence flush
  await p.keyboard.press("Escape"); await p.waitForTimeout(600);
  // 3) Route away then back so the Planning component re-renders with the new pref
  // (clicking the current route is a no-op in the SPA router).
  await p.click('a[href="/"]'); await p.waitForTimeout(800);
  await p.click('a[href="/planning"]'); await p.waitForTimeout(1600);
  const after=await paydayStat(p);
  console.log("  payday stat: "+JSON.stringify(after));
  if(after && /balance on payday \(/i.test(after) && /\$/.test(after)) pass("payday-anchored balance stat renders (date + amount): "+after);
  else fail("payday stat missing/incomplete: "+after);
  await p.screenshot({path:"e2e/screenshots/c169_payday.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,5).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
