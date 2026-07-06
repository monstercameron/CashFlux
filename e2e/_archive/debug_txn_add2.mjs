import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const b = await chromium.launch({ headless: true });
const p = await b.newPage();
p.setViewportSize({ width: 1280, height: 900 });
await p.goto("http://127.0.0.1:8099/transactions", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 30000 });
await p.waitForTimeout(3000);

// Open form
await p.keyboard.press("Escape");
await p.waitForTimeout(300);
await p.click('[aria-label="Add something new"]');
await p.waitForTimeout(500);
await p.click('button:has-text("New transaction")');
await p.waitForTimeout(1500);

// Get the backdrop structure
const structure = await p.evaluate(() => {
  const bd = document.querySelector(".flip-backdrop");
  if (!bd) return "no flip-backdrop";
  return {
    class: bd.className,
    children: [...bd.children].map(c => ({ tag: c.tagName, class: c.className.substring(0, 30) })),
    submitBtnExists: !!bd.querySelector('button[type="submit"]'),
    submitBtnClass: bd.querySelector('button[type="submit"]')?.className
  };
});
console.log("Backdrop structure:", JSON.stringify(structure, null, 2));

// Try force click via Playwright
const tag = "DEBUG_FORCE_" + Date.now();
const descInput = await p.$("#txn-add");
if (descInput) await descInput.fill(tag);
const amtInput = await p.$('.form-grid input[type="number"]');
if (amtInput) await amtInput.fill("50");

// Force click with Playwright's { force: true } option
const submitBtn = p.locator('.form-grid button[type="submit"]');
const btnCount = await submitBtn.count();
console.log("Submit btns count:", btnCount);
if (btnCount > 0) {
  await submitBtn.first().click({ force: true });
  console.log("Force-clicked submit");
}
await p.waitForTimeout(2000);

// Flush
await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(500);

const ds = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const myTxns = (ds.transactions || []).filter(t => t.description && t.description.includes("DEBUG_FORCE"));
console.log("Saved after force click:", myTxns.length, myTxns.map(t => t.description));

await b.close();
