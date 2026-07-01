import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed=0; const fail=m=>{console.error("FAIL: "+m);failed++;process.exitCode=1;}; const pass=m=>console.log("PASS: "+m);
const errs=[];
const flaggedPresent=p=>p.evaluate(()=>[...document.querySelectorAll('.card-title,h2,h3')].some(e=>/flagged activity/i.test(e.textContent||"")));
try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(4500);
  // baseline: no flagged section on /insights
  await p.click('a[href="/insights"]'); await p.waitForTimeout(1500);
  const before=await flaggedPresent(p);
  console.log("  flagged section before: "+before);
  // create duplicates: go to /transactions, duplicate a row twice (T2 needs repeats)
  await p.click('a[href="/transactions"]'); await p.waitForTimeout(1400);
  const dup=await p.evaluate(()=>{
    const btns=[...document.querySelectorAll('button')].filter(b=>/copy this transaction/i.test(b.getAttribute('aria-label')||b.getAttribute('title')||""));
    if(btns.length){ btns[0].click(); return true; } return false;
  });
  await p.waitForTimeout(700);
  await p.evaluate(()=>{const b=[...document.querySelectorAll('button')].filter(x=>/copy this transaction/i.test(x.getAttribute('aria-label')||x.getAttribute('title')||"")); if(b[0])b[0].click();});
  await p.waitForTimeout(900);
  console.log("  duplicated a transaction: "+dup);
  // back to /insights
  await p.click('a[href="/insights"]'); await p.waitForTimeout(1700);
  const after=await flaggedPresent(p);
  const detail=await p.evaluate(()=>{
    const t=[...document.querySelectorAll('.card-title,h2,h3')].find(e=>/flagged activity/i.test(e.textContent||""));
    if(!t) return null; const card=t.closest('section,article,.card,.entity-list-section')||t.parentElement;
    return {title:t.textContent.trim(), rows:card.querySelectorAll('.insight-row').length, sample:(card.textContent||"").slice(0,120)};
  });
  console.log("  flagged after dup: "+after+"  detail="+JSON.stringify(detail));
  if(after) pass("C252: 'Flagged activity' anomaly section surfaces on /insights (ungated) once a duplicate exists"); else fail("flagged section still absent after creating duplicate");
  await p.screenshot({path:"e2e/screenshots/c252_anomalies.png", fullPage:true});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
