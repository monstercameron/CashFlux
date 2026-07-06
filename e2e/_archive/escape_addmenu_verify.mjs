// Regression guard: the +Add menu must close on Escape (WAI-ARIA menu button),
// return focus to the button, still reopen + position correctly, and still close
// via a backdrop click. (Outside-click over page CONTENT is a separate, pre-existing
// backdrop-stacking gap — tracked in TODOS, not asserted here.)
import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
(async()=>{const b=await chromium.launch();
const ctx=await b.newContext({viewport:{width:1280,height:900}});const p=await ctx.newPage();
await p.goto(BASE,{waitUntil:'domcontentloaded'});
await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();});await p.waitForTimeout(500);
await p.click('.add-btn'); await p.waitForTimeout(300);
(await p.evaluate(()=>document.querySelector('.add-btn').getAttribute('aria-expanded'))==='true')?P('menu opens'):F('menu did not open');
await p.keyboard.press('Escape'); await p.waitForTimeout(300);
const a=await p.evaluate(()=>({aria:document.querySelector('.add-btn').getAttribute('aria-expanded'),hidden:document.querySelector('.add-menu').className.includes('hidden-menu'),focusOnBtn:document.activeElement===document.querySelector('.add-btn')}));
(a.aria==='false'&&a.hidden)?P('Escape closes menu'):F(`Escape did not close: ${JSON.stringify(a)}`);
(a.focusOnBtn)?P('focus returned to +Add button'):F('focus not returned to button');
await p.click('.add-btn'); await p.waitForTimeout(250);
const r=await p.evaluate(()=>{const items=[...document.querySelectorAll('.add-item')];const railR=Math.round(document.querySelector('aside.rail').getBoundingClientRect().right);return {reopened:!document.querySelector('.add-menu').className.includes('hidden-menu'),minLeft:Math.min(...items.map(i=>Math.round(i.getBoundingClientRect().left))),maxRight:Math.max(...items.map(i=>Math.round(i.getBoundingClientRect().right))),railR,vw:document.documentElement.clientWidth};});
(r.reopened&&r.minLeft>=r.railR&&r.maxRight<=r.vw)?P(`reopens + positioned OK [${r.minLeft}..${r.maxRight}]`):F(`reopen/pos ${JSON.stringify(r)}`);
await p.locator('.add-backdrop').click({force:true}); await p.waitForTimeout(250);
(await p.evaluate(()=>document.querySelector('.add-menu').className.includes('hidden-menu')))?P('backdrop click closes menu'):F('backdrop click did not close');
console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
await b.close(); process.exit(f?1:0);})();
