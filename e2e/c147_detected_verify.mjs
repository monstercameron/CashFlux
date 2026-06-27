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
try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000});
  await p.waitForTimeout(4500);
  await p.click('a[href="/planning"]'); await p.waitForTimeout(1600);
  const before=await p.evaluate(()=>{
    const card=document.querySelector('[data-testid="detected-recurring"]');
    const addBtns=[...document.querySelectorAll('[data-testid="detected-add"]')];
    const recRows=document.querySelectorAll('#recurring .rows .row').length;
    return { hasCard: !!card, title: card?card.querySelector(".row-desc")?.textContent.trim():null,
             firstName: addBtns[0]?.closest(".row")?.querySelector(".row-desc")?.textContent.trim()||null,
             count: addBtns.length, recRows };
  });
  console.log("  before: "+JSON.stringify(before));
  if(before.hasCard && before.count>0) pass("detected-recurring card present with "+before.count+" add buttons");
  else { fail("no detected card / add buttons (count="+before.count+")"); }
  if(before.title && /not in your plan/i.test(before.title)) pass("card title: "+before.title);
  // Click first add-to-plan, expect that charge to leave the detected list (now in plan).
  if(before.count>0){
    await p.click('[data-testid="detected-add"]'); await p.waitForTimeout(900);
    const after=await p.evaluate(()=>({ count:[...document.querySelectorAll('[data-testid="detected-add"]')].length }));
    console.log("  after add: detected count "+before.count+" -> "+after.count);
    if(after.count===before.count-1) pass("add-to-plan removed the charge from detected list (added to plan)");
    else fail("detected count did not decrease ("+before.count+"->"+after.count+")");
  }
  await p.screenshot({path:"e2e/screenshots/c147_detected.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
