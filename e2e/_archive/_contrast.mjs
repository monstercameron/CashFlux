import { chromium } from 'playwright';
const base='http://127.0.0.1:8099/';
function L(r,g,b){const f=v=>{v/=255;return v<=0.03928?v/12.92:Math.pow((v+0.055)/1.055,2.4)};return 0.2126*f(r)+0.7152*f(g)+0.0722*f(b)}
function parse(c){const m=c.match(/[\d.]+/g);return m?m.slice(0,3).map(Number):null}
const b=await chromium.launch();
for(const theme of ['dark','light']){
  const p=await b.newPage({viewport:{width:1440,height:1000}});
  await p.goto(base,{waitUntil:'domcontentloaded'});
  await p.waitForSelector('.card,.bento,main',{timeout:20000}).catch(()=>{});
  if(theme==='light'){ await p.evaluate(()=>{document.documentElement.setAttribute('data-theme','light');}); await p.waitForTimeout(400); }
  await p.waitForTimeout(800);
  const res=await p.evaluate(()=>{
    // walk visible text elements, sample fg vs effective bg
    function bg(el){let e=el;while(e){const c=getComputedStyle(e).backgroundColor;if(c&&c!=='rgba(0, 0, 0, 0)'&&c!=='transparent')return c;e=e.parentElement;}return getComputedStyle(document.body).backgroundColor;}
    const out=[];const seen=new Set();
    document.querySelectorAll('.card-title,.stat-label,.stat-value,.row-meta,.budget-amount,.kpi-value,.t-caption,.muted,.text-dim,.text-faint,a,button,.nav-link,.chip,.pill').forEach(el=>{
      const t=(el.textContent||'').trim();if(!t||t.length>40)return;const r=el.getBoundingClientRect();if(r.width<4||r.height<4||r.top>1000)return;
      const cs=getComputedStyle(el);const fg=cs.color;const b=bg(el);const key=fg+'|'+b+'|'+el.className;if(seen.has(key))return;seen.add(key);
      out.push({cls:(el.className||'').toString().slice(0,40),fg,bg:b,size:parseFloat(cs.fontSize),weight:cs.fontWeight,sample:t.slice(0,24)});
    });return out;
  });
  let fails=0;
  for(const o of res){const f=parse(o.fg),g=parse(o.bg);if(!f||!g)continue;const cr=(Math.max(L(...f),L(...g))+0.05)/(Math.min(L(...f),L(...g))+0.05);const large=o.size>=24||(o.size>=18.66&&parseInt(o.weight)>=700);const need=large?3:4.5;if(cr<need-0.05){fails++;console.log(`[${theme}] FAIL ${cr.toFixed(2)}:1 (need ${need}) size${o.size} "${o.sample}" .${o.cls}`)}}
  console.log(`[${theme}] ${fails} contrast fails of ${res.length} sampled`);
  await p.close();
}
await b.close();
