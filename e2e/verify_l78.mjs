import { createRequire } from "module"; import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base="http://127.0.0.1:8099"; const b=await chromium.launch();
let pass=true; const log=(c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
try { const p=await b.newPage({viewport:{width:1280,height:900}});
  await p.goto(base+"/accounts",{waitUntil:"networkidle"}); await p.waitForSelector("aside.rail",{timeout:15000});
  const s=await p.$("text=/load sample/i"); if(s){await s.click().catch(()=>{}); await p.waitForTimeout(800);}
  await p.goto(base+"/dashboard",{waitUntil:"networkidle"}); await p.waitForSelector(".bento",{timeout:15000});
  await p.keyboard.press("Alt+KeyN"); await p.waitForSelector('input[type="number"]',{timeout:5000}); await p.waitForTimeout(300);
  // T4: aria-labels on amount + description
  const amtAria=await p.evaluate(()=>document.querySelector('input[type="number"]').getAttribute("aria-label"));
  const descAria=await p.evaluate(()=>{const i=[...document.querySelectorAll('input[type="text"]')].find(x=>/desc|what/i.test(x.placeholder||"")); return i?i.getAttribute("aria-label"):null;});
  log(!!amtAria, `T4: Amount input has aria-label ("${amtAria}")`);
  log(!!descAria, `T4: Description input has aria-label ("${descAria}")`);
  // T3: default account not an investment (401k/Brokerage)
  const acct=await p.evaluate(()=>{const sel=document.querySelector('select'); const o=sel.options[sel.selectedIndex]; return o?o.textContent:null;});
  log(acct && !/401|brokerage|invest|roth|ira/i.test(acct), `T3: default account is not an investment ("${acct}")`);
  // T1: Save disabled with empty description
  await p.fill('input[type="number"]',"12.34");
  const dis1=await p.evaluate(()=>{const sv=[...document.querySelectorAll('button')].find(x=>/^save$/i.test(x.textContent.trim())); return sv?sv.disabled||sv.getAttribute("aria-disabled")==="true":null;});
  log(dis1===true, `T1: Save disabled while Description empty (disabled=${dis1})`);
  // fill desc -> Save enabled
  await p.fill('.flip-backdrop input[type="text"], input[type="text"]', "Lunch");
  await p.waitForTimeout(300);
  const dis2=await p.evaluate(()=>{const sv=[...document.querySelectorAll('button')].find(x=>/^save$/i.test(x.textContent.trim())); return sv?sv.disabled||sv.getAttribute("aria-disabled")==="true":null;});
  log(dis2===false, `T1: Save enabled once Description + Amount valid (disabled=${dis2})`);
  // T5: success toast after add
  await p.evaluate(()=>{const sv=[...document.querySelectorAll('button.set-btn.save')].find(x=>/^save$/i.test(x.textContent.trim())); sv&&sv.click();});
  await p.waitForTimeout(800);
  const toast=await p.evaluate(()=>{const t=document.querySelector(".toast");return t?t.innerText:null;});
  log(toast && /add|saved/i.test(toast), `T5: success toast after add ("${(toast||"").replace(/\s+/g,' ').trim()}")`);
} catch(e){ log(false,"exception: "+String(e)); }
finally{ await b.close(); }
console.log(pass?"\nRESULT: ALL PASS":"\nRESULT: FAILURES");
process.exit(pass?0:1);
