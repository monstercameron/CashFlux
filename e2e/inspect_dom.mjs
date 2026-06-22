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

// Inspect /documents
await goto("/documents");
const docDom = await page.evaluate(() => {
  const inputs = Array.from(document.querySelectorAll("input,textarea,select,button")).map(el => ({
    tag: el.tagName, type: el.type||"", id: el.id, name: el.name||"", placeholder: el.placeholder||"", ariaLabel: el.getAttribute("aria-label")||"", text: el.textContent?.trim().slice(0,40)||""
  }));
  return inputs;
});
console.log("DOCUMENTS DOM:", JSON.stringify(docDom));

// Inspect dashboard NW
await goto("/");
const dashNW = await page.evaluate(() => {
  const body = document.body.innerText;
  const netWorthSection = body.match(/Net[\s\S]{0,200}/i);
  return { netWorthContext: netWorthSection?.[0]?.slice(0,300) };
});
console.log("DASH NW CONTEXT:", JSON.stringify(dashNW));

await browser.close();
