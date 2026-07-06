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

const hasOninput = await p.evaluate(() => {
  const inp = document.getElementById("txn-add");
  if (!inp) return "no input";
  return {
    hasOninput: !!inp.oninput,
    onchange: !!inp.onchange,
    // Try firing input event manually and check if something changed
    beforeFire: inp.value
  };
});
console.log("oninput check:", JSON.stringify(hasOninput));

// Fire event manually and track if state changes
const result = await p.evaluate(() => {
  const inp = document.getElementById("txn-add");
  if (!inp) return "no input";
  inp.value = "TESTVALUE";
  // Fire as the browser would
  const evt = new InputEvent("input", { bubbles: true, cancelable: false, composed: true, inputType: "insertText", data: "TESTVALUE" });
  const fired = inp.dispatchEvent(evt);
  return { fired, value: inp.value, hasOninput: typeof inp.oninput };
});
console.log("Manual event result:", result);
await p.waitForTimeout(500);

// Check if description is now non-empty in the form state
// The wasm internal state is only observable at submit time.
// Submit and check if description appears.
const amtInput = await p.$('.form-grid input[type="number"]');
if (amtInput) await amtInput.fill("33");

const submitBtn = await p.$('.form-grid button:has-text("Add")');
if (submitBtn) {
  await submitBtn.click();
  await p.waitForTimeout(1500);
}

await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(500);

const ds = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = ds.transactions || [];
const last = allTxns[allTxns.length - 1];
console.log("Last txn:", JSON.stringify({ desc: last?.description, amount: last?.amount }));

await b.close();
