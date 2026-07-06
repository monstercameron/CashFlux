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

await p.click('[aria-label="Add something new"]');
await p.waitForTimeout(500);
await p.click('button:has-text("New transaction")');
await p.waitForTimeout(1500);

// Get all selects with their options and surrounding context
const info = await p.evaluate(() => {
  const form = document.querySelector(".form-grid");
  if (!form) return "no form-grid";
  const allInputs = [...form.querySelectorAll("input,select,textarea")];
  return allInputs.map((el, i) => {
    const prev = el.previousElementSibling;
    const parent = el.parentElement;
    const parentText = parent ? parent.textContent.trim().substring(0, 60) : "";
    let options = [];
    if (el.tagName === "SELECT") {
      options = [...el.options].map(o => o.value + ":" + o.text);
    }
    return {
      i,
      tag: el.tagName,
      type: el.type,
      id: el.id,
      ariaLabel: el.getAttribute("aria-label"),
      placeholder: el.placeholder,
      parentText: parentText.substring(0, 50),
      options: options.slice(0, 6)
    };
  });
});
console.log(JSON.stringify(info, null, 2));

// Also check if there's a "Property" label anywhere in the form area
const cfInfo = await p.evaluate(() => {
  const addPanel = document.querySelector(".add-panel, .add-backdrop, .form-grid")?.closest("div,section,aside") || document.body;
  const text = addPanel.innerText;
  return {
    hasProperty: text.toLowerCase().includes("property"),
    hasTaxDeductible: text.toLowerCase().includes("tax deductible"),
    hasProject: text.toLowerCase().includes("project"),
    hasReimbursable: text.toLowerCase().includes("reimbursable"),
    fullText: text.substring(0, 500)
  };
});
console.log("\n--- CF check ---");
console.log(JSON.stringify(cfInfo, null, 2));

await b.close();
