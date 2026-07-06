/**
 * GX17 — Loading & Skeleton States probe
 * Captures boot/initial-load state, notReady guard, aiLoading markup,
 * thinking bubble, button busy states. Both dark + light themes.
 * Run: node e2e/gx_17_loading.mjs
 */
import { chromium } from "playwright";
import { writeFileSync } from "fs";

const BASE = "http://localhost:8080";
const OUT = "e2e/screenshots";

async function shot(page, name) {
  const p = `${OUT}/${name}`;
  await page.screenshot({ path: p, fullPage: false });
  console.log("  📸", name);
  return p;
}

async function setLight(page) {
  await page.evaluate(() =>
    localStorage.setItem(
      "cashflux:prefs",
      JSON.stringify({ theme: "light" })
    )
  );
  await page.reload();
  await page.waitForFunction(
    () => document.documentElement.getAttribute("data-theme") === "light"
  );
}

async function setDark(page) {
  await page.evaluate(() =>
    localStorage.setItem(
      "cashflux:prefs",
      JSON.stringify({ theme: "dark" })
    )
  );
  await page.reload();
  await page
    .waitForFunction(
      () =>
        !document.documentElement.getAttribute("data-theme") ||
        document.documentElement.getAttribute("data-theme") === "dark"
    )
    .catch(() => {});
}

async function measure(page, selector) {
  return page.evaluate((sel) => {
    const el = document.querySelector(sel);
    if (!el) return null;
    const s = getComputedStyle(el);
    return {
      color: s.color,
      background: s.background,
      backgroundColor: s.backgroundColor,
      fontStyle: s.fontStyle,
      display: s.display,
      opacity: s.opacity,
    };
  }, selector);
}

async function dismissBuildError(page) {
  // The gwc dev server shows a "Build failed" overlay — dismiss it so we can
  // see the actual app screen underneath (stale wasm still runs fine).
  const btn = page.locator('button:has-text("dismiss")');
  if (await btn.count() > 0) {
    await btn.first().click();
    await page.waitForTimeout(300);
  }
}

async function domReport(page, selector) {
  return page.evaluate((sel) => {
    const els = [...document.querySelectorAll(sel)];
    return els.map((e) => ({
      tag: e.tagName,
      class: e.className,
      text: e.innerText?.slice(0, 80),
      disabled: e.disabled,
    }));
  }, selector);
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await ctx.newPage();

  // ── 1. BOOT SPLASH (capture before wasm loads) ──────────────────────────────
  console.log("\n=== 1. BOOT SPLASH (dark) ===");
  await page.goto(BASE, { waitUntil: "domcontentloaded" });
  // Don't wait for wasm — capture the splash while it's still showing
  await page.waitForTimeout(300);
  const bootVisible = await page.evaluate(() => {
    const boot = document.getElementById("boot");
    return boot ? getComputedStyle(boot).display !== "none" : false;
  });
  console.log("  boot div visible:", bootVisible);
  await shot(page, "gx17_boot_dark.png");

  const bootRingArc = await measure(page, ".boot-ring-arc");
  console.log("  .boot-ring-arc stroke (computed bg proxy):", bootRingArc);

  const bootCard = await measure(page, ".boot-card");
  console.log("  .boot-card bg:", bootCard?.backgroundColor);

  // ── 2. BOOT SPLASH LIGHT ────────────────────────────────────────────────────
  console.log("\n=== 2. BOOT SPLASH (light) ===");
  await setLight(page);
  // Navigate to force a fresh load so boot splash appears
  await page.goto(BASE, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(300);
  await shot(page, "gx17_boot_light.png");

  const bootRingArcLight = await measure(page, ".boot-ring-arc");
  console.log("  .boot-ring-arc light:", bootRingArcLight);
  const bootCardLight = await measure(page, ".boot-card");
  console.log("  .boot-card light bg:", bootCardLight?.backgroundColor);
  const bootWord = await measure(page, ".boot-word");
  console.log("  .boot-word color:", bootWord?.color);
  const bootSub = await measure(page, ".boot-sub");
  console.log("  .boot-sub color:", bootSub?.color);

  // ── 3. WAIT FOR APP TO FULLY LOAD, then check notReady ─────────────────────
  console.log("\n=== 3. INITIAL LOAD — notReady guard ===");
  // wait for wasm mount
  await page.waitForSelector("#app > *", { timeout: 30000 }).catch(() => {});
  await page.waitForTimeout(2000);
  await shot(page, "gx17_initial_light.png");

  const notReadyEls = await domReport(page, ".empty");
  console.log("  .empty elements on initial load:", JSON.stringify(notReadyEls));

  // ── 4. NAVIGATE TO TRANSACTIONS (notReady guard) ─────────────────────────
  console.log("\n=== 4. TRANSACTIONS — notReady guard ===");
  await page.goto(BASE + "/#/transactions", { waitUntil: "load" });
  await page.waitForTimeout(1500);
  await dismissBuildError(page);
  await shot(page, "gx17_transactions_light.png");
  const txEmpty = await domReport(page, ".empty");
  console.log("  .empty on /transactions:", JSON.stringify(txEmpty));
  const emptyStyle = await measure(page, ".empty");
  console.log("  .empty style:", emptyStyle);

  // ── 5. ACCOUNTS notReady ────────────────────────────────────────────────────
  console.log("\n=== 5. ACCOUNTS — notReady guard ===");
  await page.goto(BASE + "/#/accounts", { waitUntil: "load" });
  await page.waitForTimeout(1500);
  await dismissBuildError(page);
  await shot(page, "gx17_accounts_light.png");

  // ── 6. INSIGHTS — loading state ─────────────────────────────────────────────
  console.log("\n=== 6. INSIGHTS ===");
  await page.goto(BASE + "/#/insights", { waitUntil: "load" });
  await page.waitForTimeout(1500);
  await dismissBuildError(page);
  await shot(page, "gx17_insights_light.png");
  const thinkingEl = await domReport(page, "[class*='rounded-2xl'][class*='text-faint'], .rounded-2xl");
  console.log("  thinking bubble (static):", JSON.stringify(thinkingEl.slice(0, 3)));
  const sendBtn = await domReport(page, "button.btn.btn-primary");
  console.log("  send/submit buttons:", JSON.stringify(sendBtn));

  // ── 7. DOCUMENTS — loading state ─────────────────────────────────────────────
  console.log("\n=== 7. DOCUMENTS ===");
  await page.goto(BASE + "/#/documents", { waitUntil: "load" });
  await page.waitForTimeout(1500);
  await dismissBuildError(page);
  await shot(page, "gx17_documents_light.png");
  const docBtns = await domReport(page, "button");
  console.log("  document buttons:", JSON.stringify(docBtns.slice(0, 8)));

  // ── 8. ALLOCATE — aiLoading state ─────────────────────────────────────────
  console.log("\n=== 8. ALLOCATE ===");
  await page.goto(BASE + "/#/allocate", { waitUntil: "load" });
  await page.waitForTimeout(1500);
  await dismissBuildError(page);
  await shot(page, "gx17_allocate_light.png");

  // ── 9. DARK THEME — same key screens ─────────────────────────────────────
  console.log("\n=== 9. DARK THEME CHECKS ===");
  await setDark(page);

  await page.goto(BASE + "/#/insights", { waitUntil: "load" });
  await page.waitForTimeout(1500);
  await dismissBuildError(page);
  await shot(page, "gx17_insights_dark.png");
  const thinkingDark = await measure(page, "#app");
  console.log("  dark page bg:", thinkingDark?.backgroundColor);

  await page.goto(BASE + "/#/documents", { waitUntil: "load" });
  await page.waitForTimeout(1500);
  await dismissBuildError(page);
  await shot(page, "gx17_documents_dark.png");

  // boot dark
  await page.goto(BASE, { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(300);
  await shot(page, "gx17_boot_dark2.png");

  // ── 10. MEASURE — thinking bubble in light mode ──────────────────────────
  console.log("\n=== 10. THINKING BUBBLE — light-mode measurements ===");
  await setLight(page);
  await page.goto(BASE + "/#/insights", { waitUntil: "load" });
  await page.waitForTimeout(1500);
  await dismissBuildError(page);

  // The "thinking" div only shows when loading.Get() is true — inspect its CSS class chain
  // from source: tw.BgBlack04 = "rgb(0 0 0 / 0.04)", tw.TextFaint = var(--text-faint)
  // Simulate by injecting the element
  const thinkingPreview = await page.evaluate(() => {
    const d = document.createElement("div");
    d.style.cssText = "max-width:85%; border-radius:1rem; background:rgb(0 0 0/0.04); padding:0.5rem 0.875rem; font-size:0.8125rem;";
    d.textContent = "Thinking…";
    document.body.appendChild(d);
    const cs = getComputedStyle(d);
    const result = { bg: cs.backgroundColor, color: cs.color };
    document.body.removeChild(d);
    return result;
  });
  console.log("  thinking bubble bg in light (synthesized):", thinkingPreview.bg);
  console.log("  thinking bubble color in light (synthesized):", thinkingPreview.color);

  // Check text-faint in light mode
  const textFaintLight = await page.evaluate(() => {
    const el = document.documentElement;
    return getComputedStyle(el).getPropertyValue("--text-faint");
  });
  console.log("  --text-faint (light):", textFaintLight);

  // Check bgBlack04 contrast vs white
  // rgba(0,0,0,0.04) on white = ~rgba(245,245,245,1) approx
  // Luminance check: text #686870 on rgba(0,0,0,0.04)≈#f5f5f5 — should be fine

  // ── 11. MISSING: spinner/skeleton checks ───────────────────────────────────
  console.log("\n=== 11. SKELETON / SPINNER audit ===");
  const spinnerEls = await page.evaluate(() => {
    return [...document.querySelectorAll('[class*="spin"], [class*="skeleton"], [class*="shimmer"], [role="status"], [aria-busy]')]
      .map(e => ({ tag: e.tagName, class: e.className, text: e.innerText?.slice(0, 40) }));
  });
  console.log("  skeleton/spinner elements in DOM:", JSON.stringify(spinnerEls));

  await browser.close();
  console.log("\n✅ GX17 probe complete. Screenshots in", OUT);
})();
