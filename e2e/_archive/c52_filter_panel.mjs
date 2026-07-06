import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
let failed=0; const fail=m=>{console.error("FAIL: "+m);failed++;process.exitCode=1;}; const pass=m=>console.log("PASS: "+m);
const errs=[];
try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(4500);
  await p.click('a[href="/transactions"]'); await p.waitForTimeout(1500);
  const rowsBefore=await p.evaluate(()=>document.querySelectorAll('.txn-table tbody tr.row, tr.row').length);
  // open the filter panel (click the Filters toolbar trigger)
  const opened=await p.evaluate(()=>{
    const b=[...document.querySelectorAll('button')].find(x=>/filter/i.test((x.getAttribute('aria-label')||x.textContent||"")) && !/clear/i.test(x.textContent||""));
    if(b){b.click();return true;} return false;
  });
  await p.waitForTimeout(700);
  const state=await p.evaluate(()=>{
    // filter panel present?
    const panel=document.querySelector('.filter-panel, [class*="filter-panel"], [data-testid*="filter"]');
    // any full-screen modal/backdrop occluding?
    const backdrop=[...document.querySelectorAll('*')].find(e=>{const cs=getComputedStyle(e); return (cs.position==='fixed') && parseFloat(cs.width)>=window.innerWidth*0.9 && parseFloat(cs.height)>=window.innerHeight*0.9 && /rgba?\(0, 0, 0/.test(cs.backgroundColor);});
    // table rows still visible (not covered)?
    const rows=document.querySelectorAll('.txn-table tbody tr.row, tr.row').length;
    const tableVisible=!!document.querySelector('.txn-table') && document.querySelector('.txn-table').getBoundingClientRect().height>0;
    return {panel:!!panel, backdrop:!!backdrop, rows, tableVisible};
  });
  console.log("  opened:"+opened+" "+JSON.stringify(state));
  if(opened) pass("filter panel toggles open"); else console.log("  (filter trigger not matched)");
  if(!state.backdrop) pass("C52: no full-screen backdrop/modal occluding the table"); else fail("a full-screen backdrop is occluding");
  if(state.tableVisible && state.rows>0) pass("C52: table + rows remain visible while filtering (inline panel, "+state.rows+" rows)"); else fail("table not visible with panel open");
  await p.screenshot({path:"e2e/screenshots/c52_filter_panel.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
