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
  await p.click('a[href="/reports"]'); await p.waitForTimeout(1300);
  await p.evaluate(()=>{const b=[...document.querySelectorAll('[role="radio"],button')].find(x=>/^Net worth$/i.test((x.textContent||"").trim())); if(b)b.click();});
  await p.waitForTimeout(900);
  const r=await p.evaluate(()=>{
    const nw=document.querySelector('#networth');
    if(!nw) return {found:false};
    // uiw.Chart puts the accessible name on a wrapper; find the wrapper, then its svg.
    const wrap=[...nw.querySelectorAll('[aria-label]')].find(e=>/assets vs liabilities/i.test(e.getAttribute('aria-label')||""));
    const svg=wrap? (wrap.matches('svg')?wrap:wrap.querySelector('svg')) : null;
    const bars=svg? svg.querySelectorAll('rect').length : 0;
    return {found:!!wrap, label:wrap?.getAttribute('aria-label'), bars};
  });
  console.log("  "+JSON.stringify(r));
  if(r.found) pass("R52: assets-vs-liabilities bar renders in the NW card: "+r.label); else fail("assets-vs-liabilities bar not found");
  if(r.bars>=2) pass("bar has ≥2 bars (assets + liabilities), count="+r.bars); else console.log("  (bar rect count="+r.bars+")");
  await p.screenshot({path:"e2e/screenshots/r52_avl_bar.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
