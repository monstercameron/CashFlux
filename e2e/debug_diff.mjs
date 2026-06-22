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

// Compare original vs re-serialized
const result = await p.evaluate(() => {
  const raw = localStorage.getItem("cashflux:dataset") || "{}";
  const ds = JSON.parse(raw);
  const reraw = JSON.stringify(ds);

  // Find first differing char position
  let diffPos = -1;
  for (let i = 0; i < Math.min(raw.length, reraw.length); i++) {
    if (raw[i] !== reraw[i]) { diffPos = i; break; }
  }

  // Look for a transaction with unexpected encoding
  const t0 = ds.transactions[0];
  return {
    rawLen: raw.length,
    rerawLen: reraw.length,
    diffPos,
    rawAt: diffPos >= 0 ? raw.substring(Math.max(0,diffPos-20), diffPos+20) : null,
    rerawAt: diffPos >= 0 ? reraw.substring(Math.max(0,diffPos-20), diffPos+20) : null,
    t0keys: Object.keys(t0),
    t0amount: JSON.stringify(t0.amount),
    t0date: t0.date,
    // Check for any base64/binary data in accounts
    a0keys: Object.keys(ds.accounts[0] || {}),
  };
});
console.log(JSON.stringify(result, null, 2));

await b.close();
