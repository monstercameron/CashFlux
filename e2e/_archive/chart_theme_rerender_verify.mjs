import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage(); page.setViewportSize({width:1280,height:1000});
const errs=[]; page.on("pageerror",e=>errs.push(String(e)));
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const lum=rgb=>{const m=rgb.match(/[\d.]+/g).map(Number);return 0.299*m[0]+0.587*m[1]+0.114*m[2]};
const buckets=()=>page.evaluate(()=>{const ts=[...document.querySelectorAll('.cf-chart text')];const b={};ts.forEach(t=>{const k=(t.closest('.x-axis,.y-axis')?'AXIS ':'DATA ')+getComputedStyle(t).fill;b[k]=(b[k]||0)+1});return {n:ts.length,b}});
const goReports=async()=>{await page.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Reports");if(l)l.click()});await page.waitForTimeout(2500)};
const dataLums=b=>Object.keys(b.b).filter(k=>k.startsWith('DATA ')).map(k=>Math.round(lum(k.replace('DATA ',''))));

// (1) Capture genuine token values from a fresh DARK load
await page.goto(BASE+"/",{waitUntil:"domcontentloaded"});
await page.evaluate(()=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:'dark'})));
await page.reload({waitUntil:"domcontentloaded"});
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:60000});
const darkTokens = await page.evaluate(()=>{const cs=getComputedStyle(document.documentElement);const g=n=>cs.getPropertyValue(n).trim();return {text:g('--text'),dim:g('--text-dim'),faint:g('--text-faint'),border:g('--border'),attr:document.documentElement.getAttribute('data-theme')}});
console.log("DARK tokens:", JSON.stringify(darkTokens));
await goReports();
const darkFresh = await buckets();
console.log("DARK fresh render:", JSON.stringify(darkFresh), "DATA lums:", dataLums(darkFresh));
if (dataLums(darkFresh).every(l=>l>150)) P("fresh DARK: chart data text is light (readable on dark)");
else F("fresh DARK data lums not light: "+dataLums(darkFresh));

// (2) Fresh LIGHT load
await page.evaluate(()=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:'light'})));
await page.reload({waitUntil:"domcontentloaded"});
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:60000});
await page.waitForFunction(()=>document.documentElement.getAttribute('data-theme')==='light',{timeout:8000}).catch(()=>{});
await goReports();
const lightFresh = await buckets();
console.log("LIGHT fresh render:", JSON.stringify(lightFresh), "DATA lums:", dataLums(lightFresh));
if (dataLums(lightFresh).every(l=>l<100)) P("fresh LIGHT: chart data text is dark (readable on white)");
else F("fresh LIGHT data lums not dark: "+dataLums(lightFresh));

// (3) MOUNTED live switch — mimic ApplyTheme EXACTLY (set inline tokens + data-theme together),
//     which is what internal/uistate/theme.go does. Charts stay mounted; observer must re-render.
await page.evaluate((dt)=>{
  const s=document.documentElement.style;
  s.setProperty('--text',dt.text); s.setProperty('--text-dim',dt.dim);
  s.setProperty('--text-faint',dt.faint); s.setProperty('--border',dt.border);
  document.documentElement.setAttribute('data-theme', dt.attr || 'dark');
}, darkTokens);
await page.waitForTimeout(800); // allow MutationObserver re-render
const afterSwitch = await buckets();
console.log("AFTER mounted LIGHT->DARK (ApplyTheme-mimic):", JSON.stringify(afterSwitch), "DATA lums:", dataLums(afterSwitch));
if (dataLums(afterSwitch).every(l=>l>150)) P("MOUNTED switch→dark: charts re-rendered to light text (observer works on real token state)");
else F("MOUNTED switch→dark data lums not light: "+dataLums(afterSwitch));
await page.screenshot({path:'e2e/screenshots/chart_mounted_switch_dark.png'});

if (errs.length===0) P("no JS errors"); else F("JS errors: "+errs.slice(0,3).join("; "));
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`);
await browser.close();
process.exit(fail>0?1:0);
