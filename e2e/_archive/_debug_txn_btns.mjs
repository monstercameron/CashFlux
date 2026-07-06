import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const b = await chromium.launch({ headless: true });
const p = await b.newPage();
await p.goto(BASE + "/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("nav a[title]", { timeout: 60000 });
await p.waitForTimeout(800);
const link = p.locator('nav a[title="Transactions"]').first();
await link.click();
await p.waitForTimeout(1200);
const result = await p.evaluate(() => {
  const rows = [...document.querySelectorAll(".txn-table tr.row, tbody tr, .row")];
  if (rows.length === 0) return { rows: 0, allBtns: [...document.querySelectorAll("button")].map(b => ({ text: b.textContent.trim().slice(0,40), cls: b.className.slice(0,60), title: b.title, al: b.getAttribute("aria-label") })).slice(0, 20) };
  const firstRow = rows[0];
  const btns = [...firstRow.querySelectorAll("button")];
  return { rowCount: rows.length, btns: btns.map(b => ({ text: b.textContent.trim().slice(0,40), cls: b.className, title: b.title, al: b.getAttribute("aria-label") })) };
});
console.log(JSON.stringify(result, null, 2));
await b.close();
