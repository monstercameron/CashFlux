// L2/L3/L6/L11 regression gate — "boot splash fully dismissed on deep-link routes".
// Hard-loads /transactions and /accounts (both are real deep-link destinations),
// waits for the app to mount, then asserts the #boot splash is:
//   1. Not visible (display:none OR opacity=0 + pointer-events:none)
//   2. NOT the topmost element at the viewport centre (elementFromPoint)
// Tested at 1280×800 (desktop) and 390×844 (phone).
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

const VIEWPORTS = [
  { label: "1280×800 (desktop)", width: 1280, height: 800 },
  { label: "390×844 (phone)", width: 390, height: 844 },
];
const ROUTES = ["/transactions", "/accounts"];

let passed = 0;
let failed = 0;
const pass = (label) => { console.log(`PASS: ${label}`); passed++; };
const fail = (label) => { console.error(`FAIL: ${label}`); failed++; process.exitCode = 1; };

// Returns a structured state object for #boot so we can report specifics.
async function bootState(page) {
  return page.evaluate(() => {
    const b = document.getElementById("boot");
    if (!b) return { gone: true, reason: "element absent" };
    const cs = getComputedStyle(b);
    const display = cs.display;
    const opacity = parseFloat(cs.opacity);
    const pointer = cs.pointerEvents;
    const zIndex = cs.zIndex;
    const hidden = b.classList.contains("hidden");

    // Check whether #boot is the top element at viewport centre
    const cx = Math.floor(window.innerWidth / 2);
    const cy = Math.floor(window.innerHeight / 2);
    const top = document.elementFromPoint(cx, cy);
    const bootCoversCenter = top && (top === b || b.contains(top));

    const gone =
      display === "none" ||
      (opacity === 0 && pointer === "none") ||
      (opacity === 0 && hidden);

    return { gone, display, opacity, pointer, zIndex, hidden, bootCoversCenter };
  });
}

const browser = await chromium.launch({ headless: true });

try {
  for (const vp of VIEWPORTS) {
    const ctx = await browser.newContext({ viewport: { width: vp.width, height: vp.height } });
    const page = await ctx.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));

    for (const route of ROUTES) {
      const label = `${route} @ ${vp.label}`;

      // Hard navigate (no prior warm-up cache) — simulates deep-link refresh
      await page.goto(BASE + route, { waitUntil: "domcontentloaded" });

      // Wait until the app has actually mounted children inside #app
      await page.waitForFunction(
        () => {
          const a = document.getElementById("app");
          return a && a.children.length > 0;
        },
        { timeout: 60000 }
      );

      // Allow the fade + display:none removal to complete
      await page.waitForTimeout(1000);

      const s = await bootState(page);

      if (!s.gone) {
        fail(`boot splash still visible on ${label}: display=${s.display} opacity=${s.opacity} pointer=${s.pointer}`);
      } else {
        pass(`boot splash dismissed on ${label} (display=${s.display} opacity=${s.opacity})`);
      }

      if (s.bootCoversCenter) {
        fail(`boot splash is topmost element at viewport centre on ${label} — it is still intercepting interactions`);
      } else {
        pass(`boot splash does NOT cover viewport centre on ${label}`);
      }
    }

    if (errors.length) {
      fail(`page errors at ${vp.label}: ${errors.join(" | ")}`);
    }

    await ctx.close();
  }
} finally {
  await browser.close();
}

console.log(`\nResult: ${passed} passed, ${failed} failed.`);
if (!process.exitCode) console.log("PASS: splash_dismissed_check — all assertions green.");
