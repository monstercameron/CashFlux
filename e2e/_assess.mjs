import { createRequire } from "module"; import { fileURLToPath } from "url"; import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage(); page.setViewportSize({width:1280,height:1000});
const errs=[]; page.on("pageerror",e=>errs.push(String(e)));
await page.goto(BASE+"/",{waitUntil:"domcontentloaded"});
await page.waitForSelector('nav[aria-label="Main navigation"] a[title]',{timeout:60000});
const navTo = async (t)=>{await page.evaluate((t)=>{const l=Array.from(document.querySelectorAll('nav[aria-label="Main navigation"] a[title]')).find(x=>x.getAttribute("title")===t); if(l)l.click();},t); await page.waitForTimeout(1500);};
for (const scr of ["Dashboard","Reports","Goals","Budgets"]) {
  await navTo(scr); await page.waitForTimeout(900);
  await page.screenshot({path:`e2e/screenshots/assess_${scr}.png`, fullPage:true});
  console.log(scr, "shot");
}
console.log("ERRS:", errs.slice(0,5));
await browser.close();
