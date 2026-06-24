import { createRequire } from "module"; import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base="http://127.0.0.1:8099"; const b=await chromium.launch();
let pass=true; const log=(c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
const firstRowDesc=(p)=>p.evaluate(()=>{ const r=document.querySelector(".txn-table tbody tr.row"); return r? r.innerText.replace(/\s+/g," ").trim().slice(0,40):null; });
const hasDesc=(p,d)=>p.evaluate((d)=>[...document.querySelectorAll(".txn-table tbody tr.row")].some(r=>r.innerText.replace(/\s+/g," ").includes(d)), d);
try { const p=await b.newPage({viewport:{width:1280,height:900}});
  await p.goto(base+"/accounts",{waitUntil:"networkidle"}); await p.waitForSelector("aside.rail",{timeout:15000});
  const s=await p.$("text=/load sample/i"); if(s){await s.click().catch(()=>{}); await p.waitForTimeout(800);}
  await p.goto(base+"/transactions",{waitUntil:"networkidle"}); await p.waitForSelector(".txn-table tbody tr.row",{timeout:15000}); await p.waitForTimeout(500);
  // Capture an identifying token from the first row (e.g. its description text).
  const token = await p.evaluate(()=>{ const cells=[...document.querySelectorAll(".txn-table tbody tr.row")][0].querySelectorAll("td"); for(const c of cells){const t=c.innerText.trim(); if(t.length>6 && !/^[\d.,$\-\/]+$/.test(t)) return t.slice(0,20);} return null; });
  log(!!token, `captured first-row token: "${token}"`);
  const present0 = await hasDesc(p, token); log(present0, "token present before delete");
  // Delete the first row.
  const del=await p.$('.txn-table tbody tr.row .btn-del, .txn-table tbody tr.row [aria-label*="elete"]');
  await del.click(); await p.waitForTimeout(400);
  await (await p.$('.cf-dialog button.btn-danger')).click(); await p.waitForTimeout(700);
  const undo=await p.$('.toast .toast-undo, .toast-undo'); log(!!undo, "Undo toast button appears after delete");
  await undo.click(); await p.waitForTimeout(900);
  const present1 = await hasDesc(p, token); log(present1, `Undo restored the deleted transaction (token "${token}" present again)`);
} catch(e){ log(false,"exception: "+String(e)); }
finally{ await b.close(); }
console.log(pass?"\nRESULT: ALL PASS":"\nRESULT: FAILURES");
process.exit(pass?0:1);
