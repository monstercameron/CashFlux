// Guards the two drag affordances that the WONDER entrance animations can clobber:
//  • .w.drag dashboard tile drag-ghost (opacity .35) vs wonder-bento-enter fill
//  • .row[draggable]:active rule-row drag-grab (opacity .85) vs wonder-row-enter fill
// Both are functional cues (fixed with !important). See TODOS W-3 / W-4.
import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
async function boot(p){await p.goto(BASE,{waitUntil:'domcontentloaded'});
  await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
  await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();});}
async function nav(p,path){await p.evaluate(pp=>{history.pushState({},'',pp);dispatchEvent(new PopStateEvent('popstate'));},path);await p.waitForTimeout(900);}
(async()=>{const b=await chromium.launch();const ctx=await b.newContext();const p=await ctx.newPage();
await boot(p); await p.waitForSelector('.bento .w',{timeout:30000}); await p.waitForTimeout(700);
const tile=await p.evaluate(()=>{const t=document.querySelector('.bento .w');t.classList.add('drag');const o=getComputedStyle(t).opacity;t.classList.remove('drag');return o;});
(Math.abs(parseFloat(tile)-0.35)<0.05)?P(`tile drag-ghost opacity=${tile} (=.35)`):F(`tile drag-ghost opacity=${tile} (expected .35)`);
await nav(p,'/rules');
const rowEx=await p.evaluate(()=>!!document.querySelector('.row[draggable="true"]'));
if(rowEx){const client=await ctx.newCDPSession(p);const doc=await client.send('DOM.getDocument');
  const node=await client.send('DOM.querySelector',{nodeId:doc.root.nodeId,selector:'.row[draggable="true"]'});
  await client.send('CSS.enable');await client.send('CSS.forcePseudoState',{nodeId:node.nodeId,forcedPseudoClasses:['active']});
  await p.waitForTimeout(120);
  const o=await p.evaluate(()=>getComputedStyle(document.querySelector('.row[draggable="true"]')).opacity);
  (Math.abs(parseFloat(o)-0.85)<0.05)?P(`row drag-grab opacity=${o} (=.85)`):F(`row drag-grab opacity=${o} (expected .85)`);
} else log.push('INFO no draggable rule row (none seeded)');
await b.close();
console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
process.exit(f?1:0);})();
