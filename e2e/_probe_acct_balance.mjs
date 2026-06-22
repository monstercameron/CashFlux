// Probe: what does the accounts page body look like after adding a fresh account?
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
await page.goto("http://127.0.0.1:8099/accounts", { waitUntil: "domcontentloaded" });
await page.waitForTimeout(4000);

// Add a test account
const nameIn = await page.$('input[placeholder="Name"]');
const balIn  = await page.$('input[placeholder="Opening balance"]');
if (nameIn && balIn) {
  await nameIn.fill("L59 ProbeAcct");
  await balIn.fill("3000");
  const sub = await page.$('button[type="submit"]');
  if (sub) { await sub.click(); await page.waitForTimeout(2000); }
}

const txt = await page.evaluate(() => document.body.innerText);
// Find the L59 ProbeAcct section
const idx = txt.indexOf("L59 ProbeAcct");
if (idx >= 0) {
  console.log("CONTEXT AROUND ACCT:", txt.substring(Math.max(0, idx-20), idx+200));
} else {
  console.log("NOT FOUND in body. Snippet:", txt.substring(0, 800));
}
await browser.close();
