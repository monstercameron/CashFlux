// +Add menu dismissal regression guard. Each check reloads to isolate state.
//  • opens and stays open (the document pointerdown listener must not self-close)
//  • outside-click OVER PAGE CONTENT closes it (the previously-broken case)
//  • Escape closes it (kept from prior fix)
//  • a menu item still closes the menu and opens its add panel
import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
const isOpen=p=>p.evaluate(()=>!document.querySelector('.add-menu').className.includes('hidden-menu'));
async function fresh(p){await p.goto(BASE,{waitUntil:'domcontentloaded'});
  await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
  await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();});await p.waitForTimeout(400);}
(async()=>{const b=await chromium.launch();
const ctx=await b.newContext({viewport:{width:1280,height:900}});const p=await ctx.newPage();

await fresh(p);
await p.click('.add-btn'); await p.waitForTimeout(250);
(await isOpen(p))?P('opens and stays open (no self-close)'):F('self-closed on open');
const at=await p.evaluate(()=>document.elementFromPoint(800,600)?.tagName);
await p.mouse.click(800,600); await p.waitForTimeout(250);
(!(await isOpen(p)))?P(`outside-click over content (${at}) closes menu`):F('outside-click over content did NOT close');

await fresh(p);
await p.click('.add-btn'); await p.waitForTimeout(250);
const op=await isOpen(p);
await p.keyboard.press('Escape'); await p.waitForTimeout(250);
(op && !(await isOpen(p)))?P('Escape still closes'):F('Escape regressed');

await fresh(p);
await p.click('.add-btn'); await p.waitForTimeout(250);
await p.locator('.add-item',{hasText:/transaction/i}).first().click(); await p.waitForTimeout(400);
const it=await p.evaluate(()=>({closed:document.querySelector('.add-menu').className.includes('hidden-menu'),modal:!!document.querySelector('.flip-inner,.cf-dialog,.set-card,[role=dialog]')}));
(it.closed&&it.modal)?P('menu item closes menu + opens add panel'):F(`item flow ${JSON.stringify(it)}`);

console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
await b.close(); process.exit(f?1:0);})();
