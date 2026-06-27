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
const titleOf=p=>p.evaluate(()=>{const t=[...document.querySelectorAll(".card-title,h2,h3")].find(e=>/calendar/i.test(e.textContent||"")); return t?t.textContent.trim():null;});
try{
  const ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  const p=await ctx.newPage();
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000});
  await p.waitForTimeout(4500);
  await p.click('a[href="/bills"]'); await p.waitForTimeout(1500);
  const t0=await titleOf(p);
  console.log("  initial title: "+JSON.stringify(t0));
  const navPresent=await p.evaluate(()=>!!document.querySelector('[data-testid="cal-next"]')&&!!document.querySelector('[data-testid="cal-prev"]'));
  if(navPresent) pass("prev/next nav controls present"); else fail("nav controls missing");
  // This-month button should be absent at offset 0
  const todayBtn0=await p.evaluate(()=>!!document.querySelector('[data-testid="cal-today"]'));
  if(!todayBtn0) pass("'This month' button hidden at offset 0"); else fail("'This month' shown at offset 0");
  // click next
  await p.click('[data-testid="cal-next"]'); await p.waitForTimeout(600);
  const t1=await titleOf(p);
  console.log("  after next: "+JSON.stringify(t1));
  if(t1 && t1!==t0) pass("title advanced after Next: "+t1); else fail("title did not change on Next");
  const todayBtn1=await p.evaluate(()=>!!document.querySelector('[data-testid="cal-today"]'));
  if(todayBtn1) pass("'This month' button appears when offset!=0"); else fail("'This month' missing at offset 1");
  // back to today
  await p.click('[data-testid="cal-today"]'); await p.waitForTimeout(600);
  const t2=await titleOf(p);
  if(t2===t0) pass("'This month' resets to original month: "+t2); else fail("reset mismatch: "+t2+" vs "+t0);
  await p.screenshot({path:"e2e/screenshots/c148_calnav.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
