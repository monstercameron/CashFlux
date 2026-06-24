import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
(async()=>{const b=await chromium.launch();
const ctx=await b.newContext({viewport:{width:1280,height:1000}});const p=await ctx.newPage();
const errs=[];
p.on('console',m=>{if(m.type()==='error')errs.push(m.text().slice(0,80));});
p.on('pageerror',e=>errs.push('PAGEERROR '+e.message.slice(0,80)));
await p.goto(BASE,{waitUntil:'domcontentloaded'});
await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();
  history.pushState({},'','/reports');dispatchEvent(new PopStateEvent('popstate'));});
await p.waitForTimeout(1400);
// step the period forward many months via the next arrow to reach an empty future period
const nextSel='.topbar .rstep:last-child, .topbar button[aria-label*="ext" i], .rpill .rstep:last-child';
// find the forward arrow: a button near the date pill. Use the ">" step button.
for(let i=0;i<18;i++){
  const clicked=await p.evaluate(()=>{
    // the period nav: look for a button whose text is ">" or aria-label includes Next, near the month label
    const btns=[...document.querySelectorAll('.topbar button, .rstep, button')];
    const next=btns.find(b=>/^›|^>|next/i.test((b.getAttribute('aria-label')||'')+b.textContent.trim()));
    if(next){next.click();return true;}return false;
  });
  if(!clicked)break;
  await p.waitForTimeout(120);
}
await p.waitForTimeout(1200);
const m=await p.evaluate(()=>{
  const app=document.querySelector('#app');
  const txt=app?app.innerText:'';
  // signs of breakage
  const nan=(txt.match(/NaN|Infinity|undefined|\bnull\b/g)||[]).slice(0,5);
  // charts present? donut arcs / bar rects
  const svgs=document.querySelectorAll('#app svg').length;
  const arcs=[...document.querySelectorAll('#app svg path')].filter(pa=>/A/.test(pa.getAttribute('d')||'')).length;
  // is there an empty-state message?
  const emptyMsg=/no (spending|transactions|data|income)|nothing|no activity/i.test(txt);
  // period shown
  const period=(document.querySelector('.topbar .rpill, .topbar')?.textContent||'').match(/\b\w{3}\s?20\d\d\b/);
  // hero values
  const heroNet=document.querySelector('.hero-net')?.textContent||'';
  return {periodLabel:period?period[0]:'?', svgs, donutArcs:arcs, nan, emptyMsg, heroNet, len:txt.length};
});
console.log('=== Reports at far-future (empty) period ===');
console.log(JSON.stringify(m,null,1));
console.log('console/page errors:', errs.length, errs.slice(0,5));
await p.screenshot({path:'e2e/screenshots/reports_empty.png',fullPage:false});
await b.close();})();
