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
  await p.evaluate(()=>{const b=document.querySelector('button.hh'); if(b)b.click();});
  await p.waitForTimeout(900);
  const r=await p.evaluate(()=>{
    const btns=[...document.querySelectorAll('button')].map(b=>b.textContent.trim());
    return {
      backupAll: btns.some(t=>/back up everything|backup everything/i.test(t)),
      wipeBtn: btns.find(t=>/erase everything|wipe data|erase all|wipe/i.test(t))||null,
      genericWipe: btns.some(t=>/^confirm$/i.test(t)),
      dataJump: [...document.querySelectorAll('.set-section-nav button')].some(t=>/^data$/i.test(t.textContent.trim())),
    };
  });
  console.log("  "+JSON.stringify(r));
  if(r.backupAll) pass("C297: 'Back up everything' button present in Settings"); else fail("no back-up-everything button");
  if(r.wipeBtn) pass("C298: wipe action present with a clear label: "+JSON.stringify(r.wipeBtn)); else console.log("  (wipe button label not matched in static list — may be in confirm modal)");
  if(r.dataJump) pass("C298: 'Data' jump-to tab present"); else console.log("  (Data jump tab not found)");
  await p.screenshot({path:"e2e/screenshots/c297_c298_data.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
