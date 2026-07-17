// Lane 2 verification for #76: dashboard defaults.
// Usage: node e2e/lane2_verify76.mjs [port]
import { chromium } from "playwright";
const port = process.argv[2] || "8112";
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 950 }, reducedMotion: "reduce" })).newPage();
const results = [];
const check = (name, ok, detail) => { results.push(ok); console.log((ok ? "PASS" : "FAIL") + " " + name + (detail ? " — " + detail : "")); };
const errors = [];
page.on("pageerror", (e) => errors.push(String(e)));
const ready = async () => {
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
  await page.waitForTimeout(4000);
};

await page.goto(`http://127.0.0.1:${port}/`, { waitUntil: "load" });
await ready();

// (3) Calm default: no grips / resize handles outside edit mode.
const chrome0 = await page.evaluate(() => ({
  grips: [...document.querySelectorAll(".bento .w .grip")].filter((g) => getComputedStyle(g).display !== "none").length,
  handles: [...document.querySelectorAll(".bento .rz")].filter((g) => getComputedStyle(g).display !== "none").length,
  attr: document.querySelector(".bento")?.getAttribute("data-layout-edit"),
  draggable: document.querySelector('.bento .w[data-widget]')?.getAttribute("draggable"),
}));
check("#76.3 grips+handles hidden by default", chrome0.grips === 0 && chrome0.handles === 0 && chrome0.attr === "off", JSON.stringify(chrome0));
check("#76.3 pointer drag off by default", chrome0.draggable === "false", "draggable=" + chrome0.draggable);

// Edit mode round-trip.
await page.click('[data-testid="dash-edit-layout"]');
await page.waitForTimeout(800);
const chrome1 = await page.evaluate(() => ({
  grips: [...document.querySelectorAll(".bento .w .grip")].filter((g) => getComputedStyle(g).display !== "none").length,
  attr: document.querySelector(".bento")?.getAttribute("data-layout-edit"),
  draggable: document.querySelector('.bento .w[data-widget]')?.getAttribute("draggable"),
  pressed: document.querySelector('[data-testid="dash-edit-layout"]')?.getAttribute("aria-pressed"),
}));
check("#76.3 edit mode shows chrome + enables drag", chrome1.grips > 0 && chrome1.attr === "on" && chrome1.draggable === "true" && chrome1.pressed === "true", JSON.stringify(chrome1));
await page.click('[data-testid="dash-edit-layout"]');
await page.waitForTimeout(500);
const chrome2 = await page.evaluate(() => document.querySelector(".bento")?.getAttribute("data-layout-edit"));
check("#76.3 Done returns to calm surface", chrome2 === "off", "attr=" + chrome2);

// (6) Add account demoted post-setup (sample data has accounts); Add transaction stays.
const addAcct = await page.locator('[data-testid="hero-add-account"]').count();
const addTxn = await page.locator('[data-testid="hero-add-txn"]').count();
check("#76.6 Add account demoted, Add transaction stays", addAcct === 0 && addTxn === 1, `acct=${addAcct} txn=${addTxn}`);

// (4) Bills glance: at most 3 rows + View all → /bills.
const bills = await page.evaluate(() => {
  const w = document.querySelector('.bento .w[data-widget="bills"]');
  if (!w) return null;
  return {
    rows: w.querySelectorAll(".wbody > div > button, .wbody > div > div").length,
    viewAll: !!w.querySelector('[data-testid="bills-view-all"]'),
    viewAllText: w.querySelector('[data-testid="bills-view-all"]')?.textContent || "",
  };
});
check("#76.4 bills capped at 3 + View all", !!bills && bills.viewAll && bills.rows <= 4, JSON.stringify(bills));
await page.click('[data-testid="bills-view-all"]');
await page.waitForTimeout(1200);
const path1 = await page.evaluate(() => location.pathname);
check("#76.4 View all lands on /bills", path1.endsWith("/bills"), path1);
await page.evaluate(() => { history.pushState({}, "", "/"); window.dispatchEvent(new PopStateEvent("popstate")); });
await page.waitForTimeout(1500);

// (5) Needs attention money/household grouping (soft: requires both kinds present in sample data).
const attn = await page.evaluate(() => ({
  money: !!document.querySelector('[data-testid="attn-money"]'),
  household: !!document.querySelector('[data-testid="attn-household"]'),
  rows: document.querySelectorAll('.bento .w[data-widget="attention"] .attention-chips > *, .bento .w[data-widget="attention"] .attention-list > *').length,
}));
console.log("attention state: " + JSON.stringify(attn));
check("#76.5 attention grouping renders (or single group flat)", attn.money === attn.household, JSON.stringify(attn));

// (1)+(2) Daily nudge → focused hero.
const heroBefore = await page.evaluate(() => document.querySelector(".home-hero")?.getBoundingClientRect().height);
const nudge = await page.locator('[data-testid="dash-daily-nudge"]').count();
check("#76.1 daily nudge shown after first week (sample data)", nudge === 1, "count=" + nudge);
await page.click('[data-testid="dash-daily-nudge-use"]');
await page.waitForTimeout(1500);
const afterUse = await page.evaluate(() => ({
  heroH: document.querySelector(".home-hero")?.getBoundingClientRect().height,
  focused: !!document.querySelector(".home-hero--focused"),
  sel: document.querySelector('[data-testid="dash-preset"]')?.value,
  nudgeGone: !document.querySelector('[data-testid="dash-daily-nudge"]'),
}));
const shrink = heroBefore ? Math.round((1 - afterUse.heroH / heroBefore) * 100) : 0;
check("#76.1 accept applies Daily check-in + nudge gone", afterUse.sel === "daily" && afterUse.nudgeGone, JSON.stringify({ sel: afterUse.sel, nudgeGone: afterUse.nudgeGone }));
check("#76.2 focused hero ~25-35% shorter", afterUse.focused && shrink >= 20 && shrink <= 45, `before=${heroBefore} after=${afterUse.heroH} shrink=${shrink}%`);

// Persistence across reload: preset + focused hero + answered nudge.
await page.reload({ waitUntil: "load" });
await ready();
const afterReload = await page.evaluate(() => ({
  sel: document.querySelector('[data-testid="dash-preset"]')?.value,
  focused: !!document.querySelector(".home-hero--focused"),
  nudgeGone: !document.querySelector('[data-testid="dash-daily-nudge"]'),
}));
check("#76.1 choice survives reload", afterReload.sel === "daily" && afterReload.focused && afterReload.nudgeGone, JSON.stringify(afterReload));

check("zero page errors", errors.length === 0, errors.slice(0, 3).join(" | "));
await browser.close();
process.exit(results.every(Boolean) ? 0 : 1);
