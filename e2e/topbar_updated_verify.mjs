// topbar_updated_verify.mjs — locks the top-bar "Updated …" freshness stamp
// (parity scan: persistent scope/period strip needs an "Updated X min ago" leg).
//   1. The stamp renders in the top bar with the last change in its title.
//   2. Clicking it opens /activity (where the change and its Undo live).
//   3. On crowded widths (1440) it collapses to icon-only but stays clickable.
// Usage: node e2e/topbar_updated_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";

const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (name, ok, detail = "") => {
  console.log(`${ok ? "PASS" : "FAIL"}: ${name}${detail ? " — " + detail : ""}`);
  ok ? pass++ : fail++;
};

const browser = await chromium.launch();

// Wide viewport: label visible.
{
  const page = await (await browser.newContext({ viewport: { width: 1900, height: 950 }, reducedMotion: "reduce" })).newPage();
  await page.goto(BASE + "/dashboard", { waitUntil: "load" });
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
  await page.waitForTimeout(2000);
  const stamp = page.locator('[data-testid="topbar-updated"]');
  check("stamp renders in the top bar", (await stamp.count()) === 1);
  const text = (await stamp.count()) ? await stamp.innerText() : "";
  check("wide viewport shows the Updated label", /Updated/.test(text), text);
  const title = (await stamp.count()) ? await stamp.getAttribute("title") : "";
  check("title names the last change", /Last change:/.test(title || ""), title);
  await page.screenshot({ path: "e2e/topbar_updated_wide.png", clip: { x: 0, y: 0, width: 1900, height: 60 } });
  await stamp.click();
  await page.waitForTimeout(1200);
  check("clicking the stamp opens /activity", page.url().endsWith("/activity"), page.url());
  await page.close();
}

// Crowded viewport: icon-only but still clickable.
{
  const page = await (await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" })).newPage();
  await page.goto(BASE + "/dashboard", { waitUntil: "load" });
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
  await page.waitForTimeout(2000);
  const stamp = page.locator('[data-testid="topbar-updated"]');
  const text = (await stamp.count()) ? await stamp.innerText() : "(missing)";
  check("crowded viewport collapses to icon-only", text.trim() === "", JSON.stringify(text));
  await stamp.click();
  await page.waitForTimeout(1200);
  check("icon-only stamp still routes to /activity", page.url().endsWith("/activity"), page.url());
  await page.close();
}

console.log(`\nRESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
