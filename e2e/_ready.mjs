// Shared boot-ready helper for CashFlux e2e gates.
//
// Usage:
//   import { ready } from "./_ready.mjs";
//   await ready(page);   // call after page.goto(); before any assertions
//
// Waits for:
//   1. The main nav rail (nav[aria-label="Main navigation"]) to be present — the
//      app shell has mounted.
//   2. The boot splash (#boot) to be gone (display:none, opacity:0, or .hidden) OR
//      #app to have at least one child element — whichever comes first.
//
// This replaces ad-hoc `page.waitForSelector("#app *") + waitForTimeout(N)` calls
// and avoids false "splash still visible" failures that were just timing artifacts.

const TIMEOUT = 60_000;

/**
 * Wait for the CashFlux app to be fully booted and the nav rail to be visible.
 * @param {import("playwright").Page} page
 */
export async function ready(page) {
  // 1. Nav rail present → app shell has mounted.
  await page.waitForSelector('nav[aria-label="Main navigation"]', { timeout: TIMEOUT });

  // 2. Boot splash gone OR app has rendered children.
  await page.waitForFunction(
    () => {
      const boot = document.getElementById("boot");
      if (!boot) return true; // element removed entirely
      const cs = getComputedStyle(boot);
      const gone =
        cs.display === "none" ||
        Number(cs.opacity) === 0 ||
        boot.classList.contains("hidden");
      if (gone) return true;
      // Fallback: #app has children even if #boot is still fading.
      const app = document.getElementById("app");
      return app ? app.children.length > 0 : false;
    },
    { timeout: TIMEOUT }
  );
}
