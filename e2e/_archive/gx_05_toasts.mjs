// GX5 — Toasts & Notices probe
// Surfaces: toast (bottom-center snackbar) + /notifications center + inline .notice
// Viewports: 1280x900 and 768x1024  Themes: dark, light
// Run: node e2e/gx_05_toasts.mjs
// Exit 0 on success; exit 1 on fatal error.

import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOTS = path.join(__dirname, "screenshots");
fs.mkdirSync(SHOTS, { recursive: true });

const VIEWPORTS = [
  { w: 1280, h: 900, label: "1280" },
  { w: 768, h: 1024, label: "768" },
];

const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
const log  = (m) => console.log(m);

// ── helpers ──────────────────────────────────────────────────────────────────

async function waitApp(page) {
  await page.waitForSelector("#app", { timeout: 60_000 });
  // Wait for WASM to finish booting (spinner disappears)
  await page.waitForFunction(
    () => !document.querySelector(".loading-spinner"),
    { timeout: 30_000 }
  ).catch(() => {}); // non-fatal: some builds don't have a spinner class
}

async function setTheme(page, theme) {
  // Use the prefs localStorage key the app reads at boot.
  if (theme === "light") {
    await page.evaluate(() => {
      // Force light data-theme directly; also persist so reload sticks.
      document.documentElement.setAttribute("data-theme", "light");
      localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" }));
    });
  } else {
    await page.evaluate(() => {
      document.documentElement.setAttribute("data-theme", "dark");
      localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "dark" }));
    });
  }
  await page.reload({ waitUntil: "domcontentloaded" });
  await waitApp(page);
}

async function measureToast(page) {
  const el = await page.$(".toast");
  if (!el) return null;
  return page.evaluate((el) => {
    const s = window.getComputedStyle(el);
    const r = el.getBoundingClientRect();
    return {
      background: s.backgroundColor,
      color: s.color,
      borderColor: s.borderColor,
      borderRadius: s.borderRadius,
      boxShadow: s.boxShadow,
      fontSize: s.fontSize,
      padding: s.padding,
      position: s.position,
      bottom: s.bottom,
      left: s.left,
      transform: s.transform,
      zIndex: s.zIndex,
      text: el.querySelector(".toast-msg")?.textContent?.trim() || "",
      hasUndo: !!el.querySelector(".toast-undo"),
      hasDismiss: !!el.querySelector(".toast-x"),
      isErr: el.classList.contains("toast-err"),
      width: r.width,
    };
  }, el);
}

async function measureAriaLive(page) {
  return page.evaluate(() => {
    const regions = [...document.querySelectorAll("[aria-live]")];
    return regions.map((el) => ({
      tagName: el.tagName,
      ariaLive: el.getAttribute("aria-live"),
      role: el.getAttribute("role"),
      classList: el.className,
      text: el.textContent?.trim() || "",
    }));
  });
}

async function screenshot(page, name) {
  const dest = path.join(SHOTS, name);
  await page.screenshot({ path: dest, fullPage: false });
  log(`  📸 ${name}`);
}

// ── action helpers ────────────────────────────────────────────────────────────

async function triggerBillPaid(page) {
  // Navigate to /bills and click the first "Mark Paid" / "Log Payment" button
  await page.goto(BASE + "/bills", { waitUntil: "domcontentloaded" });
  await waitApp(page);
  // Try "Log Payment" or "Mark Paid" text buttons
  const btn = await page.$('button:has-text("Log Payment"), button:has-text("Mark Paid"), button:has-text("Paid")');
  if (btn) {
    await btn.click();
    await page.waitForTimeout(700);
    // If a confirm/form dialog appeared, try to submit it
    const submit = await page.$('button[type="submit"], button:has-text("Save"), button:has-text("Confirm")');
    if (submit) { await submit.click(); await page.waitForTimeout(500); }
    return true;
  }
  return false;
}

async function triggerSubscriptionAction(page) {
  await page.goto(BASE + "/subscriptions", { waitUntil: "domcontentloaded" });
  await waitApp(page);
  // Click "Add Reminder" if present
  const btn = await page.$('button:has-text("Add Reminder"), button:has-text("Remind"), button:has-text("Cancel Sub")');
  if (btn) { await btn.click(); await page.waitForTimeout(700); return true; }
  return false;
}

async function triggerAccountAction(page) {
  await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
  await waitApp(page);
  // Try "Mark all updated" or similar
  const btn = await page.$('button:has-text("Mark"), button:has-text("Updated"), button:has-text("Archive")');
  if (btn) { await btn.click(); await page.waitForTimeout(700); return true; }
  return false;
}

// Force a toast via JS (inject directly into the atom store)
async function injectToast(page, text, isErr = false) {
  return page.evaluate(({ text, isErr }) => {
    // Find the atom by reading the DOM live-region which is always mounted
    // Alternative: dispatch a custom event the WASM listens to — not available.
    // Best we can do: look for a known action button.
    return false;
  }, { text, isErr });
}

// ── main ──────────────────────────────────────────────────────────────────────

const browser = await chromium.launch({ headless: true });
const measurements = {};

try {
  for (const { w, h, label } of VIEWPORTS) {
    for (const theme of ["dark", "light"]) {
      log(`\n── viewport ${w}×${h} | theme: ${theme} ──`);
      const page = await browser.newPage();
      page.setViewportSize({ width: w, height: h });
      const errors = [];
      page.on("pageerror", (e) => errors.push(String(e)));

      // ── Boot ──
      await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
      await waitApp(page);
      await setTheme(page, theme);

      const key = `${theme}_${label}`;

      // ── A11y: aria-live regions at rest ──
      const liveRegions = await measureAriaLive(page);
      log(`  aria-live regions at rest: ${liveRegions.length}`);
      liveRegions.forEach((r) => log(`    [${r.ariaLive}] role=${r.role} class="${r.classList}" text="${r.text}"`));
      measurements[`ariaLive_${key}`] = liveRegions;

      // ── Action 1: Bills — Mark Paid ──
      const billPaid = await triggerBillPaid(page);
      if (billPaid) {
        log(`  bills mark-paid: triggered`);
      } else {
        log(`  bills mark-paid: no button found (silent path)`);
      }
      await page.waitForTimeout(400);
      const toastAfterBill = await measureToast(page);
      log(`  toast after bills action: ${toastAfterBill ? JSON.stringify(toastAfterBill) : "none (silent)"}`);
      measurements[`toastBill_${key}`] = toastAfterBill;
      if (toastAfterBill) {
        await screenshot(page, `gx05_toast_bill_${theme}_${label}.png`);
      }

      // ── Action 2: Subscriptions ──
      await page.goto(BASE + "/subscriptions", { waitUntil: "domcontentloaded" });
      await waitApp(page);
      const subTriggered = await triggerSubscriptionAction(page);
      await page.waitForTimeout(400);
      const toastSub = await measureToast(page);
      log(`  toast after sub action: ${toastSub ? JSON.stringify(toastSub) : "none (silent)"}`);
      measurements[`toastSub_${key}`] = toastSub;
      if (toastSub) {
        await screenshot(page, `gx05_toast_sub_${theme}_${label}.png`);
      }

      // ── Action 3: Accounts ──
      await page.goto(BASE + "/accounts", { waitUntil: "domcontentloaded" });
      await waitApp(page);
      await triggerAccountAction(page);
      await page.waitForTimeout(400);
      const toastAcct = await measureToast(page);
      log(`  toast after account action: ${toastAcct ? JSON.stringify(toastAcct) : "none (silent)"}`);
      measurements[`toastAcct_${key}`] = toastAcct;
      if (toastAcct) {
        await screenshot(page, `gx05_toast_account_${theme}_${label}.png`);
      }

      // ── Notifications center (/notifications) ──
      await page.goto(BASE + "/notifications", { waitUntil: "domcontentloaded" });
      await waitApp(page);
      await screenshot(page, `gx05_noticescenter_${theme}_${label}.png`);

      // Measure card theming
      const centerBg = await page.evaluate(() => {
        const card = document.querySelector(".card, section.card, [class*='card']");
        if (!card) return null;
        const s = window.getComputedStyle(card);
        return { background: s.backgroundColor, color: s.color, borderColor: s.borderColor };
      });
      log(`  /notifications card bg: ${centerBg ? JSON.stringify(centerBg) : "no .card found"}`);
      measurements[`centerCard_${key}`] = centerBg;

      // Check empty state text
      const emptyText = await page.$eval(".empty, .empty-state", (el) => el.textContent?.trim()).catch(() => null);
      log(`  /notifications empty state: ${emptyText || "(not shown — items present or class not found)"}`);
      measurements[`centerEmpty_${key}`] = emptyText;

      // ── Bell button (top-bar) ──
      await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
      await waitApp(page);
      const bellMeasure = await page.evaluate(() => {
        const btn = document.querySelector(".notify-btn");
        if (!btn) return null;
        const s = window.getComputedStyle(btn);
        const r = btn.getBoundingClientRect();
        return {
          background: s.backgroundColor,
          color: s.color,
          borderRadius: s.borderRadius,
          width: r.width,
          height: r.height,
          hasBadge: !!btn.querySelector(".notify-badge"),
          badgeText: btn.querySelector(".notify-badge")?.textContent?.trim() || "",
        };
      });
      log(`  bell button: ${bellMeasure ? JSON.stringify(bellMeasure) : "not found"}`);
      measurements[`bell_${key}`] = bellMeasure;

      // Full shell screenshot at home
      await screenshot(page, `gx05_shell_${theme}_${label}.png`);

      // ── Light mode toast color check — inject by navigating to bills ──
      // (reuse the existing bill trigger since it's the most reliable)
      if (billPaid) {
        await triggerBillPaid(page);
        await page.waitForTimeout(300);
        const toastLive = await measureToast(page);
        if (toastLive) {
          measurements[`toastLive_${key}`] = toastLive;
          await screenshot(page, `gx05_toast_live_${theme}_${label}.png`);
        }
      }

      // ── aria-live with active toast ──
      const liveActive = await measureAriaLive(page);
      measurements[`ariaLiveActive_${key}`] = liveActive;
      log(`  aria-live regions (after actions): ${liveActive.length}`);
      liveActive.forEach((r) => log(`    [${r.ariaLive}] role=${r.role} text="${r.text.slice(0,80)}"`));

      if (errors.length) {
        log(`  page errors: ${errors.join(" | ")}`);
      }

      await page.close();
    }
  }

  // ── Summary ──
  log("\n── Summary ──");
  log("Action→feedback:");
  for (const vp of ["1280", "768"]) {
    const bill = measurements[`toastBill_dark_${vp}`];
    const sub  = measurements[`toastSub_dark_${vp}`];
    const acct = measurements[`toastAcct_dark_${vp}`];
    log(`  [${vp}] bills-paid: ${bill ? `TOAST "${bill.text}"` : "SILENT"}`);
    log(`  [${vp}] subscription action: ${sub ? `TOAST "${sub.text}"` : "SILENT"}`);
    log(`  [${vp}] account action: ${acct ? `TOAST "${acct.text}"` : "SILENT"}`);
  }

  log("\nToast styling (dark, 1280):");
  const t = measurements["toastBill_dark_1280"] || measurements["toastSub_dark_1280"] || measurements["toastAcct_dark_1280"];
  if (t) {
    log(`  bg: ${t.background}`);
    log(`  color: ${t.color}`);
    log(`  border-color: ${t.borderColor}`);
    log(`  border-radius: ${t.borderRadius}`);
    log(`  box-shadow: ${t.boxShadow}`);
    log(`  font-size: ${t.fontSize}`);
    log(`  has-undo: ${t.hasUndo}`);
    log(`  has-dismiss: ${t.hasDismiss}`);
    log(`  is-error: ${t.isErr}`);
    log(`  z-index: ${t.zIndex}`);
  } else {
    log("  No toast triggered in dark/1280 — check individual action logs above.");
  }

  log("\nToast styling (light, 1280):");
  const tl = measurements["toastBill_light_1280"] || measurements["toastSub_light_1280"] || measurements["toastAcct_light_1280"];
  if (tl) {
    log(`  bg: ${tl.background}`);
    log(`  color: ${tl.color}`);
    log(`  border-color: ${tl.borderColor}`);
  } else {
    log("  No toast triggered in light/1280.");
  }

  log("\nNotifications center card (dark vs light):");
  log(`  dark/1280: ${JSON.stringify(measurements["centerCard_dark_1280"])}`);
  log(`  light/1280: ${JSON.stringify(measurements["centerCard_light_1280"])}`);

  log("\nBell button (dark vs light):");
  log(`  dark/1280: ${JSON.stringify(measurements["bell_dark_1280"])}`);
  log(`  light/1280: ${JSON.stringify(measurements["bell_light_1280"])}`);

  log("\naria-live regions (rest, dark, 1280):");
  (measurements["ariaLive_dark_1280"] || []).forEach((r) =>
    log(`  [${r.ariaLive}] role=${r.role} class="${r.classList}" text="${r.text}"`)
  );

  // Save JSON
  const outPath = path.join(SHOTS, "gx05_measurements.json");
  fs.writeFileSync(outPath, JSON.stringify(measurements, null, 2));
  log(`\nMeasurements written to ${outPath}`);

  if (!process.exitCode) log("\nPASS — GX5 toast/notices probe complete.");

} catch (err) {
  console.error("FATAL:", err);
  process.exitCode = 1;
} finally {
  await browser.close();
}
