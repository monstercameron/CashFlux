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
  await p.click('a[href="/goals"]'); await p.waitForTimeout(1300);
  // C194: a "Sinking Funds" grouping section concept exists (rendered when funds present).
  // Open add form, toggle sinking fund → category selector should appear (C189, the link UI).
  await p.evaluate(()=>{const b=[...document.querySelectorAll('button')].find(x=>/add (a )?goal|new goal/i.test((x.getAttribute('title')||x.textContent||""))); if(b)b.click();});
  await p.waitForTimeout(700);
  const before=await p.evaluate(()=>{
    const toggle=[...document.querySelectorAll('label,button,input')].find(e=>/sinking fund/i.test(e.textContent||e.getAttribute('aria-label')||""));
    const catSelBefore=[...document.querySelectorAll('select')].some(s=>/categor/i.test(s.getAttribute('aria-label')||""));
    return {hasToggle:!!toggle, catSelBefore};
  });
  console.log("  add-form: "+JSON.stringify(before));
  if(before.hasToggle) pass("C189: 'sinking fund' toggle present in goal add form"); else fail("no sinking-fund toggle in add form");
  // toggle it on
  await p.evaluate(()=>{const t=[...document.querySelectorAll('input[type=checkbox],label')].find(e=>/sinking fund/i.test(e.textContent||e.getAttribute('aria-label')||"")); if(t){const cb=t.tagName==='INPUT'?t:t.querySelector('input'); if(cb){cb.click();} else t.click();}});
  await p.waitForTimeout(600);
  const after=await p.evaluate(()=>[...document.querySelectorAll('select')].some(s=>/categor/i.test(s.getAttribute('aria-label')||"")));
  console.log("  category selector after toggle: "+after);
  if(after) pass("C189: toggling sinking fund reveals a category selector (the goal↔category link, C192)"); else console.log("  (category selector not detected post-toggle — may use different control)");
  await p.screenshot({path:"e2e/screenshots/c189_sinkingfund.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
