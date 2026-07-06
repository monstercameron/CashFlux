import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
async function boot(p,theme){await p.goto(BASE,{waitUntil:'domcontentloaded'});
  await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:'domcontentloaded'});
  await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
  await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();
    history.pushState({},'','/reports');dispatchEvent(new PopStateEvent('popstate'));});
  await p.waitForTimeout(1800);}
(async()=>{const b=await chromium.launch();
for(const theme of ['dark','light']){
const ctx=await b.newContext({viewport:{width:1280,height:1000}});const p=await ctx.newPage();
await boot(p,theme);
const m=await p.evaluate(()=>{
  // a donut SVG = has >1 arc path (d containing 'A') AND no axis (no rect bars). Find SVGs whose paths are arcs.
  const svgs=[...document.querySelectorAll('#app svg')];
  let donuts=[];
  for(const s of svgs){
    const arcs=[...s.querySelectorAll('path')].filter(pa=>/A/.test(pa.getAttribute('d')||''));
    const texts=[...s.querySelectorAll('text')].map(t=>t.textContent.trim()).filter(Boolean);
    const pcts=texts.filter(t=>/^\d+%$/.test(t));
    const labels=texts.filter(t=>/^[A-Za-z]/.test(t));
    if(arcs.length>=2){donuts.push({arcs:arcs.length, totalTexts:texts.length, pcts:pcts.length, sampleLabels:labels.slice(0,4), samplePcts:pcts.slice(0,4)});}
  }
  return {donutCount:donuts.length, donuts:donuts.slice(0,3)};
});
const withLegend=m.donuts.filter(d=>d.pcts>0 && d.sampleLabels.length>0);
(withLegend.length>0)?P(`${theme}: donut(s) now have a legend — ${JSON.stringify(withLegend[0])}`):F(`${theme}: no donut legend found (${JSON.stringify(m)})`);
await p.evaluate(()=>{const c=[...document.querySelectorAll('#app *')].filter(e=>{const cs=getComputedStyle(e);return /(auto|scroll)/.test(cs.overflowY)&&e.scrollHeight>e.clientHeight+50;}).sort((a,b)=>b.scrollHeight-a.scrollHeight)[0];if(c)c.scrollTop=1300;});
await p.waitForTimeout(500);
await p.screenshot({path:`e2e/screenshots/donut_legend_${theme}.png`});
await ctx.close();
}
await b.close();
console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
process.exit(f?1:0);})();
