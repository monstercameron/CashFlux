// mobile_shell_verify.mjs — locks the 390x844 phone-shell contract (parity
// scan: rail/promo obstruction + menus 5-9 losing primary navigation), updated
// for the UX-01 single-nav design (f82f0749):
//   1. The rail is REMOVED at phone width — the bottom bar is the only nav.
//   2. The top bar wraps: title row + full-width context strip, no overlap.
//   3. The bar holds four destinations + a More toggle, plus a floating
//      quick-add; the More sheet reaches the remaining five (menus 5-9).
//   4. Navigating via the sheet works and the More tab lights up for a
//      sheet-resident route, so the active destination always shows.
// Usage: node e2e/mobile_shell_verify.mjs   (server on :8097 serving web/)
import { chromium } from "playwright";
const BASE = "http://127.0.0.1:8097";
let pass = 0, fail = 0;
const check = (n, ok, d = "") => { console.log(`${ok ? "PASS" : "FAIL"}: ${n}${d ? " — " + d : ""}`); ok ? pass++ : fail++; };
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, reducedMotion: "reduce" })).newPage();
const errors = []; page.on("pageerror", (e) => errors.push(String(e)));
await page.goto(BASE + "/dashboard", { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 60000 });
await page.waitForTimeout(2200);

// 1. Single nav: the rail is gone; main owns the full width.
const geo = await page.evaluate(() => {
  const rail = document.querySelector("aside.rail");
  const main = document.querySelector("main");
  const railVisible = rail && getComputedStyle(rail).display !== "none";
  return { railVisible, mainX: main ? Math.round(main.getBoundingClientRect().x) : -1, mainW: main ? Math.round(main.getBoundingClientRect().width) : -1 };
});
check("rail is removed at phone width (single nav)", !geo.railVisible, JSON.stringify(geo));
check("content owns the full width", geo.mainX <= 4 && geo.mainW >= 380, JSON.stringify(geo));

// 2. Top bar wrap: title and context strip don't overlap.
const tb = await page.evaluate(() => {
  const title = document.querySelector(".tb-title");
  const ctxEl = document.querySelector(".tb-context");
  const t = title ? title.getBoundingClientRect() : null;
  const c = ctxEl ? ctxEl.getBoundingClientRect() : null;
  return { titleW: t ? Math.round(t.width) : -1, ctxW: c ? Math.round(c.width) : -1, ctxBelowTitle: t && c ? c.y >= t.y + t.height - 2 : false };
});
check("top-bar title has real width", tb.titleW > 40, JSON.stringify(tb));
check("context strip wraps to its own full-width row", tb.ctxW >= 260 && tb.ctxBelowTitle, JSON.stringify(tb));

// 3. Bar inventory: four destinations + More toggle + floating quick-add.
const barInfo = await page.evaluate(() => {
  const items = [...document.querySelectorAll(".mobile-tabbar .mobile-tab-item:not(.mobile-tab-more)")];
  const more = document.querySelector('[data-testid="mobile-tab-more"]');
  const fab = document.querySelector('[data-testid="mobile-tab-fab"]');
  const visible = (el) => el && getComputedStyle(el).display !== "none";
  return { count: items.length, labels: items.map((i) => i.getAttribute("aria-label")), more: visible(more), fab: visible(fab) };
});
check("bar holds four fixed destinations", barInfo.count === 4, JSON.stringify(barInfo.labels));
check("More toggle present", barInfo.more);
check("floating quick-add present", barInfo.fab);
await page.screenshot({ path: "e2e/mobile_shell_dash.png" });

// 4. The More sheet reaches menus 5-9 and navigates.
await page.locator('[data-testid="mobile-tab-more"]').click();
await page.waitForTimeout(700);
const sheet = page.locator('[data-testid="mobile-more-sheet"]');
check("More sheet opens", (await sheet.count()) === 1);
const sheetLabels = await sheet.locator(".mobile-sheet-item").evaluateAll((els) => els.map((e) => e.getAttribute("aria-label")));
check("sheet reaches the remaining five destinations", sheetLabels.length === 5, JSON.stringify(sheetLabels));
await sheet.locator('.mobile-sheet-item[aria-label*="Notification"]').first().click();
await page.waitForTimeout(1600);
check("sheet item navigates to /notifications", page.url().endsWith("/notifications"), page.url());
check("sheet closed after picking", (await page.locator('[data-testid="mobile-more-sheet"]').count()) === 0);
const moreActive = await page.locator('[data-testid="mobile-tab-more"]').evaluate((el) => el.className.includes("active"));
check("More tab lights up for a sheet-resident route", moreActive);
await page.screenshot({ path: "e2e/mobile_shell_notif.png" });

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 2).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
