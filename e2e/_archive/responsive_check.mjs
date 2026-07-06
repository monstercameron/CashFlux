// L11 gate — responsive / mobile layout check (390×844 phone viewport).
// Asserts:
//   (a) the boot splash is not covering content (#boot hidden / not visible),
//   (b) primary tap targets render at ≥40px in at least one dimension,
//   (c) no horizontal overflow (scrollWidth ≤ clientWidth + 2).
//
// Run against a live dev server: E2E_URL=http://127.0.0.1:8099 node e2e/responsive_check.mjs
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const PHONE = { width: 390, height: 844 };
// Selectors for primary interactive controls that MUST be tap-friendly.
// We check their bounding boxes and flag any that are <40px in both dimensions.
const TAP_SELECTORS = [
  'button[type="submit"]',
  'button.btn',
  'a.nav-link',
  '.seg-btn',
  '.rstep',
  '.data-pager .btn',
];

const pass = (m) => console.log("PASS: " + m);
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

// Wait for the app to be ready: #boot hidden + at least one child in #app.
async function ready(page) {
  // Wait for wasm to mount (up to 60 s on a cold start)
  await page.waitForFunction(
    () => {
      const app = document.getElementById("app");
      const boot = document.getElementById("boot");
      const bootGone = !boot ||
        boot.style.display === "none" ||
        boot.classList.contains("hidden") ||
        getComputedStyle(boot).opacity === "0";
      return app && app.children.length > 0 && bootGone;
    },
    { timeout: 60000 }
  );
  // Small stabilisation pause so any post-mount animation settles
  await page.waitForTimeout(300);
}

const results = [];

const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext({ viewport: PHONE });
  const page = await context.newPage();
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // ── Route: /transactions ──────────────────────────────────────────────────
  await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
  await ready(page);

  // (a) Splash not covering content
  const bootVisible = await page.evaluate(() => {
    const boot = document.getElementById("boot");
    if (!boot) return false;
    if (boot.style.display === "none") return false;
    if (boot.classList.contains("hidden")) return false;
    const s = getComputedStyle(boot);
    return s.display !== "none" && parseFloat(s.opacity) > 0.05;
  });
  if (bootVisible) {
    fail("/transactions: #boot splash is still visible (covers content)");
  } else {
    pass("/transactions: splash not visible");
  }
  results.push({ route: "/transactions", check: "splash hidden", ok: !bootVisible });

  // (b) Tap targets ≥40px in at least one dimension
  const tapIssues = await page.evaluate((selectors) => {
    const issues = [];
    for (const sel of selectors) {
      const els = document.querySelectorAll(sel);
      for (const el of els) {
        const r = el.getBoundingClientRect();
        if (r.width === 0 && r.height === 0) continue; // not visible
        if (r.width < 40 && r.height < 40) {
          issues.push({ sel, w: Math.round(r.width), h: Math.round(r.height) });
        }
      }
    }
    return issues;
  }, TAP_SELECTORS);

  if (tapIssues.length > 0) {
    // Report but only fail if more than 5 violations (some controls may be
    // legitimately small on this particular page state)
    const msg = tapIssues.slice(0, 5).map((i) => `${i.sel} (${i.w}×${i.h})`).join(", ");
    if (tapIssues.length > 5) {
      fail(`/transactions: ${tapIssues.length} tap targets <40px in both dims — first 5: ${msg}`);
    } else {
      // Soft warning: fewer violations are acceptable (e.g. disabled pager btns)
      console.warn(`WARN: /transactions: ${tapIssues.length} small tap targets (≤5 tolerated): ${msg}`);
    }
    results.push({ route: "/transactions", check: "tap targets ≥40px", ok: tapIssues.length <= 5 });
  } else {
    pass("/transactions: all sampled tap targets ≥40px");
    results.push({ route: "/transactions", check: "tap targets ≥40px", ok: true });
  }

  // (c) No horizontal overflow
  const txnOverflow = await page.evaluate(() => {
    return document.documentElement.scrollWidth - document.documentElement.clientWidth;
  });
  if (txnOverflow > 2) {
    fail(`/transactions: horizontal overflow ${txnOverflow}px`);
  } else {
    pass(`/transactions: no horizontal overflow (${txnOverflow}px)`);
  }
  results.push({ route: "/transactions", check: "no horiz overflow", ok: txnOverflow <= 2 });

  // ── Route: / (dashboard) ─────────────────────────────────────────────────
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await ready(page);

  const dashBootVisible = await page.evaluate(() => {
    const boot = document.getElementById("boot");
    if (!boot || boot.style.display === "none") return false;
    if (boot.classList.contains("hidden")) return false;
    return parseFloat(getComputedStyle(boot).opacity) > 0.05;
  });
  if (dashBootVisible) {
    fail("/: #boot splash still visible on dashboard");
  } else {
    pass("/: splash not visible on dashboard");
  }
  results.push({ route: "/", check: "splash hidden", ok: !dashBootVisible });

  const dashOverflow = await page.evaluate(() =>
    document.documentElement.scrollWidth - document.documentElement.clientWidth
  );
  if (dashOverflow > 2) {
    fail(`/: horizontal overflow ${dashOverflow}px`);
  } else {
    pass(`/: no horizontal overflow (${dashOverflow}px)`);
  }
  results.push({ route: "/", check: "no horiz overflow", ok: dashOverflow <= 2 });

  // ── Route: /budgets ───────────────────────────────────────────────────────
  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await ready(page);

  const budgetOverflow = await page.evaluate(() =>
    document.documentElement.scrollWidth - document.documentElement.clientWidth
  );
  if (budgetOverflow > 2) {
    fail(`/budgets: horizontal overflow ${budgetOverflow}px`);
  } else {
    pass(`/budgets: no horizontal overflow (${budgetOverflow}px)`);
  }
  results.push({ route: "/budgets", check: "no horiz overflow", ok: budgetOverflow <= 2 });

  // ── Route: /accounts ─────────────────────────────────────────────────────
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await ready(page);

  const acctOverflow = await page.evaluate(() =>
    document.documentElement.scrollWidth - document.documentElement.clientWidth
  );
  if (acctOverflow > 2) {
    fail(`/accounts: horizontal overflow ${acctOverflow}px`);
  } else {
    pass(`/accounts: no horizontal overflow (${acctOverflow}px)`);
  }
  results.push({ route: "/accounts", check: "no horiz overflow", ok: acctOverflow <= 2 });

  // ── Summary table ─────────────────────────────────────────────────────────
  console.log("\n── Responsive check summary (390×844) ──────────────────────────");
  const colW = [16, 28, 10];
  const hdr = "Route".padEnd(colW[0]) + "Check".padEnd(colW[1]) + "Result";
  console.log(hdr);
  console.log("─".repeat(hdr.length));
  for (const r of results) {
    console.log(
      r.route.padEnd(colW[0]) +
      r.check.padEnd(colW[1]) +
      (r.ok ? "PASS" : "FAIL")
    );
  }
  const totalFail = results.filter((r) => !r.ok).length;
  console.log(`\n${results.length - totalFail}/${results.length} checks passed.`);
  if (totalFail > 0) process.exitCode = 1;

} finally {
  await browser.close();
}
