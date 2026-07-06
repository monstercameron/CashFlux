import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE="http://127.0.0.1:8099"; const browser=await chromium.launch({headless:true});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const run=async theme=>{
  const p=await browser.newPage(); p.setViewportSize({width:1280,height:1000});
  await p.goto(BASE+"/",{waitUntil:"domcontentloaded",timeout:20000});
  await p.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:"domcontentloaded"}); await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
  await p.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Dashboard");if(l)l.click();});
  await p.waitForTimeout(1300);
  const r=await p.evaluate(()=>{
    let rules=[]; for(const ss of document.styleSheets){try{for(const rule of ss.cssRules){if(rule.selectorText&&/::selection/.test(rule.selectorText))rules.push(rule.cssText.slice(0,90));}}catch(e){}}
    // resolve color-mix accent value
    const probe=document.createElement('span'); probe.style.background='color-mix(in srgb, var(--accent) 28%, transparent)'; document.body.appendChild(probe);
    const bg=getComputedStyle(probe).backgroundColor; probe.remove();
    return {ruleCount:rules.length, sample:rules[0]||null, mixResolves: bg && bg!=='rgba(0, 0, 0, 0)' && !/transparent/.test(bg), bg};
  });
  console.log(`[${theme}] ::selection rules=${r.ruleCount} mixBg=${r.bg} resolves=${r.mixResolves}`);
  if(r.ruleCount>=1) P(`${theme}: ::selection rule present`); else F(`${theme}: no ::selection rule`);
  if(r.mixResolves) P(`${theme}: accent color-mix resolves to a real color (${r.bg})`); else F(`${theme}: color-mix didn't resolve (${r.bg})`);
  // apply a real selection over the greeting and screenshot
  await p.evaluate(()=>{const g=document.querySelector('.home-hero-greeting')||document.querySelector('h2'); if(g){const rng=document.createRange();rng.selectNodeContents(g);const s=window.getSelection();s.removeAllRanges();s.addRange(rng);}});
  await p.waitForTimeout(200);
  await p.screenshot({path:`e2e/screenshots/selection_${theme}.png`, clip:{x:236,y:90,width:520,height:90}}).catch(async()=>{await p.screenshot({path:`e2e/screenshots/selection_${theme}.png`});});
  await p.close();
};
await run("dark"); await run("light"); await browser.close();
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`); process.exit(fail>0?1:0);
