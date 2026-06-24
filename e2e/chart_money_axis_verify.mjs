import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
async function boot(p,theme){await p.goto(BASE,{waitUntil:'domcontentloaded'});
  await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:'domcontentloaded'});
  await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
  await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();
    history.pushState({},'','/reports');dispatchEvent(new PopStateEvent('popstate'));});
  await p.waitForTimeout(1700);}
(async()=>{const b=await chromium.launch();
for(const theme of ['dark','light']){
const ctx=await b.newContext({viewport:{width:1280,height:1000}});const p=await ctx.newPage();
await boot(p,theme);
const m=await p.evaluate(()=>{
  // collect all SVG axis-tick texts that are purely numeric-ish (value axis) vs $-prefixed
  const texts=[...document.querySelectorAll('#app svg text')].map(t=>t.textContent.trim()).filter(Boolean);
  const moneyTicks=texts.filter(t=>/^\$[\d.,]+[kKmM]?$/.test(t));   // "$1.5k", "$500"
  const bareNumTicks=texts.filter(t=>/^[\d,]{3,}$/.test(t));         // bare "500","1,000","1500"
  const catLabels=texts.filter(t=>/housing|groceries|dining|electricity|rent/i.test(t));
  return {total:texts.length, moneyTicks:[...new Set(moneyTicks)].slice(0,8), bareNumTicks:[...new Set(bareNumTicks)].slice(0,8), catLabels:catLabels.slice(0,4)};
});
(m.moneyTicks.length>0)?P(`${theme}: chart Y-axis shows currency ticks: ${JSON.stringify(m.moneyTicks)}`):F(`${theme}: no $-prefixed ticks found (${JSON.stringify(m.bareNumTicks)})`);
(m.bareNumTicks.length===0)?P(`${theme}: no bare-number value ticks remain`):log.push(`INFO ${theme}: residual bare ticks (could be other charts): ${JSON.stringify(m.bareNumTicks)}`);
(m.catLabels.length>0 && !m.catLabels.some(c=>c.startsWith('$')))?P(`${theme}: category labels NOT money-formatted (${JSON.stringify(m.catLabels)})`):log.push(`INFO ${theme}: catLabels=${JSON.stringify(m.catLabels)}`);
await p.screenshot({path:`e2e/screenshots/chart_money_${theme}.png`});
await ctx.close();
}
await b.close();
console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
process.exit(f?1:0);})();
