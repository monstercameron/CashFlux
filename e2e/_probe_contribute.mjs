// Probe: what does the contribute form look like after clicking Contribute?
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const browser = await chromium.launch({ headless: true });
const page = await browser.newPage();
page.setViewportSize({ width: 1280, height: 900 });

// First add an account
await page.goto("http://127.0.0.1:8099/accounts", { waitUntil: "domcontentloaded" });
await page.waitForTimeout(3000);
const nameIn = await page.$('input[placeholder="Name"]');
const balIn  = await page.$('input[placeholder="Opening balance"]');
if (nameIn && balIn) {
  await nameIn.fill("L59 ProbeAcct2");
  await balIn.fill("2000");
  const sub = await page.$('button[type="submit"]');
  if (sub) { await sub.click(); await page.waitForTimeout(1500); }
}

// Add a goal
await page.goto("http://127.0.0.1:8099/goals", { waitUntil: "domcontentloaded" });
await page.waitForTimeout(3000);
const gNameIn = await page.$('#goal-add');
const gTargIn = await page.$('input[placeholder="Target (USD)"]');
const gSaveIn = await page.$('input[placeholder="Saved so far"]');
if (gNameIn && gTargIn && gSaveIn) {
  await gNameIn.fill("L59 ProbeGoal");
  await gTargIn.fill("500");
  await gSaveIn.fill("475");
  const sub2 = await page.$('button[type="submit"]');
  if (sub2) { await sub2.click(); await page.waitForTimeout(2000); }
}

// Now click Contribute on the goal
const bodyAfterAdd = await page.evaluate(() => document.body.innerText);
console.log("Has ProbeGoal:", bodyAfterAdd.includes("L59 ProbeGoal"));
console.log("Has 95%:", /95\s*%/.test(bodyAfterAdd));

// Click contribute button
const contribs = await page.$$('button');
for (const b of contribs) {
  const txt = await b.evaluate(el => el.textContent?.trim() ?? "");
  if (/contribute/i.test(txt)) {
    await b.click();
    await page.waitForTimeout(1000);
    break;
  }
}

// Inspect the form
const inputs2 = await page.evaluate(() =>
  Array.from(document.querySelectorAll("input")).map(el => ({
    type: el.type, placeholder: el.placeholder, ariaLabel: el.getAttribute("aria-label"), id: el.id,
    parentClass: el.parentElement?.className ?? ""
  }))
);
console.log("INPUTS AFTER CONTRIBUTE CLICK:", JSON.stringify(inputs2, null, 2));

const btns2 = await page.evaluate(() =>
  Array.from(document.querySelectorAll("button")).map(b => ({ txt: b.textContent?.trim(), type: b.type }))
);
console.log("BUTTONS:", btns2.map(b => b.txt + "(" + b.type + ")").join(", "));

await browser.close();
