import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const b = await chromium.launch({ headless: true });
const p = await b.newPage();
p.setViewportSize({ width: 1280, height: 900 });
const jsErrors = [];
p.on("pageerror", e => jsErrors.push(e.message));
p.on("console", msg => { if (msg.type() !== "log") console.log("CONSOLE:", msg.type(), msg.text()); });

await p.goto("http://127.0.0.1:8099/transactions", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 30000 });
await p.waitForTimeout(3000);

// Check if the form is inline
const txnAddInput = await p.$("#txn-add");
console.log("txn-add input found:", !!txnAddInput);

// Count txns before
const dsBefore = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
console.log("Txns before:", (dsBefore.transactions || []).length);

// Fill the form
const tag = "INLINE_" + Date.now();
const descInput = await p.$("#txn-add");
if (descInput) {
  await descInput.fill(tag);
  console.log("Filled description:", tag);
}

const amtInput = await p.$('.form-grid input[type="number"]');
if (amtInput) {
  await amtInput.fill("75");
  console.log("Filled amount: 75");
}

// Check current form state
const formState = await p.evaluate(() => {
  const form = document.querySelector(".form-grid");
  if (!form) return "no form-grid";
  return {
    inputs: [...form.querySelectorAll("input")].map(i => ({ id: i.id, placeholder: i.placeholder, value: i.value, type: i.type })),
    btns: [...form.querySelectorAll("button")].map(b => ({ text: b.textContent.trim(), type: b.type, disabled: b.disabled }))
  };
});
console.log("Form state:", JSON.stringify(formState, null, 2));

// Click submit
const submitBtn = await p.$('.form-grid button[type="submit"], .form-grid button:has-text("Add")');
console.log("Submit button found:", !!submitBtn);
if (submitBtn) {
  await submitBtn.click();
  console.log("Clicked submit");
  await p.waitForTimeout(1000);
}

// Flush
await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(500);

const dsAfter = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const newTxns = (dsAfter.transactions || []).filter(t => t.description && t.description.includes("INLINE_"));
console.log("New txns after submit:", newTxns.length);
console.log("Total txns:", (dsAfter.transactions || []).length);
if (jsErrors.length) console.log("JS errors:", jsErrors);

await p.screenshot({ path: path.join(__dirname, "debug_inline_after.png") });
await b.close();
