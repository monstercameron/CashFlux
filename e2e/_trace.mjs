import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage(); page.setViewportSize({width:1280,height:1000});
await page.goto(BASE+"/",{waitUntil:"domcontentloaded"});
await page.evaluate(()=>localStorage.setItem('cashflux:prefs',JSON.stringify({theme:'light'})));
await page.reload({waitUntil:"domcontentloaded"});
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:60000});
await page.evaluate(()=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")==="Insights"); if(l)l.click();});
await page.waitForTimeout(1500);
const info = await page.evaluate(()=>{
  const span=[...document.querySelectorAll('span')].find(s=>s.textContent.trim()==="New chat" && ![...s.children].length);
  if(!span) return "not found";
  const btn=span.closest('button');
  const out={};
  out.spanColor=getComputedStyle(span).color;
  out.btnColor=getComputedStyle(btn).color;
  out.btnClass=btn.className;
  out.rootText=getComputedStyle(document.documentElement).getPropertyValue('--text').trim();
  out.dataTheme=document.documentElement.getAttribute('data-theme');
  // inline style on button?
  out.btnInline=btn.getAttribute('style');
  out.btnStyleColor=btn.style.color;
  // walk up looking for a color setter
  let e=span, chain=[];
  while(e&&e!==document.body){const cs=getComputedStyle(e); chain.push(e.tagName+'.'+(typeof e.className==='string'?e.className.split(' ').slice(0,3).join('.'):'')+' color='+cs.color+(e.style.color?' INLINE='+e.style.color:'')); e=e.parentElement;}
  out.chain=chain.slice(0,6);
  return out;
});
console.log(JSON.stringify(info,null,2));
await browser.close();
