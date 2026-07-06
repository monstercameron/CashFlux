import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
async function boot(p){await p.goto(BASE,{waitUntil:'domcontentloaded'});
  await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
  await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();});await p.waitForTimeout(500);}
(async()=>{const b=await chromium.launch();
for(const vw of [1280,1100,1025,1024,768,390]){
const ctx=await b.newContext({viewport:{width:vw,height:850}});const p=await ctx.newPage();
await boot(p);
await p.click('.add-btn').catch(()=>{});
await p.waitForTimeout(350);
const m=await p.evaluate(()=>{
  const items=[...document.querySelectorAll('.add-item')];
  if(!items.length) return {noItems:true};
  const lefts=items.map(it=>Math.round(it.getBoundingClientRect().left));
  const rights=items.map(it=>Math.round(it.getBoundingClientRect().right));
  const rail=document.querySelector('aside.rail');const railR=rail?Math.round(rail.getBoundingClientRect().right):0;
  return {minLeft:Math.min(...lefts),maxRight:Math.max(...rights),vw:document.documentElement.clientWidth,railR};
});
if(m.noItems){F(`${vw}: no menu items`);await ctx.close();continue;}
const overflowR=m.maxRight>m.vw+1;
const overlapRail=m.minLeft<m.railR;
(!overflowR && !overlapRail)?P(`${vw}px: menu OK (items [${m.minLeft}..${m.maxRight}], vw=${m.vw}, railR=${m.railR})`):F(`${vw}px: overflowRight=${overflowR} overlapRail=${overlapRail} items=[${m.minLeft}..${m.maxRight}] vw=${m.vw} railR=${m.railR}`);
await ctx.close();
}
await b.close();
console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
process.exit(f?1:0);})();
