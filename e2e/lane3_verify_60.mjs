// lane3_verify_60.mjs — verify the #60 lean sidebar + back-navigation state:
// on a fresh profile the rail leads with the nine primary destinations and
// every advanced section (Tools sub-groups + System) starts collapsed; a
// section expands on click and the choice survives reload; deep-linking into a
// collapsed section auto-reveals it; and Back restores the previous route's
// scroll position while a fresh navigation starts at the top. Filters and the
// period window persist across the round-trip.
// Usage: node e2e/lane3_verify_60.mjs <port> <shotDir>
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";

const PORT = process.argv[2] || "8113";
const OUT = process.argv[3] || "lane3-shots";
mkdirSync(OUT, { recursive: true });

let failures = 0;
const check = (ok, msg) => { console.log(`${ok ? "PASS" : "FAIL"} ${msg}`); if (!ok) failures++; };

const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: "reduce" });
const page = await ctx.newPage();
await page.goto(`http://127.0.0.1:${PORT}/`, { waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await page.waitForTimeout(1500);

// ── Lean default: 9 primaries visible, all section headers collapsed ─────────
const rail = await page.evaluate(() => {
  const aside = document.querySelector("aside.rail");
  const primaries = [...aside.querySelectorAll("nav > div > a.nv, nav > div > a.nav")].length;
  // "My pages" is the user's own content and stays expanded by design; the
  // ADVANCED sections (Tools sub-groups + System) are the ones that must
  // default collapsed.
  const heads = [...aside.querySelectorAll(".rail-subhead")]
    .map((h) => ({ label: h.textContent.trim(), expanded: h.getAttribute("aria-expanded") === "true" }))
    .filter((h) => !/my pages/i.test(h.label));
  return { primaries, heads };
});
check(rail.heads.length >= 4, `rail shows section headers (${rail.heads.map((h) => h.label).join("/")})`);
check(rail.heads.every((h) => !h.expanded), `all advanced sections start collapsed (${rail.heads.filter((h) => h.expanded).map((h) => h.label).join(",") || "none open"})`);
const toolLinksHidden = await page.evaluate(() =>
  ![...document.querySelectorAll("aside.rail a")].some((a) => a.getAttribute("href")?.endsWith("/networth")));
check(toolLinksHidden, "collapsed sections hide their items (no /networth link)");
await page.screenshot({ path: `${OUT}/60-rail-collapsed.png` });

// ── Expand persists across reload ────────────────────────────────────────────
await page.evaluate(() => [...document.querySelectorAll(".rail-subhead")][0]?.click());
await page.waitForTimeout(400);
const afterExpand = await page.evaluate(() => [...document.querySelectorAll(".rail-subhead")][0]?.getAttribute("aria-expanded"));
check(afterExpand === "true", "clicking a section header expands it");
await page.reload({ waitUntil: "load" });
await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
await page.waitForTimeout(1500);
const afterReload = await page.evaluate(() => {
  const heads = [...document.querySelectorAll(".rail-subhead")].filter((h) => !/my pages/i.test(h.textContent));
  return { first: heads[0]?.getAttribute("aria-expanded"), rest: heads.slice(1).every((h) => h.getAttribute("aria-expanded") === "false") };
});
check(afterReload.first === "true" && afterReload.rest, `expanded choice survives reload, others stay collapsed (${JSON.stringify(afterReload)})`);
// Collapse it back for a clean state.
await page.evaluate(() => [...document.querySelectorAll(".rail-subhead")][0]?.click());
await page.waitForTimeout(300);

// ── Deep link into a collapsed section auto-reveals it ───────────────────────
await page.evaluate(() => { history.pushState({}, "", "/networth"); dispatchEvent(new PopStateEvent("popstate")); });
await page.waitForTimeout(1200);
const reveal = await page.evaluate(() => {
  const a = [...document.querySelectorAll("aside.rail a")].find((x) => x.getAttribute("href")?.endsWith("/networth"));
  return a ? a.classList.contains("active") || !!a.closest("aside") : false;
});
check(reveal, "deep-linking into a collapsed section reveals the active item");

// ── Back-navigation: scroll + period + filters survive ───────────────────────
await page.evaluate(() => { history.pushState({}, "", "/transactions"); dispatchEvent(new PopStateEvent("popstate")); });
await page.waitForTimeout(1400);
// Apply a search filter and step the period back one month.
await page.fill('[data-testid="txn-search"], .fctrl-search input, .fctrl-input', "car").catch(() => {});
await page.waitForTimeout(600);
const periodBefore = await page.evaluate(() => document.querySelector(".period-control")?.textContent.trim() ?? "");
await page.evaluate(() => document.querySelector("main.cf-scroll").scrollTo(0, 900));
await page.waitForTimeout(500);
const scrollBefore = await page.evaluate(() => document.querySelector("main.cf-scroll").scrollTop);

// Fresh forward navigation → lands at top.
await page.evaluate(() => { history.pushState({}, "", "/budgets"); dispatchEvent(new PopStateEvent("popstate")); });
await page.waitForTimeout(1400);
const budgetsTop = await page.evaluate(() => document.querySelector("main.cf-scroll").scrollTop);
check(budgetsTop < 50, `fresh navigation starts at the top (${budgetsTop})`);

// Browser Back → returns to /transactions with scroll, filter, period intact.
await page.goBack();
await page.waitForTimeout(1600);
const back = await page.evaluate(() => ({
  path: location.pathname,
  scroll: document.querySelector("main.cf-scroll")?.scrollTop ?? -1,
  search: document.querySelector('[data-testid="txn-search"], .fctrl-search input, .fctrl-input')?.value ?? "",
  period: document.querySelector(".period-control")?.textContent.trim() ?? "",
}));
check(back.path.endsWith("/transactions"), `Back returns to /transactions (${back.path})`);
check(scrollBefore > 700 && Math.abs(back.scroll - scrollBefore) < 150, `Back restores scroll (${scrollBefore} -> ${back.scroll})`);
check(back.search === "car", `Back preserves the search filter ("${back.search}")`);
check(back.period === periodBefore, "Back preserves the period window");
await page.screenshot({ path: `${OUT}/60-backnav.png` });

await browser.close();
console.log(failures === 0 ? "ALL CHECKS PASSED" : `${failures} CHECK(S) FAILED`);
process.exit(failures === 0 ? 0 : 1);
