// mobile_shell_verify.mjs — locks the 390x844 phone-shell contract (parity
// scan: rail/promo obstruction + menus 5-9 losing primary navigation):
//   1. The icon rail is 56px and never overlaps content; no footer prose.
//   2. The top bar wraps: title row + full-width context strip, no overlap.
//   3. The bottom tab bar reaches ALL nine primary destinations (scrollable),
//      pins +Add, and scrolls the active tab into view.
//   4. Menus 5-9 render with working navigation.
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

// 1. Rail geometry + no prose.
const geo = await page.evaluate(() => {
  const rail = document.querySelector("aside.rail");
  const main = document.querySelector("main");
  const foot = document.querySelector("aside.rail .rail-foot-info");
  const wsHead = document.querySelector("aside.rail .ws-switch-head");
  const visible = (el) => el && getComputedStyle(el).display !== "none";
  return {
    railW: rail ? rail.getBoundingClientRect().width : -1,
    mainX: main ? main.getBoundingClientRect().x : -1,
    footVisible: visible(foot),
    wsHeadVisible: visible(wsHead),
  };
});
check("rail is a 56px icon column beside (not over) content", geo.railW === 56 && geo.mainX >= 56, JSON.stringify(geo));
check("rail footer prose is hidden on phones", !geo.footVisible && !geo.wsHeadVisible);

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

// 3. Tab bar reaches all nine destinations.
const tabInfo = await page.evaluate(() => {
  const items = [...document.querySelectorAll(".mobile-tabbar .mobile-tab-item:not(.mobile-tab-add)")];
  const add = document.querySelector(".mobile-tabbar .mobile-tab-add");
  return { count: items.length, labels: items.map((i) => i.getAttribute("aria-label")), addVisible: add ? getComputedStyle(add).display !== "none" : false };
});
check("tab bar lists all nine destinations", tabInfo.count === 9, JSON.stringify(tabInfo.labels));
check("+Add stays pinned", tabInfo.addVisible);
await page.screenshot({ path: "e2e/mobile_shell_dash.png" });

// 4. Navigate to a menu-5-9 page via the tab bar: Notifications.
const notifTab = page.locator('.mobile-tabbar .mobile-tab-item[aria-label*="Notification"], .mobile-tabbar .mobile-tab-item:has-text("Notifications")').first();
await notifTab.scrollIntoViewIfNeeded();
await notifTab.click();
await page.waitForTimeout(1600);
check("tab navigates to /notifications", page.url().endsWith("/notifications"), page.url());
const activeVisible = await page.evaluate(() => {
  const el = document.querySelector(".mobile-tabbar .mobile-tab-item.active");
  if (!el) return false;
  const r = el.getBoundingClientRect();
  const bar = document.querySelector(".mobile-tab-scroll").getBoundingClientRect();
  return r.x >= bar.x - 2 && r.x + r.width <= bar.x + bar.width + 2;
});
check("active tab is scrolled into view", activeVisible);
await page.screenshot({ path: "e2e/mobile_shell_notif.png" });

console.log(`\npageerrors: ${errors.length} ${errors.slice(0, 2).join(" | ")}`);
console.log(`RESULT: ${pass} passed, ${fail} failed`);
await browser.close();
process.exit(fail === 0 ? 0 : 1);
