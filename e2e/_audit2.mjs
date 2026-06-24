import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage(); page.setViewportSize({width:1280,height:1000});
const errs=[]; page.on("pageerror",e=>{const m=String(e); if(!m.includes("already exited"))errs.push(m);});
await page.goto(BASE+"/",{waitUntil:"domcontentloaded"});
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:60000});
const navTo = async (t)=>{const ok=await page.evaluate((t)=>{const l=[...document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')].find(x=>x.getAttribute("title")===t); if(l){l.click();return true} return false;},t); await page.waitForTimeout(1400); return ok;};
for (const scr of ["Planning","Allocate","Insights","Accounts","Subscriptions","Bills","To-do"]) {
  const ok = await navTo(scr);
  await page.waitForTimeout(700);
  await page.screenshot({path:`e2e/screenshots/audit_${scr.replace(/[^a-z]/gi,'')}.png`, fullPage:true});
  console.log(scr, ok?"shot":"NAV-MISS");
}
console.log("ERRS:", errs.slice(0,6));
await browser.close();
