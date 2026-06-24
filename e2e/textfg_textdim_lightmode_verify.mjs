import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage(); page.setViewportSize({width:1280,height:1000});
let pass=0,fail=0; const P=m=>{console.log("PASS: "+m);pass++}; const F=m=>{console.log("FAIL: "+m);fail++};
const lum=rgb=>{const m=rgb.match(/[\d.]+/g).map(Number);return 0.2126*m[0]+0.7152*m[1]+0.0722*m[2]};
const load=async theme=>{await page.goto(BASE+"/",{waitUntil:"domcontentloaded"});await page.evaluate(t=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:t})),theme);await page.reload({waitUntil:"domcontentloaded"});await page.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:60000});await page.waitForTimeout(800);};
const go=async t=>{await page.evaluate(t=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")===t);if(l)l.click();},t);await page.waitForTimeout(1400);};
const colorOf=txt=>page.evaluate(t=>{const s=[...document.querySelectorAll('span,a')].find(e=>e.textContent.trim()===t&&![...e.children].length);return s?getComputedStyle(s).color:null;},txt);
const crumb=()=>page.evaluate(()=>{const c=[...document.querySelectorAll('a,span')].find(e=>e.textContent.trim()==="Dashboard"&&![...e.children].length);return c?getComputedStyle(c).color:null;});

for (const theme of ["light","dark"]) {
  await load(theme);
  const dash=await crumb();
  await go("Insights");
  const nc=await colorOf("New chat"), ep=await colorOf("Edit prompt");
  console.log(`\n[${theme}] breadcrumb=${dash} | New chat=${nc} | Edit prompt=${ep}`);
  const ncL=nc?lum(nc):null, crumbL=dash?lum(dash):null;
  if (theme==="light") {
    if (ncL!==null && ncL<120) P("light: New chat is dark (readable on white)"); else F("light New chat lum="+ncL);
    if (crumbL!==null && crumbL<160) P("light: breadcrumb readable (lum "+Math.round(crumbL)+")"); else F("light breadcrumb lum="+Math.round(crumbL));
  } else {
    if (ncL!==null && ncL>180) P("dark: New chat still light (no regression, lum "+Math.round(ncL)+")"); else F("dark New chat lum="+Math.round(ncL));
    if (crumbL!==null && crumbL>140) P("dark: breadcrumb still light (no regression, lum "+Math.round(crumbL)+")"); else F("dark breadcrumb lum="+Math.round(crumbL));
  }
  await page.screenshot({path:`e2e/screenshots/textfix_insights_${theme}.png`});
}
console.log(`\nRESULT: ${pass} PASS / ${fail} FAIL`);
await browser.close(); process.exit(fail>0?1:0);
