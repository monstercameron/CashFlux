// Verify the custom-page-row ⋯ menu a11y after creating a page. Raw mouse +
// querySelector snapshots (rail re-renders). Menu trigger lives in the rail.
import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
const TRIG='aside.rail button[aria-haspopup="menu"]';
const aria=p=>p.evaluate(t=>document.querySelector(t)?.getAttribute('aria-expanded'),TRIG);
const open=p=>p.evaluate(t=>{const b=document.querySelector(t);if(!b)return false;const m=b.parentElement.querySelector('[role="menu"]');return !!m;},TRIG);
async function clickTrig(p){const c=await p.evaluate(t=>{const r=document.querySelector(t).getBoundingClientRect();return {x:Math.round(r.x+r.width/2),y:Math.round(r.y+r.height/2)};},TRIG);await p.mouse.click(c.x,c.y);await p.waitForTimeout(350);}
(async()=>{const b=await chromium.launch();
const ctx=await b.newContext({viewport:{width:1280,height:900}});const p=await ctx.newPage();
await p.goto(BASE,{waitUntil:'domcontentloaded'});
await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();});
await p.waitForTimeout(600);
// 1) click "New page"
const npClicked=await p.evaluate(()=>{const a=[...document.querySelectorAll('aside.rail a,aside.rail button')].find(e=>/new page/i.test(e.textContent));if(a){a.setAttribute('data-np','1');return true;}return false;});
if(!npClicked){console.log('  FAIL no "New page" action found');console.log('\n0 PASS / 1 FAIL');await b.close();process.exit(1);}
await p.click('[data-np]'); await p.waitForTimeout(400);
// 2) prompt dialog -> fill + confirm
const inp=await p.$('#cf-dialog-input, .cf-dialog input');
if(inp){await inp.fill('Test Page Z'); await p.keyboard.press('Enter'); await p.waitForTimeout(800);}
else {console.log('  INFO no prompt dialog input — page may have been created directly');}
// 3) find the rail ⋯ trigger
const have=await p.evaluate(t=>!!document.querySelector(t),TRIG);
if(!have){console.log('  FAIL no rail ⋯ menu trigger after creating page');console.log('\n0 PASS / 1 FAIL');await p.screenshot({path:'e2e/screenshots/custompage_fail.png'});await b.close();process.exit(1);}
(await aria(p)==='false')?P('aria-expanded=false when closed'):F(`aria=${await aria(p)} initially`);
await clickTrig(p);
(await aria(p)==='true')?P('aria-expanded=true when open'):F(`aria=${await aria(p)} when open`);
(await open(p))?P('menu opens and stays open'):F('menu did not open/stay');
// outside-click over content
const at=await p.evaluate(()=>document.elementFromPoint(700,500)?.tagName);
await p.mouse.click(700,500); await p.waitForTimeout(350);
(!(await open(p)))?P(`outside-click over content (${at}) closes menu`):F('outside-click did not close');
(await aria(p)==='false')?P('aria-expanded reset to false'):F('aria not reset');
// Escape
await clickTrig(p);
const was=await open(p);
await p.keyboard.press('Escape'); await p.waitForTimeout(350);
(was && !(await open(p)))?P('Escape closes menu'):F('Escape did not close');
(await p.evaluate(t=>document.activeElement===document.querySelector(t),TRIG))?P('Escape returns focus to trigger'):F('focus not returned');
console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
await p.screenshot({path:'e2e/screenshots/custompage_menu.png'});
await b.close(); process.exit(f?1:0);})();
