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

const tag = "DIRECTONINPUT_" + Date.now();

// Call the oninput handler DIRECTLY (not via event dispatch)
const result = await p.evaluate((val) => {
  const inp = document.getElementById("txn-add");
  if (!inp) return { error: "no input" };
  const hasOninput = !!inp.oninput;

  // Call oninput directly with a fake event
  if (inp.oninput) {
    const fakeEvent = {
      target: { value: val },
      preventDefault: () => {},
      stopPropagation: () => {}
    };
    inp.value = val;
    inp.oninput(fakeEvent);
  }

  return { called: !!inp.oninput, value: inp.value };
}, tag);
console.log("Direct oninput call:", result);

await p.waitForTimeout(500); // let scheduler run

// Set amount the same way
await p.evaluate(() => {
  const amtInput = document.querySelector(".form-grid input[type=number]");
  if (!amtInput) return;
  amtInput.value = "44";
  if (amtInput.oninput) {
    amtInput.oninput({ target: { value: "44" }, preventDefault: () => {}, stopPropagation: () => {} });
  }
});
await p.waitForTimeout(500);

// Submit
const submitBtn = await p.$('.form-grid button:has-text("Add")');
if (submitBtn) {
  await submitBtn.click();
  await p.waitForTimeout(1500);
}

await p.evaluate(() => window.dispatchEvent(new Event("visibilitychange")));
await p.waitForTimeout(500);

const ds = JSON.parse(await p.evaluate(() => localStorage.getItem("cashflux:dataset") || "{}"));
const allTxns = ds.transactions || [];
const found = allTxns.filter(t => t.description && t.description.includes("DIRECTONINPUT_"));
const last = allTxns[allTxns.length - 1];
console.log("Found:", found.length);
console.log("Last txn:", JSON.stringify({ desc: last?.description, amount: last?.amount }));

await b.close();
