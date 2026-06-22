import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const b = await chromium.launch({ headless: true });
const p = await b.newPage();
p.setViewportSize({ width: 1280, height: 900 });
p.on("console", m => console.log("B>", m.text()));

await p.goto("http://127.0.0.1:8099/", { waitUntil: "domcontentloaded" });
await p.waitForSelector("#app", { timeout: 60000 });
await p.waitForTimeout(5000);

// Read the workspace registry
const wsReg = await p.evaluate(() => {
  const ws = localStorage.getItem("cashflux:workspaces");
  return ws ? JSON.parse(ws) : null;
});
console.log("Workspace registry:", JSON.stringify(wsReg, null, 2));

// Check for any ws-data blobs
const wsBlobs = await p.evaluate(() => {
  const blobs = [];
  for (let i = 0; i < localStorage.length; i++) {
    const k = localStorage.key(i);
    if (k && k.startsWith('cashflux:ws-data:')) {
      blobs.push({ key: k, len: localStorage.getItem(k)?.length || 0 });
    }
  }
  return blobs;
});
console.log("WS blobs:", JSON.stringify(wsBlobs));

await b.close();
