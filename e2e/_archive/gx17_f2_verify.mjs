/**
 * GX17-F2 — LoadingCard component verification
 * Injects the LoadingCard HTML directly into the page to render it in both
 * dark and light themes, then measures aria attrs and shimmer animation.
 *
 * Run: node e2e/gx17_f2_verify.mjs
 * Requires playwright: npm install playwright (or npx playwright install chromium)
 */
import { chromium } from "playwright";

const BASE = "http://127.0.0.1:8099";
const OUT = "e2e/screenshots";

// LoadingCard HTML as rendered by ui.LoadingCard() in primitives.go
const LOADING_CARD_HTML = `
<section
  class="card"
  role="status"
  aria-live="polite"
  aria-busy="true"
  aria-label="Loading…"
  style="padding:1.5rem; max-width:520px; margin:2rem auto;"
>
  <div class="skeleton shimmer" style="height:1rem;width:40%;margin-bottom:1rem;border-radius:6px;"></div>
  <div class="skeleton shimmer" style="height:0.85rem;width:90%;margin-bottom:0.5rem;border-radius:6px;"></div>
  <div class="skeleton shimmer" style="height:0.85rem;width:75%;margin-bottom:0.5rem;border-radius:6px;"></div>
  <div class="skeleton shimmer" style="height:0.85rem;width:55%;margin-bottom:0.5rem;border-radius:6px;"></div>
</section>
`;

async function shot(page, name) {
  const p = `${OUT}/${name}`;
  await page.screenshot({ path: p, fullPage: false });
  console.log("  screenshot:", name);
  return p;
}

(async () => {
  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
  const page = await ctx.newPage();

  // --- DARK ---
  console.log("\n=== DARK theme ===");
  await page.goto(BASE, { waitUntil: "networkidle", timeout: 30000 });
  await page.waitForSelector("#app > *", { timeout: 30000 }).catch(() => {});
  await page.waitForTimeout(1000);

  // Force dark
  await page.evaluate(() => {
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "dark" }));
  });
  await page.reload({ waitUntil: "networkidle" });
  await page.waitForTimeout(1500);

  // Inject the LoadingCard into a clean overlay
  await page.evaluate((html) => {
    const overlay = document.createElement("div");
    overlay.id = "gx17-f2-test-overlay";
    overlay.style.cssText = "position:fixed;inset:0;z-index:99999;display:flex;align-items:center;justify-content:center;background:var(--bg-page,#111);";
    overlay.innerHTML = html;
    document.body.appendChild(overlay);
  }, LOADING_CARD_HTML);
  await page.waitForTimeout(300);
  await shot(page, "gx17_loadingcard_dark.png");

  // Measure aria attrs
  const ariaAttrs = await page.evaluate(() => {
    const el = document.querySelector("[role='status']");
    if (!el) return null;
    return {
      role: el.getAttribute("role"),
      ariaLive: el.getAttribute("aria-live"),
      ariaBusy: el.getAttribute("aria-busy"),
      ariaLabel: el.getAttribute("aria-label"),
      classList: el.className,
    };
  });
  console.log("  aria attrs:", JSON.stringify(ariaAttrs));

  // Measure shimmer animation-name (default — wonder on)
  const shimmerAnim = await page.evaluate(() => {
    const el = document.querySelector(".shimmer");
    if (!el) return null;
    return {
      animationName: getComputedStyle(el).animationName,
      background: getComputedStyle(el).background,
    };
  });
  console.log("  .shimmer animation-name (default):", shimmerAnim?.animationName);
  console.log("  .shimmer background:", shimmerAnim?.background?.slice(0, 80));

  // Measure shimmer animation-name with data-wonder=off
  const shimmerAnimOff = await page.evaluate(() => {
    document.documentElement.setAttribute("data-wonder", "off");
    const el = document.querySelector(".shimmer");
    const result = el ? getComputedStyle(el).animationName : null;
    document.documentElement.removeAttribute("data-wonder");
    return result;
  });
  console.log("  .shimmer animation-name (data-wonder=off):", shimmerAnimOff);

  // cleanup overlay
  await page.evaluate(() => {
    const el = document.getElementById("gx17-f2-test-overlay");
    if (el) el.remove();
  });

  // --- LIGHT ---
  console.log("\n=== LIGHT theme ===");
  await page.evaluate(() => {
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" }));
  });
  await page.reload({ waitUntil: "networkidle" });
  await page.waitForTimeout(1500);

  await page.evaluate((html) => {
    const overlay = document.createElement("div");
    overlay.id = "gx17-f2-test-overlay";
    overlay.style.cssText = "position:fixed;inset:0;z-index:99999;display:flex;align-items:center;justify-content:center;background:var(--bg-page,#fff);";
    overlay.innerHTML = html;
    document.body.appendChild(overlay);
  }, LOADING_CARD_HTML);
  await page.waitForTimeout(300);
  await shot(page, "gx17_loadingcard_light.png");

  // Confirm real screen still renders after guard swap — navigate to /accounts
  console.log("\n=== Real screen: /accounts ===");
  await page.evaluate(() => {
    const el = document.getElementById("gx17-f2-test-overlay");
    if (el) el.remove();
  });
  await page.goto(BASE + "/#/accounts", { waitUntil: "networkidle" });
  await page.waitForTimeout(1500);
  const accountsContent = await page.evaluate(() => {
    const app = document.querySelector("#app");
    return { html: app?.innerHTML?.slice(0, 300), hasCard: !!document.querySelector(".card") };
  });
  console.log("  /accounts hasCard:", accountsContent.hasCard);
  console.log("  /accounts app innerHTML[:300]:", accountsContent.html);
  await shot(page, "gx17_accounts_after_swap.png");

  // Check no role=status visible in real accounts screen (guard only shows when app==nil)
  const roleStatus = await page.evaluate(() => {
    return [...document.querySelectorAll("[role='status']")].map(e => ({
      class: e.className, text: e.innerText?.slice(0, 50)
    }));
  });
  console.log("  [role=status] in /accounts (should be none):", JSON.stringify(roleStatus));

  await browser.close();
  console.log("\nDone. Screenshots in", OUT);
})();
