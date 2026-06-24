import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const b = await chromium.launch({headless:true}); const p = await b.newPage(); p.setViewportSize({width:1280,height:1100});
const errs=[]; p.on("pageerror",e=>{const m=String(e);if(!m.includes("already exited"))errs.push(m.slice(0,80));});
await p.goto("http://127.0.0.1:8099/",{waitUntil:"domcontentloaded",timeout:20000});
await p.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:30000});
const nav=async t=>{await p.evaluate(t=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")===t);if(l)l.click();},t);await p.waitForTimeout(1300);};
for(const scr of ["Notifications","Allocate","Insights","To-do"]){
  await nav(scr); await p.waitForTimeout(700);
  const leak=await p.evaluate(()=>{
    const root=document.querySelector('#cf-page-view')||document.body; const it=root.innerText||'';
    return {ruleLeak:/\{\[\{|border-top-width|map\[|0x[0-9a-f]{6}|\bundefined\b|\bNaN\b|%!\w/.test(it)?(it.match(/\{\[\{[^}]*\}|border-top-width[^,;]*|map\[[^\]]*\]|\bundefined\b|\bNaN\b|%!\w\([^)]*\)/)||[])[0]:null,
      doublePct:/%%/.test(it)?(it.match(/\S*%%\S*/)||[])[0]:null,
      emptyVal:/\$\s*$|:\s*$| -- |—\s*—/.test(it)?null:null};
  });
  console.log(`[${scr}] leak=${JSON.stringify(leak.ruleLeak)} doublePct=${JSON.stringify(leak.doublePct)}`);
  await p.screenshot({path:`e2e/screenshots/glam2_${scr.replace(/[^a-z]/gi,'')}.png`, fullPage:true});
}
console.log("errs:", errs.length, errs.slice(0,3));
await b.close();
