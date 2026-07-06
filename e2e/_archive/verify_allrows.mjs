import { createRequire } from "module"; import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base="http://127.0.0.1:8099"; const b=await chromium.launch();
let pass=true; const log=(c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
const rows=(p)=>p.evaluate(()=>document.querySelectorAll(".txn-table tbody tr.row").length);
const settle=async(p,want,ms=6000)=>{const t0=Date.now();while(Date.now()-t0<ms){if(await rows(p)===want)return true;await p.waitForTimeout(200);}return false;};
try { const p=await b.newPage({viewport:{width:1280,height:900}});
  await p.goto(base+"/accounts",{waitUntil:"networkidle"}); await p.waitForSelector("aside.rail",{timeout:15000});
  const s=await p.$("text=/load sample/i"); if(s){await s.click().catch(()=>{}); await p.waitForTimeout(800);}
  await p.goto(base+"/transactions",{waitUntil:"networkidle"}); await p.waitForSelector(".data-pager",{timeout:15000}); await p.waitForTimeout(500);
  const total=await p.evaluate(()=>Number(document.querySelector(".data-pager").innerText.match(/of (\d+)/)[1]));
  log(total>100, `ledger total = ${total} (>100, so a cap would hide rows)`);
  await (await p.$('.pager-sizes button:has-text("All")')).click();
  log(await settle(p,total), `"All" renders every row (${await rows(p)} == ${total})`);
  await (await p.$('.pager-sizes button:has-text("25")')).click();
  log(await settle(p,25), `"25" caps to 25 rows (${await rows(p)})`);
} catch(e){ log(false,"exception: "+String(e)); }
finally{ await b.close(); }
console.log(pass?"\nRESULT: ALL PASS":"\nRESULT: FAILURES");
process.exit(pass?0:1);
