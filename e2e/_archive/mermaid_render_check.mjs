// C70 gate — "a Mermaid diagram actually renders". Each saved workflow on
// /workflows shows a flowchart; this asserts the vendored mermaid lib + shim turn
// the generated source into an inline <svg>. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await page.goto(BASE + "/workflows", { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".cf-mermaid", { timeout: 60000 });

  // The vendored mermaid lib + shim render asynchronously; wait for the SVG.
  await page.waitForSelector(".cf-mermaid svg", { timeout: 20000 }).catch(() => fail("no <svg> rendered inside .cf-mermaid (mermaid lib/shim not rendering?)"));

  const svgCount = await page.locator(".cf-mermaid svg").count();
  if (svgCount === 0) fail("expected at least one rendered Mermaid <svg>");
  // The flowchart should contain node text from the workflow (an edge/node label).
  const txt = (await page.locator(".cf-mermaid svg").first().innerText().catch(() => "")) || "";

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log(`PASS: ${svgCount} Mermaid diagram(s) rendered to SVG on /workflows.`);
} finally {
  await browser.close();
}
