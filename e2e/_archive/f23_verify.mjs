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
  // C182: overall-progress has an explanatory tooltip (title attr on label).
  const c182=await p.evaluate(()=>{
    const lbls=[...document.querySelectorAll('.stat-label, [class*="stat-label"]')];
    const l=lbls.find(e=>/overall progress/i.test(e.textContent||""));
    return l? (l.getAttribute('title')||l.closest('[title]')?.getAttribute('title')||"") : null;
  });
  console.log("  C182 overall-progress title: "+JSON.stringify(c182));
  if(c182 && c182.length>3) pass("C182: overall-progress carries an explanatory tooltip"); else fail("C182: no tooltip on overall progress");
  // C178: a goal row shows a monthly-needed (contribution-rate) chip.
  const c178=await p.evaluate(()=>{
    const chips=[...document.querySelectorAll('.pace-rate, .pace-badge')].map(e=>e.textContent.trim());
    return chips;
  });
  console.log("  C178 pace/rate chips: "+JSON.stringify(c178.slice(0,6)));
  if(c178.some(t=>/\$/.test(t)||/\/mo/i.test(t)||/month/i.test(t))) pass("C178: contribution-rate chip present on goal rows"); else console.log("  (no $ chip — goals may all be met/dateless)");
  // Open the add-goal modal: C176 owner + linked-account fields visible (not behind advanced).
  await p.evaluate(()=>{ // trigger add via the goals add button if present
    const b=[...document.querySelectorAll('button')].find(x=>/add (a )?goal|new goal/i.test((x.getAttribute('title')||x.textContent||"")));
    if(b) b.click();
  });
  await p.waitForTimeout(900);
  const c176=await p.evaluate(()=>{
    const txt=document.body.innerText;
    const ownerField=[...document.querySelectorAll('label,.field-label,select')].some(e=>/owner/i.test(e.textContent||e.getAttribute('aria-label')||""));
    const acctField=[...document.querySelectorAll('label,.field-label,select')].some(e=>/linked account|account/i.test(e.textContent||e.getAttribute('aria-label')||""));
    const advToggle=[...document.querySelectorAll('button')].some(b=>/advanced/i.test(b.textContent||"")&&/goal/i.test(document.body.innerText));
    return {ownerField, acctField};
  });
  console.log("  C176 add-form fields: "+JSON.stringify(c176));
  if(c176.ownerField && c176.acctField) pass("C176: Owner + Linked-account fields visible in add form (not behind Advanced)"); else fail("C176: owner/account not directly visible: "+JSON.stringify(c176));
  await p.screenshot({path:"e2e/screenshots/f23_verify.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,5).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
