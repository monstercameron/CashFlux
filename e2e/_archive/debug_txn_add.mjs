import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const b = await chromium.launch({ headless: false }); // visible for debugging
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

// Fill description
const descInput = await p.$("#txn-add");
if (descInput) await descInput.fill("L53_DEBUG_TXN_" + Date.now());

// Fill amount
const amtInput = await p.$('.form-grid input[type="number"]');
if (amtInput) { await amtInput.fill(""); await amtInput.fill("100"); }

// Fill custom property field
await p.evaluate(() => {
  const form = document.querySelector(".form-grid");
  if (!form) return console.log("NO FORM");
  const inputs = [...form.querySelectorAll("input[type=text]")];
  console.log("text inputs:", inputs.map(i => i.id + "/" + i.placeholder));
  const propInput = inputs.find(i => i.placeholder.toLowerCase().includes("l53 property") || i.placeholder.toLowerCase().includes("l53_property"));
  if (propInput) {
    propInput.value = "TestStreet";
    propInput.dispatchEvent(new Event("input", { bubbles: true }));
    propInput.dispatchEvent(new Event("change", { bubbles: true }));
    console.log("Set property to TestStreet");
  } else {
    console.log("No property input found. All inputs:", inputs.map(i => i.placeholder));
  }
});

// Check form state before submit
const formState = await p.evaluate(() => {
  const form = document.querySelector(".form-grid");
  if (!form) return "no form";
  return {
    inputs: [...form.querySelectorAll("input")].map(i => ({ id: i.id, placeholder: i.placeholder, value: i.value, type: i.type })),
    selects: [...form.querySelectorAll("select")].map(s => ({ ariaLabel: s.getAttribute("aria-label"), value: s.value, firstOpt: s.options[0]?.text })),
    buttons: [...form.querySelectorAll("button")].map(b => ({ text: b.textContent.trim(), type: b.type, disabled: b.disabled }))
  };
});
console.log("Form state:", JSON.stringify(formState, null, 2));

// Try to submit
const submitted = await p.evaluate(() => {
  const btn = document.querySelector('.flip-backdrop .form-grid button, .form-grid button, .add-panel button');
  console.log("Submit btn found:", btn?.textContent?.trim());
  if (btn) { btn.click(); return "clicked:" + btn.textContent.trim(); }
  return "no btn";
});
console.log("Submit result:", submitted);
await p.waitForTimeout(2000);

// Check what happened
const ds = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const myTxns = (ds.transactions || []).filter(t => t.description && t.description.includes("L53_DEBUG"));
console.log("Saved transactions:", myTxns.length, myTxns.map(t => ({ desc: t.description, custom: t.custom })));

await p.screenshot({ path: path.join(__dirname, "debug_txn_add_result.png") });
await b.close();
