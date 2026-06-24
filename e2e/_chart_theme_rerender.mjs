import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage(); page.setViewportSize({width:1280,height:1000});
const errs=[]; page.on("pageerror",e=>errs.push(String(e)));
let pass=0,fail=0; const P=(m)=>{console.log("PASS: "+m);pass++;}; const F=(m)=>{console.log("FAIL: "+m);fail++;};

const buckets = ()=>page.evaluate(()=>{const ts=Array.from(document.querySelectorAll('.cf-chart text'));const b={};ts.forEach(t=>{const k=(t.closest('.x-axis,.y-axis')?'AXIS ':'DATA ')+getComputedStyle(t).fill;b[k]=(b[k]||0)+1;});return {n:ts.length,b};});
const goReports = async()=>{await page.evaluate(()=>{const l=Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')).find(x=>x.getAttribute("title")==="Reports"); if(l)l.click();}); await page.waitForTimeout(2500);};

// ---- Case 1: render in LIGHT, switch to DARK (the case the CSS pin did NOT cover) ----
await page.goto(BASE+"/",{waitUntil:"domcontentloaded"});
await page.evaluate(()=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:'light'})));
await page.reload({waitUntil:"domcontentloaded"});
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:60000});
await page.waitForFunction(()=>document.documentElement.getAttribute('data-theme')==='light',{timeout:10000}).catch(()=>{});
await goReports();
const light = await buckets();
console.log("RENDERED LIGHT:", JSON.stringify(light));
// switch to dark WITHOUT reload
await page.evaluate(()=>document.documentElement.setAttribute('data-theme','dark'));
await page.waitForTimeout(700); // allow MutationObserver re-render
const afterDark = await buckets();
console.log("AFTER SWITCH->DARK:", JSON.stringify(afterDark));
// In dark, DATA text should be light (high lum ~244), NOT the baked light #1c1c1e (lum~28)
const lum=(rgb)=>{const m=rgb.match(/[\d.]+/g).map(Number);return 0.299*m[0]+0.587*m[1]+0.114*m[2];};
const dataKeys = Object.keys(afterDark.b).filter(k=>k.startsWith('DATA '));
const dataLums = dataKeys.map(k=>lum(k.replace('DATA ','')));
console.log("dark DATA lums:", dataLums.map(x=>Math.round(x)));
if (dataLums.length && dataLums.every(l=>l>150)) P("Case1 dark→ DATA text re-rendered to LIGHT fill (readable on dark)");
else F("Case1 DATA text still dark after switch to dark: lums="+dataLums.map(x=>Math.round(x)));
await page.screenshot({path:'e2e/screenshots/rerender_switch_to_dark.png'});

// ---- Case 2: render in DARK, switch to LIGHT ----
await page.evaluate(()=>document.documentElement.setAttribute('data-theme','light'));
await page.waitForTimeout(700);
const back = await buckets();
const dKeys = Object.keys(back.b).filter(k=>k.startsWith('DATA '));
const dLums = dKeys.map(k=>lum(k.replace('DATA ','')));
console.log("AFTER SWITCH->LIGHT:", JSON.stringify(back), "DATA lums:", dLums.map(x=>Math.round(x)));
if (dLums.length && dLums.every(l=>l<100)) P("Case2 light→ DATA text re-rendered to DARK fill (readable on white)");
else F("Case2 DATA text not dark in light: lums="+dLums.map(x=>Math.round(x)));

if (errs.length===0) P("no JS errors"); else F("JS errors: "+errs.slice(0,3).join("; "));
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`);
await browser.close();
process.exit(fail>0?1:0);
