import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099";
const ident=t=>(!t||t==="none"||t==="matrix(1, 0, 0, 1, 0, 0)");
let failed=0; const fail=m=>{console.error("FAIL: "+m);failed++;process.exitCode=1;}; const pass=m=>console.log("PASS: "+m);
const errs=[];
async function tileHoverTransform(p){
  // hover the first bento .w tile and read its computed transform
  const w = await p.$('.bento .w');
  if(!w) return null;
  await w.hover();
  await p.waitForTimeout(350);
  return p.evaluate(()=>{const el=document.querySelector('.bento .w:hover')||document.querySelector('.bento .w'); return getComputedStyle(el).transform;});
}
try{
  const browser = await chromium.launch({ headless: true });
  // CASE 1: default (full wonder)
  let ctx=await browser.newContext({viewport:{width:1440,height:1000}});
  let p=await ctx.newPage();
  p.on("pageerror",e=>errs.push(String(e))); p.on("console",m=>{if(m.type()==="error")errs.push(m.text());});
  await p.goto(BASE+"/",{waitUntil:"networkidle"});
  await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(4500);
  await p.evaluate(()=>document.documentElement.removeAttribute("data-wonder"));
  const tiles=await p.evaluate(()=>document.querySelectorAll('.bento .w').length);
  console.log("  bento tiles:", tiles);
  const t1=await tileHoverTransform(p);
  console.log("  default hover .w transform:", t1);
  if(!ident(t1)) pass("W-3: bento tile hover yields a lift (non-identity) in default mode"); else fail("no hover lift on .w: "+t1);
  // .w.drag → identity (lift must not fight drag ghost)
  const dragT=await p.evaluate(()=>{
    const el=document.querySelector('.bento .w'); el.classList.add('drag');
    const t=getComputedStyle(el).transform; el.classList.remove('drag'); return t;
  });
  console.log("  .w.drag transform:", dragT);
  if(ident(dragT)) pass("W-3: .w.drag → identity (hover lift suppressed during drag — drag-safe)"); else fail(".w.drag not identity: "+dragT);
  await ctx.close();
  // CASE 2: data-wonder=off → identity
  ctx=await browser.newContext({viewport:{width:1440,height:1000}}); p=await ctx.newPage();
  await p.goto(BASE+"/",{waitUntil:"networkidle"}); await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(4500);
  await p.evaluate(()=>document.documentElement.setAttribute("data-wonder","off"));
  const t2=await tileHoverTransform(p);
  console.log("  off hover .w transform:", t2);
  if(ident(t2)) pass("W-3: data-wonder=off → tile hover identity (fully static)"); else fail("off not identity: "+t2);
  await ctx.close();
  // CASE 3: reduced-motion → identity
  ctx=await browser.newContext({viewport:{width:1440,height:1000}, reducedMotion:"reduce"}); p=await ctx.newPage();
  await p.goto(BASE+"/",{waitUntil:"networkidle"}); await p.waitForSelector("#app",{timeout:60000}); await p.waitForTimeout(4500);
  const t3=await tileHoverTransform(p);
  console.log("  reduced-motion hover .w transform:", t3);
  if(ident(t3)) pass("W-3: prefers-reduced-motion → tile hover identity"); else fail("reduced-motion not identity: "+t3);
  await p.screenshot({path:"e2e/screenshots/w3_bento_verify.png"});
  await ctx.close();
  console.log("errors:", errs.length); if(errs.length){errs.slice(0,4).forEach(e=>console.log("  ERR:"+e));fail("console errors");}
  await browser.close();
}catch(e){fail("exception: "+e.message);}
console.log(failed?"RESULT: FAILED":"RESULT: PASSED");
process.exit(failed?1:0);
