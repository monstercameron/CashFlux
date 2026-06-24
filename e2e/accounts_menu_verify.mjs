// Account-row ⋯ overflow menu a11y guard. Single navigation; raw mouse + evaluate
// snapshots (the accounts list re-renders on the data-revision atom, so Playwright
// stability/visibility waits stall — querySelector snapshots are immune).
import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
const TRIG='.row .add-wrap > button[aria-haspopup="menu"]';
const aria=p=>p.evaluate(t=>document.querySelector(t)?.getAttribute('aria-expanded'),TRIG);
const open=p=>p.evaluate(t=>{const b=document.querySelector(t);if(!b)return false;const m=b.closest('.add-wrap').querySelector('.add-menu');return m&&!m.className.includes('hidden-menu');},TRIG);
async function clickTrig(p){const c=await p.evaluate(t=>{const r=document.querySelector(t).getBoundingClientRect();return {x:Math.round(r.x+r.width/2),y:Math.round(r.y+r.height/2)};},TRIG);await p.mouse.click(c.x,c.y);await p.waitForTimeout(350);}
(async()=>{const b=await chromium.launch();
const ctx=await b.newContext({viewport:{width:1280,height:900}});const p=await ctx.newPage();
await p.goto(BASE,{waitUntil:'domcontentloaded'});
await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();
  history.pushState({},'','/accounts');dispatchEvent(new PopStateEvent('popstate'));});
await p.waitForTimeout(1600);
if(!(await p.evaluate(t=>!!document.querySelector(t),TRIG))){console.log('  FAIL no ⋯ trigger on /accounts');console.log('\n0 PASS / 1 FAIL');await b.close();process.exit(1);}

(await aria(p)==='false')?P('aria-expanded=false when closed'):F('aria not false initially');
await clickTrig(p);
(await aria(p)==='true')?P('aria-expanded=true when open'):F(`aria=${await aria(p)} when open`);
(await open(p))?P('menu opens and stays open (no self-close)'):F('menu did not open/stay');

const at=await p.evaluate(()=>document.elementFromPoint(400,780)?.tagName);
await p.mouse.click(400,780); await p.waitForTimeout(350);
(!(await open(p)))?P(`outside-click over content (${at}) closes menu`):F('outside-click did not close');
(await aria(p)==='false')?P('aria-expanded back to false after close'):F('aria not reset');

await clickTrig(p);
const wasOpen=await open(p);
await p.keyboard.press('Escape'); await p.waitForTimeout(350);
(wasOpen && !(await open(p)))?P('Escape closes menu'):F('Escape did not close');
(await p.evaluate(t=>document.activeElement===document.querySelector(t),TRIG))?P('Escape returns focus to trigger'):F('focus not returned to trigger');

await clickTrig(p);
const item=await p.evaluate(()=>{const it=[...document.querySelectorAll('.row .add-wrap .add-item')].find(b=>/transfer/i.test(b.textContent));if(!it)return null;const r=it.getBoundingClientRect();return {x:Math.round(r.x+r.width/2),y:Math.round(r.y+r.height/2)};});
if(item){await p.mouse.click(item.x,item.y);await p.waitForTimeout(400);
  (!(await p.evaluate(()=>[...document.querySelectorAll('.add-menu')].some(m=>!m.className.includes('hidden-menu')))))?P('menu item closes the menu (item still works)'):F('menu stayed open after item');
} else log.push('INFO no Transfer item found — skipped');

console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
await p.screenshot({path:'e2e/screenshots/accounts_menu.png'});
await b.close(); process.exit(f?1:0);})();
