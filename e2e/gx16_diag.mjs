import { chromium } from "playwright";
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
await page.goto("http://localhost:8080/dashboard");
await page.waitForTimeout(3000);
const theme = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
const hasWasm = await page.evaluate(() => !!document.querySelector("#app"));
const appContent = await page.evaluate(() => (document.querySelector("#app") || {}).innerHTML?.slice(0, 300) || "no #app");
console.log("data-theme:", theme);
console.log("#app exists:", hasWasm);
console.log("#app innerHTML[:300]:", appContent);

// Set light via localStorage and reload
await page.evaluate(() => {
  localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" }));
});
await page.reload();
await page.waitForTimeout(3000);
const theme2 = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
console.log("After reload data-theme:", theme2);
const svgCount = await page.evaluate(() => document.querySelectorAll("svg").length);
console.log("SVG count:", svgCount);
const cfCharts = await page.evaluate(() => document.querySelectorAll(".cf-chart").length);
console.log(".cf-chart count:", cfCharts);
await browser.close();
