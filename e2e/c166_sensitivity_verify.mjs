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
const subCount=p=>p.evaluate(()=>document.querySelectorAll('[data-testid^="sub-row"], .subscription-row, [data-testid="subscription-row"]').length);
try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000});
  await p.waitForTimeout(4500);
  await p.click('a[href="/subscriptions"]'); await p.waitForTimeout(1500);
  // Open detect-prefs panel
  await p.evaluate(()=>{const b=document.querySelector('[data-testid="subs-detect-prefs-toggle"]'); if(b)b.click();});
  await p.waitForTimeout(500);
  const sel=await p.$('[data-testid="subs-detect-min-occur"]');
  if(sel) pass("detection-sensitivity select present in detect-prefs panel"); else { fail("sensitivity select missing"); }
  // count rows at default (2)
  const countAt=async()=>p.evaluate(()=>{
    // count rows that look like subscription entries: rows with a "How to cancel" link or cancel button
    const rows=[...document.querySelectorAll('.row')].filter(r=>/cancel/i.test(r.textContent||""));
    return rows.length;
  });
  const c2=await countAt();
  console.log("  rows at sensitivity=2: "+c2);
  // set to 4 (strict) -> should reduce or equal
  if(sel){
    await p.selectOption('[data-testid="subs-detect-min-occur"]','4');
    await p.waitForTimeout(900);
    const c4=await countAt();
    console.log("  rows at sensitivity=4: "+c4);
    if(c4<=c2) pass(`stricter sensitivity did not increase rows (${c2} -> ${c4})`); else fail(`rows increased ${c2}->${c4}`);
    // verify persistence: the select keeps value 4 after reload
    await p.reload({waitUntil:"networkidle"}); await p.waitForTimeout(4500);
    await p.click('a[href="/subscriptions"]'); await p.waitForTimeout(1200);
    await p.evaluate(()=>{const b=document.querySelector('[data-testid="subs-detect-prefs-toggle"]'); if(b)b.click();});
    await p.waitForTimeout(500);
    const val=await p.evaluate(()=>document.querySelector('[data-testid="subs-detect-min-occur"]')?.value);
    console.log("  persisted value after reload: "+val);
    if(val==="4") pass("sensitivity choice persisted across reload"); else fail("did not persist: "+val);
    // reset to 2 for cleanliness
    await p.selectOption('[data-testid="subs-detect-min-occur"]','2'); await p.waitForTimeout(400);
  }
  await p.screenshot({path:"e2e/screenshots/c166_sensitivity.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
