// e2e: the Cover editor opens as a flip modal, spreads an overspend across multiple
// checked source budgets (equal by default), and honors per-source ratios.
import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const require = createRequire(path.join(path.dirname(fileURLToPath(import.meta.url)), "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const results=[]; let errs=[];
const check=(n,c,d="")=>{results.push({n,ok:!!c});console.log((c?"PASS ":"FAIL ")+n+(d?" — "+d:""));};
const idb = (p, fn, arg) => p.evaluate(async ([fn,arg]) => {
  const db = await new Promise(r=>{const q=indexedDB.open("cashflux-kv");q.onsuccess=()=>r(q.result);});
  const get=k=>new Promise(r=>{const t=db.transaction("kv","readonly");const rq=t.objectStore("kv").get(k);rq.onsuccess=()=>r(rq.result);});
  const put=(k,v)=>new Promise(r=>{const t=db.transaction("kv","readwrite");t.objectStore("kv").put(v,k);t.oncomplete=()=>r();});
  const ds = JSON.parse(await get("cashflux:dataset"));
  if (fn==="over") { ds.transactions.push({id:"over-baby",accountId:"acct-checking",date:"2026-07-01T00:00:00Z",desc:"Baby splurge",categoryId:"cat-baby",amount:{Amount:-50000,Currency:"USD"},source:"manual"}); await put("cashflux:dataset", JSON.stringify(ds)); return true; }
  if (fn==="limits") { const m={}; for(const bg of ds.budgets) m[bg.id]=bg.limit.Amount; return m; }
}, [fn,arg]);
try {
  const p = await b.newPage({ viewport:{width:1440,height:1000} });
  p.on("pageerror",e=>{const m=String(e);if(!m.includes("already exited"))errs.push(m);});
  await p.goto(BASE+"/",{waitUntil:"domcontentloaded"});
  await p.waitForSelector("#app .bento",{timeout:30000}).catch(()=>{});
  await p.waitForTimeout(1000);
  if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1800); }
  await idb(p,"over");
  await p.goto(BASE+"/budgets",{waitUntil:"domcontentloaded"});
  await p.waitForSelector(".bento-budgets .budget",{timeout:30000}).catch(()=>{});
  await p.waitForTimeout(700);

  check("C1 over-budget row shows a Cover button", await p.evaluate(()=>!!document.querySelector('[data-testid="budget-cover-btn-bud-baby"]')));
  await p.locator('[data-testid="budget-cover-btn-bud-baby"]').click();
  await p.waitForTimeout(700);
  check("C2 Cover opens the flip modal with source checkboxes", await p.evaluate(()=>document.querySelectorAll('[data-testid^="cover-src-"]').length>=2));
  check("C3 amount prefilled to the $100 overspend", await p.evaluate(()=>document.getElementById("budget-cover-amt")?.value==="100.00"), await p.evaluate(()=>document.getElementById("budget-cover-amt")?.value));

  // Check two sources → equal split $50/$50.
  const boxes = await p.$$('[data-testid^="cover-src-"]');
  await boxes[0].click(); await boxes[1].click(); await p.waitForTimeout(300);
  const shares1 = await p.evaluate(()=>[...document.querySelectorAll(".cover-src-share")].map(e=>e.textContent).filter(Boolean));
  check("C4 two checked sources split equally ($50/$50)", shares1.filter(s=>s.includes("50.00")).length===2, JSON.stringify(shares1));

  // Set the first selected source's ratio to 3 → $75/$25.
  const weights = await p.$$(".cover-src-weight");
  await weights[0].fill("3"); await p.waitForTimeout(300);
  const shares2 = await p.evaluate(()=>[...document.querySelectorAll(".cover-src-share")].map(e=>e.textContent).filter(Boolean));
  check("C5 ratio 3:1 splits $75/$25", shares2.some(s=>s.includes("75.00")) && shares2.some(s=>s.includes("25.00")), JSON.stringify(shares2));

  // Reset to equal ratio + a small amount every source can afford, then apply.
  await weights[0].fill("1"); await p.waitForTimeout(150);
  await p.fill("#budget-cover-amt", "20"); await p.waitForTimeout(250);
  await p.locator('.acct-edit-form button[type="submit"]').click();
  await p.waitForTimeout(1200);
  // Modal closed + baby's limit rose $400 → $420 (checked live on the row, not the
  // autosaved dataset which lags behind).
  const closed = await p.evaluate(()=>!document.getElementById("budget-cover-amt"));
  const babyAmt = await p.evaluate(()=>{
    const row=[...document.querySelectorAll(".bento-budgets .budget")].find(r=>r.textContent.includes("Baby & Childcare"));
    return row ? row.querySelector(".budget-amount")?.textContent||"" : "";
  });
  check("C6 cover applied: modal closed and baby's limit rose to $420", closed && babyAmt.includes("420.00"), `closed=${closed} amt=${babyAmt}`);

  // C7 — recurring: reopen cover, check a source, toggle recurring on, submit → badge.
  await p.locator('[data-testid="budget-cover-btn-bud-baby"]').click();
  await p.waitForTimeout(600);
  await (await p.$$('[data-testid^="cover-src-"]'))[0].click();
  await p.fill("#budget-cover-amt", "10"); await p.waitForTimeout(150);
  await p.locator('[data-testid="cover-recurring"]').click(); await p.waitForTimeout(150);
  await p.locator('.acct-edit-form button[type="submit"]').click();
  await p.waitForTimeout(1000);
  check("C7 recurring toggle saves a standing cover (badge shows on the row)", await p.evaluate(()=>!!document.querySelector('[data-testid="recurring-badge-bud-baby"]')));

  // C8 — the ⋯ menu offers "Remove recurring coverage".
  const babyRow = p.locator('.bento-budgets .budget', { hasText: 'Baby & Childcare' }).first();
  await babyRow.locator('.add-wrap button[aria-haspopup="menu"]').click(); await p.waitForTimeout(400);
  check("C8 ⋯ menu offers Remove recurring coverage", await p.evaluate(()=>!!document.querySelector('.add-menu:not(.hidden-menu) [data-testid="remove-recurring-btn-bud-baby"]')));

  // C9 — removing it (with confirm) clears the badge.
  await p.locator('.add-menu:not(.hidden-menu) [data-testid="remove-recurring-btn-bud-baby"]').click(); await p.waitForTimeout(400);
  await p.locator("#cf-dialog-confirm").click(); await p.waitForTimeout(800);
  check("C9 confirming removal clears the recurring badge", await p.evaluate(()=>!document.querySelector('[data-testid="recurring-badge-bud-baby"]')));

  const pass=results.filter(r=>r.ok).length, fail=results.length-pass;
  console.log("\n════════════════════════════════════════════");
  console.log(`RESULT: ${pass} PASS · ${fail} FAIL`);
  if(fail) console.log("FAILED: "+results.filter(r=>!r.ok).map(r=>r.n).join(", "));
  console.log("page errors: "+(errs.length?JSON.stringify([...new Set(errs)].slice(0,5)):"none"));
  console.log("════════════════════════════════════════════");
} finally { await b.close(); }
