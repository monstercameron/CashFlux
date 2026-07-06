// Guards the transactions DataTable responsive behaviour:
//  • ≤900px (phones/tablets): .txn-table switches to the stacked card layout
//    (display:block) and does NOT clip its right-hand columns.
//  • ≥desktop: stays a real table, fits without clipping, and the sticky <th>
//    header pins to the scroll-container top on scroll.
// Background: the 8-col table has a ~949px intrinsic min-width; the card
// breakpoint was raised 760→900 (2026-06-24) to cover tablet widths.
import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
async function boot(p){await p.goto(BASE,{waitUntil:'domcontentloaded'});
  await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
  await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();});}
async function txn(p){await p.evaluate(()=>{history.pushState({},'','/transactions');dispatchEvent(new PopStateEvent('popstate'));});await p.waitForTimeout(1100);}
(async()=>{const b=await chromium.launch();
for(const vw of [768,880]){
  const ctx=await b.newContext({viewport:{width:vw,height:850}});const p=await ctx.newPage();
  await boot(p);await txn(p);
  const m=await p.evaluate(()=>{const t=document.querySelector('.txn-table');const r=t.getBoundingClientRect();
    return {disp:getComputedStyle(t).display,rows:document.querySelectorAll('.txn-table tbody tr.row').length,
      clipped:Math.round(r.right)>document.documentElement.clientWidth+1};});
  (m.disp==='block'&&!m.clipped&&m.rows>0)?P(`${vw}px: card layout, ${m.rows} rows, no clip`):F(`${vw}px: ${JSON.stringify(m)}`);
  await ctx.close();
}
{ const ctx=await b.newContext({viewport:{width:1280,height:850}});const p=await ctx.newPage();
  await boot(p);await txn(p);
  const m=await p.evaluate(()=>{const t=document.querySelector('.txn-table');const r=t.getBoundingClientRect();
    return {disp:getComputedStyle(t).display,clipped:Math.round(r.right)>document.documentElement.clientWidth+1};});
  (m.disp==='table'&&!m.clipped)?P(`1280px: table fits, no clip`):F(`1280px: ${JSON.stringify(m)}`);
  const before=await p.evaluate(()=>Math.round(document.querySelector('.txn-table thead th').getBoundingClientRect().top));
  await p.evaluate(()=>{const c=document.querySelector('.cf-scroll');if(c)c.scrollTop=700;});
  await p.waitForTimeout(300);
  const after=await p.evaluate(()=>Math.round(document.querySelector('.txn-table thead th').getBoundingClientRect().top));
  // sticky => th pins near the container top (≈0), well above its initial offset
  (after<before-100&&Math.abs(after)<12)?P(`1280px: sticky th header pins on scroll (${before}→${after})`):F(`1280px: sticky th not pinning (${before}→${after})`);
  await ctx.close();
}
await b.close();
console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
process.exit(f?1:0);})();
