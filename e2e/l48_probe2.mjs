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

  // Dump all localStorage keys
  const keys = await page.evaluate(() => Object.keys(localStorage));
  console.log("localStorage keys:", JSON.stringify(keys));

  // Dump full dataset
  const d = await page.evaluate(() => {
    const raw = localStorage.getItem("cashflux:dataset");
    if (!raw) return null;
    const obj = JSON.parse(raw);
    return Object.keys(obj);
  });
  console.log("dataset top-level keys:", JSON.stringify(d));

  // Navigate to /members and check DOM
  await page.evaluate((r) => {
    window.history.pushState({}, "", r);
    window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
  }, "/members");
  await page.waitForTimeout(1500);
  const memBody = await page.evaluate(() => document.body.innerText);
  console.log("members page text (first 500):", memBody.slice(0, 500));

  // What's the member-add input ID?
  const inputsInfo = await page.evaluate(() =>
    Array.from(document.querySelectorAll("input")).map(i => ({
      id: i.id, placeholder: i.placeholder, ariaLabel: i.getAttribute("aria-label"), type: i.type
    }))
  );
  console.log("inputs on /members:", JSON.stringify(inputsInfo));

} finally {
  await browser.close();
}
