import { createRequire } from "module"; import path from "path";
const require = createRequire(path.join(process.cwd(), ".tools", "package.json"));
const { chromium } = require("playwright");
const base="http://127.0.0.1:8099"; const b=await chromium.launch();
let pass=true; const log=(c,m)=>{console.log((c?"PASS ":"FAIL ")+m); if(!c)pass=false;};
try { const p=await b.newPage({viewport:{width:1280,height:900}});
  await p.goto(base+"/accounts",{waitUntil:"networkidle"}); await p.waitForSelector("aside.rail",{timeout:15000});
  const s=await p.$("text=/load sample/i"); if(s){await s.click().catch(()=>{}); await p.waitForTimeout(800);}
  await p.goto(base+"/transactions",{waitUntil:"networkidle"}); await p.waitForSelector(".txn-table tbody tr.row",{timeout:15000}); await p.waitForTimeout(500);
  const delSel='.txn-table tbody tr.row .btn-del, .txn-table tbody tr.row [aria-label*="elete"]';
  const dels=await p.$$(delSel);
  log(dels.length>=3, `found ${dels.length} row delete buttons`);
  await dels[1].focus();
  await p.keyboard.press("Enter"); // keyboard-activate (a11y path) → opens ConfirmModal
  await p.waitForTimeout(400);
  // Click the modal's confirm/delete button.
  const confirmBtn = await p.$('.cf-dialog button.btn-danger, .cf-dialog-actions button.btn-danger');
  log(!!confirmBtn, "confirm modal appeared with a confirm button");
  await confirmBtn.click();
  await p.waitForTimeout(900);
  const after = await p.evaluate(()=>{ const a=document.activeElement;
    return { tag:a.tagName, isBody:a===document.body, inTable: !!(a.closest && a.closest(".txn-table")), label:(a.getAttribute&&a.getAttribute("aria-label"))||a.className }; });
  log(!after.isBody, `focus not dropped to <body> after delete (active=${after.tag})`);
  log(after.inTable, `focus restored to a control inside the transactions table (${after.label})`);
} catch(e){ log(false,"exception: "+String(e)); }
finally{ await b.close(); }
console.log(pass?"\nRESULT: ALL PASS":"\nRESULT: FAILURES");
process.exit(pass?0:1);
