import { createRequire } from "module"; import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base="http://127.0.0.1:8099"; const b=await chromium.launch();
let pass=true; const log=(c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
const rowCount=(p)=>p.evaluate(()=>document.querySelectorAll(".txn-table tbody tr.row").length);
try { const p=await b.newPage({viewport:{width:1280,height:900}});
  await p.goto(base+"/accounts",{waitUntil:"networkidle"}); await p.waitForSelector("aside.rail",{timeout:15000});
  const s=await p.$("text=/load sample/i"); if(s){await s.click().catch(()=>{}); await p.waitForTimeout(800);}
  await p.goto(base+"/transactions",{waitUntil:"networkidle"}); await p.waitForSelector(".txn-table tbody tr.row",{timeout:15000}); await p.waitForTimeout(500);
  const c0=await rowCount(p); log(c0>0, `transactions before delete: ${c0}`);
  // Delete first row → confirm modal → confirm.
  const del=await p.$('.txn-table tbody tr.row .btn-del, .txn-table tbody tr.row [aria-label*="elete"]');
  await del.click(); await p.waitForTimeout(400);
  await (await p.$('.cf-dialog button.btn-danger, .cf-dialog-actions button.btn-danger')).click();
  await p.waitForTimeout(700);
  const c1=await rowCount(p); log(c1===c0-1, `row deleted: ${c0} -> ${c1}`);
  // The toast must show an Undo button.
  const undo=await p.$('.toast .toast-undo, .toast-undo');
  log(!!undo, "undo toast button appears after delete");
  // Click Undo → row restored.
  await undo.click(); await p.waitForTimeout(800);
  const c2=await rowCount(p); log(c2===c0, `Undo restored the transaction: ${c1} -> ${c2} (expected ${c0})`);
} catch(e){ log(false,"exception: "+String(e)); }
finally{ await b.close(); }
console.log(pass?"\nRESULT: ALL PASS":"\nRESULT: FAILURES");
process.exit(pass?0:1);
