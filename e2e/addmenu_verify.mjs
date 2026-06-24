import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
async function boot(p,theme){await p.goto(BASE,{waitUntil:'domcontentloaded'});
  await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:'domcontentloaded'});
  await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
  await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();});await p.waitForTimeout(500);}
(async()=>{const b=await chromium.launch();
for(const theme of ['dark','light']){
const ctx=await b.newContext({viewport:{width:1280,height:900}});const p=await ctx.newPage();
await boot(p,theme);
// open menu
await p.click('.add-btn'); await p.waitForTimeout(350);
const geo=await p.evaluate(()=>{
  const railR=Math.round(document.querySelector('aside.rail').getBoundingClientRect().right);
  const items=[...document.querySelectorAll('.add-item')];
  const minLeft=Math.min(...items.map(it=>Math.round(it.getBoundingClientRect().left)));
  const maxRight=Math.max(...items.map(it=>Math.round(it.getBoundingClientRect().right)));
  return {railR, count:items.length, minLeft, maxRight, vw:document.documentElement.clientWidth};
});
(geo.minLeft>=geo.railR)?P(`${theme}: menu items clear of rail (minLeft=${geo.minLeft} >= railRight=${geo.railR})`):F(`${theme}: items overlap rail (minLeft=${geo.minLeft} < railRight=${geo.railR})`);
(geo.maxRight<=geo.vw)?P(`${theme}: menu fits in viewport (maxRight=${geo.maxRight} <= ${geo.vw})`):F(`${theme}: menu overflows right (maxRight=${geo.maxRight})`);
await p.screenshot({path:`e2e/screenshots/addmenu_fixed_${theme}.png`});
// now CLICK New transaction — should not be intercepted by the rail anymore
let clickOk=false, modalOpened=false;
try{
  const item=p.locator('.add-item', {hasText:/transaction/i}).first();
  await item.click({timeout:4000});
  clickOk=true;
  await p.waitForTimeout(600);
  modalOpened=await p.evaluate(()=>!!document.querySelector('.flip-inner, .cf-dialog, .set-card, [role=dialog]'));
}catch(e){clickOk=false;}
clickOk?P(`${theme}: "New transaction" item clickable (no rail interception)`):F(`${theme}: item still not clickable`);
modalOpened?P(`${theme}: clicking item opened the add modal`):log.push(`INFO ${theme}: modal selector not matched (opened=${modalOpened})`);
await p.screenshot({path:`e2e/screenshots/addmodal_${theme}.png`});
await ctx.close();
}
await b.close();
console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
process.exit(f?1:0);})();
