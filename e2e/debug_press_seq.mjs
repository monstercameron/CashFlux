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

const tag = "SEQ_" + Date.now();
const desc = p.locator("#txn-add");
// pressSequentially types char by char with keydown/keypress/input/keyup for each
await desc.pressSequentially(tag, { delay: 20 });
await p.waitForTimeout(200);

const domVal = await desc.inputValue();
console.log("DOM value after pressSequentially:", domVal);

// Check if wasm state reflects this — try submitting (should get "positiveAmount" error if desc was accepted)
const submitBtn = p.locator('.form-grid button:has-text("Add")');
await submitBtn.click();
await p.waitForTimeout(500);
const errEl = p.locator('#txn-err');
const errText = await errEl.textContent().catch(() => "no error");
console.log("Error (no amount):", errText);

// Now fill amount
await p.locator('.form-grid input[type="number"]').fill("66");
await p.waitForTimeout(200);
await submitBtn.click();
await p.waitForTimeout(1500);

await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(500);

const ds = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = ds.transactions || [];
const found = allTxns.filter(t => t.description && t.description.includes("SEQ_"));
const last = allTxns[allTxns.length - 1];
console.log("Found by desc:", found.length);
console.log("Last txn:", JSON.stringify({ desc: last?.description, amount: last?.amount?.Amount }));

await b.close();
