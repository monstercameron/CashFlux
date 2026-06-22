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

await goto("/accounts");

// Click the first "More actions" button to see what menu items appear
const moreBtn = await page.$('button[aria-label="More actions"]');
if (moreBtn) {
  await moreBtn.click();
  await page.waitForTimeout(500);
  // Now grab visible buttons/items
  const menuItems = await page.evaluate(() => {
    return Array.from(document.querySelectorAll("button,a,[role='menuitem']"))
      .filter(el => el.offsetParent !== null)
      .map(el => ({ tag: el.tagName, text: el.textContent?.trim().slice(0,60), ariaLabel: el.getAttribute("aria-label")||"" }));
  });
  console.log("MENU ITEMS AFTER MORE:", JSON.stringify(menuItems.slice(-20)));
}

// Also check the accounts page body to find how net worth is shown
const bodyText = await page.evaluate(() => document.body.innerText.slice(0,1500));
console.log("ACCOUNTS BODY:", bodyText);

await browser.close();
