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

const tag = "FILLWAIT_" + Date.now();

const descInput = p.locator("#txn-add");
await descInput.fill(tag);
await p.waitForTimeout(1000); // Wait 1 second for wasm to process the input event

// Check DOM value is still there (wasm re-render might reset it)
const domValAfterWait = await descInput.inputValue();
console.log("DOM value after 1s wait:", domValAfterWait);

// Submit (without amount, should show amount error if desc accepted)
const submitBtn = p.locator('.form-grid button:has-text("Add")');
await submitBtn.click();
await p.waitForTimeout(500);
const errText = await p.locator('#txn-err').textContent().catch(() => "no err element");
console.log("Error (no amount):", errText);

// Fill amount with 1s wait
await p.locator('.form-grid input[type="number"]').fill("77");
await p.waitForTimeout(1000);
await submitBtn.click();
await p.waitForTimeout(2000);

await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(1000);

const ds = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = ds.transactions || [];
const found = allTxns.filter(t => t.description && t.description.includes("FILLWAIT_"));
const last = allTxns[allTxns.length - 1];
console.log("Found:", found.length);
console.log("Last txn desc:", last?.description?.substring(0, 30), "amount:", last?.amount?.Amount);

await b.close();
