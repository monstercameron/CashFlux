// B34 Appearance page — runtime verification.
// Checks: deep-link render, rail nav, control + persist, settings link, no console errors.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SS_DIR = path.join(__dirname, "screenshots");
if (!fs.existsSync(SS_DIR)) fs.mkdirSync(SS_DIR, { recursive: true });

const browser = await chromium.launch({ headless: true });

let passed = 0;
let failed = 0;

const pass = (label, detail = "") => {
  console.log(`PASS [${label}]${detail ? ": " + detail : ""}`);
  passed++;
};
const fail = (label, detail = "") => {
  console.error(`FAIL [${label}]${detail ? ": " + detail : ""}`);
  failed++;
};

// Shared error collector — attach once per page.
const attachErrors = (page, errors) => page.on("pageerror", (e) => errors.push(String(e)));

// Wait for the WASM app to boot: #app must have real content (not just the boot splash).
const waitForApp = async (page, timeout = 60000) => {
  await page.waitForSelector("#app", { timeout });
  // Wait until #app has at least one child element rendered by Go/WASM (not just a blank div).
  await page.waitForFunction(
    () => {
      const app = document.querySelector("#app");
      return app && app.children.length > 0 && app.textContent.trim().length > 5;
    },
    { timeout },
  );
};

// ─── TEST 1: Deep-link render ────────────────────────────────────────────────
{
  const errors = [];
  const page = await browser.newPage();
  attachErrors(page, errors);

  try {
    await page.goto(BASE + "/appearance", { waitUntil: "domcontentloaded" });
    await waitForApp(page);

    // Give the router a moment to settle on the /appearance route.
    await page.waitForTimeout(500);

    const bodyText = await page.evaluate(() => document.body.innerText);
    const html = await page.evaluate(() => document.body.innerHTML);

    // Look for appearance-specific content signals.
    const hasAppearanceHeading = /Appearance/i.test(bodyText);
    const hasMotion = /Motion/i.test(bodyText);
    const hasDarkLight = /Dark|Light|System/i.test(bodyText);
    const hasAccent = /Accent/i.test(bodyText);

    // Check for theme-mode segmented control buttons.
    const segButtons = await page.$$eval("[data-seg-opt], .seg-opt, [role='radio']", (els) =>
      els.map((e) => e.textContent.trim()),
    );

    // Check for accent swatch picker.
    const swatches = await page.$$(".swatch, [data-swatch], [data-color]");

    await page.screenshot({ path: path.join(SS_DIR, "b34_appearance_render.png"), fullPage: true });

    if (!hasAppearanceHeading && !hasMotion) {
      fail(
        "1-DEEP-LINK",
        `Appearance page content NOT found. bodyText preview: "${bodyText.slice(0, 300)}"`,
      );
    } else {
      const found = [];
      if (hasAppearanceHeading) found.push('"Appearance" heading');
      if (hasDarkLight) found.push("theme mode (Dark/Light/System)");
      if (hasMotion) found.push('"Motion" control');
      if (hasAccent) found.push('"Accent" label');
      if (swatches.length > 0) found.push(`${swatches.length} accent swatch(es)`);
      if (segButtons.length > 0) found.push(`seg buttons: [${segButtons.join(", ")}]`);
      pass("1-DEEP-LINK", `Found: ${found.join(" | ")}`);
    }
  } finally {
    await page.close();
  }
}

// ─── TEST 2: Rail nav ────────────────────────────────────────────────────────
{
  const errors = [];
  const page = await browser.newPage();
  attachErrors(page, errors);

  try {
    await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await waitForApp(page);

    // Find the Appearance nav item in the rail (translated as "Appearance" from nav.appearance key).
    // The rail uses <a> or <button> elements with data-route or just matching text.
    const railLinks = await page.$$eval(
      "nav a, nav button, [data-route], aside a, aside button",
      (els) => els.map((e) => ({ text: e.textContent.trim(), href: e.getAttribute("href") || "", cls: e.className })),
    );

    const appearanceNavItem = railLinks.find((l) => /Appearance/i.test(l.text));

    if (!appearanceNavItem) {
      // Try a broader search.
      const allLinks = await page.$$eval("a, button", (els) =>
        els.map((e) => e.textContent.trim()).filter((t) => t.length > 0),
      );
      fail(
        "2-RAIL-NAV",
        `No "Appearance" rail item found. All link/button texts: ${JSON.stringify(allLinks.slice(0, 30))}`,
      );
    } else {
      // Click the rail item.
      const navEl = page.locator("nav a, nav button, [data-route], aside a, aside button").filter({ hasText: /Appearance/i }).first();
      // Fallback: broader selector.
      const clickTarget = await navEl.count() > 0
        ? navEl
        : page.locator("a, button").filter({ hasText: /^Appearance$/i }).first();

      await clickTarget.click();
      await page.waitForTimeout(600);

      const url = page.url();
      const bodyText = await page.evaluate(() => document.body.innerText);
      const onAppearancePage = url.includes("/appearance") || /Motion|Accent/i.test(bodyText);

      if (onAppearancePage) {
        pass("2-RAIL-NAV", `Clicked "Appearance" rail item → URL: ${url} (content: Motion=${/Motion/.test(bodyText)}, Accent=${/Accent/.test(bodyText)})`);
      } else {
        fail("2-RAIL-NAV", `Clicked rail item but landed on: ${url} | bodyText: "${bodyText.slice(0, 200)}"`);
      }
    }
  } finally {
    await page.close();
  }
}

// ─── TEST 3: Control works + persists ────────────────────────────────────────
{
  const errors = [];
  const page = await browser.newPage();
  attachErrors(page, errors);

  try {
    await page.goto(BASE + "/appearance", { waitUntil: "domcontentloaded" });
    await waitForApp(page);
    await page.waitForTimeout(500);

    // Read current data-wonder (motion) attribute on documentElement.
    const getWonder = () =>
      page.evaluate(() => document.documentElement.getAttribute("data-wonder") || "none");
    const getTheme = () =>
      page.evaluate(() => document.documentElement.getAttribute("data-theme") || "none");
    const getMotionPref = () =>
      page.evaluate(() => {
        try {
          const p = JSON.parse(localStorage.getItem("cashflux:prefs") || "{}");
          return p.motion || p.Motion || "none";
        } catch {
          return "error";
        }
      });

    const wonderBefore = await getWonder();
    const themeBefore = await getTheme();
    const motionBefore = await getMotionPref();

    // Strategy: find the Motion "Off" segment button and click it (if not already off).
    // The Motion segmented control has options: Full, Subtle, Off.
    // We look for a button with text "Off" near/after a "Motion" label.
    // Use page.locator with text.
    const offBtn = page.locator("button, [role='radio'], [data-seg-opt]").filter({ hasText: /^Off$/i });
    const offCount = await offBtn.count();

    let controlClicked = false;
    let measuredChange = "n/a";

    if (offCount > 0) {
      // Click the first "Off" button (Motion Off).
      await offBtn.first().click();
      await page.waitForTimeout(400);

      const motionAfter = await getMotionPref();
      const wonderAfter = await getWonder();
      measuredChange = `motion pref: ${motionBefore} → ${motionAfter}, data-wonder: ${wonderBefore} → ${wonderAfter}`;
      controlClicked = true;

      // Reload and check persistence.
      await page.reload({ waitUntil: "domcontentloaded" });
      await waitForApp(page);
      await page.waitForTimeout(400);

      const motionAfterReload = await getMotionPref();
      const wonderAfterReload = await getWonder();

      const persisted = motionAfterReload === "off" || motionAfterReload === "Off" || motionAfterReload === "MotionOff";

      if (persisted) {
        pass(
          "3-CONTROL-PERSIST",
          `${measuredChange} | after reload: motion=${motionAfterReload}, wonder=${wonderAfterReload} — PERSISTED`,
        );
      } else {
        fail(
          "3-CONTROL-PERSIST",
          `${measuredChange} | after reload: motion=${motionAfterReload} (expected off/Off/MotionOff)`,
        );
      }
    } else {
      // Fallback: try toggling Light theme mode.
      const lightBtn = page.locator("button, [role='radio'], [data-seg-opt]").filter({ hasText: /^Light$/i });
      const lightCount = await lightBtn.count();

      if (lightCount > 0 && themeBefore !== "light") {
        await lightBtn.first().click();
        await page.waitForTimeout(400);

        const themeAfter = await getTheme();
        const themePref = await page.evaluate(() => {
          try { return JSON.parse(localStorage.getItem("cashflux:prefs") || "{}").theme || "none"; }
          catch { return "error"; }
        });

        measuredChange = `data-theme: ${themeBefore} → ${themeAfter}, pref: ${themePref}`;
        controlClicked = true;

        await page.reload({ waitUntil: "domcontentloaded" });
        await waitForApp(page);
        await page.waitForTimeout(400);

        const themeAfterReload = await getTheme();

        if (themeAfterReload === "light") {
          pass("3-CONTROL-PERSIST", `${measuredChange} | after reload: data-theme=${themeAfterReload} — PERSISTED`);
        } else {
          fail("3-CONTROL-PERSIST", `${measuredChange} | after reload: data-theme=${themeAfterReload} (expected light)`);
        }
      } else {
        fail(
          "3-CONTROL-PERSIST",
          `Could not find clickable Motion "Off" or theme "Light" button. offCount=${offCount}, lightCount=${lightCount}`,
        );
      }
    }
  } finally {
    await page.close();
  }
}

// ─── TEST 4: Settings link → /appearance ─────────────────────────────────────
{
  const errors = [];
  const page = await browser.newPage();
  attachErrors(page, errors);

  try {
    await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await waitForApp(page);

    // Open Settings via the household card (button.hh at rail bottom).
    const hhBtn = page.locator("button.hh").first();
    const hhCount = await hhBtn.count();

    if (hhCount === 0) {
      fail("4-SETTINGS-LINK", "button.hh not found in DOM");
    } else {
      await hhBtn.click();
      await page.waitForTimeout(600);

      // Find "Appearance & theme →" button/link in the settings panel.
      const appearanceLink = page
        .locator("button, a")
        .filter({ hasText: /Appearance.*theme|Appearance & theme/i })
        .first();
      const linkCount = await appearanceLink.count();

      if (linkCount === 0) {
        // Log what buttons are visible in the settings panel.
        const visibleBtns = await page.$$eval("button", (els) =>
          els.map((e) => e.textContent.trim()).filter((t) => t.length > 0),
        );
        fail("4-SETTINGS-LINK", `"Appearance & theme →" not found. Visible buttons: ${JSON.stringify(visibleBtns.slice(0, 40))}`);
      } else {
        await appearanceLink.click();
        await page.waitForTimeout(600);

        const url = page.url();
        const bodyText = await page.evaluate(() => document.body.innerText);
        const onAppearancePage = url.includes("/appearance") || (/Motion/i.test(bodyText) && /Accent/i.test(bodyText));

        if (onAppearancePage) {
          pass("4-SETTINGS-LINK", `"Appearance & theme →" clicked → URL: ${url}`);
        } else {
          fail("4-SETTINGS-LINK", `Clicked link but landed on: ${url} | content: "${bodyText.slice(0, 200)}"`);
        }
      }
    }
  } finally {
    await page.close();
  }
}

// ─── TEST 5: No console errors (overall page errors captured above) ────────────
// We open the appearance page once more cleanly and check.
{
  const errors = [];
  const page = await browser.newPage();
  attachErrors(page, errors);

  try {
    await page.goto(BASE + "/appearance", { waitUntil: "domcontentloaded" });
    await waitForApp(page);
    await page.waitForTimeout(500);

    if (errors.length === 0) {
      pass("5-NO-CONSOLE-ERRORS", "No page errors on /appearance");
    } else {
      fail("5-NO-CONSOLE-ERRORS", `${errors.length} error(s): ${errors.join(" | ")}`);
    }
  } finally {
    await page.close();
  }
}

await browser.close();

console.log(`\n─── B34 Appearance Verification ───`);
console.log(`PASS: ${passed}  FAIL: ${failed}`);
if (failed > 0) process.exitCode = 1;
