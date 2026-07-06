import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const b = await chromium.launch({ headless: true });
const p = await b.newPage();
p.setViewportSize({ width: 1280, height: 900 });
p.on("pageerror", e => console.log("JSERR:", e.message));

await p.goto("http://127.0.0.1:8099/transactions", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 30000 });
await p.waitForTimeout(4000);

const countBefore = await p.evaluate(() => {
  const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  return (ds.transactions || []).length;
});
console.log("Count before:", countBefore);

// Don't fill anything — just press submit. Should fail with "positiveAmount" error.
const submitBtn = await p.$('.form-grid button:has-text("Add")');
console.log("Submit found:", !!submitBtn);
if (submitBtn) {
  await submitBtn.click();
  await p.waitForTimeout(1000);
}

// Check for error message
const errMsg = await p.evaluate(() => {
  // look for error text
  const errEl = document.querySelector('[id*="txn-err"], .error, [class*="err"]');
  return errEl ? errEl.textContent : "no error element found; body includes: " + (document.body.innerText.substring(300, 500));
});
console.log("Error message:", errMsg);

// Now fill amount only
await p.evaluate(() => {
  const amtInput = document.querySelector(".form-grid input[type=number]");
  if (amtInput) {
    amtInput.value = "50";
    amtInput.oninput && amtInput.oninput({ target: { value: "50" }, preventDefault: ()=>{} });
  }
});
await p.waitForTimeout(300);

if (submitBtn) {
  await submitBtn.click();
  await p.waitForTimeout(1000);
}

await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(500);

const countAfter = await p.evaluate(() => {
  const ds = JSON.parse(localStorage.getItem("cashflux:dataset") || "{}");
  return (ds.transactions || []).length;
});
console.log("Count after second submit:", countAfter);

// Check for error again
const errMsg2 = await p.evaluate(() => {
  const errEl = document.querySelector('[id*="txn-err"], .error, [class*="err"]');
  return errEl ? errEl.textContent : "no error element";
});
console.log("Error after second submit:", errMsg2);

await b.close();
