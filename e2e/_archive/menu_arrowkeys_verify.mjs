// Verify WAI-ARIA arrow-key roving focus on the accounts ⋯ menu (via DismissPopover),
// plus Escape still closes (regression after the keyCb restructure).
import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const log=[];const P=s=>log.push('PASS '+s);const F=s=>log.push('FAIL '+s);
const TRIG='.row .add-wrap > button[aria-haspopup="menu"]';
// describe the active element: its menuitem index within its wrap, or 'trigger'/'other'
const activeInfo=p=>p.evaluate(t=>{const a=document.activeElement;if(!a)return 'none';
  if(a===document.querySelector(t))return 'trigger';
  if(a.getAttribute('role')==='menuitem'){const items=[...a.closest('.add-wrap').querySelectorAll('[role="menuitem"]')];return 'item#'+items.indexOf(a)+':'+a.textContent.trim().slice(0,14);}
  return 'other:'+a.tagName;},TRIG);
const isOpen=p=>p.evaluate(t=>{const m=document.querySelector(t).closest('.add-wrap').querySelector('.add-menu');return m&&!m.className.includes('hidden-menu');},TRIG);
(async()=>{const b=await chromium.launch();
const ctx=await b.newContext({viewport:{width:1280,height:900}});const p=await ctx.newPage();
await p.goto(BASE,{waitUntil:'domcontentloaded'});
await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();
  history.pushState({},'','/accounts');dispatchEvent(new PopStateEvent('popstate'));});
await p.waitForTimeout(1600);
const c=await p.evaluate(t=>{const r=document.querySelector(t).getBoundingClientRect();return {x:Math.round(r.x+r.width/2),y:Math.round(r.y+r.height/2)};},TRIG);
await p.mouse.click(c.x,c.y); await p.waitForTimeout(300);
const nItems=await p.evaluate(t=>document.querySelector(t).closest('.add-wrap').querySelectorAll('[role="menuitem"]').length,TRIG);
log.push('INFO opened; menuitems='+nItems+', active='+await activeInfo(p));
await p.keyboard.press('ArrowDown'); await p.waitForTimeout(120);
(await activeInfo(p)==='item#0'+(await p.evaluate(()=>'')) || (await activeInfo(p)).startsWith('item#0'))?P('ArrowDown from trigger focuses first item ('+await activeInfo(p)+')'):F('ArrowDown→first failed: '+await activeInfo(p));
await p.keyboard.press('ArrowDown'); await p.waitForTimeout(120);
((await activeInfo(p)).startsWith('item#1'))?P('ArrowDown moves to item#1 ('+await activeInfo(p)+')'):F('ArrowDown→item#1 failed: '+await activeInfo(p));
await p.keyboard.press('ArrowUp'); await p.waitForTimeout(120);
((await activeInfo(p)).startsWith('item#0'))?P('ArrowUp moves back to item#0'):F('ArrowUp failed: '+await activeInfo(p));
await p.keyboard.press('End'); await p.waitForTimeout(120);
((await activeInfo(p)).startsWith('item#'+(nItems-1)))?P('End jumps to last item ('+await activeInfo(p)+')'):F('End failed: '+await activeInfo(p));
await p.keyboard.press('Home'); await p.waitForTimeout(120);
((await activeInfo(p)).startsWith('item#0'))?P('Home jumps to first item'):F('Home failed: '+await activeInfo(p));
await p.keyboard.press('ArrowUp'); await p.waitForTimeout(120);
((await activeInfo(p)).startsWith('item#'+(nItems-1)))?P('ArrowUp from first wraps to last'):F('ArrowUp wrap failed: '+await activeInfo(p));
// Escape still closes + refocus
await p.keyboard.press('Escape'); await p.waitForTimeout(250);
(!(await isOpen(p)))?P('Escape still closes (regression check)'):F('Escape regressed');
(await activeInfo(p)==='trigger')?P('Escape returns focus to trigger'):F('focus not on trigger: '+await activeInfo(p));
console.log(log.map(s=>'  '+s).join('\n'));
const f=log.filter(s=>s.startsWith('FAIL')).length;
console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${f} FAIL`);
await b.close(); process.exit(f?1:0);})();
