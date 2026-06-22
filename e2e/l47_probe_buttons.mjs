import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();

// Navigate via URL first to get the SPA loaded
await page.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await page.waitForSelector("#app", { timeout: 30000 });
await page.waitForTimeout(2000);

// Push to /settings via client-side routing
await page.evaluate(() => {
  window.history.pushState({}, "", "/settings");
  window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
});
await page.waitForTimeout(2000);

console.log("URL:", page.url());
const btns = await page.evaluate(() =>
  Array.from(document.querySelectorAll("button")).map(b => b.textContent.trim()).filter(t => t.length > 0)
);
console.log("ALL BUTTONS:", JSON.stringify(btns));

// Screenshot
await page.screenshot({ path: "e2e/l47_probe_settings.png" });
await browser.close();
