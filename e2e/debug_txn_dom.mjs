import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const b = await chromium.launch({ headless: true });
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

// Get full DOM around the form
const domInfo = await p.evaluate(() => {
  const form = document.querySelector(".form-grid");
  if (!form) return "no form-grid";

  // Walk up to find the container
  let el = form;
  const path = [];
  while (el && el !== document.body && path.length < 8) {
    path.push(el.tagName + "." + el.className.replace(/\s+/g, ".").substring(0, 40));
    el = el.parentElement;
  }

  // Also check what elements are at the button's position
  const btn = document.querySelector('.form-grid button[type="submit"]');
  const btnRect = btn ? btn.getBoundingClientRect() : null;
  let elemAtPoint = null;
  if (btnRect) {
    elemAtPoint = document.elementFromPoint(btnRect.x + btnRect.width/2, btnRect.y + btnRect.height/2);
  }

  return {
    formParentChain: path,
    submitBtn: btn ? { text: btn.textContent.trim(), disabled: btn.disabled, rect: btnRect } : null,
    elemAtBtnCenter: elemAtPoint ? elemAtPoint.tagName + "." + elemAtPoint.className.substring(0,40) : null
  };
});
console.log(JSON.stringify(domInfo, null, 2));

// Check what's at z-index on top of the button
const zInfo = await p.evaluate(() => {
  const btn = document.querySelector('.form-grid button[type="submit"]');
  if (!btn) return "no btn";
  const rect = btn.getBoundingClientRect();
  const cx = rect.x + rect.width/2, cy = rect.y + rect.height/2;
  const elements = document.elementsFromPoint(cx, cy);
  return elements.map(e => e.tagName + "." + e.className.substring(0,40)).slice(0, 8);
});
console.log("Elements at button center:", zInfo);

await b.close();
