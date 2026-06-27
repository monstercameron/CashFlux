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
  await p.click('a[href="/goals"]'); await p.waitForTimeout(1400);
  const before=await p.evaluate(()=>[...document.querySelectorAll('[role="progressbar"]')].length);
  // open add form
  await p.evaluate(()=>{const b=[...document.querySelectorAll('button')].find(x=>/add (a )?goal|new goal/i.test((x.getAttribute('title')||x.textContent||""))); if(b)b.click();});
  await p.waitForTimeout(700);
  const formThere=await p.$('[data-testid="goal-add-form"]');
  if(!formThere){ fail("goal-add-form did not open"); }
  else {
    await p.fill('#goal-add','ZZ Test Goal');
    // target = first number input within the form
    const numSel='[data-testid="goal-add-form"] input[type="number"]';
    await p.fill(numSel,'1000');
    await p.waitForTimeout(300);
    // submit
    await p.click('[data-testid="goal-add-form"] button[type="submit"]');
    await p.waitForTimeout(1200);
    const after=await p.evaluate(()=>({n:[...document.querySelectorAll('[role="progressbar"]')].length, hasZZ:/ZZ Test Goal/.test(document.body.innerText)}));
    console.log("  before="+before+" after="+JSON.stringify(after));
    if(after.hasZZ && after.n>before) pass("C177: added goal reflected immediately in the list (no reload)");
    else fail("C177: not reflected (after="+JSON.stringify(after)+")");
  }
  await p.screenshot({path:"e2e/screenshots/c177_add.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,5).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
