// lane3_verify_69.mjs — verify the UX-04 header rebalance: no "Dashboard ›"
// crumb, page titles never truncate at desktop widths, no top-bar horizontal
// overflow, music/lock fold below 1280px with labeled More-menu equivalents.
// Usage: node e2e/lane3_verify_69.mjs <port> <shotDir>
import { chromium } from "playwright";
import { mkdirSync } from "node:fs";

const PORT = process.argv[2] || "8113";
const OUT = process.argv[3] || "lane3-shots";
mkdirSync(OUT, { recursive: true });

let failures = 0;
const check = (ok, msg) => { console.log(`${ok ? "PASS" : "FAIL"} ${msg}`); if (!ok) failures++; };

const browser = await chromium.launch();
for (const width of [1440, 1280, 1100, 900]) {
  const ctx = await browser.newContext({ viewport: { width, height: 900 }, reducedMotion: "reduce" });
  const page = await ctx.newPage();
  await page.goto(`http://127.0.0.1:${PORT}/`, { waitUntil: "load" });
  await page.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", { timeout: 90000 });
  await page.waitForTimeout(1500);

  for (const path of ["/transactions", "/notifications", "/"]) {
    await page.evaluate((p) => { history.pushState({}, "", p); dispatchEvent(new PopStateEvent("popstate")); }, path);
    await page.waitForTimeout(900);
    const t = await page.evaluate(() => {
      const h1 = document.querySelector(".topbar h1");
      const bar = document.querySelector(".topbar");
      return {
        text: h1?.textContent ?? "",
        truncated: h1 ? h1.scrollWidth > h1.clientWidth + 1 : true,
        crumb: !!document.querySelector(".topbar .tb-title button"),
        overflow: bar ? bar.scrollWidth > bar.clientWidth + 1 : true,
      };
    });
    check(!t.truncated, `${width}px ${path}: title "${t.text}" not truncated`);
    check(!t.crumb, `${width}px ${path}: no "Dashboard ›" crumb`);
    check(!t.overflow, `${width}px ${path}: top bar does not overflow horizontally`);
    // Hidden popover panels inside the controls inflate scrollWidth, so measure
    // the visible controls' boxes against the strip's box instead.
    const ctxClipped = await page.evaluate(() => {
      const c = document.querySelector(".topbar .tb-context");
      if (!c) return null;
      const cr = c.getBoundingClientRect();
      for (const k of c.children) {
        const r = k.getBoundingClientRect();
        if (r.width === 0 || getComputedStyle(k).display === "none") continue;
        if (r.right > cr.right + 2 || r.left < cr.left - 2) {
          return `${k.className.toString().split(" ")[0]} [${r.left.toFixed(1)},${r.right.toFixed(1)}] vs [${cr.left.toFixed(1)},${cr.right.toFixed(1)}]`;
        }
      }
      return null;
    });
    check(!ctxClipped, `${width}px ${path}: context strip (period control) fully visible${ctxClipped ? " — " + ctxClipped : ""}`);
  }

  const muzakVisible = await page.evaluate(() => {
    const b = document.querySelector(".topbar .muzak-btn");
    return b ? getComputedStyle(b).display !== "none" : false;
  });
  check(width > 1280 ? muzakVisible : !muzakVisible,
    `${width}px: music button ${width > 1280 ? "inline" : "folded"} (visible=${muzakVisible})`);

  await page.click('[data-testid="topbar-more"]');
  await page.waitForTimeout(300);
  const moreRows = await page.evaluate(() =>
    [...document.querySelectorAll(".topbar-more .add-menu .add-item span")].map((s) => s.textContent.trim()));
  check(moreRows.some((r) => /music/i.test(r)), `${width}px: More menu has a labeled music row (${moreRows.join(" | ")})`);
  await page.keyboard.press("Escape");

  await page.screenshot({ path: `${OUT}/69-topbar-${width}.png` });
  await ctx.close();
}
await browser.close();
console.log(failures === 0 ? "ALL CHECKS PASSED" : `${failures} CHECK(S) FAILED`);
process.exit(failures === 0 ? 0 : 1);
