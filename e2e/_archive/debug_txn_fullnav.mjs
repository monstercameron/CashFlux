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

// First navigate to customize (like the L53 script does)
await p.goto("http://127.0.0.1:8099/customize", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 30000 });
await p.waitForTimeout(3000);

// Now pushNav to /transactions
await p.evaluate(() => {
  window.history.pushState({}, "", "/transactions");
  window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
});
await p.waitForTimeout(2000);

const tag = "AFTERNAV_" + Date.now();
const descInput = await p.$("#txn-add");
console.log("txn-add found:", !!descInput);

if (descInput) {
  await descInput.fill(tag);
  await p.waitForTimeout(200);
  const val = await descInput.evaluate(el => el.value);
  console.log("Input value after fill:", val);
}

const amtInput = await p.$('.form-grid input[type="number"]');
if (amtInput) await amtInput.fill("77");

// Check if value is reflected in wasm state by looking at form
const formVals = await p.evaluate(() => {
  const inp = document.getElementById("txn-add");
  return inp ? inp.value : "not found";
});
console.log("DOM input value:", formVals);

const submitBtn = await p.$('.form-grid button:has-text("Add")');
console.log("Submit btn:", !!submitBtn);
if (submitBtn) {
  await submitBtn.click();
  await p.waitForTimeout(2000);
}

await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(1000);

const ds = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = ds.transactions || [];
const found = allTxns.filter(t => t.description && t.description.includes("AFTERNAV_"));
console.log("Found:", found.length, "Last txn desc:", allTxns[allTxns.length-1]?.description, "amount:", allTxns[allTxns.length-1]?.amount);

await b.close();
