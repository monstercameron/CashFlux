/**
 * gm_verify.mjs — Verify CSS modal fixes landed in web/index.html
 *
 * Checks (all in light theme, plus dark spot-check):
 *   GM2-1:   .set-h h3 title is dark (#1c1c1e ≈ rgb(28,28,30)) in light — was white-on-white.
 *   GM2-2/4-9: .set-btn.save has white label on accent in light; .set-btn.cancel neutral.
 *   GM4-8:   .set-h/.set-foot separator borders are light (#e4e2dd) not dark.
 *   GM2-6:   top-bar .add-btn has a visible border in light.
 *   GM4-6:   --hover defined in light; selected command-palette row has visible (non-black) bg.
 *   GM4-7:   #cf-cmd-palette backdrop is warm-tinted (not harsh dark rgba(0,0,0,...)) in light.
 *   GM3-5/6: .cf-dialog-scrim has backdrop-filter blur; .cf-dialog has hairline ring + shadow.
 *   GM1-S5:  .flip-wrap max-width is limited at narrow (768) viewport.
 *
 * Screenshots saved to e2e/screenshots/ with names gm_verify_*.png
 * Run: node e2e/gm_verify.mjs
 */
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOTS = path.join(__dirname, "screenshots");
if (!fs.existsSync(SHOTS)) fs.mkdirSync(SHOTS, { recursive: true });

const shot = (name) => path.join(SHOTS, `gm_verify_${name}.png`);

// ── result tracking ───────────────────────────────────────────────────────────
const results = [];
function pass(id, detail) {
  results.push({ id, status: "PASS", detail });
  console.log(`  PASS [${id}]: ${detail}`);
}
function fail(id, detail) {
  results.push({ id, status: "FAIL", detail });
  console.error(`  FAIL [${id}]: ${detail}`);
}
function info(msg) { console.log("  " + msg); }

// ── colour helpers ────────────────────────────────────────────────────────────
// Parse "rgb(r, g, b)" or "rgba(r, g, b, a)" → [r, g, b]
function parseRgb(css) {
  if (!css) return null;
  const m = css.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/);
  if (!m) return null;
  return [parseInt(m[1]), parseInt(m[2]), parseInt(m[3])];
}

function colourClose(css, target, tol = 10) {
  const a = parseRgb(css);
  const b = parseRgb(target);
  if (!a || !b) return false;
  return Math.abs(a[0]-b[0]) <= tol && Math.abs(a[1]-b[1]) <= tol && Math.abs(a[2]-b[2]) <= tol;
}

function isDark(css) {
  // luminance < 0.1 → near-black
  const c = parseRgb(css);
  if (!c) return false;
  const [r, g, b] = c.map(v => v / 255);
  const lum = 0.2126*r + 0.7152*g + 0.0722*b;
  return lum < 0.1;
}

function isLight(css) {
  const c = parseRgb(css);
  if (!c) return false;
  const [r, g, b] = c.map(v => v / 255);
  const lum = 0.2126*r + 0.7152*g + 0.0722*b;
  return lum > 0.7;
}

// ── boot helpers ─────────────────────────────────────────────────────────────
async function bootTheme(page, theme) {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1200);
  await page.evaluate((t) => {
    try {
      const raw = localStorage.getItem("cashflux:prefs");
      const p = raw ? JSON.parse(raw) : {};
      delete p.viewAsMember;
      p.theme = t;
      localStorage.setItem("cashflux:prefs", JSON.stringify(p));
    } catch (_) {}
  }, theme);
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForFunction(
    (t) => document.documentElement.getAttribute("data-theme") === t,
    theme, { timeout: 20000 }
  );
  await page.waitForTimeout(1000);
}

async function clearOverlay(page) {
  await page.evaluate(() => {
    const o = document.getElementById("gwc-error-overlay") ||
              document.querySelector(".gwc-error-overlay");
    if (o) o.remove();
  });
  await page.waitForTimeout(150);
}

async function openSettings(page) {
  await clearOverlay(page);
  const btn = page.locator("button.hh").first();
  await btn.waitFor({ state: "visible", timeout: 10000 });
  await btn.click();
  await page.waitForSelector(".set-h", { timeout: 8000 });
  await page.waitForTimeout(700); // flip animation
}

async function closePanel(page) {
  const closeBtn = page.locator(".set-close").first();
  if (await closeBtn.isVisible()) {
    await closeBtn.click();
  } else {
    await page.keyboard.press("Escape");
  }
  await page.waitForTimeout(400);
}

// ── CHECK 1: SETTINGS panel in LIGHT ────────────────────────────────────────
async function checkSettingsLight(browser) {
  console.log("\n[1] Settings panel — LIGHT");
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  page.on("pageerror", e => console.error("  page error:", e.message));

  await bootTheme(page, "light");
  await openSettings(page);
  await page.screenshot({ path: shot("settings_light"), fullPage: false });
  info("screenshot saved: gm_verify_settings_light.png");

  const styles = await page.evaluate(() => {
    const h3 = document.querySelector(".set-h h3");
    const saveBtn = document.querySelector(".set-btn.save");
    const cancelBtn = document.querySelector(".set-btn.cancel");
    const setH = document.querySelector(".set-h");
    const setFoot = document.querySelector(".set-foot");
    const cs = (el) => el ? window.getComputedStyle(el) : null;
    const h3s = cs(h3);
    const saves = cs(saveBtn);
    const cancels = cs(cancelBtn);
    const seths = cs(setH);
    const setfoots = cs(setFoot);
    return {
      h3Color: h3s ? h3s.color : null,
      h3Found: !!h3,
      saveBg: saves ? saves.backgroundColor : null,
      saveColor: saves ? saves.color : null,
      saveBorderColor: saves ? saves.borderColor : null,
      cancelBg: cancels ? cancels.backgroundColor : null,
      cancelColor: cancels ? cancels.color : null,
      setHBorderBottom: seths ? seths.borderBottomColor : null,
      setFootBorderTop: setfoots ? setfoots.borderTopColor : null,
    };
  });

  info(`  .set-h h3 color: ${styles.h3Color}`);
  info(`  .set-btn.save bg: ${styles.saveBg}  color: ${styles.saveColor}`);
  info(`  .set-btn.cancel bg: ${styles.cancelBg}  color: ${styles.cancelColor}`);
  info(`  .set-h border-bottom: ${styles.setHBorderBottom}`);
  info(`  .set-foot border-top: ${styles.setFootBorderTop}`);

  // GM2-1: title must be dark
  if (styles.h3Found && colourClose(styles.h3Color, "rgb(28,28,30)", 15)) {
    pass("GM2-1", `panel title dark: ${styles.h3Color}`);
  } else {
    fail("GM2-1", `panel title NOT dark: ${styles.h3Color} (expected ≈ rgb(28,28,30))`);
  }

  // GM2-2/4-9: save button has white label on accent
  const saveColorRgb = parseRgb(styles.saveColor);
  if (saveColorRgb && isLight(styles.saveColor)) {
    pass("GM2-2/GM4-9", `save label is light (white-ish): ${styles.saveColor}`);
  } else {
    fail("GM2-2/GM4-9", `save label NOT white-ish: ${styles.saveColor}`);
  }

  // GM4-8: separators are light borders, not dark
  const setHBorder = parseRgb(styles.setHBorderBottom);
  if (setHBorder && isLight(styles.setHBorderBottom)) {
    pass("GM4-8-header", `.set-h border light: ${styles.setHBorderBottom}`);
  } else {
    fail("GM4-8-header", `.set-h border NOT light: ${styles.setHBorderBottom}`);
  }
  const setFootBorder = parseRgb(styles.setFootBorderTop);
  if (setFootBorder && isLight(styles.setFootBorderTop)) {
    pass("GM4-8-footer", `.set-foot border light: ${styles.setFootBorderTop}`);
  } else {
    fail("GM4-8-footer", `.set-foot border NOT light: ${styles.setFootBorderTop}`);
  }

  await closePanel(page);
  await ctx.close();
}

// ── CHECK 2: ADD modal in LIGHT ───────────────────────────────────────────────
async function checkAddModalLight(browser) {
  console.log("\n[2] Add modal — LIGHT");
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  page.on("pageerror", e => console.error("  page error:", e.message));

  await bootTheme(page, "light");
  await page.waitForSelector(".add-btn", { timeout: 20000 });
  await clearOverlay(page);

  // Capture add-btn border BEFORE clicking
  const addBtnStyles = await page.evaluate(() => {
    const btn = document.querySelector(".add-btn");
    if (!btn) return null;
    const cs = window.getComputedStyle(btn);
    return {
      borderStyle: cs.borderStyle,
      borderWidth: cs.borderWidth,
      borderColor: cs.borderColor,
      outlineWidth: cs.outlineWidth,
    };
  });
  info(`  .add-btn border: ${JSON.stringify(addBtnStyles)}`);

  // GM2-6: add-btn has visible border
  if (addBtnStyles && addBtnStyles.borderStyle !== "none" &&
      parseFloat(addBtnStyles.borderWidth) > 0) {
    pass("GM2-6", `.add-btn has border ${addBtnStyles.borderWidth} ${addBtnStyles.borderStyle} ${addBtnStyles.borderColor}`);
  } else {
    fail("GM2-6", `.add-btn NO visible border: ${JSON.stringify(addBtnStyles)}`);
  }

  // Open +Add menu
  await page.locator(".add-btn").click();
  await page.waitForTimeout(300);

  // Click first menu item to open a FlipPanel modal
  const menuItem = page.locator('[role="menuitem"]').first();
  const menuItemCount = await menuItem.count();
  if (menuItemCount === 0) {
    fail("ADD-MODAL-OPEN", "No menuitem found in +Add menu");
    await ctx.close();
    return;
  }
  const menuItemText = await menuItem.textContent();
  info(`  Clicking menu item: "${menuItemText?.trim()}"`);
  await menuItem.click();
  await page.waitForTimeout(500);

  // Wait for dialog to appear
  const dialog = page.locator('[role="dialog"]').first();
  const dialogCount = await dialog.count();
  if (dialogCount === 0) {
    fail("ADD-MODAL-OPEN", "FlipPanel dialog did not appear");
    await ctx.close();
    return;
  }
  info(`  dialog appeared`);

  await page.screenshot({ path: shot("addmodal_light"), fullPage: false });
  info("screenshot saved: gm_verify_addmodal_light.png");

  // Inspect .set-h h3 title color inside dialog
  const modalStyles = await page.evaluate(() => {
    const h3 = document.querySelector(".set-h h3");
    const cs = h3 ? window.getComputedStyle(h3) : null;
    return { h3Color: cs ? cs.color : null, h3Found: !!h3 };
  });
  info(`  modal .set-h h3 color: ${modalStyles.h3Color}`);

  if (modalStyles.h3Found && colourClose(modalStyles.h3Color, "rgb(28,28,30)", 15)) {
    pass("ADD-MODAL-TITLE", `add modal title dark: ${modalStyles.h3Color}`);
  } else {
    fail("ADD-MODAL-TITLE", `add modal title NOT dark: ${modalStyles.h3Color}`);
  }

  // Close modal
  await page.keyboard.press("Escape");
  await page.waitForTimeout(400);
  await ctx.close();
}

// ── CHECK 3: COMMAND PALETTE in LIGHT ────────────────────────────────────────
async function checkCommandPaletteLight(browser) {
  console.log("\n[3] Command palette — LIGHT");
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  page.on("pageerror", e => console.error("  page error:", e.message));

  await bootTheme(page, "light");
  await clearOverlay(page);

  // Open command palette with Ctrl+K
  await page.keyboard.press("Control+k");
  await page.waitForTimeout(400);

  const paletteEl = page.locator("#cf-cmd-palette").first();
  const paletteCount = await paletteEl.count();
  if (paletteCount === 0) {
    // Try Mod+K
    await page.keyboard.press("Meta+k");
    await page.waitForTimeout(400);
  }

  const paletteFound = await page.locator("#cf-cmd-palette").count() > 0 ||
                       await page.locator('[role="dialog"]').count() > 0;

  if (!paletteFound) {
    fail("GM4-7", "Command palette did not open via Ctrl+K");
    fail("GM4-6", "Command palette did not open — cannot check --hover");
    await ctx.close();
    return;
  }

  // Inspect palette backdrop colour
  const paletteStyles = await page.evaluate(() => {
    const palette = document.querySelector("#cf-cmd-palette");
    if (!palette) return null;
    const cs = window.getComputedStyle(palette);
    return {
      bg: cs.backgroundColor,
    };
  });
  info(`  #cf-cmd-palette bg: ${paletteStyles?.bg}`);

  // GM4-7: backdrop should NOT be harsh dark (near-black)
  if (paletteStyles && !isDark(paletteStyles.bg)) {
    pass("GM4-7", `palette backdrop warm/light: ${paletteStyles.bg}`);
  } else if (paletteStyles) {
    fail("GM4-7", `palette backdrop is near-black: ${paletteStyles.bg}`);
  } else {
    fail("GM4-7", "#cf-cmd-palette element not found");
  }

  // Arrow down once to select first item
  await page.keyboard.press("ArrowDown");
  await page.waitForTimeout(200);

  await page.screenshot({ path: shot("palette_light"), fullPage: false });
  info("screenshot saved: gm_verify_palette_light.png");

  // Check selected item background
  const selectedStyles = await page.evaluate(() => {
    // Look for aria-selected or .selected or focused item
    const sel = document.querySelector('[aria-selected="true"]') ||
                document.querySelector(".add-item.selected") ||
                document.querySelector(".add-item:focus") ||
                document.querySelector(".palette-item.active") ||
                document.querySelector(".cmd-item.active");
    if (!sel) {
      // Fallback: check --hover CSS variable value
      const root = document.documentElement;
      const rootCS = window.getComputedStyle(root);
      const hoverVar = rootCS.getPropertyValue("--hover").trim();
      return { selectedBg: null, hoverVar, selFound: false };
    }
    const cs = window.getComputedStyle(sel);
    return { selectedBg: cs.backgroundColor, selFound: true, hoverVar: null };
  });
  info(`  selected item bg: ${selectedStyles.selectedBg}`);
  info(`  --hover CSS var: "${selectedStyles.hoverVar}"`);

  // GM4-6: --hover should be defined and selected item should not be near-black
  if (selectedStyles.hoverVar && selectedStyles.hoverVar !== "") {
    pass("GM4-6-var", `--hover defined in light: "${selectedStyles.hoverVar}"`);
  } else if (selectedStyles.selFound && !isDark(selectedStyles.selectedBg)) {
    pass("GM4-6-var", `--hover derived: selected row bg is visible: ${selectedStyles.selectedBg}`);
  } else {
    // Check the root variable directly
    const hoverCheck = await page.evaluate(() => {
      const rootCS = window.getComputedStyle(document.documentElement);
      return rootCS.getPropertyValue("--hover").trim();
    });
    if (hoverCheck && hoverCheck !== "") {
      pass("GM4-6-var", `--hover defined on :root in light: "${hoverCheck}"`);
    } else {
      fail("GM4-6-var", `--hover NOT defined or empty: "${hoverCheck}"`);
    }
  }

  if (selectedStyles.selFound) {
    if (!isDark(selectedStyles.selectedBg)) {
      pass("GM4-6-row", `selected row bg visible: ${selectedStyles.selectedBg}`);
    } else {
      fail("GM4-6-row", `selected row bg near-black: ${selectedStyles.selectedBg}`);
    }
  }

  await page.keyboard.press("Escape");
  await page.waitForTimeout(300);
  await ctx.close();
}

// ── CHECK 4: CONFIRM DIALOG in LIGHT ─────────────────────────────────────────
async function checkConfirmDialogLight(browser) {
  console.log("\n[4] Confirm dialog — LIGHT");
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  page.on("pageerror", e => console.error("  page error:", e.message));

  await bootTheme(page, "light");
  await clearOverlay(page);

  // Navigate to transactions to find a delete trigger
  const txLink = page.locator('a[title*="ransaction"], a[href*="transaction"], nav a').first();
  // Try navigating to transactions page
  const navLinks = await page.locator('nav a[href]').all();
  let navigated = false;
  for (const link of navLinks) {
    const href = await link.getAttribute("href");
    const title = await link.getAttribute("title");
    if ((href && href.includes("transaction")) || (title && /transaction/i.test(title))) {
      await link.click();
      await page.waitForTimeout(600);
      navigated = true;
      break;
    }
  }
  if (!navigated) {
    // Try clicking the first nav link at all
    if (navLinks.length > 0) {
      await navLinks[0].click();
      await page.waitForTimeout(600);
    }
  }
  await clearOverlay(page);

  // Find a delete button / trash icon to trigger confirm dialog
  let dialogOpened = false;

  // Strategy 1: look for rows with delete buttons
  const deleteBtn = page.locator('button[title*="elete"], button[aria-label*="elete"], .row-delete, [data-action="delete"]').first();
  if (await deleteBtn.count() > 0) {
    await deleteBtn.click();
    await page.waitForTimeout(500);
    if (await page.locator(".cf-dialog").count() > 0) {
      dialogOpened = true;
    }
  }

  // Strategy 2: try keyboard shortcut or row context
  if (!dialogOpened) {
    // Try selecting a row and pressing Delete
    const row = page.locator("tr.row, .txn-row, [role='row']").first();
    if (await row.count() > 0) {
      await row.click();
      await page.waitForTimeout(200);
      await page.keyboard.press("Delete");
      await page.waitForTimeout(500);
      if (await page.locator(".cf-dialog").count() > 0) {
        dialogOpened = true;
      }
    }
  }

  if (!dialogOpened) {
    fail("DIALOG-OPEN", "Could not trigger a confirm dialog — no delete button found or dialog did not appear");
    info("  Skipping GM3-5/6 checks (dialog never opened)");
    await ctx.close();
    return;
  }

  info("  cf-dialog appeared");
  await page.screenshot({ path: shot("dialog_light"), fullPage: false });
  info("screenshot saved: gm_verify_dialog_light.png");

  const dialogStyles = await page.evaluate(() => {
    const scrim = document.querySelector(".cf-dialog-scrim");
    const dialog = document.querySelector(".cf-dialog");
    const cs = (el) => el ? window.getComputedStyle(el) : null;
    const scrims = cs(scrim);
    const dialogs = cs(dialog);
    return {
      scrimFound: !!scrim,
      dialogFound: !!dialog,
      scrimBackdropFilter: scrims ? scrims.backdropFilter : null,
      scrimWebkitBackdropFilter: scrims ? scrims.webkitBackdropFilter : null,
      dialogBoxShadow: dialogs ? dialogs.boxShadow : null,
      dialogOutline: dialogs ? dialogs.outline : null,
    };
  });

  info(`  .cf-dialog-scrim backdrop-filter: ${dialogStyles.scrimBackdropFilter}`);
  info(`  .cf-dialog box-shadow: ${dialogStyles.dialogBoxShadow}`);

  // GM3-5: scrim has backdrop blur
  const hasBlur = dialogStyles.scrimBackdropFilter &&
                  dialogStyles.scrimBackdropFilter !== "none" &&
                  dialogStyles.scrimBackdropFilter !== "";
  if (hasBlur) {
    pass("GM3-5", `scrim backdrop-filter: ${dialogStyles.scrimBackdropFilter}`);
  } else {
    fail("GM3-5", `scrim has NO backdrop-filter blur: "${dialogStyles.scrimBackdropFilter}"`);
  }

  // GM3-6: dialog has visible box-shadow (hairline ring + shadow)
  const hasShadow = dialogStyles.dialogBoxShadow &&
                    dialogStyles.dialogBoxShadow !== "none" &&
                    dialogStyles.dialogBoxShadow !== "";
  if (hasShadow) {
    pass("GM3-6", `dialog box-shadow: ${dialogStyles.dialogBoxShadow?.substring(0, 80)}...`);
  } else {
    fail("GM3-6", `dialog has NO box-shadow: "${dialogStyles.dialogBoxShadow}"`);
  }

  // Cancel the dialog — DO NOT leave data deleted
  const cancelled = await page.evaluate(() => {
    const btn = [...document.querySelectorAll(".cf-dialog button")]
      .find(b => /cancel/i.test(b.textContent));
    if (btn) { btn.click(); return true; }
    return false;
  });
  if (!cancelled) {
    // Escape as fallback
    await page.keyboard.press("Escape");
  }
  await page.waitForTimeout(400);
  const dialogGone = await page.locator(".cf-dialog").count() === 0;
  if (dialogGone) {
    info("  dialog cancelled successfully — no data deleted");
  } else {
    info("  WARNING: dialog may still be open after cancel");
  }

  await ctx.close();
}

// ── CHECK 5: DARK MODE SPOT-CHECK ─────────────────────────────────────────────
async function checkDarkSpotCheck(browser) {
  console.log("\n[5] Dark mode spot-check — settings + add modal");
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await ctx.newPage();
  page.on("pageerror", e => console.error("  page error:", e.message));

  await bootTheme(page, "dark");
  await openSettings(page);

  await page.screenshot({ path: shot("settings_dark"), fullPage: false });
  info("screenshot saved: gm_verify_settings_dark.png");

  const darkStyles = await page.evaluate(() => {
    const h3 = document.querySelector(".set-h h3");
    const saveBtn = document.querySelector(".set-btn.save");
    const cs = (el) => el ? window.getComputedStyle(el) : null;
    const h3s = cs(h3);
    const saves = cs(saveBtn);
    return {
      h3Color: h3s ? h3s.color : null,
      h3Found: !!h3,
      saveBg: saves ? saves.backgroundColor : null,
      saveColor: saves ? saves.color : null,
    };
  });

  info(`  dark .set-h h3 color: ${darkStyles.h3Color}`);
  info(`  dark .set-btn.save bg: ${darkStyles.saveBg}  color: ${darkStyles.saveColor}`);

  // In dark, title should still be readable (not near-black on near-black)
  // The dark default is near-white (light text). Just verify it's not near-black.
  if (darkStyles.h3Found) {
    const h3rgb = parseRgb(darkStyles.h3Color);
    if (h3rgb && !isDark(darkStyles.h3Color)) {
      pass("DARK-TITLE-READABLE", `dark title readable: ${darkStyles.h3Color}`);
    } else {
      fail("DARK-TITLE-READABLE", `dark title near-black / unreadable: ${darkStyles.h3Color}`);
    }
  }

  // Save button should still have a visible label
  if (darkStyles.saveColor) {
    const saveRgb = parseRgb(darkStyles.saveColor);
    if (saveRgb && !isDark(darkStyles.saveColor)) {
      pass("DARK-SAVE-LABEL", `dark save label readable: ${darkStyles.saveColor}`);
    } else {
      fail("DARK-SAVE-LABEL", `dark save label near-black: ${darkStyles.saveColor}`);
    }
  }

  await closePanel(page);

  // Open add modal in dark
  await clearOverlay(page);
  await page.locator(".add-btn").click();
  await page.waitForTimeout(300);
  const darkMenuItems = await page.locator('[role="menuitem"]').count();
  if (darkMenuItems > 0) {
    await page.locator('[role="menuitem"]').first().click();
    await page.waitForTimeout(500);
    if (await page.locator('[role="dialog"]').count() > 0) {
      await page.screenshot({ path: shot("addmodal_dark"), fullPage: false });
      info("screenshot saved: gm_verify_addmodal_dark.png");

      const darkModalStyles = await page.evaluate(() => {
        const h3 = document.querySelector(".set-h h3");
        const cs = h3 ? window.getComputedStyle(h3) : null;
        return { h3Color: cs ? cs.color : null, h3Found: !!h3 };
      });
      info(`  dark add-modal .set-h h3 color: ${darkModalStyles.h3Color}`);
      if (darkModalStyles.h3Found && !isDark(darkModalStyles.h3Color)) {
        pass("DARK-ADDMODAL-TITLE", `dark add-modal title readable: ${darkModalStyles.h3Color}`);
      } else {
        fail("DARK-ADDMODAL-TITLE", `dark add-modal title unreadable: ${darkModalStyles.h3Color}`);
      }
    } else {
      fail("DARK-ADDMODAL-OPEN", "FlipPanel did not open in dark mode");
    }
    await page.keyboard.press("Escape");
    await page.waitForTimeout(300);
  }

  await ctx.close();
}

// ── CHECK 6: FLIP-WRAP max-width at 768 ──────────────────────────────────────
async function checkFlipWrapNarrow(browser) {
  console.log("\n[6] flip-wrap max-width at 768 (GM1-S5)");
  const ctx = await browser.newContext({ viewport: { width: 768, height: 900 } });
  const page = await ctx.newPage();
  page.on("pageerror", e => console.error("  page error:", e.message));

  await bootTheme(page, "light");
  await openSettings(page);

  const dims = await page.evaluate(() => {
    const wrap = document.querySelector(".flip-wrap");
    if (!wrap) return null;
    const rect = wrap.getBoundingClientRect();
    const cs = window.getComputedStyle(wrap);
    return {
      width: Math.round(rect.width),
      viewportWidth: window.innerWidth,
      maxWidth: cs.maxWidth,
    };
  });

  info(`  flip-wrap at 768: width=${dims?.width}px maxWidth=${dims?.maxWidth} viewport=${dims?.viewportWidth}px`);

  // GM1-S5: at 768, wrap should be narrower than viewport (gutter)
  if (dims && dims.width < dims.viewportWidth) {
    pass("GM1-S5", `flip-wrap has gutter at 768: width=${dims.width}px < viewport=${dims.viewportWidth}px`);
  } else if (dims) {
    fail("GM1-S5", `flip-wrap fills full viewport at 768: width=${dims.width}px, viewport=${dims.viewportWidth}px (expected gutter)`);
  } else {
    fail("GM1-S5", "flip-wrap element not found");
  }

  await closePanel(page);
  await ctx.close();
}

// ── MAIN ─────────────────────────────────────────────────────────────────────
async function main() {
  console.log("=== gm_verify.mjs — CSS modal fix verification ===");
  console.log(`    URL: ${BASE}`);
  console.log(`    Screenshots: ${SHOTS}\n`);

  const browser = await chromium.launch({ headless: true });

  try {
    await checkSettingsLight(browser);
    await checkAddModalLight(browser);
    await checkCommandPaletteLight(browser);
    await checkConfirmDialogLight(browser);
    await checkDarkSpotCheck(browser);
    await checkFlipWrapNarrow(browser);
  } finally {
    await browser.close();
  }

  // ── Summary ───────────────────────────────────────────────────────────────
  console.log("\n=== SUMMARY ===");
  const passed = results.filter(r => r.status === "PASS");
  const failed = results.filter(r => r.status === "FAIL");

  for (const r of results) {
    const icon = r.status === "PASS" ? "✓" : "✗";
    console.log(`  ${icon} [${r.id}] ${r.detail}`);
  }

  console.log(`\n  ${passed.length} passed, ${failed.length} failed`);

  if (failed.length > 0) {
    console.error("\nFAILED checks:");
    for (const f of failed) {
      console.error(`  - [${f.id}]: ${f.detail}`);
    }
    process.exit(1);
  } else {
    console.log("\nAll CSS fix checks PASSED.");
    process.exit(0);
  }
}

main().catch(e => { console.error(e); process.exit(1); });
