// lane3_verify_50.mjs — verify the UX-01 dual-nav fix at 390px and 320px:
// rail fully hidden, 5-slot bottom bar with unclipped labels, More sheet with
// the remaining destinations, floating quick-add, active state on a More
// destination lighting the More tab. Usage: node e2e/lane3_verify_50.mjs <port> <shotDir>
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";

const PORT = process.argv[2] || "8113";
const OUT = process.argv[3] || "lane3-shots";
mkdirSync(OUT, { recursive: true });

let failures = 0;
const check = (ok, msg) => { console.log(`${ok ? "PASS" : "FAIL"} ${msg}`); if (!ok) failures++; };

const browser = await chromium.launch();
for (const width of [390, 320]) {
  const ctx = await browser.newContext({ viewport: { width, height: 844 }, reducedMotion: "reduce" });
  const page = await ctx.newPage();
  await page.goto(`http://127.0.0.1:${PORT}/`, { waitUntil: "load" });
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
  await page.waitForTimeout(1500);

  const railDisplay = await page.evaluate(() => {
    const r = document.querySelector("aside.rail");
    return r ? getComputedStyle(r).display : "absent";
  });
  check(railDisplay === "none" || railDisplay === "absent", `${width}px: rail removed (display=${railDisplay})`);

  const bar = await page.evaluate(() => {
    const nav = document.querySelector(".mobile-tabbar");
    if (!nav || getComputedStyle(nav).display === "none") return null;
    const items = [...nav.querySelectorAll(".mobile-tab-item")];
    return {
      count: items.length,
      labels: items.map((el) => el.querySelector(".mobile-tab-label")?.textContent ?? ""),
      clipped: items.some((el) => {
        const lbl = el.querySelector(".mobile-tab-label");
        return lbl && (lbl.scrollWidth > el.clientWidth || lbl.getBoundingClientRect().right > innerWidth + 1);
      }),
      scrollable: nav.scrollWidth > nav.clientWidth + 1,
    };
  });
  check(!!bar, `${width}px: tab bar visible`);
  if (bar) {
    check(bar.count === 5, `${width}px: exactly 5 slots (got ${bar.count}: ${bar.labels.join("/")})`);
    check(!bar.clipped, `${width}px: no clipped tab labels`);
    check(!bar.scrollable, `${width}px: bar does not scroll`);
  }

  const fabVisible = await page.evaluate(() => {
    const f = document.querySelector('[data-testid="mobile-tab-fab"]');
    if (!f) return false;
    const r = f.getBoundingClientRect();
    return getComputedStyle(f).display !== "none" && r.width >= 44 && r.bottom <= innerHeight;
  });
  check(fabVisible, `${width}px: floating quick-add visible with >=44px target`);

  await page.screenshot({ path: `${OUT}/50-home-${width}.png` });

  // Open the More sheet, count its rows, then navigate to Reports through it.
  await page.click('[data-testid="mobile-tab-more"]');
  await page.waitForTimeout(400);
  const sheet = await page.evaluate(() => {
    const s = document.querySelector('[data-testid="mobile-more-sheet"]');
    if (!s || getComputedStyle(s).display === "none") return null;
    return { rows: [...s.querySelectorAll(".mobile-sheet-item")].map((el) => el.textContent.trim()) };
  });
  check(!!sheet && sheet.rows.length === 5, `${width}px: More sheet shows 5 destinations (${sheet ? sheet.rows.join("/") : "no sheet"})`);
  await page.screenshot({ path: `${OUT}/50-moresheet-${width}.png` });

  await page.evaluate(() => {
    [...document.querySelectorAll('[data-testid="mobile-more-sheet"] .mobile-sheet-item')]
      .find((el) => el.getAttribute("href")?.endsWith("/reports"))?.click();
  });
  await page.waitForTimeout(1200);
  const afterNav = await page.evaluate(() => ({
    path: location.pathname,
    sheetGone: !document.querySelector('[data-testid="mobile-more-sheet"]'),
    moreActive: document.querySelector('[data-testid="mobile-tab-more"]')?.classList.contains("active") ?? false,
  }));
  check(afterNav.path.endsWith("/reports"), `${width}px: sheet row navigates to /reports (at ${afterNav.path})`);
  check(afterNav.sheetGone, `${width}px: sheet closes after picking a destination`);
  check(afterNav.moreActive, `${width}px: More tab shows active state for a sheet destination`);
  await page.screenshot({ path: `${OUT}/50-reports-${width}.png` });

  await ctx.close();
}
await browser.close();
console.log(failures === 0 ? "ALL CHECKS PASSED" : `${failures} CHECK(S) FAILED`);
process.exit(failures === 0 ? 0 : 1);
