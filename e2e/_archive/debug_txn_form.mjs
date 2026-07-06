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

// Click add btn, then new transaction
await p.click('[aria-label="Add something new"]');
await p.waitForTimeout(500);
await p.click('button:has-text("New transaction")');
await p.waitForTimeout(1500);
await p.screenshot({ path: path.join(__dirname, "debug_txn_form.png") });

const formInfo = await p.evaluate(() => {
  // find the add/edit panel
  const form = document.querySelector(".add-form, form, .modal, .drawer, .panel, .add-panel");
  if (!form) {
    // try body-level search
    const allInputs = [...document.querySelectorAll("input,select,textarea")];
    return "no .form: inputs count=" + allInputs.length + " body=" + document.body.innerText.substring(0, 300);
  }
  const inputs = [...form.querySelectorAll("input,select,textarea")].map(i => ({
    type: i.type, name: i.name, id: i.id, placeholder: i.placeholder,
    ariaLabel: i.getAttribute("aria-label"), class: i.className.substring(0, 30)
  }));
  const labels = [...form.querySelectorAll("label")].map(l => l.textContent.trim().substring(0, 40));
  const btns = [...form.querySelectorAll("button")].map(b => b.textContent.trim().substring(0, 25));
  return JSON.stringify({ formClass: form.className, inputs, labels, btns }, null, 2);
});
console.log(formInfo);
await b.close();
