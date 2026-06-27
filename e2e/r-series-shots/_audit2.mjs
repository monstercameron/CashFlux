import { chromium } from 'playwright';
const base='http://127.0.0.1:8099/';
function L(r,g,b){const f=v=>{v/=255;return v<=0.03928?v/12.92:Math.pow((v+0.055)/1.055,2.4)};return 0.2126*f(r)+0.7152*f(g)+0.0722*f(b)}
function parse(c){const m=c&&c.match(/[\d.]+/g);return m?m.slice(0,3).map(Number):null}
const b=await chromium.launch();
for(const theme of ['dark','light']){
  const p=await b.newPage({viewport:{width:1440,height:1000}});
  await p.goto(base,{waitUntil:'domcontentloaded'});
  if(theme==='light'){await p.evaluate(()=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:'light'})));await p.reload({waitUntil:'domcontentloaded'});await p.waitForFunction(()=>document.documentElement.getAttribute('data-theme')==='light',{timeout:8000}).catch(()=>{});}
  await p.waitForSelector('.bento .w, .card',{timeout:20000}).catch(()=>{});
  await p.waitForTimeout(1800);
  const res=await p.evaluate(()=>{
    function vis(el){const r=el.getBoundingClientRect();const s=getComputedStyle(el);return r.width>3&&r.height>3&&r.top<1000&&r.top>=0&&s.visibility!=='hidden'&&s.opacity!=='0'}
    function bg(el){let e=el;while(e){const c=getComputedStyle(e).backgroundColor;if(c&&c!=='rgba(0, 0, 0, 0)'&&!/, 0\)$/.test(c))return c;e=e.parentElement;}return getComputedStyle(document.body).backgroundColor;}
    const out=[];const seen=new Set();
    document.querySelectorAll('.bento .w *, .card *').forEach(el=>{
      if(el.children.length>0)return; const t=(el.textContent||'').trim(); if(!t||t.length>50)return; if(!vis(el))return;
      const cs=getComputedStyle(el); const key=cs.color+'|'+(el.className||'')+'|'+t.slice(0,10); if(seen.has(key))return; seen.add(key);
      out.push({cls:(el.className||'').toString().slice(0,30),fg:cs.color,bg:bg(el),size:parseFloat(cs.fontSize),weight:cs.fontWeight,t:t.slice(0,24)});
    });return out;
  });
  let fails=[];
  for(const o of res){const f=parse(o.fg),g=parse(o.bg);if(!f||!g)continue;const cr=(Math.max(L(...f),L(...g))+0.05)/(Math.min(L(...f),L(...g))+0.05);const large=o.size>=24||(o.size>=18.66&&parseInt(o.weight)>=700);const need=large?3:4.5;if(cr<need-0.05)fails.push({...o,cr:cr.toFixed(2),need})}
  console.log(`\n[${theme}] ${fails.length} fails of ${res.length} sampled`);
  fails.slice(0,12).forEach(o=>console.log(`  ${o.cr}:1 (need ${o.need}) s${o.size} .${o.cls} "${o.t}"`));
  await p.close();
}
await b.close();
