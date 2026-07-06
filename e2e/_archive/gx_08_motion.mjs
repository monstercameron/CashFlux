/**
 * GX8 — Micro-interactions & Motion probe
 * Measures transition durations/easings, hover/active states, reduced-motion coverage.
 * Run: node e2e/gx_08_motion.mjs
 * Exit 0 — evidence-harvest / audit script, not a pass/fail gate.
 */
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const OUT = path.join(__dirname, "screenshots");
fs.mkdirSync(OUT, { recursive: true });

let idx = 0;
async function ss(page, name) {
  const file = path.join(OUT, `gx08_${String(++idx).padStart(2, "0")}_${name}.png`);
  await page.screenshot({ path: file, fullPage: false });
  console.log("  📸", path.basename(file));
  return path.basename(file);
}

async function waitApp(page) {
  await page.waitForSelector("#app", { timeout: 60_000 });
  await page.waitForFunction(() => !document.querySelector(".loading-spinner"), { timeout: 30_000 }).catch(() => {});
  await page.waitForTimeout(400);
}

async function setTheme(page, theme) {
  await page.evaluate((t) => {
    document.documentElement.setAttribute("data-theme", t);
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: t }));
  }, theme);
  await page.reload({ waitUntil: "domcontentloaded" });
  await waitApp(page);
}

/** Measure computed transition on first matching element */
async function measureTransition(page, selector) {
  return page.evaluate((sel) => {
    const el = document.querySelector(sel);
    if (!el) return null;
    const s = getComputedStyle(el);
    return {
      duration: s.transitionDuration,
      easing: s.transitionTimingFunction,
      property: s.transitionProperty,
    };
  }, selector);
}

/** Extract keyframes, reduced-motion blocks, all transition rules from live stylesheet */
async function analyzeStylesheet(page) {
  return page.evaluate(() => {
    const keyframes = [];
    const reducedMotionBlocks = [];
    const noPreferenceBlocks = [];
    const transitionRules = [];

    function processRules(rules, depth = 0) {
      for (const rule of rules) {
        if (rule.type === CSSRule.KEYFRAMES_RULE) {
          keyframes.push(rule.name);
        } else if (rule.type === CSSRule.MEDIA_RULE) {
          const cond = rule.conditionText || rule.media?.mediaText || "";
          const inner = [];
          try { for (const r of rule.cssRules) inner.push(r.cssText.substring(0, 150)); } catch {}
          if (cond.includes("prefers-reduced-motion")) {
            if (cond.includes("reduce")) reducedMotionBlocks.push({ cond, rules: inner });
            else noPreferenceBlocks.push({ cond, rules: inner });
          }
          try { processRules(Array.from(rule.cssRules), depth + 1); } catch {}
        } else if (rule.type === CSSRule.STYLE_RULE) {
          const text = rule.style.cssText;
          if (text.includes("transition") || text.includes("animation")) {
            transitionRules.push({ sel: rule.selectorText, text: text.substring(0, 200) });
          }
        }
      }
    }

    for (const sheet of document.styleSheets) {
      try { processRules(Array.from(sheet.cssRules)); } catch {}
    }
    return { keyframes, reducedMotionBlocks, noPreferenceBlocks, transitionRules };
  });
}

/** Collect all unique transition durations from computed styles of many elements */
async function collectAllDurations(page) {
  return page.evaluate(() => {
    const selectors = [
      ".btn", ".btn-primary", ".nav-link", ".nav", ".nv",
      ".row", ".chip-x", ".chip-suggest", ".w", ".gear-inline",
      ".rz", ".bar-fill", ".flip-inner", ".flip-backdrop",
      ".switch", ".switch::after", "aside.rail", ".toast",
      ".attention-item", ".wm-step-btn", ".wm-arrow",
      ".wb-node", ".wb-tile", ".menu-btn", ".data-btn",
      ".member-add", ".seg-btn", ".mobile-tabbar .tab-item",
      ".btn-link", ".btn-del", ".btn-ghost-danger",
      ".set-btn", ".set-close", ".disclosure-toggle",
    ];
    const results = [];
    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (!el) { results.push({ sel, found: false }); continue; }
      const s = getComputedStyle(el);
      results.push({
        sel,
        found: true,
        duration: s.transitionDuration,
        easing: s.transitionTimingFunction,
        property: s.transitionProperty,
        animName: s.animationName,
        animDuration: s.animationDuration,
      });
    }
    return results;
  });
}

(async () => {
  const browser = await chromium.launch({ headless: true });

  try {
    // ─── DARK THEME ──────────────────────────────────────────────────────────
    console.log("\n=== DARK THEME ===");
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    const page = await ctx.newPage();
    await page.goto(BASE, { waitUntil: "domcontentloaded" });
    await waitApp(page);
    await setTheme(page, "dark");

    await ss(page, "dark_shell");

    // 1. Stylesheet analysis
    console.log("\n--- Stylesheet analysis (dark) ---");
    const css = await analyzeStylesheet(page);
    console.log("@keyframes:", css.keyframes.join(", "));
    console.log("\nprefers-reduced-motion: reduce blocks:");
    for (const b of css.reducedMotionBlocks) {
      console.log(" ", b.cond);
      for (const r of b.rules) console.log("   ", r);
    }
    console.log("\nprefers-reduced-motion: no-preference blocks:");
    for (const b of css.noPreferenceBlocks) {
      console.log(" ", b.cond);
      for (const r of b.rules) console.log("   ", r);
    }

    // 2. Transactions page — hover states
    await page.goto(BASE + "/#/transactions", { waitUntil: "domcontentloaded" });
    await waitApp(page);
    await ss(page, "dark_transactions_baseline");

    // Measure transitions on key elements
    console.log("\n--- Transition measurements (dark, /transactions) ---");
    const elements = [
      [".btn", ".btn"],
      [".nav-link", ".nav-link"],
      [".row", ".row"],
      ["aside.rail", "aside.rail"],
      [".switch", ".switch"],
      [".bar-fill", ".bar-fill"],
      [".chip-x", ".chip-x"],
      [".chip-suggest", ".chip-suggest"],
    ];
    for (const [sel, label] of elements) {
      const m = await measureTransition(page, sel);
      if (m) console.log(`  ${label}: dur=${m.duration} ease=${m.easing} prop=${m.property}`);
      else console.log(`  ${label}: NOT IN DOM`);
    }

    // Hover nav link
    const navLink = await page.$(".nav-link");
    if (navLink) {
      await navLink.hover();
      await page.waitForTimeout(200);
      await ss(page, "dark_navlink_hover");
    }

    // Hover btn
    const btn = await page.$(".btn");
    if (btn) {
      await btn.hover();
      await page.waitForTimeout(200);
      await ss(page, "dark_btn_hover");
    }

    // Hover row
    const row = await page.$(".row");
    if (row) {
      await row.hover();
      await page.waitForTimeout(200);
      await ss(page, "dark_row_hover");
    }

    // 3. Budgets — bar-fill + wb-node hover
    await page.goto(BASE + "/#/budgets", { waitUntil: "domcontentloaded" });
    await waitApp(page);
    await ss(page, "dark_budgets_baseline");

    const barFill = await measureTransition(page, ".bar-fill");
    console.log("\n  bar-fill transition (budgets):", barFill);

    const wbNode = await page.$(".wb-node");
    if (wbNode) {
      await wbNode.hover();
      await page.waitForTimeout(200);
      await ss(page, "dark_wbnode_hover");
    }

    // 4. Dashboard — bento widget hover + gear-inline
    await page.goto(BASE + "/#/dashboard", { waitUntil: "domcontentloaded" });
    await waitApp(page);
    await ss(page, "dark_dashboard_baseline");

    const widget = await page.$(".w");
    if (widget) {
      await widget.hover();
      await page.waitForTimeout(200);
      await ss(page, "dark_bento_hover");
    }

    const gearInline = await measureTransition(page, ".gear-inline");
    console.log("\n  gear-inline transition:", gearInline);
    const rzHandle = await measureTransition(page, ".rz");
    console.log("  rz (resize handle) transition:", rzHandle);

    // 5. Settings modal (FlipPanel)
    console.log("\n--- FlipPanel modal (dark) ---");
    try {
      // Try the gear-abs button (settings)
      const gearAbs = await page.$(".gear-abs");
      if (gearAbs) {
        await gearAbs.click();
        await page.waitForTimeout(600);
        const flipOpacity = await page.evaluate(() => {
          const el = document.querySelector(".flip-backdrop");
          return el ? getComputedStyle(el).opacity : null;
        });
        console.log("  flip-backdrop opacity (open):", flipOpacity);
        await ss(page, "dark_flip_open");

        const flipInner = await measureTransition(page, ".flip-inner");
        const flipBackdrop = await measureTransition(page, ".flip-backdrop");
        console.log("  flip-inner:", flipInner);
        console.log("  flip-backdrop:", flipBackdrop);

        const closeBtn = await page.$(".set-close, .set-btn.close");
        if (closeBtn) { await closeBtn.click(); await page.waitForTimeout(400); }
        await ss(page, "dark_flip_closed");
      } else {
        console.log("  .gear-abs not found on dashboard");
      }
    } catch (e) {
      console.log("  FlipPanel open failed:", e.message);
    }

    // 6. Full duration inventory
    console.log("\n--- Full transition duration inventory (dark) ---");
    const durations = await collectAllDurations(page);
    const byDur = {};
    for (const d of durations) {
      if (!d.found) continue;
      const key = d.duration || "none";
      if (!byDur[key]) byDur[key] = [];
      byDur[key].push(d.sel);
    }
    console.log("  Grouped by computed transitionDuration:");
    for (const [dur, sels] of Object.entries(byDur).sort()) {
      console.log(`  ${dur}: ${sels.join(", ")}`);
    }
    const notFound = durations.filter(d => !d.found).map(d => d.sel);
    if (notFound.length) console.log("  NOT IN DOM:", notFound.join(", "));

    // ─── LIGHT THEME ─────────────────────────────────────────────────────────
    console.log("\n=== LIGHT THEME ===");
    await setTheme(page, "light");
    await ss(page, "light_shell");

    await page.goto(BASE + "/#/transactions", { waitUntil: "domcontentloaded" });
    await waitApp(page);
    await ss(page, "light_transactions_baseline");

    // Hover states light
    const navLinkL = await page.$(".nav-link");
    if (navLinkL) { await navLinkL.hover(); await page.waitForTimeout(200); await ss(page, "light_navlink_hover"); }
    const btnL = await page.$(".btn");
    if (btnL) { await btnL.hover(); await page.waitForTimeout(200); await ss(page, "light_btn_hover"); }
    const rowL = await page.$(".row");
    if (rowL) { await rowL.hover(); await page.waitForTimeout(200); await ss(page, "light_row_hover"); }

    await page.goto(BASE + "/#/budgets", { waitUntil: "domcontentloaded" });
    await waitApp(page);
    await ss(page, "light_budgets");

    // ─── REDUCED MOTION ───────────────────────────────────────────────────────
    console.log("\n=== REDUCED MOTION EMULATION ===");
    await ctx.close();
    const rmCtx = await browser.newContext({
      viewport: { width: 1280, height: 800 },
      reducedMotion: "reduce",
    });
    const rmPage = await rmCtx.newPage();
    await rmPage.goto(BASE, { waitUntil: "domcontentloaded" });
    await waitApp(rmPage);
    await ss(rmPage, "rm_shell");

    // Check boot animations are suppressed
    const rmBoot = await rmPage.evaluate(() => {
      const ring = document.querySelector(".boot-ring");
      const app = document.querySelector("#app");
      return {
        bootRingAnim: ring ? getComputedStyle(ring).animationName : "NOT_IN_DOM",
        appSettleAnim: app ? getComputedStyle(app).animationName : "NOT_IN_DOM",
        bootTransition: (() => {
          const boot = document.querySelector("#boot");
          return boot ? getComputedStyle(boot).transitionDuration : "NOT_IN_DOM";
        })(),
      };
    });
    console.log("  Boot animations under reduced-motion:", rmBoot);

    await rmPage.goto(BASE + "/#/transactions", { waitUntil: "domcontentloaded" });
    await waitApp(rmPage);
    await ss(rmPage, "rm_transactions");

    // Check which interactive elements still have transitions under reduced-motion
    const rmInteractive = await rmPage.evaluate(() => {
      const checks = [
        ".btn", ".nav-link", ".row", ".chip-x", ".chip-suggest",
        ".wb-node", ".attention-item", ".wm-step-btn", ".wm-arrow",
        ".gear-inline", ".rz", ".w", ".menu-btn", ".data-btn",
        ".mobile-tabbar .tab-item", ".btn-link", ".seg-btn",
      ];
      const results = [];
      for (const sel of checks) {
        const el = document.querySelector(sel);
        if (!el) { results.push({ sel, found: false }); continue; }
        const s = getComputedStyle(el);
        const dur = s.transitionDuration;
        const hasTrans = dur && dur !== "0s" && dur !== "0ms";
        results.push({ sel, found: true, duration: dur, easing: s.transitionTimingFunction, hasTrans });
      }
      return results;
    });

    console.log("\n  Interactive elements under reduced-motion:");
    const unguarded = [];
    for (const r of rmInteractive) {
      if (!r.found) { console.log(`  - ${r.sel}: NOT IN DOM`); continue; }
      if (r.hasTrans) {
        console.log(`  ⚠ UNGUARDED ${r.sel}: dur=${r.duration} ease=${r.easing}`);
        unguarded.push(r.sel);
      } else {
        console.log(`  ✓ ${r.sel}: no transition (${r.duration})`);
      }
    }
    console.log(`\n  Total unguarded interactive transitions: ${unguarded.length}`);
    console.log("  Unguarded:", unguarded.join(", ") || "none");

    await rmPage.goto(BASE + "/#/budgets", { waitUntil: "domcontentloaded" });
    await waitApp(rmPage);
    await ss(rmPage, "rm_budgets");

    // Check bar-fill under reduced-motion
    const rmBarFill = await measureTransition(rmPage, ".bar-fill");
    console.log("\n  bar-fill under reduced-motion:", rmBarFill);
    // (bar-fill is inside @media no-preference, so should be 0s under reduce)

    // Check cf-jump-flash under reduced-motion
    const rmJumpFlash = await rmPage.evaluate(() => {
      const el = document.querySelector(".cf-jump-flash");
      if (!el) return "NOT_IN_DOM";
      return getComputedStyle(el).animationName;
    });
    console.log("  cf-jump-flash anim under reduced-motion:", rmJumpFlash);

    // ─── ACTIVE/PRESS RULE VERIFICATION ───────────────────────────────────────
    console.log("\n=== :ACTIVE PRESS FEEDBACK ===");
    const activeRuleText = await rmPage.evaluate(() => {
      for (const sheet of document.styleSheets) {
        try {
          for (const rule of sheet.cssRules) {
            if (rule.type === CSSRule.MEDIA_RULE) {
              const cond = rule.conditionText || rule.media?.mediaText || "";
              if (cond.includes("no-preference")) {
                for (const inner of rule.cssRules) {
                  if (inner.cssText?.includes(":active") && inner.cssText?.includes("scale")) {
                    return inner.cssText.substring(0, 400);
                  }
                }
              }
            }
          }
        } catch {}
      }
      return null;
    });
    console.log("  :active scale rule (under no-preference):", activeRuleText);

    // ─── SUMMARY ─────────────────────────────────────────────────────────────
    console.log("\n=== SUMMARY ===");
    console.log(`  Screenshots saved to: e2e/screenshots/`);
    console.log(`  Total screenshots: ${idx}`);
    const shots = fs.readdirSync(OUT).filter(f => f.startsWith("gx08_"));
    console.log("  Files:", shots.join(", "));

    await rmCtx.close();
  } finally {
    await browser.close();
  }
})();
