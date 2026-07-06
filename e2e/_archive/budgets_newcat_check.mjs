// e2e: "Add budget" creates a new category (named after the budget) by default so
// transactions can be assigned straight to it.
import { createRequire } from "module";
import { fileURLToPath } from "url"; import path from "path";
const require = createRequire(path.join(path.dirname(fileURLToPath(import.meta.url)), "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8091";
const b = await chromium.launch({ headless: true });
const results = []; let errs = [];
const check = (n, c, d="") => { results.push({n,ok:!!c}); console.log((c?"PASS ":"FAIL ")+n+(d?" — "+d:"")); };
try {
  const p = await b.newPage({ viewport: { width: 1440, height: 1000 } });
  p.on("pageerror", e => { const m=String(e); if(!m.includes("already exited")) errs.push(m); });
  await p.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await p.waitForSelector("#app .bento", { timeout: 30000 }).catch(()=>{});
  await p.waitForTimeout(1000);
  if (await p.locator('[data-testid="hero-load-sample"]').count()) { await p.locator('[data-testid="hero-load-sample"]').click(); await p.waitForTimeout(1800); }
  await p.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await p.waitForSelector(".bento-budgets .budget", { timeout: 30000 }).catch(()=>{});
  await p.waitForTimeout(500);

  const catsBefore = await p.evaluate(async () => {
    const db = await new Promise(r=>{const q=indexedDB.open("cashflux-kv");q.onsuccess=()=>r(q.result);});
    const raw = await new Promise(r=>{const t=db.transaction("kv","readonly");const rq=t.objectStore("kv").get("cashflux:dataset");rq.onsuccess=()=>r(rq.result);});
    try { return (JSON.parse(raw).categories||[]).length; } catch { return -1; }
  });

  await p.locator('[data-testid="budgets-add"]').click();
  await p.waitForSelector('[data-testid="budget-add-form"]', { timeout: 10000 });
  check("N1 add form opens with the New-category name field (default mode)", await p.evaluate(()=>!!document.querySelector('[data-testid="budget-new-cat-name"]')));
  await p.fill("#budget-add", "Vacation Fund");
  await p.fill('[data-testid="budget-add-form"] input[type="number"]', "500");
  await p.locator('[data-testid="budget-add-form"] button[type="submit"]').click();
  await p.waitForTimeout(1200);

  const after = await p.evaluate(async () => {
    const db = await new Promise(r=>{const q=indexedDB.open("cashflux-kv");q.onsuccess=()=>r(q.result);});
    const raw = await new Promise(r=>{const t=db.transaction("kv","readonly");const rq=t.objectStore("kv").get("cashflux:dataset");rq.onsuccess=()=>r(rq.result);});
    let cats=[]; try { cats=JSON.parse(raw).categories||[]; } catch {}
    const rows=[...document.querySelectorAll(".bento-budgets .budget .row-desc")].map(e=>e.textContent);
    return { catCount: cats.length, hasVacationCat: cats.some(c=>c.name==="Vacation Fund" && c.kind==="expense"),
      hasVacationRow: rows.some(t=>t.includes("Vacation Fund")) };
  });
  check("N2 a new expense category 'Vacation Fund' was created", after.hasVacationCat, `catsBefore=${catsBefore} catsAfter=${after.catCount}`);
  check("N3 the new budget row appears", after.hasVacationRow);
  const p2 = results; const pass=p2.filter(r=>r.ok).length, fail=p2.length-pass;
  console.log("\n════════════════════════════════════════════");
  console.log(`RESULT: ${pass} PASS · ${fail} FAIL`);
  if(fail) console.log("FAILED: "+p2.filter(r=>!r.ok).map(r=>r.n).join(", "));
  console.log("page errors: "+(errs.length?JSON.stringify([...new Set(errs)].slice(0,5)):"none"));
  console.log("════════════════════════════════════════════");
} finally { await b.close(); }
