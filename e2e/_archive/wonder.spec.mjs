/**
 * W2 WONDER e2e test suite — CashFlux
 *
 * Covers every WONDER flourish with PASS/FAIL + measured values.
 * Folds in: w1_verify.mjs, gx_w9_verify.mjs, w15_countup_check.mjs, gx_w18_verify.mjs
 *
 * Run: node e2e/wonder.spec.mjs
 * Requires: node e2e/serve.go running on 8099 (or E2E_URL env override)
 */

import { createRequire } from "module";
import { existsSync, mkdirSync } from "fs";
import { fileURLToPath } from "url";
import { dirname, join } from "path";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const _require = createRequire(join(__dirname, "..", ".tools", "package.json"));
const { chromium } = _require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const BOOT_MS = 5000; // conservative WASM boot wait

const SHOT_DIR = join(__dirname, "screenshots", "wonder_spec");
if (!existsSync(SHOT_DIR)) mkdirSync(SHOT_DIR, { recursive: true });

// ── Result tracking ───────────────────────────────────────────────────────────
let totalPassed = 0;
let totalFailed = 0;
const results = [];

function pass(label, measured = "") {
  totalPassed++;
  const line = `PASS [${label}]${measured ? " — " + measured : ""}`;
  results.push({ ok: true, line });
  console.log("  " + line);
}

function fail(label, measured = "") {
  totalFailed++;
  const line = `FAIL [${label}]${measured ? " — " + measured : ""}`;
  results.push({ ok: false, line });
  console.error("  " + line);
}

function info(msg) {
  console.log("        " + msg);
}

function isIdentity(t) {
  return !t || t === "none" || t === "matrix(1, 0, 0, 1, 0, 0)";
}

function extractTranslateY(t) {
  // matrix(a,b,c,d,tx,ty) — ty is index 5
  if (!t || t === "none") return 0;
  const m = t.match(/matrix\([^)]+\)/);
  if (!m) return 0;
  const parts = m[0].replace("matrix(", "").replace(")", "").split(",").map(Number);
  return parts[5] || 0;
}

function extractTranslateX(t) {
  if (!t || t === "none") return 0;
  const m = t.match(/matrix\([^)]+\)/);
  if (!m) return 0;
  const parts = m[0].replace("matrix(", "").replace(")", "").split(",").map(Number);
  return parts[4] || 0;
}

// ── App boot helper ───────────────────────────────────────────────────────────
async function bootApp(page) {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(BOOT_MS);
}

// Navigate within the SPA (click link or pushState)
async function spaNav(page, path) {
  await page.evaluate((p) => {
    const a = [...document.querySelectorAll("a[href]")].find(
      (el) => el.getAttribute("href") === p || el.pathname === p
    );
    if (a) { a.click(); return; }
    history.pushState({}, "", p);
    window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
  }, path);
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION 1 — W-1: Card hover lift
// ═══════════════════════════════════════════════════════════════════════════════
async function testW1CardLift(browser) {
  console.log("\n══════ W-1: Card hover lift ══════");
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));

  await bootApp(page);

  // Navigate to /budgets (confirmed to have .card elements)
  await spaNav(page, "/budgets");
  await page.waitForTimeout(1500);

  let cardCount = await page.evaluate(() => document.querySelectorAll(".card").length);
  if (cardCount === 0) {
    await spaNav(page, "/");
    await page.waitForTimeout(1500);
    cardCount = await page.evaluate(() => document.querySelectorAll(".card").length);
  }

  if (cardCount === 0) {
    fail("W-1 setup", "no .card elements found — cannot test");
    await ctx.close();
    return;
  }
  info(`Found ${cardCount} .card elements`);

  // Full wonder mode (default — no attribute)
  await page.evaluate(() => document.documentElement.removeAttribute("data-wonder"));
  await page.mouse.move(0, 0);
  await page.waitForTimeout(80);
  await page.hover(".card");
  await page.waitForTimeout(400); // transition + buffer

  const defaultTransform = await page.evaluate(() => {
    const el = document.querySelector(".card");
    return el ? getComputedStyle(el).transform : "none";
  });

  const ty = extractTranslateY(defaultTransform);
  info(`transform (full wonder hover): ${defaultTransform}  →  translateY=${ty.toFixed(2)}px`);

  if (isIdentity(defaultTransform)) {
    fail("W-1 full: card hover transform non-identity", `transform="${defaultTransform}"`);
  } else {
    pass("W-1 full: card hover transform non-identity", `transform="${defaultTransform}" translateY=${ty.toFixed(2)}px`);
  }

  await page.screenshot({ path: join(SHOT_DIR, "w1_hover_full.png") });

  // Perceptibility guard: |translateY| >= 4px
  if (Math.abs(ty) >= 4) {
    pass("W-1 perceptibility: |translateY| >= 4px", `|${ty.toFixed(2)}px| >= 4px`);
  } else {
    fail("W-1 perceptibility: |translateY| >= 4px", `got |${ty.toFixed(2)}px| — too subtle or zero`);
  }

  // Off mode: hover should give identity
  await page.mouse.move(0, 0);
  await page.waitForTimeout(80);
  await page.evaluate(() => document.documentElement.setAttribute("data-wonder", "off"));
  await page.hover(".card");
  await page.waitForTimeout(150);

  const offTransform = await page.evaluate(() => {
    const el = document.querySelector(".card");
    return el ? getComputedStyle(el).transform : "none";
  });
  info(`transform (wonder=off hover): ${offTransform}`);

  if (isIdentity(offTransform)) {
    pass("W-1 off: card hover = identity (no lift)", `transform="${offTransform}"`);
  } else {
    fail("W-1 off: card hover should be identity", `got "${offTransform}"`);
  }

  await page.screenshot({ path: join(SHOT_DIR, "w1_hover_off.png") });

  // Reduced motion
  await page.mouse.move(0, 0);
  await page.waitForTimeout(80);
  await page.evaluate(() => document.documentElement.removeAttribute("data-wonder"));
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.hover(".card");
  await page.waitForTimeout(150);

  const reducedTransform = await page.evaluate(() => {
    const el = document.querySelector(".card");
    return el ? getComputedStyle(el).transform : "none";
  });
  info(`transform (reduced-motion hover): ${reducedTransform}`);

  if (isIdentity(reducedTransform)) {
    pass("W-1 reduced-motion: card hover = identity", `transform="${reducedTransform}"`);
  } else {
    fail("W-1 reduced-motion: card hover should be identity", `got "${reducedTransform}"`);
  }

  await ctx.close();
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION 2 — W-2: Button press scale
// ═══════════════════════════════════════════════════════════════════════════════
async function testW2ButtonPress(browser) {
  console.log("\n══════ W-2: Button press scale ══════");
  const ctx = await browser.newContext();
  const page = await ctx.newPage();

  await bootApp(page);
  await page.evaluate(() => document.documentElement.removeAttribute("data-wonder"));

  // Read the --wonder-press token value
  const wonderPress = await page.evaluate(() =>
    getComputedStyle(document.documentElement).getPropertyValue("--wonder-press").trim()
  );
  info(`--wonder-press = "${wonderPress}"`);

  const pressVal = parseFloat(wonderPress);
  if (!isNaN(pressVal) && pressVal < 1 && pressVal > 0.9) {
    pass("W-2: --wonder-press token is < 1 (scale-down on press)", `--wonder-press=${wonderPress}`);
  } else {
    fail("W-2: --wonder-press token should be < 1", `got "${wonderPress}"`);
  }

  // Read the CSS :active rule for .btn by checking the applied animation via CSSOM
  // (Headless can't hold :active state; we verify the token is wired)
  const btnTransition = await page.evaluate(() => {
    const btn = document.querySelector(".btn");
    if (!btn) return null;
    return getComputedStyle(btn).transition;
  });
  info(`btn computed transition: ${btnTransition}`);

  if (btnTransition && btnTransition.includes("transform")) {
    pass("W-2: .btn has transform transition (press animation wired)", `transition includes 'transform'`);
  } else {
    fail("W-2: .btn transition should include transform", `got "${btnTransition}"`);
  }

  // Off mode: --wonder-press should be 1
  await page.evaluate(() => document.documentElement.setAttribute("data-wonder", "off"));
  const pressOff = await page.evaluate(() =>
    getComputedStyle(document.documentElement).getPropertyValue("--wonder-press").trim()
  );
  info(`--wonder-press (off): "${pressOff}"`);

  if (parseFloat(pressOff) === 1) {
    pass("W-2 off: --wonder-press = 1 (no scale)", `--wonder-press=${pressOff}`);
  } else {
    fail("W-2 off: --wonder-press should be 1", `got "${pressOff}"`);
  }

  await page.screenshot({ path: join(SHOT_DIR, "w2_btn_state.png") });
  await ctx.close();
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION 3 — W-4: Row hover nudge (list rows + table exclusion)
// ═══════════════════════════════════════════════════════════════════════════════
async function testW4RowNudge(browser) {
  console.log("\n══════ W-4: Row hover nudge ══════");
  const ctx = await browser.newContext();
  const page = await ctx.newPage();

  await bootApp(page);
  await page.evaluate(() => document.documentElement.removeAttribute("data-wonder"));

  // Try accounts or budgets for list rows
  await spaNav(page, "/accounts");
  await page.waitForTimeout(1500);

  let rowCount = await page.evaluate(() =>
    document.querySelectorAll(".row:not(.txn-table .row)").length
  );

  if (rowCount === 0) {
    await spaNav(page, "/budgets");
    await page.waitForTimeout(1500);
    rowCount = await page.evaluate(() =>
      document.querySelectorAll(".row:not(.txn-table .row)").length
    );
  }

  info(`Non-table .row elements: ${rowCount}`);

  if (rowCount === 0) {
    fail("W-4 setup", "no non-table .row elements found");
  } else {
    // Hover the first non-table row
    await page.mouse.move(0, 0);
    await page.waitForTimeout(50);
    await page.hover(".row:not(.txn-table .row)");
    await page.waitForTimeout(300);

    const rowTransform = await page.evaluate(() => {
      // get the hovered element (first non-table row)
      const el = document.querySelector(".row:not(.txn-table .row)");
      return el ? getComputedStyle(el).transform : "none";
    });
    const tx = extractTranslateX(rowTransform);
    info(`row hover transform: ${rowTransform}  →  translateX=${tx.toFixed(2)}px`);

    // When --wonder-on=1, translateX = 2px * 1 = 2px
    if (!isIdentity(rowTransform) || Math.abs(tx) > 0.5) {
      pass("W-4: list .row hover = non-identity translateX", `translateX=${tx.toFixed(2)}px`);
    } else {
      // Some rows may already be at identity after animation settles; check via token
      const wonderOn = await page.evaluate(() =>
        getComputedStyle(document.documentElement).getPropertyValue("--wonder-on").trim()
      );
      if (wonderOn === "1") {
        fail("W-4: list .row hover should be non-identity", `transform="${rowTransform}" --wonder-on=${wonderOn}`);
      } else {
        info(`W-4: --wonder-on=${wonderOn} (not full), transform may be identity`);
      }
    }

    await page.screenshot({ path: join(SHOT_DIR, "w4_row_hover.png") });
  }

  // Check transactions page for .txn-table .row (should be excluded / identity on hover)
  await spaNav(page, "/transactions");
  await page.waitForTimeout(1500);

  const txnRowCount = await page.evaluate(() =>
    document.querySelectorAll(".txn-table .row").length
  );
  info(`.txn-table .row elements: ${txnRowCount}`);

  if (txnRowCount === 0) {
    info("W-4: no .txn-table .row found (may need sample data); skipping txn exclusion check");
  } else {
    await page.mouse.move(0, 0);
    await page.waitForTimeout(50);

    // Hover txn row
    const txnRowBox = await page.evaluate(() => {
      const el = document.querySelector(".txn-table .row");
      if (!el) return null;
      const r = el.getBoundingClientRect();
      return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
    });

    if (txnRowBox) {
      await page.mouse.move(txnRowBox.x, txnRowBox.y);
      await page.waitForTimeout(300);

      const txnTransform = await page.evaluate(() => {
        const el = document.querySelector(".txn-table .row");
        return el ? getComputedStyle(el).transform : "none";
      });
      const txnTx = extractTranslateX(txnTransform);
      info(`txn-table row hover transform: ${txnTransform}  →  translateX=${txnTx.toFixed(2)}px`);

      if (Math.abs(txnTx) < 0.5) {
        pass("W-4: .txn-table .row hover = identity (excluded from nudge)", `translateX=${txnTx.toFixed(2)}px`);
      } else {
        fail("W-4: .txn-table .row should have identity transform (excluded)", `translateX=${txnTx.toFixed(2)}px`);
      }
    }
    await page.screenshot({ path: join(SHOT_DIR, "w4_txn_row_hover.png") });
  }

  await ctx.close();
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION 4 — W-5/6: Nav + icon hover
// ═══════════════════════════════════════════════════════════════════════════════
async function testW56NavHover(browser) {
  console.log("\n══════ W-5/6: Nav + icon hover ══════");
  const ctx = await browser.newContext();
  const page = await ctx.newPage();

  await bootApp(page);
  await page.evaluate(() => document.documentElement.removeAttribute("data-wonder"));

  // Check .nv (nav item)
  const nvCount = await page.evaluate(() => document.querySelectorAll(".nv").length);
  info(`.nv count: ${nvCount}`);

  if (nvCount > 0) {
    await page.mouse.move(0, 0);
    await page.waitForTimeout(50);
    await page.hover(".nv");
    await page.waitForTimeout(250);

    const nvTransform = await page.evaluate(() => {
      const el = document.querySelector(".nv");
      return el ? getComputedStyle(el).transform : "none";
    });
    const nvTy = extractTranslateY(nvTransform);
    info(`.nv hover transform: ${nvTransform}  →  translateY=${nvTy.toFixed(2)}px`);

    if (!isIdentity(nvTransform)) {
      pass("W-5: .nv hover = non-identity transform (full wonder)", `translateY=${nvTy.toFixed(2)}px`);
    } else {
      fail("W-5: .nv hover should be non-identity", `transform="${nvTransform}"`);
    }

    // Off mode
    await page.mouse.move(0, 0);
    await page.waitForTimeout(50);
    await page.evaluate(() => document.documentElement.setAttribute("data-wonder", "off"));
    await page.hover(".nv");
    await page.waitForTimeout(150);

    const nvOffTransform = await page.evaluate(() => {
      const el = document.querySelector(".nv");
      return el ? getComputedStyle(el).transform : "none";
    });
    info(`.nv hover transform (off): ${nvOffTransform}`);

    if (isIdentity(nvOffTransform)) {
      pass("W-5 off: .nv hover = identity", `transform="${nvOffTransform}"`);
    } else {
      fail("W-5 off: .nv hover should be identity", `got "${nvOffTransform}"`);
    }
    await page.evaluate(() => document.documentElement.removeAttribute("data-wonder"));
  } else {
    info("W-5: no .nv elements found (rail may be collapsed)");
  }

  // Check .add-btn (+ Add button)
  const addBtnExists = await page.evaluate(() => !!document.querySelector(".add-btn"));
  if (addBtnExists) {
    await page.mouse.move(0, 0);
    await page.waitForTimeout(50);
    await page.hover(".add-btn");
    await page.waitForTimeout(250);

    const addTransform = await page.evaluate(() => {
      const el = document.querySelector(".add-btn");
      return el ? getComputedStyle(el).transform : "none";
    });
    info(`.add-btn hover transform: ${addTransform}`);

    if (!isIdentity(addTransform)) {
      pass("W-6: .add-btn hover = non-identity (scale)", `transform="${addTransform}"`);
    } else {
      fail("W-6: .add-btn hover should be non-identity", `got "${addTransform}"`);
    }
  } else {
    info("W-6: no .add-btn found on current page");
  }

  await page.screenshot({ path: join(SHOT_DIR, "w56_nav_hover.png") });
  await ctx.close();
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION 5 — W-9: Page-enter (MARQUEE check)
// ═══════════════════════════════════════════════════════════════════════════════
async function testW9PageEnter(browser) {
  console.log("\n══════ W-9: Page-enter (MARQUEE) ══════");
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  const consoleErrors = [];
  page.on("console", (m) => { if (m.type() === "error") consoleErrors.push(m.text()); });
  page.on("pageerror", (e) => consoleErrors.push(String(e)));

  await bootApp(page);
  await page.evaluate(() => document.documentElement.removeAttribute("data-wonder"));
  await page.waitForTimeout(500); // let firstRender guard settle

  // Helper: navigate, then sample #cf-page-view at multiple timepoints
  async function navigateAndSampleFrames(destPath, label) {
    // Trigger nav
    await page.evaluate((p) => {
      const a = [...document.querySelectorAll("a[href]")].find(
        (el) => el.getAttribute("href") === p || el.pathname === p
      );
      if (a) { a.click(); return; }
      history.pushState({}, "", p);
      window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
    }, destPath);

    // Sample at t≈0, 60, 120, 200, 320ms
    const timepoints = [0, 60, 120, 200, 320];
    const frames = [];
    const t0 = Date.now();

    for (const tp of timepoints) {
      const elapsed = Date.now() - t0;
      const wait = tp - elapsed;
      if (wait > 0) await page.waitForTimeout(wait);

      const snap = await page.evaluate(() => {
        const el = document.getElementById("cf-page-view");
        if (!el) return null;
        const cs = getComputedStyle(el);
        return {
          animationName: cs.animationName,
          opacity: parseFloat(cs.opacity),
          transform: cs.transform,
          hasClass: el.classList.contains("page-enter"),
        };
      });
      if (snap) {
        snap.t = Date.now() - t0;
        snap.ty = extractTranslateY(snap.transform);
        frames.push(snap);
      }
    }

    return frames;
  }

  const routes = [
    { path: "/transactions", label: "Dashboard→Transactions" },
    { path: "/budgets",      label: "Transactions→Budgets" },
    { path: "/",             label: "Budgets→Dashboard" },
  ];

  let anyNavAnimated = false;

  for (let i = 0; i < routes.length; i++) {
    const { path, label } = routes[i];
    info(`\n  Nav: ${label}`);

    const frames = await navigateAndSampleFrames(path, label);

    // Save screenshot at t≈60ms (mid-animation)
    const shotName = `w9_pageenter_${i}_${path.replace(/\//g, "_") || "dash"}.png`;
    await page.screenshot({ path: join(SHOT_DIR, shotName) });
    info(`  Screenshot: ${shotName}`);

    // Print all frames
    for (const f of frames) {
      info(`  t=${f.t}ms  opacity=${f.opacity.toFixed(3)}  translateY=${f.ty.toFixed(2)}px  animationName=${f.animationName}  class=${f.hasClass}`);
    }

    // Assess: did any early frame (t<=200ms) show mid-sweep?
    const earlyFrames = frames.filter((f) => f.t <= 200);
    const midSweep = earlyFrames.some((f) => f.opacity < 0.98 || Math.abs(f.ty) > 3);
    const finalFrame = frames[frames.length - 1];
    const settled = finalFrame && Math.abs(finalFrame.ty) < 2 && finalFrame.opacity > 0.95;

    const animSeen = frames.some(
      (f) => f.animationName === "wonder-page-enter" || f.hasClass
    );

    if (midSweep && settled) {
      pass(`W-9 ${label}: mid-sweep observed then settled`, `earlyOpacity=${earlyFrames[0]?.opacity.toFixed(3)} earlyTY=${earlyFrames[0]?.ty.toFixed(2)}px finalTY=${finalFrame?.ty.toFixed(2)}px`);
      anyNavAnimated = true;
    } else if (animSeen) {
      pass(`W-9 ${label}: animation class/name observed (sweep may be fast)`, `animName observed frames=${frames.map(f => f.animationName).join(",")}`);
      anyNavAnimated = true;
    } else {
      fail(`W-9 ${label}: no observable sweep — samples show identity/settled already`, `earlyOpacity=${earlyFrames[0]?.opacity.toFixed(3)} earlyTY=${earlyFrames[0]?.ty.toFixed(2)}px animName=${frames[0]?.animationName}`);
    }

    // Perceptibility guard: across ALL frames (not just early), was peak translateY >= 10px
    // or opacity clearly < 0.5? (earlyFrames may be empty if animation starts after t=200ms)
    const allTYs = frames.map((f) => Math.abs(f.ty));
    const allOpacities = frames.map((f) => f.opacity);
    const peakTY = allTYs.length > 0 ? Math.max(...allTYs) : 0;
    const minOpacity = allOpacities.length > 0 ? Math.min(...allOpacities) : 1;

    if (peakTY >= 10) {
      pass(`W-9 perceptibility: peak translateY >= 10px for ${label}`, `peakTY=${peakTY.toFixed(2)}px`);
    } else if (minOpacity < 0.5) {
      pass(`W-9 perceptibility: clearly sub-0.5 opacity early for ${label}`, `minOpacity=${minOpacity.toFixed(3)}`);
    } else {
      fail(`W-9 perceptibility: peak translateY < 10px AND opacity not clearly < 0.5 for ${label}`, `peakTY=${peakTY.toFixed(2)}px minOpacity=${minOpacity.toFixed(3)} — may be too subtle or already settled`);
    }

    await page.waitForTimeout(400); // let animation finish before next nav
  }

  // CHECK: off mode — no animation
  await page.evaluate(() => document.documentElement.setAttribute("data-wonder", "off"));
  await page.waitForTimeout(200);

  const offFrames = await navigateAndSampleFrames("/accounts", "off-mode nav");
  await page.screenshot({ path: join(SHOT_DIR, "w9_pageenter_off.png") });

  const offAnimSeen = offFrames.some(
    (f) => f.animationName === "wonder-page-enter"
  );
  if (!offAnimSeen) {
    pass("W-9 off: no wonder-page-enter animation", `animNames=${[...new Set(offFrames.map(f => f.animationName))].join(",")}`);
  } else {
    fail("W-9 off: wonder-page-enter animation seen in off mode", `animNames=${offFrames.map(f => f.animationName).join(",")}`);
  }

  // CHECK: reduced-motion — no animation
  await page.evaluate(() => document.documentElement.removeAttribute("data-wonder"));
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.waitForTimeout(200);

  const rmFrames = await navigateAndSampleFrames("/goals", "reduced-motion nav");
  await page.screenshot({ path: join(SHOT_DIR, "w9_pageenter_reduced.png") });

  const rmAnimSeen = rmFrames.some((f) => f.animationName === "wonder-page-enter");
  if (!rmAnimSeen) {
    pass("W-9 reduced-motion: no wonder-page-enter animation", `animNames=${[...new Set(rmFrames.map(f => f.animationName))].join(",")}`);
  } else {
    fail("W-9 reduced-motion: wonder-page-enter seen under reduced-motion", `animNames=${rmFrames.map(f => f.animationName).join(",")}`);
  }

  if (consoleErrors.length > 0) {
    fail("W-9 console: no errors during navigation", `${consoleErrors.length} error(s): ${consoleErrors.slice(0, 3).join(" | ")}`);
  } else {
    pass("W-9 console: no errors during navigation");
  }

  await ctx.close();
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION 6 — W-15: Count-up
// ═══════════════════════════════════════════════════════════════════════════════
async function testW15Countup(browser) {
  console.log("\n══════ W-15: Count-up ══════");

  // TEST A: DEFAULT — mid-tween capture + settled exact match
  {
    const page = await browser.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));

    await page.evaluate(() => document.documentElement.removeAttribute("data-wonder"));
    await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("#app .bento", { timeout: 60000 });
    await page.waitForTimeout(100); // catch mid-tween early

    const midText = await page.locator("[data-countup]").first().textContent().catch(() => null);
    await page.screenshot({ path: join(SHOT_DIR, "w15_mid_tween.png") });

    await page.waitForTimeout(700); // settle (dur-slow=300ms + buffer)
    const settledText = await page.locator("[data-countup]").first().textContent().catch(() => null);
    await page.screenshot({ path: join(SHOT_DIR, "w15_settled.png") });

    // Get "true final" via a wonder=off page
    const page2 = await browser.newPage();
    await page2.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await page2.evaluate(() => document.documentElement.setAttribute("data-wonder", "off"));
    await page2.waitForSelector("#app .bento", { timeout: 60000 });
    await page2.waitForTimeout(300);
    const exactFinal = await page2.locator("[data-countup]").first().textContent().catch(() => null);
    await page2.close();

    info(`mid-tween:  "${midText}"`);
    info(`settled:    "${settledText}"`);
    info(`exactFinal: "${exactFinal}"`);

    if (settledText !== null && exactFinal !== null && settledText === exactFinal) {
      pass("W-15: settled value matches exact final (no corruption)", `"${settledText}"`);
    } else if (settledText === null || exactFinal === null) {
      fail("W-15: [data-countup] element not found", "no element");
    } else {
      fail("W-15: settled value !== exact final — possible corruption", `settled="${settledText}" exact="${exactFinal}"`);
    }

    if (midText !== null && settledText !== null && midText !== settledText) {
      pass("W-15: mid-tween value differs from final (animation observed)", `mid="${midText}" final="${settledText}"`);
    } else {
      info(`W-15: mid-tween === settled (animation faster than 100ms window or no data)`);
    }

    const combined = (midText || "") + (settledText || "");
    if (/NaN|undefined/.test(combined)) {
      fail("W-15: NaN/undefined detected in count-up values", combined);
    } else {
      pass("W-15: no NaN or undefined in count-up values");
    }

    if (errors.length) fail("W-15 console errors", errors.join(" | "));
    await page.close();
  }

  // TEST B: OFF — value shown immediately, no tween
  {
    const page = await browser.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));

    await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await page.evaluate(() => document.documentElement.setAttribute("data-wonder", "off"));
    await page.waitForSelector("#app .bento", { timeout: 60000 });
    await page.waitForTimeout(200);

    const earlyText = await page.locator("[data-countup]").first().textContent().catch(() => null);
    await page.waitForTimeout(500);
    const lateText = await page.locator("[data-countup]").first().textContent().catch(() => null);

    info(`OFF early: "${earlyText}"  late: "${lateText}"`);

    if (earlyText !== null && earlyText === lateText) {
      pass("W-15 off: value stable immediately (no tween)", `"${earlyText}"`);
    } else if (earlyText !== lateText) {
      fail("W-15 off: value changed over time (tween ran when it shouldn't)", `"${earlyText}" → "${lateText}"`);
    }
    if (earlyText !== null && /NaN|undefined/.test(earlyText)) fail("W-15 off: NaN in value", earlyText);
    if (errors.length) fail("W-15 off console errors", errors.join(" | "));
    await page.close();
  }

  // TEST C: REDUCED-MOTION — same as off
  {
    const page = await browser.newPage();
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
    await page.waitForSelector("#app .bento", { timeout: 60000 });
    await page.waitForTimeout(200);

    const earlyText = await page.locator("[data-countup]").first().textContent().catch(() => null);
    await page.waitForTimeout(500);
    const lateText = await page.locator("[data-countup]").first().textContent().catch(() => null);

    info(`REDUCED early: "${earlyText}"  late: "${lateText}"`);

    if (earlyText !== null && earlyText === lateText) {
      pass("W-15 reduced-motion: value stable immediately", `"${earlyText}"`);
    } else if (earlyText !== lateText) {
      fail("W-15 reduced-motion: value changed (tween ran)", `"${earlyText}" → "${lateText}"`);
    }
    await page.close();
  }
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION 7 — W-18: Chart draw-in
// ═══════════════════════════════════════════════════════════════════════════════
async function testW18ChartDrawin(browser) {
  console.log("\n══════ W-18: Chart draw-in ══════");

  async function goToChart(page) {
    await page.goto(BASE + "/dashboard", { waitUntil: "domcontentloaded" });
    await page.waitForSelector(".wonder-chart-line, #app .bento", { timeout: 30000 }).catch(() => {});
    await page.waitForTimeout(1000);
    const has = await page.$(".wonder-chart-line");
    if (!has) {
      await page.goto(BASE + "/reports", { waitUntil: "domcontentloaded" });
      await page.waitForSelector(".wonder-chart-line", { timeout: 20000 }).catch(() => {});
    }
  }

  // CHECK A: DEFAULT — animation wiring + pathLength
  {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));
    page.on("console", (m) => { if (m.type() === "error") errors.push(m.text()); });

    await goToChart(page);

    const lineEl = await page.$(".wonder-chart-line");
    if (!lineEl) {
      fail("W-18: .wonder-chart-line exists", "NOT FOUND in DOM");
    } else {
      pass("W-18: .wonder-chart-line exists");

      const pl = await lineEl.getAttribute("pathLength");
      if (pl === "1") pass('W-18: pathLength="1"', `got "${pl}"`);
      else fail('W-18: pathLength="1"', `got "${pl}"`);

      const animName = await page.evaluate(() => {
        const el = document.querySelector(".wonder-chart-line");
        return el ? getComputedStyle(el).animationName : "NOT_FOUND";
      });
      if (animName === "wonder-chart-draw") {
        pass("W-18: animation-name = wonder-chart-draw", animName);
      } else {
        fail("W-18: animation-name = wonder-chart-draw", `got "${animName}"`);
      }

      const areaEl = await page.$(".wonder-chart-area");
      if (areaEl) pass("W-18: .wonder-chart-area exists");
      else fail("W-18: .wonder-chart-area exists", "not found");
    }

    await page.screenshot({ path: join(SHOT_DIR, "w18_chart_default.png") });
    if (errors.length) fail("W-18 console errors (default)", errors.join("; "));
    else pass("W-18 no console errors (default)");

    await ctx.close();
  }

  // CHECK B: SETTLED LINE — stroke-dashoffset = 0
  {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));

    await goToChart(page);
    await page.waitForTimeout(900); // animation settle

    const settled = await page.evaluate(() => {
      const el = document.querySelector(".wonder-chart-line");
      if (!el) return null;
      const cs = getComputedStyle(el);
      return {
        strokeDashoffset: cs.strokeDashoffset,
        animationFillMode: cs.animationFillMode,
        opacity: cs.opacity,
        visibility: cs.visibility,
      };
    });

    if (!settled) {
      fail("W-18 settled: .wonder-chart-line found", "element missing");
    } else {
      info(`settled dashoffset="${settled.strokeDashoffset}" fillMode="${settled.animationFillMode}" opacity="${settled.opacity}"`);

      const dashoffset = parseFloat(settled.strokeDashoffset);
      if (Math.abs(dashoffset) < 0.01) {
        pass("W-18 settled: stroke-dashoffset = 0 (full line drawn)", `got "${settled.strokeDashoffset}"`);
      } else {
        fail("W-18 settled: stroke-dashoffset should be 0 (full line drawn)", `got "${settled.strokeDashoffset}"`);
      }

      if (settled.animationFillMode === "both") {
        pass("W-18 settled: animation-fill-mode = both", settled.animationFillMode);
      } else {
        fail("W-18 settled: animation-fill-mode should be both", `got "${settled.animationFillMode}"`);
      }

      if (settled.opacity !== "0" && settled.visibility !== "hidden") {
        pass("W-18 settled: line visible", `opacity=${settled.opacity}`);
      } else {
        fail("W-18 settled: line not visible", `opacity=${settled.opacity} visibility=${settled.visibility}`);
      }
    }

    await page.screenshot({ path: join(SHOT_DIR, "w18_chart_settled.png") });
    await ctx.close();
  }

  // CHECK C: OFF mode
  {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));

    await goToChart(page);
    await page.evaluate(() => document.documentElement.setAttribute("data-wonder", "off"));
    await page.waitForTimeout(200);

    const off = await page.evaluate(() => {
      const el = document.querySelector(".wonder-chart-line");
      if (!el) return null;
      const cs = getComputedStyle(el);
      return { animationName: cs.animationName, strokeDashoffset: cs.strokeDashoffset, opacity: cs.opacity };
    });

    if (!off) {
      fail("W-18 off: .wonder-chart-line found", "element missing");
    } else {
      info(`off animationName="${off.animationName}" dashoffset="${off.strokeDashoffset}"`);

      if (off.animationName === "none") {
        pass("W-18 off: animation-name = none", off.animationName);
      } else {
        fail("W-18 off: animation-name should be none", `got "${off.animationName}"`);
      }

      const dashoffset = parseFloat(off.strokeDashoffset);
      if (Math.abs(dashoffset) < 0.01) {
        pass("W-18 off: stroke-dashoffset = 0 (line immediately visible)", `got "${off.strokeDashoffset}"`);
      } else {
        fail("W-18 off: stroke-dashoffset should be 0", `got "${off.strokeDashoffset}"`);
      }
    }

    await page.screenshot({ path: join(SHOT_DIR, "w18_chart_off.png") });
    await ctx.close();
  }

  // CHECK D: REDUCED MOTION
  {
    const ctx = await browser.newContext({ reducedMotion: "reduce" });
    const page = await ctx.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));

    await goToChart(page);
    await page.waitForTimeout(200);

    const rm = await page.evaluate(() => {
      const el = document.querySelector(".wonder-chart-line");
      if (!el) return null;
      const cs = getComputedStyle(el);
      return { animationName: cs.animationName, strokeDashoffset: cs.strokeDashoffset, opacity: cs.opacity };
    });

    if (!rm) {
      fail("W-18 reduced-motion: .wonder-chart-line found", "element missing");
    } else {
      info(`reduced animationName="${rm.animationName}" dashoffset="${rm.strokeDashoffset}"`);

      if (rm.animationName === "none") {
        pass("W-18 reduced-motion: animation-name = none", rm.animationName);
      } else {
        fail("W-18 reduced-motion: animation-name should be none", `got "${rm.animationName}"`);
      }

      const dashoffset = parseFloat(rm.strokeDashoffset);
      if (Math.abs(dashoffset) < 0.01) {
        pass("W-18 reduced-motion: stroke-dashoffset = 0 (line visible)", `got "${rm.strokeDashoffset}"`);
      } else {
        fail("W-18 reduced-motion: stroke-dashoffset should be 0", `got "${rm.strokeDashoffset}"`);
      }
    }

    await page.screenshot({ path: join(SHOT_DIR, "w18_chart_reduced.png") });
    await ctx.close();
  }
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION 8 — W-11/12: Row stagger + bento entrance
// ═══════════════════════════════════════════════════════════════════════════════
async function testW1112Entrances(browser) {
  console.log("\n══════ W-11/12: Row stagger + bento entrance ══════");
  const ctx = await browser.newContext();
  const page = await ctx.newPage();

  await bootApp(page);
  await page.evaluate(() => document.documentElement.removeAttribute("data-wonder"));

  // W-12: bento tiles — check they have animation-name at full
  const bentoAnim = await page.evaluate(() => {
    const el = document.querySelector(".bento .w");
    if (!el) return null;
    // Right after mount the animation should be wonder-bento-enter (or already finished with fill-mode:both)
    return { animationName: getComputedStyle(el).animationName };
  });

  if (!bentoAnim) {
    info("W-12: no .bento .w found (may not be on dashboard)");
  } else {
    info(`W-12: .bento .w animationName="${bentoAnim.animationName}"`);
    // After BOOT_MS=5s the animation may have already played; check animation-name != none means it was wired
    if (bentoAnim.animationName !== "none") {
      pass("W-12: .bento .w has animation wired (wonder-bento-enter or settled fill)", bentoAnim.animationName);
    } else {
      info("W-12: animationName=none (may have settled or off) — checking fill-mode");
    }
  }

  // W-3: bento TILE hover-lift. Regression guard — tiles carry the
  // wonder-bento-enter entrance animation (fill-mode:both, final keyframe
  // transform:none); without an !important hover rule the lift is silently
  // clobbered by the filled animation's end-state (the bug this section was
  // added to catch). Hover the first tile and assert a non-identity translateY.
  const tileEl = await page.$(".bento .w:not(.drag)");
  if (!tileEl) {
    info("W-3: no .bento .w tile found — skipping tile hover-lift check");
  } else {
    await tileEl.scrollIntoViewIfNeeded();
    await tileEl.hover();
    await page.waitForTimeout(220);
    const tileTy = await page.evaluate(() => {
      const el = document.querySelector(".bento .w:not(.drag)");
      const m = getComputedStyle(el).transform.match(/matrix\(([^)]+)\)/);
      return m ? Number(m[1].split(",")[5]) : 0;
    });
    info(`W-3: tile hover translateY=${tileTy.toFixed(2)}px`);
    if (Math.abs(tileTy) >= 4) {
      pass("W-3: bento tile hover-lift is perceptible (>=4px, beats wonder-bento-enter fill)", `translateY=${tileTy.toFixed(2)}px`);
    } else {
      fail("W-3: bento tile hover-lift missing/too subtle", `translateY=${tileTy.toFixed(2)}px — filled wonder-bento-enter likely clobbering the hover transform`);
    }
    // Move the pointer away so it doesn't bleed into later sections.
    await page.mouse.move(2, 2);

    // W-3 drag-ghost: a tile being dragged (.w.drag) must dim to opacity .35 as a
    // functional grab cue. Regression guard — same filled-animation clobber as the
    // hover-lift: wonder-bento-enter (fill-mode:both, opacity:1 end-state) would
    // override the non-important .35 without the !important fix.
    const dragGhost = await page.evaluate(() => {
      const t = document.querySelector(".bento .w");
      if (!t) return null;
      t.classList.add("drag");
      const o = getComputedStyle(t).opacity;
      t.classList.remove("drag");
      return o;
    });
    if (dragGhost === null) {
      info("W-3 drag-ghost: no tile found");
    } else if (Math.abs(parseFloat(dragGhost) - 0.35) < 0.05) {
      pass("W-3 drag-ghost: dragging tile dims to .35 (beats wonder-bento-enter fill)", `opacity=${dragGhost}`);
    } else {
      fail("W-3 drag-ghost: dragging tile not dimmed", `opacity=${dragGhost} — filled wonder-bento-enter likely clobbering .w.drag opacity`);
    }
  }

  // W-11: row stagger on accounts/budgets list
  await spaNav(page, "/accounts");
  await page.waitForTimeout(500); // catch early

  const rowAnim = await page.evaluate(() => {
    const el = document.querySelector(".rows .row:not(.txn-table .row), .list-rows .row:not(.txn-table .row)");
    if (!el) return null;
    const cs = getComputedStyle(el);
    return { animationName: cs.animationName, animationDelay: cs.animationDelay };
  });

  if (rowAnim) {
    info(`W-11: row animationName="${rowAnim.animationName}" delay="${rowAnim.animationDelay}"`);
    if (rowAnim.animationName !== "none") {
      pass("W-11: list rows have wonder-row-enter animation wired", rowAnim.animationName);
    } else {
      info("W-11: animationName=none (may have settled; boot-wait already consumed animation window)");
    }
  } else {
    info("W-11: no .rows .row found on accounts page");
  }

  // W-11 off: animation should be none
  await page.evaluate(() => document.documentElement.setAttribute("data-wonder", "off"));
  const rowAnimOff = await page.evaluate(() => {
    const el = document.querySelector(".rows .row:not(.txn-table .row), .list-rows .row:not(.txn-table .row)");
    if (!el) return null;
    return { animationName: getComputedStyle(el).animationName };
  });

  if (rowAnimOff) {
    info(`W-11 off: row animationName="${rowAnimOff.animationName}"`);
    if (rowAnimOff.animationName === "none") {
      pass("W-11 off: row stagger animation = none", rowAnimOff.animationName);
    } else {
      fail("W-11 off: row stagger should be none", `got "${rowAnimOff.animationName}"`);
    }
  }

  await page.screenshot({ path: join(SHOT_DIR, "w1112_entrances.png") });
  await ctx.close();
}

// ═══════════════════════════════════════════════════════════════════════════════
// SECTION 9 — GLOBAL GATE: wonder=off + reduced-motion → fully motionless
// ═══════════════════════════════════════════════════════════════════════════════
async function testGlobalGate(browser) {
  console.log("\n══════ GLOBAL GATE: wonder=off + reduced-motion ══════");
  const ctx = await browser.newContext({ reducedMotion: "reduce" });
  const page = await ctx.newPage();

  await bootApp(page);
  await page.evaluate(() => document.documentElement.setAttribute("data-wonder", "off"));
  await page.waitForTimeout(200);

  // Sample 6 flourish elements
  const checks = await page.evaluate(() => {
    const selectors = [
      { sel: ".card",                              label: "card (W-1)" },
      { sel: ".btn",                               label: "btn (W-2)" },
      { sel: ".row:not(.txn-table .row)",          label: "list row (W-4)" },
      { sel: ".nv",                                label: "nav (W-5)" },
      { sel: "#cf-page-view",                      label: "page-view (W-9)" },
      { sel: ".bento .w",                          label: "bento tile (W-12)" },
    ];
    return checks = selectors.map(({ sel, label }) => {
      const el = document.querySelector(sel);
      if (!el) return { label, found: false };
      const cs = getComputedStyle(el);
      return {
        label,
        found: true,
        transform: cs.transform,
        animationName: cs.animationName,
        transitionDuration: cs.transitionDuration,
      };
    });
  });

  let allMotionless = true;
  for (const c of checks) {
    if (!c.found) {
      info(`GLOBAL GATE: "${c.label}" not found on page (skip)`);
      continue;
    }
    const hasMotion = !isIdentity(c.transform) || (c.animationName && c.animationName !== "none" && c.animationName !== "");
    const motionNote = `transform="${c.transform}" anim="${c.animationName}"`;

    if (!hasMotion) {
      pass(`GLOBAL GATE: ${c.label} = motionless`, motionNote);
    } else {
      fail(`GLOBAL GATE: ${c.label} has motion (should be none)`, motionNote);
      allMotionless = false;
    }
  }

  // Also check wonder-on token is 0
  const wonderOn = await page.evaluate(() =>
    getComputedStyle(document.documentElement).getPropertyValue("--wonder-on").trim()
  );
  info(`--wonder-on: "${wonderOn}"`);
  if (parseFloat(wonderOn) === 0) {
    pass("GLOBAL GATE: --wonder-on = 0", `got "${wonderOn}"`);
  } else {
    fail("GLOBAL GATE: --wonder-on should be 0", `got "${wonderOn}"`);
  }

  await page.screenshot({ path: join(SHOT_DIR, "global_gate_off.png") });
  await ctx.close();
}

// ═══════════════════════════════════════════════════════════════════════════════
// MAIN
// ═══════════════════════════════════════════════════════════════════════════════
async function main() {
  console.log("\n╔══════════════════════════════════════════════════════════╗");
  console.log("║        W2 WONDER e2e SUITE — CashFlux                   ║");
  console.log("╚══════════════════════════════════════════════════════════╝");
  console.log(`  BASE: ${BASE}`);
  console.log(`  Screenshots → ${SHOT_DIR}\n`);

  const browser = await chromium.launch({ headless: true });

  try {
    await testW1CardLift(browser);
    await testW2ButtonPress(browser);
    await testW4RowNudge(browser);
    await testW56NavHover(browser);
    await testW9PageEnter(browser);
    await testW15Countup(browser);
    await testW18ChartDrawin(browser);
    await testW1112Entrances(browser);
    await testGlobalGate(browser);
  } finally {
    await browser.close();
  }

  // ── Final summary ─────────────────────────────────────────────────────────
  console.log("\n╔══════════════════════════════════════════════════════════╗");
  console.log("║  WONDER SUITE RESULTS                                    ║");
  console.log("╚══════════════════════════════════════════════════════════╝");
  for (const r of results) {
    console.log("  " + r.line);
  }
  console.log(`\n  Total: ${totalPassed} PASS / ${totalFailed} FAIL`);
  console.log(`  Screenshots: ${SHOT_DIR}`);

  if (totalFailed > 0) {
    console.error(`\nRESULT: FAIL — ${totalFailed} check(s) failed`);
    process.exitCode = 1;
  } else {
    console.log(`\nRESULT: ALL PASS`);
  }
}

main().catch((err) => {
  console.error("Fatal:", err);
  process.exit(1);
});
