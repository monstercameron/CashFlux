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

const tag = "TYPED_" + Date.now();

// Try clicking the input first, then typing
const descInput = await p.$("#txn-add");
if (descInput) {
  await descInput.click();
  await p.waitForTimeout(100);
  await p.keyboard.type(tag);
  console.log("Typed description:", tag);
}

// Amount: click first then type
const amtInput = await p.$('.form-grid input[type="number"]');
if (amtInput) {
  await amtInput.click();
  await p.waitForTimeout(100);
  await p.keyboard.type("99");
}

// Check current values
const vals = await p.evaluate(() => ({
  desc: document.getElementById("txn-add")?.value,
  amt: document.querySelector(".form-grid input[type=number]")?.value
}));
console.log("Input values:", vals);

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
const found = allTxns.filter(t => t.description && t.description.includes("TYPED_"));
console.log("Found TYPED_ txns:", found.length);
const last = allTxns[allTxns.length - 1];
console.log("Last txn:", JSON.stringify({ desc: last?.description, amount: last?.amount }, null, 2));

await b.close();
