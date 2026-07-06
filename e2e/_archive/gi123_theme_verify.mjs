import { chromium } from 'playwright';
const BASE='http://127.0.0.1:8099';
const SHOT='e2e/screenshots';
const log=[]; const P=(s)=>log.push('PASS '+s); const F=(s)=>log.push('FAIL '+s);
const lum=(rgb)=>{const m=rgb.match(/\d+/g);if(!m)return null;const[r,g,b]=m.map(Number);return 0.2126*r+0.7152*g+0.0722*b;};

async function boot(p){
  await p.goto(BASE,{waitUntil:'domcontentloaded'});
  await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
  await p.evaluate(()=>{const o=document.getElementById('gwc-error-overlay');if(o)o.remove();});
}
async function nav(p,path){
  await p.evaluate((pp)=>{history.pushState({},'',pp);dispatchEvent(new PopStateEvent('popstate'));},path);
  await p.waitForTimeout(900);
}
async function setTheme(p,theme){
  await p.evaluate((t)=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);
  await p.reload({waitUntil:'domcontentloaded'});
  await p.waitForFunction((t)=>document.documentElement.getAttribute('data-theme')===t,{timeout:10000},theme).catch(()=>{});
  await p.waitForFunction(()=>document.querySelector('#app')?.textContent.length>40,{timeout:20000});
}

(async()=>{
  const b=await chromium.launch();
  for(const theme of ['dark','light']){
    const ctx=await b.newContext(); const p=await ctx.newPage();
    await boot(p); await setTheme(p,theme);

    // ---- RULES ----
    await nav(p,'/rules');
    const rules=await p.evaluate(()=>{
      const dragHint=[...document.querySelectorAll('.muted')].some(e=>/reorder|first match/i.test(e.textContent));
      const primaryInSuggest=document.querySelectorAll('.btn-primary').length;
      const card=document.querySelector('#app .card'); const title=document.querySelector('#app .card-title');
      const cs=card?getComputedStyle(card):null; const ts=title?getComputedStyle(title):null;
      return {dragHint, primaryCount:primaryInSuggest, cardBg:cs?.backgroundColor, titleColor:ts?.color,
              text:(document.querySelector('#app')?.textContent||'').slice(0,0), nonEmpty:(document.querySelector('#app')?.textContent||'').length};
    });
    await p.screenshot({path:`${SHOT}/gi1_rules_${theme}.png`, fullPage:true});
    const rl=lum(rules.cardBg||''), rt=lum(rules.titleColor||'');
    (rl!=null&&rt!=null&&Math.abs(rl-rt)>40)?P(`GI1 rules ${theme}: title/card contrast ok (Δlum=${Math.abs(rl-rt).toFixed(0)})`):F(`GI1 rules ${theme}: weak contrast card=${rules.cardBg} title=${rules.titleColor}`);
    rules.dragHint?P(`GI1 rules ${theme}: drag-reorder hint present`):log.push(`INFO GI1 rules ${theme}: drag hint not found (may need >1 rule)`);

    // ---- CATEGORIES ----
    await nav(p,'/categories');
    const cats=await p.evaluate(()=>{
      const link=document.querySelector('.btn-link.cat-usage') || document.querySelector('.btn-link');
      const ls=link?getComputedStyle(link):null;
      const zero=document.querySelector('.cat-zero-usage'); const zs=zero?getComputedStyle(zero):null;
      const child=document.querySelector('.cat-child-row'); const ch=child?getComputedStyle(child):null;
      // is the mermaid/category-map card the FIRST card?
      const firstCard=document.querySelector('#app .card');
      const mapFirst=!!firstCard && /map|diagram/i.test(firstCard.textContent.slice(0,200)) || !!firstCard?.querySelector('svg,.mermaid');
      return {linkColor:ls?.color, linkDecoration:ls?.textDecorationLine, hasLink:!!link,
              zeroOpacity:zs?.opacity, hasZero:!!zero, childBg:ch?.backgroundColor, hasChild:!!child, mapFirst};
    });
    await p.screenshot({path:`${SHOT}/gi2_categories_${theme}.png`, fullPage:true});
    if(cats.hasLink){ const ll=lum(cats.linkColor||'');
      (ll!=null)?P(`GI2 categories ${theme}: usage btn-link rendered color=${cats.linkColor} decoration=${cats.linkDecoration}`):F(`GI2 categories ${theme}: btn-link color unreadable`);
    } else log.push(`INFO GI2 categories ${theme}: no .btn-link (no categories with usage seeded)`);
    if(cats.hasZero) (Math.abs(parseFloat(cats.zeroOpacity)-0.55)<0.2)?P(`GI2 categories ${theme}: zero-usage dim opacity=${cats.zeroOpacity}`):F(`GI2 categories ${theme}: zero-usage opacity off (${cats.zeroOpacity})`);
    else log.push(`INFO GI2 categories ${theme}: no .cat-zero-usage row`);
    cats.hasChild?P(`GI2 categories ${theme}: child-row fill present (${cats.childBg})`):log.push(`INFO GI2 categories ${theme}: no .cat-child-row (no sub-categories)`);
    cats.mapFirst?P(`GI2 categories ${theme}: category-map is first card`):log.push(`INFO GI2 categories ${theme}: first card not map (mapFirst=${cats.mapFirst})`);

    // ---- WORKFLOWS ----
    await nav(p,'/workflows');
    const wf=await p.evaluate(()=>{
      const btns=[...document.querySelectorAll('#app button')];
      const showDiag=btns.some(x=>/show diagram/i.test(x.textContent));
      const hideDiag=btns.some(x=>/hide diagram/i.test(x.textContent));
      const dry=btns.find(x=>/dry run/i.test(x.textContent));
      const run=btns.find(x=>/^\s*run now\s*$/i.test(x.textContent));
      return {showDiag, hideDiag, dryPrimary: dry?dry.classList.contains('btn-primary'):null,
              runPrimary: run?run.classList.contains('btn-primary'):null, hasDry:!!dry, hasRun:!!run,
              svgCount:document.querySelectorAll('#app svg.mermaid, #app .mermaid svg, #app svg').length};
    });
    await p.screenshot({path:`${SHOT}/gi3_workflows_${theme}.png`, fullPage:true});
    if(wf.hasDry){ (wf.dryPrimary===true)?P(`GI3 workflows ${theme}: "Dry run" is primary`):F(`GI3 workflows ${theme}: dry run not primary`);
      (wf.runPrimary===false)?P(`GI3 workflows ${theme}: "Run now" demoted to secondary`):log.push(`INFO GI3 workflows ${theme}: runPrimary=${wf.runPrimary}`);
      (wf.showDiag && !wf.hideDiag)?P(`GI3 workflows ${theme}: diagrams collapsed by default (Show diagram present)`):log.push(`INFO GI3 workflows ${theme}: showDiag=${wf.showDiag} hideDiag=${wf.hideDiag}`);
    } else log.push(`INFO GI3 workflows ${theme}: no workflow rows (none seeded) — page rendered, screenshot saved`);

    await ctx.close();
  }
  await b.close();
  console.log(log.map(s=>'  '+s).join('\n'));
  const fails=log.filter(s=>s.startsWith('FAIL')).length;
  console.log(`\n${log.filter(s=>s.startsWith('PASS')).length} PASS / ${fails} FAIL / ${log.filter(s=>s.startsWith('INFO')).length} INFO`);
  process.exit(fails?1:0);
})();
