import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
(async()=>{const b=await chromium.launch();
for(const theme of ['dark','light']){
const ctx=await b.newContext({viewport:{width:1280,height:900}});const p=await ctx.newPage();
await p.goto(BASE,{waitUntil:'domcontentloaded'});
await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
await p.reload({waitUntil:'domcontentloaded'});
await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();
  history.pushState({},'','/reports');dispatchEvent(new PopStateEvent('popstate'));});
await p.waitForTimeout(1600);
// find the Money flow card's SVG text nodes
const labels=await p.evaluate(()=>{
  const cards=[...document.querySelectorAll('#app .card, #app section')];
  const flow=cards.find(c=>/Money flow/i.test(c.textContent));
  if(!flow) return {found:false};
  const texts=[...flow.querySelectorAll('svg text')].map(t=>t.textContent.trim()).filter(Boolean);
  return {found:true, texts};
});
// scroll to it + screenshot
await p.evaluate(()=>{const cards=[...document.querySelectorAll('#app .card, #app section')];
  const flow=cards.find(c=>/Money flow/i.test(c.textContent)); if(flow) flow.scrollIntoView();});
await p.waitForTimeout(500);
await p.screenshot({path:`e2e/screenshots/sankey_${theme}.png`});
const raw=labels.texts? labels.texts.filter(t=>/\d{5,}/.test(t)) : [];
const dollar=labels.texts? labels.texts.filter(t=>/\$\d/.test(t)) : [];
console.log(theme, JSON.stringify({found:labels.found, sample:(labels.texts||[]).slice(0,6), rawBigNums:raw, dollarLabels:dollar.slice(0,6)}));
await ctx.close();
}
await b.close();})();
