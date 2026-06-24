import { chromium } from "playwright";
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
const page = await ctx.newPage();
await page.goto(BASE, { waitUntil: "networkidle", timeout: 30000 });
await page.evaluate(() => localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" })));
await page.goto(BASE + "/#/accounts", { waitUntil: "networkidle", timeout: 30000 });
await page.waitForFunction(() => document.querySelector("#app")?.children?.length > 0, { timeout: 15000 }).catch(()=>{});
await page.waitForTimeout(3000);
const info = await page.evaluate(() => ({
  hasCard: !!document.querySelector(".card"),
  hasSkeleton: !!document.querySelector(".skeleton"),
  roleStatus: [...document.querySelectorAll("[role='status']")].map(e=>({cls:e.className,busy:e.getAttribute("aria-busy")})),
  appChildren: document.querySelector("#app")?.children?.length,
  bodySnip: document.querySelector("#app")?.innerHTML?.slice(0,400)
}));
console.log(JSON.stringify(info, null, 2));
await page.screenshot({ path: "e2e/screenshots/gx17_accounts_real.png" });
await browser.close();
