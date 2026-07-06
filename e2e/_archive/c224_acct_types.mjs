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
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(4500);
  await p.click('a[href="/accounts"]'); await p.waitForTimeout(1200);
  // open add-account form
  await p.evaluate(()=>{const b=[...document.querySelectorAll('button')].find(x=>/add (an )?account|new account|\+/i.test((x.getAttribute('title')||x.getAttribute('aria-label')||x.textContent||""))); if(b)b.click();});
  await p.waitForTimeout(800);
  const opts=await p.evaluate(()=>{
    const sel=[...document.querySelectorAll('select')].find(s=>[...s.options].some(o=>/checking|savings/i.test(o.textContent)));
    return sel? [...sel.options].map(o=>o.textContent.trim()) : null;
  });
  if(!opts){ fail("account-type select not found in add form"); }
  else {
    console.log("  type options: "+JSON.stringify(opts));
    const has=re=>opts.some(o=>re.test(o));
    if(has(/propert/i)) pass("C224: Property type offered"); else fail("no Property type");
    if(has(/vehicle/i)) pass("C224: Vehicle type offered"); else fail("no Vehicle type");
    if(has(/retire|401/i)) pass("C73: Retirement type offered"); else fail("no Retirement type");
    if(has(/crypto/i)) pass("C73: Crypto type offered"); else fail("no Crypto type");
    // labels humanized (no raw snake_case)
    if(!opts.some(o=>/_/.test(o))) pass("type labels humanized (no raw snake_case)"); else console.log("  (some raw label: "+opts.filter(o=>/_/.test(o))+")");
  }
  await p.screenshot({path:"e2e/screenshots/c224_acct_types.png"});
  console.log("errors: "+errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
}catch(e){fail("exception: "+e.message);}finally{await browser.close();}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
