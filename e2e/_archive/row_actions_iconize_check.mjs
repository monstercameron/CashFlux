// C49/C50/C51/C52/C54/C62/C65 — row action icon-only collapse at narrow widths.
// At 390×844 (phone), per-row action buttons (.row .btn span, .budget-head .btn span)
// must be hidden (display:none) while the button itself remains present and ≥40px tall.
// Primary CTA buttons (Save / Cancel / Add — inside form action bars, not .row) must
// NOT be hidden.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");
const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };

const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.setViewportSize({ width: 390, height: 844 });
  page.on("pageerror", (e) => fail("page error: " + e.message));

  // ── Goals page (/goals) — has .budget-head with Contribute + Edit buttons ──
  await page.goto(BASE + "/goals", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1200);

  // Check that at least one .budget-head .btn span is hidden (display:none)
  const goalSpanHidden = await page.evaluate(() => {
    const spans = document.querySelectorAll(".budget-head .btn span");
    if (spans.length === 0) return null; // no seeded goals — skip, not a fail
    for (const span of spans) {
      const style = window.getComputedStyle(span);
      if (style.display !== "none") return false;
    }
    return true;
  });
  if (goalSpanHidden === false) {
    fail("/goals: .budget-head .btn span text labels are NOT hidden at 390px — row action wrap fix not applied");
  } else if (goalSpanHidden === null) {
    console.log("INFO: /goals has no seeded .budget-head rows — skipping goals span check");
  } else {
    console.log("PASS: /goals .budget-head .btn span labels are display:none at 390px");
  }

  // Check buttons themselves are still present and ≥40px
  const goalBtnOk = await page.evaluate(() => {
    const btns = document.querySelectorAll(".budget-head .btn");
    if (btns.length === 0) return null;
    for (const btn of btns) {
      const rect = btn.getBoundingClientRect();
      if (rect.height < 40) return `button too small: ${rect.height}px`;
    }
    return true;
  });
  if (goalBtnOk === false || (typeof goalBtnOk === "string")) {
    fail("/goals: .budget-head .btn — " + goalBtnOk);
  } else if (goalBtnOk !== null) {
    console.log("PASS: /goals .budget-head .btn buttons are present and ≥40px");
  }

  // ── Accounts page (/accounts) — has .row .btn (Edit, Transactions, Update balance) ──
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1200);

  const acctSpanHidden = await page.evaluate(() => {
    const spans = document.querySelectorAll(".row .btn span");
    if (spans.length === 0) return null;
    for (const span of spans) {
      const style = window.getComputedStyle(span);
      if (style.display !== "none") return false;
    }
    return true;
  });
  if (acctSpanHidden === false) {
    fail("/accounts: .row .btn span text labels are NOT hidden at 390px — row action wrap fix not applied");
  } else if (acctSpanHidden === null) {
    console.log("INFO: /accounts has no seeded .row buttons — skipping");
  } else {
    console.log("PASS: /accounts .row .btn span labels are display:none at 390px");
  }

  const acctBtnOk = await page.evaluate(() => {
    const btns = document.querySelectorAll(".row .btn");
    if (btns.length === 0) return null;
    for (const btn of btns) {
      const rect = btn.getBoundingClientRect();
      if (rect.height < 40) return `button too small: ${rect.height}px (${btn.title || btn.textContent.trim().slice(0,20)})`;
    }
    return true;
  });
  if (typeof acctBtnOk === "string") {
    fail("/accounts: .row .btn — " + acctBtnOk);
  } else if (acctBtnOk !== null) {
    console.log("PASS: /accounts .row .btn buttons are present and ≥40px");
  }

  // ── Budgets page (/budgets) — has .budget-head .btn (Edit) ──
  await page.goto(BASE + "/budgets", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1200);

  const budgetSpanHidden = await page.evaluate(() => {
    const spans = document.querySelectorAll(".budget-head .btn span");
    if (spans.length === 0) return null;
    for (const span of spans) {
      const style = window.getComputedStyle(span);
      if (style.display !== "none") return false;
    }
    return true;
  });
  if (budgetSpanHidden === false) {
    fail("/budgets: .budget-head .btn span text labels are NOT hidden at 390px");
  } else if (budgetSpanHidden === null) {
    console.log("INFO: /budgets has no seeded .budget-head rows — skipping");
  } else {
    console.log("PASS: /budgets .budget-head .btn span labels are display:none at 390px");
  }

  // ── Primary CTA buttons must NOT be hidden ──
  // Primary CTAs (Save, Cancel, Add) live outside .row / .budget-head.
  // Check that a generic .btn span NOT inside .row or .budget-head is still visible.
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1200);

  const ctaVisible = await page.evaluate(() => {
    // A form submit button (.btn[type=submit] or .btn-primary) outside a .row
    const ctaBtns = document.querySelectorAll("form .btn, .btn-primary, button[type='submit'].btn");
    for (const btn of ctaBtns) {
      // Skip if inside .row or .budget-head
      if (btn.closest(".row") || btn.closest(".budget-head")) continue;
      const span = btn.querySelector("span");
      if (!span) continue; // no span to check
      const style = window.getComputedStyle(span);
      if (style.display === "none") return `CTA button span is hidden: "${span.textContent.trim()}"`;
    }
    return true;
  });
  if (typeof ctaVisible === "string") {
    fail("/accounts: primary CTA — " + ctaVisible);
  } else {
    console.log("PASS: primary CTA button spans outside .row/.budget-head are not hidden at 390px");
  }

  if (!process.exitCode) {
    console.log("PASS: C49/C50/C51/C52/C54/C62/C65 — row action buttons collapse to icon-only at 390px (labels hidden, buttons ≥40px, primary CTAs unaffected).");
  }
} finally {
  await browser.close();
}
