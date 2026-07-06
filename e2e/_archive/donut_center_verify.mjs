import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
async function boot(p,theme){await p.goto(BASE,{waitUntil:'domcontentloaded'});
  await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:'domcontentloaded'});
  await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>200,{timeout:30000});
  await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();
    history.pushState({},'','/reports');dispatchEvent(new PopStateEvent('popstate'));});
  await p.waitForTimeout(1800);}
(async()=>{const b=await chromium.launch();
for(const theme of ['dark','light']){
const ctx=await b.newContext({viewport:{width:1280,height:1000}});const p=await ctx.newPage();
await boot(p,theme);
const m=await p.evaluate(()=>{
  const svgs=[...document.querySelectorAll('#app svg')];
  let donut=null;
  for(const s of svgs){const arcs=[...s.querySelectorAll('path')].filter(pa=>/A/.test(pa.getAttribute('d')||''));
    if(arcs.length>=2){const texts=[...s.querySelectorAll('text')].map(t=>t.textContent.trim());
      donut={arcs:arcs.length,
        centerTotal:texts.find(t=>/^[$€£¥][\d.]+[kKmM]?$/.test(t)||/^[$€£¥]\d/.test(t)),
        hasTotalCaption:texts.includes('total'),
        pcts:texts.filter(t=>/^\d+%$/.test(t)).length,
        labels:texts.filter(t=>/^[A-Za-z]/.test(t)&&t!=='total').slice(0,3)};
      break;}}
  return donut;
});
if(!m){F(`${theme}: no donut found`);}
else{
  (m.centerTotal)?P(`${theme}: donut center shows total = "${m.centerTotal}"`):F(`${theme}: no center total found`);
  (m.hasTotalCaption)?P(`${theme}: center "total" caption present`):log.push(`INFO ${theme}: no caption (small donut?)`);
  (m.pcts>0 && m.labels.length>0)?P(`${theme}: legend still intact (${m.pcts} pcts, labels ${JSON.stringify(m.labels)})`):F(`${theme}: legend broke`);
}
await p.evaluate(()=>{const svgs=[...document.querySelectorAll('#app svg')];for(const s of svgs){const arcs=[...s.querySelectorAll('path')].filter(pa=>/A/.test(pa.getAttribute('d')||''));if(arcs.length>=2){s.scrollIntoView({block:'center'});break;}}});
await p.waitForTimeout(500);
await p.screenshot({path:`e2e/screenshots/donut_center_${theme}.png`});
await ctx.close();
}
await b.close();
console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
process.exit(f?1:0);})();
