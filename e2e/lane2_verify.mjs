// Lane 2 verification: #44 preset select persistence + #45 phantom-scroll measurement
// across layouts. Usage: node e2e/lane2_verify.mjs [port]
import { chromium } from "playwright";
const port = process.argv[2] || "8112";
const base = `http://127.0.0.1:${port}`;
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
const results = [];
const check = (name, ok, detail) => { results.push({ name, ok, detail }); console.log((ok ? "PASS" : "FAIL") + " " + name + (detail ? " — " + detail : "")); };

const ready = async () => {
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
  await page.waitForTimeout(3500);
};
const phantom = async () => page.evaluate(() => {
  const main = document.getElementById("main");
  let maxBottom = 0;
  for (const el of main.querySelectorAll(".bento > .w, .w.chrome-hover, .topbar")) {
    const r = el.getBoundingClientRect();
    if (r.height > 0) maxBottom = Math.max(maxBottom, r.bottom + main.scrollTop);
  }
  return { scrollH: main.scrollHeight, lastContent: Math.round(maxBottom), phantom: Math.round(main.scrollHeight - maxBottom) };
});

await page.goto(base + "/", { waitUntil: "load" });
await ready();

// --- #45 baseline (default layout)
const p0 = await phantom();
console.log("default layout:", JSON.stringify(p0));

// --- #44: apply "Daily check-in" preset
await page.selectOption('[data-testid="dash-preset"]', "daily");
await page.waitForTimeout(1200);
const selNow = await page.$eval('[data-testid="dash-preset"]', (s) => ({ value: s.value, label: s.selectedOptions[0]?.textContent }));
check("#44 select reflects pick immediately", selNow.value === "daily", JSON.stringify(selNow));

// --- #45 under the daily preset (fewer widgets)
const p1 = await phantom();
console.log("daily preset:", JSON.stringify(p1));

// --- #44: reload, select must still show Daily check-in
await page.reload({ waitUntil: "load" });
await ready();
const selAfter = await page.$eval('[data-testid="dash-preset"]', (s) => ({ value: s.value, label: s.selectedOptions[0]?.textContent }));
check("#44 select persists across reload", selAfter.value === "daily", JSON.stringify(selAfter));

const p2 = await phantom();
console.log("daily preset after reload:", JSON.stringify(p2));

// --- restore default
await page.selectOption('[data-testid="dash-preset"]', "default");
await page.waitForTimeout(1200);
const p3 = await phantom();
console.log("default preset restored:", JSON.stringify(p3));

console.log("SUMMARY " + JSON.stringify({ p0, p1, p2, p3 }));
await browser.close();
process.exit(results.every((r) => r.ok) ? 0 : 1);
