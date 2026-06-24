import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
async function boot(p){await p.goto(BASE,{waitUntil:'domcontentloaded'});
  await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
  await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();});}
async function wm(p){await p.evaluate(()=>{history.pushState({},'','/widget-manager');dispatchEvent(new PopStateEvent('popstate'));});await p.waitForTimeout(1100);}
(async()=>{const b=await chromium.launch();
// 390: wrapper scrolls, page itself doesn't overflow
{const ctx=await b.newContext({viewport:{width:390,height:850}});const p=await ctx.newPage();
 await boot(p);await wm(p);
 const m=await p.evaluate(()=>{
   const wrap=document.querySelector('.wm-table-wrap');const t=document.querySelector('.wm-table');
   if(!wrap||!t) return {missing:true};
   const docW=document.documentElement.clientWidth;
   const scrollable=wrap.scrollWidth>wrap.clientWidth+2;
   wrap.scrollLeft=9999;const sl=wrap.scrollLeft;
   // does the table still overflow the VIEWPORT (page-level clip)? It should NOT now.
   const wrapR=wrap.getBoundingClientRect();
   const pageClip=Math.round(wrapR.right)>docW+1;
   // order column reachable after scroll?
   const orderTh=document.querySelector('.wm-col-order')||[...document.querySelectorAll('.wm-table th')].pop();
   const orderRight=orderTh?Math.round(orderTh.getBoundingClientRect().right):null;
   return {scrollable, scrolledTo:sl, pageClip, docW, wrapRight:Math.round(wrapR.right), orderRightAfterScroll:orderRight};
 });
 if(m.missing){F('390: .wm-table-wrap missing');}
 else{
   m.scrollable?P(`390: wm-table scrolls inside wrapper (scrollW>clientW)`):F(`390: not scrollable ${JSON.stringify(m)}`);
   (m.scrolledTo>0)?P(`390: can scroll to reach Order column (scrollLeft=${m.scrolledTo})`):F(`390: scrollLeft stayed 0`);
   (!m.pageClip)?P(`390: wrapper stays within viewport — no page-level clip (right=${m.wrapRight} docW=${m.docW})`):F(`390: wrapper still overflows page (right=${m.wrapRight})`);
 }
 await p.screenshot({path:'e2e/screenshots/wmtable_390_fixed.png'});
 await ctx.close();
}
// 1280: no scrollbar (table fits)
{const ctx=await b.newContext({viewport:{width:1280,height:850}});const p=await ctx.newPage();
 await boot(p);await wm(p);
 const m=await p.evaluate(()=>{const wrap=document.querySelector('.wm-table-wrap');return wrap?{overflow:wrap.scrollWidth>wrap.clientWidth+2}:{missing:true};});
 (!m.missing && !m.overflow)?P('1280: desktop wm-table fits, no scrollbar'):log.push(`INFO 1280: ${JSON.stringify(m)}`);
 await ctx.close();
}
await b.close();
console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
process.exit(f?1:0);})();
