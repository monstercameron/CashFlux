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

const tag = "MANUAL_" + Date.now();

// Method: set value then dispatch input event manually
await p.evaluate((val) => {
  const inp = document.getElementById("txn-add");
  if (!inp) return;
  inp.value = val;
  // Dispatch input event (what GoWebComponents listens for via oninput)
  inp.dispatchEvent(new InputEvent("input", { bubbles: true, cancelable: true, inputType: "insertText", data: val }));
}, tag);
await p.waitForTimeout(300);

// Set amount
await p.evaluate(() => {
  const inp = document.querySelector(".form-grid input[type=number]");
  if (!inp) return;
  inp.value = "55";
  inp.dispatchEvent(new InputEvent("input", { bubbles: true, cancelable: true }));
});
await p.waitForTimeout(300);

// Submit
const submitBtn = await p.$('.form-grid button:has-text("Add")');
if (submitBtn) {
  await submitBtn.click();
  await p.waitForTimeout(1500);
}

await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(1000);

const ds = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = ds.transactions || [];
const found = allTxns.filter(t => t.description && t.description.includes("MANUAL_"));
console.log("Found:", found.length);
const last = allTxns[allTxns.length - 1];
console.log("Last txn:", JSON.stringify({ desc: last?.description, amount: last?.amount }, null, 2));

await b.close();
