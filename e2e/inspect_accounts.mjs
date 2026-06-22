import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
page.setViewportSize({ width: 1280, height: 900 });
const waitNav = (page) => page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
const goto = async (hash) => { await page.goto(BASE + hash, { waitUntil: "domcontentloaded" }); await waitNav(page); await page.waitForTimeout(1500); };

// Inspect /accounts buttons and structure
await goto("/accounts");
const accBody = await page.evaluate(() => {
  const body = document.body.innerText;
  // Find L44 Omar section
  const idx = body.indexOf("L44 Omar");
  return { fragment: idx >= 0 ? body.slice(Math.max(0,idx-50), idx+300) : "NOT FOUND", fullBody: body.slice(0, 800) };
});
console.log("ACCOUNTS L44 FRAGMENT:", JSON.stringify(accBody.fragment));

// Check all buttons on accounts page
const btns = await page.evaluate(() => {
  return Array.from(document.querySelectorAll("button")).map(b => ({
    text: b.textContent?.trim().slice(0,60),
    ariaLabel: b.getAttribute("aria-label")||"",
    visible: b.offsetParent !== null
  })).filter(b => b.text || b.ariaLabel);
});
console.log("ACCOUNTS BUTTONS:", JSON.stringify(btns.filter(b => b.visible)));

await browser.close();
