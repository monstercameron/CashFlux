import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
(async()=>{const b=await chromium.launch();
const p=await(await b.newContext()).newPage();
const errs=[];
p.on('console',m=>{errs.push('['+m.type()+'] '+m.text().slice(0,120));});
p.on('pageerror',e=>errs.push('PAGEERROR '+e.message.slice(0,120)));
await p.goto(BASE,{waitUntil:'domcontentloaded'});
await p.waitForTimeout(6000);
const s=await p.evaluate(()=>{
  const app=document.querySelector('#app');
  return {appExists:!!app, appLen:(app?.textContent||'').length, appHTML:(app?.innerHTML||'').slice(0,200),
    bootVisible:!!document.querySelector('#boot:not(.hidden)'), overlay:!!document.getElementById('gwc-error-overlay'),
    overlayText:(document.getElementById('gwc-error-overlay')?.textContent||'').slice(0,200), title:document.title};
});
console.log('boot state:', JSON.stringify(s,null,1));
console.log('console msgs ('+errs.length+'):'); errs.slice(0,15).forEach(e=>console.log('  '+e));
await b.close();})();
