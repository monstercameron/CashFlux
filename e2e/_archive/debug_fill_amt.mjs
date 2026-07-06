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

const tag = "AMTTEST_" + Date.now();

// Fill description with pressSequentially (we know that updates wasm state)
const descLoc = p.locator("#txn-add");
await descLoc.pressSequentially(tag, { delay: 15 });
await p.waitForTimeout(500);
console.log("Desc DOM:", await descLoc.inputValue());

// Fill amount with fill + long wait
const amtLoc = p.locator('.form-grid input[type="number"]');
await amtLoc.fill("55");
await p.waitForTimeout(1000);
console.log("Amt DOM:", await amtLoc.inputValue());

// Check error - if amount is still empty after fill, we'll see amount error
const submitBtn = p.locator('.form-grid button:has-text("Add")');
await submitBtn.click();
await p.waitForTimeout(500);
const errText1 = await p.locator('#txn-err').textContent().catch(() => "no err");
console.log("Error after submit:", errText1);
await p.waitForTimeout(1500);

await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(1000);
const ds = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = ds.transactions || [];
const found = allTxns.filter(t => t.description && t.description.includes("AMTTEST_"));
const last = allTxns[allTxns.length - 1];
console.log("Found:", found.length, "Last:", JSON.stringify({ desc: last?.description?.substring(0,20), amt: last?.amount?.Amount }));

await b.close();
