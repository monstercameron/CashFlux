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

const countBefore = await p.evaluate(() => (JSON.parse(localStorage.getItem("cashflux:dataset") || "{}").transactions || []).length);
console.log("Count before:", countBefore);

const tag = "KBTYPE_" + Date.now();

// Method 1: click + type char by char
const descInput = await p.$("#txn-add");
if (descInput) {
  await descInput.click();
  await p.waitForTimeout(100);
  // Type the tag char by char
  for (const ch of tag) {
    await p.keyboard.press(ch);
    await p.waitForTimeout(10);
  }
  const val = await descInput.evaluate(el => el.value);
  console.log("After char-by-char type, DOM value:", val.substring(0, 20));
}

// Check wasm state by submitting (it will fail if amount empty, but error msg reveals desc state)
const submitBtn = await p.$('.form-grid button:has-text("Add")');
if (submitBtn) {
  await submitBtn.click();
  await p.waitForTimeout(500);
}
const errMsg = await p.evaluate(() => {
  const errEl = document.querySelector('#txn-err, [id*="txn-err"]');
  return errEl ? errEl.textContent : "no error";
});
console.log("Error after char-by-char (no amount):", errMsg);
// "Enter a positive amount" means desc was accepted; "desc is required" means it wasn't

// Now fill amount
const amtInput = await p.$('.form-grid input[type="number"]');
if (amtInput) {
  await amtInput.click();
  await amtInput.fill("42");
}

// Submit
if (submitBtn) {
  await submitBtn.click();
  await p.waitForTimeout(1500);
}

await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(500);

const ds = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = ds.transactions || [];
const found = allTxns.filter(t => t.description && t.description.includes("KBTYPE_"));
const last = allTxns[allTxns.length - 1];
console.log("Found:", found.length, "Last desc:", last?.description, "amount:", last?.amount?.Amount);

await b.close();
