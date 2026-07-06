import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const b = await chromium.launch({ headless: true });
const p = await b.newPage();
p.setViewportSize({ width: 1280, height: 900 });

await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(4000);

// Show schema of the first 3 transactions in localStorage
const txns = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}")).transactions || [];
console.log("First 3 transactions:");
txns.slice(0,3).forEach(t => console.log(JSON.stringify(t)));

await b.close();
