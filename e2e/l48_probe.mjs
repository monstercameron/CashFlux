import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });
  await page.waitForTimeout(2000);

  const d = await page.evaluate(() => JSON.parse(localStorage.getItem("cashflux:dataset") || "{}"));
  console.log("members:", JSON.stringify(d.members?.map(m => ({ id: m.id, name: m.name }))));

  await page.evaluate((r) => {
    window.history.pushState({}, "", r);
    window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
  }, "/split");
  await page.waitForTimeout(1500);

  const opts = await page.evaluate(() => {
    const sel = document.querySelector('select[aria-label*="paid" i], select[title*="payer" i], select[aria-label*="payer" i]');
    return sel ? Array.from(sel.options).map(o => ({ val: o.value, text: o.text })) : null;
  });
  console.log("payer opts:", JSON.stringify(opts));

  // Check member toggles
  const switches = await page.evaluate(() => {
    return Array.from(document.querySelectorAll('[role="switch"]')).map(s => ({
      label: s.getAttribute("aria-label"),
      checked: s.getAttribute("aria-checked"),
    }));
  });
  console.log("role=switch elements:", JSON.stringify(switches));

} finally {
  await browser.close();
}
