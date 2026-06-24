// GX9-F3 verification — cold-boot splash respects the saved theme preference.
// The inline <head> script reads cashflux:prefs and sets data-theme on <html>
// synchronously, before the boot splash (#boot) paints.  This test confirms:
//   1. light pref  → data-theme="light"  + #boot bg ≈ rgb(247,246,243) (light)
//   2. dark  pref  → data-theme="dark"   + #boot bg ≈ rgb(14,14,15)   (dark)
//   3. no post-load regression: after WASM mounts, data-theme is still "light"
//      when the pref was light (WASM must not double-flip to dark).
// Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const ARTIFACTS = __dirname;

// Expected background colors (CSS rgb strings).
const LIGHT_BG = "rgb(247, 246, 243)"; // #f7f6f3
const DARK_BG  = "rgb(14, 14, 15)";    // #0e0e0f

// Tolerance: allow ±5 on each channel (rendering rounding, sub-pixel AA).
function rgbClose(actual, expected, tol = 5) {
  const parse = (s) => s.match(/\d+/g).map(Number);
  const a = parse(actual), e = parse(expected);
  if (!a || !e || a.length < 3 || e.length < 3) return false;
  return a.every((v, i) => Math.abs(v - e[i]) <= tol);
}

const browser = await chromium.launch({ headless: true });
const fail = (m) => { console.error("FAIL: " + m); process.exitCode = 1; };
let passed = 0;

async function measureSplash(page, label) {
  // Poll for #boot to exist in DOM (it's in the static HTML so should be
  // immediate, but give the parser a moment).
  let bootBg = null;
  let dataTheme = null;
  const deadline = Date.now() + 2000;
  while (Date.now() < deadline) {
    const result = await page.evaluate(() => {
      const boot = document.getElementById("boot");
      if (!boot) return null;
      const style = window.getComputedStyle(boot);
      return {
        bg: style.backgroundColor,
        theme: document.documentElement.getAttribute("data-theme"),
        bootVisible: !boot.classList.contains("hidden"),
      };
    });
    if (result) {
      bootBg = result.bg;
      dataTheme = result.theme;
      console.log(`  [${label}] #boot found — bg="${bootBg}" data-theme="${dataTheme}" visible=${result.bootVisible}`);
      break;
    }
    await new Promise(r => setTimeout(r, 20));
  }
  return { bootBg, dataTheme };
}

try {
  // ── Step 1: light pref → light splash ────────────────────────────────────
  {
    const page = await browser.newPage();
    // Seed the pref before first load so the inline script picks it up.
    await page.goto(BASE + "/", { waitUntil: "commit" });
    await page.evaluate(() =>
      localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" }))
    );

    // Reload and race to measure #boot BEFORE the app finishes mounting.
    // waitUntil:"commit" fires as soon as the response is committed — the HTML
    // parser has started but JS may not have run yet; we then poll quickly.
    await page.reload({ waitUntil: "commit" });

    const { bootBg, dataTheme } = await measureSplash(page, "light");

    // Screenshot while the boot splash is still likely visible.
    await page.screenshot({ path: path.join(ARTIFACTS, "gx9_verify_splash_light.png") });
    console.log("  Screenshot: gx9_verify_splash_light.png");

    // Assertions
    if (dataTheme !== "light") {
      fail(`[light] data-theme="${dataTheme}", want "light"`);
    } else {
      console.log(`  PASS data-theme="light"`);
      passed++;
    }

    if (!bootBg) {
      fail(`[light] #boot not found in DOM within 2s`);
    } else if (!rgbClose(bootBg, LIGHT_BG)) {
      fail(`[light] #boot bg="${bootBg}", want ≈${LIGHT_BG} (light surface)`);
      if (rgbClose(bootBg, DARK_BG)) {
        console.error("  → got the DARK background — inline script not applied before paint");
      }
    } else {
      console.log(`  PASS #boot bg="${bootBg}" ≈ light`);
      passed++;
    }

    // ── Step 3: post-load regression check (still in light page) ────────────
    // Wait for WASM to mount and app to become interactive.
    try {
      await page.waitForSelector("#app", { timeout: 60000 });
      const postTheme = await page.evaluate(() =>
        document.documentElement.getAttribute("data-theme")
      );
      if (postTheme !== "light") {
        fail(`[light post-load] data-theme="${postTheme}" after app mount — WASM double-flipped to dark`);
      } else {
        console.log(`  PASS post-load data-theme="light" (no WASM regression)`);
        passed++;
      }
    } catch (e) {
      fail(`[light post-load] waiting for #app: ${e.message}`);
    }

    await page.close();
  }

  // ── Step 2: dark pref → dark splash ──────────────────────────────────────
  {
    const page = await browser.newPage();
    await page.goto(BASE + "/", { waitUntil: "commit" });
    await page.evaluate(() =>
      localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "dark" }))
    );

    await page.reload({ waitUntil: "commit" });

    const { bootBg, dataTheme } = await measureSplash(page, "dark");

    await page.screenshot({ path: path.join(ARTIFACTS, "gx9_verify_splash_dark.png") });
    console.log("  Screenshot: gx9_verify_splash_dark.png");

    if (dataTheme !== "dark") {
      fail(`[dark] data-theme="${dataTheme}", want "dark"`);
    } else {
      console.log(`  PASS data-theme="dark"`);
      passed++;
    }

    if (!bootBg) {
      fail(`[dark] #boot not found in DOM within 2s`);
    } else if (!rgbClose(bootBg, DARK_BG)) {
      fail(`[dark] #boot bg="${bootBg}", want ≈${DARK_BG} (dark surface)`);
    } else {
      console.log(`  PASS #boot bg="${bootBg}" ≈ dark`);
      passed++;
    }

    await page.close();
  }

  if (!process.exitCode) {
    console.log(`\nPASS: all ${passed}/5 GX9-F3 checks passed — cold-boot splash honours the saved theme preference.`);
  } else {
    console.log(`\n${passed}/5 checks passed — see FAIL lines above.`);
  }
} finally {
  await browser.close();
}
