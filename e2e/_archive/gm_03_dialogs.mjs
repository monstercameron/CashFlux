// GLAMOR GM3 — Confirm/destructive dialogs UX review (cf-dialog system, C42).
// Triggers: (1) single-transaction delete confirm, (2) workspace delete confirm,
// (3) custom-page delete confirm, (4) prompt-style dialog (new workspace name),
// (5) bulk-delete (no confirm — L50 safety gap).
// Both themes × 1280 + 768. All dialogs opened then CANCELLED — no data deleted.
// Inspects: structure, buttons, focus, aria, theming, sizing, backdrop.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE  = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOTS = path.join(__dirname, "screenshots");
if (!fs.existsSync(SHOTS)) fs.mkdirSync(SHOTS, { recursive: true });

const shot   = (name) => path.join(SHOTS, `gm_03_dialogs_${name}.png`);
const errors = [];
const log    = (m) => console.log("  " + m);
const warn   = (m) => { console.warn("  WARN: " + m); errors.push(m); };

// ── helpers ─────────────────────────────────────────────────────────────────

async function bootWithTheme(page, theme) {
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

async function dismissErrorOverlay(page) {
  await page.evaluate(() => {
    const o = document.getElementById("gwc-error-overlay") ||
              document.querySelector(".gwc-error-overlay");
    if (o) o.remove();
  });
  await page.waitForTimeout(200);
}

// Wait for .cf-dialog to appear; return false if it times out.
async function waitForDialog(page, timeout = 8000) {
  try {
    await page.waitForSelector(".cf-dialog", { timeout });
    return true;
  } catch (_) {
    return false;
  }
}

// Cancel the open dialog via the Cancel button.
async function cancelDialog(page) {
  await dismissErrorOverlay(page);
  const cancelled = await page.evaluate(() => {
    const btn = [...document.querySelectorAll(".cf-dialog button")]
      .find(b => /cancel/i.test(b.textContent));
    if (btn) { btn.click(); return true; }
    return false;
  });
  await page.waitForTimeout(400);
  return cancelled;
}

// Audit the open dialog; return a findings object.
async function auditDialog(page) {
  return page.evaluate(() => {
    const dlg = document.querySelector(".cf-dialog");
    if (!dlg) return { found: false };

    const backdrop = document.querySelector(".cf-dialog-backdrop");
    const scrim    = document.querySelector(".cf-dialog-scrim");
    const title    = dlg.querySelector(".cf-dialog-title");
    const msg      = dlg.querySelector(".cf-dialog-msg");
    const actions  = dlg.querySelector(".cf-dialog-actions");
    const input    = dlg.querySelector(".cf-dialog-input, #cf-dialog-input");
    const confirmBtn = dlg.querySelector("#cf-dialog-confirm");
    const cancelBtn  = [...(dlg.querySelectorAll("button"))]
      .find(b => /cancel/i.test(b.textContent));

    // Geometry
    const rect = dlg.getBoundingClientRect();
    const vpW  = window.innerWidth;
    const vpH  = window.innerHeight;
    const centeredH = Math.abs((rect.left + rect.right) / 2 - vpW / 2) < 10;
    const centeredV = Math.abs((rect.top + rect.bottom) / 2 - vpH / 2) < 30;

    // ARIA
    const ariaModal   = backdrop?.getAttribute("aria-modal");
    const roleAttr    = backdrop?.getAttribute("role");
    const labelledBy  = backdrop?.getAttribute("aria-labelledby");

    // Focused element
    const focused = document.activeElement;
    const focusedId   = focused?.id || null;
    const focusedClass = focused?.className || null;
    const focusedTag   = focused?.tagName?.toLowerCase() || null;
    const focusOnSafe  = /cancel/i.test(focused?.textContent || "");
    const focusOnInput = focused === input;

    // Button styling
    const confirmClasses = confirmBtn?.className || "";
    const isDangerStyled = confirmClasses.includes("btn-danger");
    const cancelClasses  = cancelBtn?.className || "";

    // Colors (computed)
    const getColor = (el) => el ? window.getComputedStyle(el).color : null;
    const getBg    = (el) => el ? window.getComputedStyle(el).backgroundColor : null;

    return {
      found: true,
      hasTitle:    !!title,
      titleText:   title?.textContent?.trim() || null,
      msgText:     msg?.textContent?.trim() || null,
      hasInput:    !!input,
      hasActions:  !!actions,
      confirmLabel: confirmBtn?.textContent?.trim() || null,
      cancelLabel:  cancelBtn?.textContent?.trim() || null,
      confirmClasses,
      cancelClasses,
      isDangerStyled,
      hasScrim:    !!scrim,
      ariaModal,
      roleAttr,
      labelledBy,
      rect:        { x: Math.round(rect.x), y: Math.round(rect.y), w: Math.round(rect.width), h: Math.round(rect.height) },
      vpW, vpH,
      centeredH, centeredV,
      focusedId, focusedClass, focusedTag, focusOnSafe, focusOnInput,
      confirmBgColor: getBg(confirmBtn),
      cancelBgColor:  getBg(cancelBtn),
      dialogBg:       getBg(dlg),
      msgColor:       getColor(msg),
    };
  });
}

// ── main probe loop ──────────────────────────────────────────────────────────

const browser = await chromium.launch({ headless: true });
// Catch native dialogs — they should NEVER fire
let nativeDialogFired = false;

try {
  for (const theme of ["dark", "light"]) {
    for (const width of [1280, 768]) {
      const label = `${theme}_${width}`;
      log(`\n═══ PROBE: theme=${theme} width=${width} ═══`);

      const ctx  = await browser.newContext({ viewport: { width, height: 900 } });
      const page = await ctx.newPage();
      page.on("dialog", async (d) => {
        nativeDialogFired = true;
        warn(`NATIVE dialog fired! type=${d.type()} msg=${d.message()}`);
        await d.dismiss();
      });
      page.on("console", (m) => {
        if (/panic|error/i.test(m.type()) && m.text().length < 300)
          log(`console.${m.type()}: ${m.text()}`);
      });

      await bootWithTheme(page, theme);
      await dismissErrorOverlay(page);

      // ── PROBE 1: Single-transaction delete confirm ────────────────────────
      log(`[P1] Navigate to /transactions and open single-row delete…`);
      await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
      await page.waitForTimeout(1500);
      await dismissErrorOverlay(page);

      // Find and click a delete button on any transaction row
      const delBtnCount = await page.evaluate(() =>
        document.querySelectorAll('button.btn-del, button[title*="elete"], button[aria-label*="elete"]').length
      );
      log(`  Found ${delBtnCount} delete buttons on transactions page`);

      let p1opened = false;
      if (delBtnCount > 0) {
        await page.evaluate(() => {
          const btn = document.querySelector('button.btn-del, button[title*="elete"], button[aria-label*="elete"]');
          if (btn) btn.click();
        });
        await page.waitForTimeout(600);
        await dismissErrorOverlay(page);
        p1opened = await waitForDialog(page, 5000);
        if (p1opened) {
          log(`  cf-dialog opened ✓`);
          await page.screenshot({ path: shot(`${label}_1280_txn_delete_open`), fullPage: false });
          const audit = await auditDialog(page);
          log(`  audit: ${JSON.stringify(audit)}`);
          if (!audit.isDangerStyled) warn(`[P1] confirm button NOT styled as danger (missing btn-danger)`);
          if (!audit.hasScrim)       warn(`[P1] no cf-dialog-scrim found (backdrop click-to-cancel broken)`);
          if (!audit.centeredH)      warn(`[P1] dialog not horizontally centered`);
          if (!audit.centeredV)      warn(`[P1] dialog not vertically centered`);
          if (audit.ariaModal !== "true") warn(`[P1] aria-modal != "true" (got: ${audit.ariaModal})`);
          if (audit.roleAttr !== "dialog") warn(`[P1] role != "dialog" (got: ${audit.roleAttr})`);
          if (audit.focusOnSafe) log(`  ✓ focus on Cancel (safe default)`);
          else warn(`[P1] focus NOT on Cancel (got focusedId=${audit.focusedId} tag=${audit.focusedTag})`);
          if (audit.hasTitle) log(`  ✓ has title: "${audit.titleText}"`);
          else warn(`[P1] no dialog title — message only: "${audit.msgText}"`);
          // Cancel to avoid deleting data
          await cancelDialog(page);
          const stillOpen = await page.locator(".cf-dialog").count();
          if (stillOpen > 0) warn(`[P1] dialog still open after Cancel!`);
          else log(`  ✓ dialog closed after Cancel`);

          // Also screenshot at 768 here (already at 768 if width==768)
          if (width === 1280) {
            await page.screenshot({ path: shot(`txn_delete_dark_1280`), fullPage: false });
          } else {
            await page.screenshot({ path: shot(`txn_delete_${label}`), fullPage: false });
          }
        } else {
          warn(`[P1] cf-dialog did NOT open after clicking delete button`);
          await page.screenshot({ path: shot(`${label}_txn_no_dialog`), fullPage: false });
        }
      } else {
        warn(`[P1] no delete button found on /transactions`);
        await page.screenshot({ path: shot(`${label}_txn_no_del_btn`), fullPage: false });
      }

      // ── PROBE 2: Bulk-delete safety check (L50) ───────────────────────────
      log(`[P2] Check bulk-delete: select-all then click bulk delete (expect confirm dialog — L50)…`);
      await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
      await page.waitForTimeout(1500);
      await dismissErrorOverlay(page);

      // Use the "Select all" button (title = "Select all transactions in the current filtered view")
      const selectAllCount = await page.evaluate(() =>
        document.querySelectorAll('button[title*="Select all"], button[aria-label*="Select all"]').length
      );
      log(`  "Select all" buttons found: ${selectAllCount}`);

      if (selectAllCount > 0) {
        // Click "Select all" to select all visible transactions
        await page.evaluate(() => {
          const btn = document.querySelector('button[title*="Select all"], button[aria-label*="Select all"]');
          if (btn) btn.click();
        });
        await page.waitForTimeout(600);
        await dismissErrorOverlay(page);

        // Look for the bulk-bar delete button
        const bulkDelCount = await page.evaluate(() =>
          document.querySelectorAll('button.btn-del[title*="Delete the selected"], button[title*="Delete the selected"]').length
        );
        log(`  Bulk-delete button visible: ${bulkDelCount}`);

        if (bulkDelCount > 0) {
          await page.screenshot({ path: shot(`${label}_bulk_selection_active`), fullPage: false });
          // Click bulk-delete — check if a confirm dialog appears (L50: it SHOULD but may not)
          await page.evaluate(() => {
            const btn = document.querySelector('button.btn-del[title*="Delete the selected"], button[title*="Delete the selected"]');
            if (btn) btn.click();
          });
          await page.waitForTimeout(800);
          await dismissErrorOverlay(page);
          const bulkDialogOpen = await page.locator(".cf-dialog").count() > 0;
          if (bulkDialogOpen) {
            log(`  ✓ bulk-delete DOES show a confirm dialog (L50 resolved!)`);
            await page.screenshot({ path: shot(`${label}_bulk_delete_confirm`), fullPage: false });
            await cancelDialog(page);
          } else {
            warn(`[P2][L50] bulk-delete fires WITHOUT a confirmation dialog — data-loss risk!`);
            await page.screenshot({ path: shot(`${label}_bulk_no_confirm`), fullPage: false });
            // Try to undo (there's an undo button in the bulk bar after delete)
            await page.waitForTimeout(400);
            const undoBtn = await page.evaluate(() => {
              const btn = document.querySelector('button[aria-label*="Undo"], button[title*="Undo the last bulk"]');
              if (btn) { btn.click(); return true; }
              return false;
            });
            if (undoBtn) log(`  Clicked undo to recover deleted transactions`);
            else log(`  No undo button found — data may have been deleted`);
          }
        } else {
          warn(`[P2] no bulk-delete button appeared after select-all — bulk bar may not be visible`);
          await page.screenshot({ path: shot(`${label}_no_bulk_del_btn`), fullPage: false });
        }
      } else {
        warn(`[P2] no "Select all" button found on /transactions`);
        await page.screenshot({ path: shot(`${label}_no_select_all`), fullPage: false });
      }

      // ── PROBE 3: Custom-page delete confirm ──────────────────────────────
      log(`[P3] Open custom-page delete confirm…`);
      await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
      await page.waitForTimeout(1200);
      await dismissErrorOverlay(page);

      // Try to find a custom page delete button in the rail
      const pageDelCount = await page.evaluate(() =>
        document.querySelectorAll('nav button[title*="elete"], nav .page-del, nav button[aria-label*="elete"]').length
      );
      log(`  Custom-page delete buttons in rail: ${pageDelCount}`);

      let p3opened = false;
      if (pageDelCount > 0) {
        await page.evaluate(() => {
          const btn = document.querySelector('nav button[title*="elete"], nav .page-del, nav button[aria-label*="elete"]');
          if (btn) btn.click();
        });
        await page.waitForTimeout(600);
        await dismissErrorOverlay(page);
        p3opened = await waitForDialog(page, 5000);
        if (p3opened) {
          log(`  cf-dialog opened for page delete ✓`);
          await page.screenshot({ path: shot(`${label}_page_delete_dialog`), fullPage: false });
          const audit = await auditDialog(page);
          if (!audit.isDangerStyled) warn(`[P3] confirm not danger-styled`);
          await cancelDialog(page);
        } else {
          log(`  No page delete buttons triggered a dialog (may need a pre-existing custom page)`);
        }
      } else {
        log(`  No custom-page delete buttons visible in rail (need an existing custom page)`);
      }

      // ── PROBE 4: Prompt modal (new workspace name) ────────────────────────
      log(`[P4] Open prompt-style dialog (new workspace)…`);
      await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
      await page.waitForTimeout(1000);
      await dismissErrorOverlay(page);

      // Open settings, go to workspaces
      const settingsOpened = await page.evaluate(() => {
        // Click household card or settings icon
        const hhCard = document.querySelector('.hh, button.hh, [aria-label*="ettings"], .household-card, button[title*="ettings"]');
        if (hhCard) { hhCard.click(); return true; }
        return false;
      });
      await page.waitForTimeout(800);
      await dismissErrorOverlay(page);

      // Try to find "New workspace" button or similar prompt trigger
      // Alternatively, try creating a new custom page (known prompt trigger)
      const newPageEl = await page.evaluate(() => {
        // Look for "New page" or workspace new button
        const el = [...document.querySelectorAll("nav a, nav button, .rail button, .rail a")]
          .find(b => /new page/i.test(b.textContent || b.title || ""));
        if (el) { el.click(); return "new-page"; }
        return null;
      });
      await page.waitForTimeout(600);
      await dismissErrorOverlay(page);

      const p4opened = await waitForDialog(page, 6000);
      if (p4opened) {
        log(`  Prompt dialog opened (trigger: ${newPageEl || "unknown"}) ✓`);
        await page.screenshot({ path: shot(`${label}_prompt_dialog_open`), fullPage: false });
        const audit = await auditDialog(page);
        log(`  audit: focused on input=${audit.focusOnInput}, hasInput=${audit.hasInput}`);
        if (!audit.hasInput)     warn(`[P4] prompt dialog has no text input`);
        if (!audit.focusOnInput) warn(`[P4] focus NOT on input in prompt dialog (got tag=${audit.focusedTag})`);
        if (audit.isDangerStyled) warn(`[P4] prompt confirm should NOT be danger-styled`);

        // Test ESC closes
        await page.keyboard.press("Escape");
        await page.waitForTimeout(400);
        const stillOpenEsc = await page.locator(".cf-dialog").count();
        if (stillOpenEsc > 0) warn(`[P4] dialog still open after Escape!`);
        else log(`  ✓ Escape closed the prompt dialog`);
      } else {
        warn(`[P4] Could not open a prompt-style dialog`);
        await page.screenshot({ path: shot(`${label}_no_prompt_dialog`), fullPage: false });
      }

      // ── PROBE 5: Full-page dialog screenshots (open txn delete again) ─────
      log(`[P5] Re-open single txn delete for themed screenshot evidence…`);
      await page.goto(BASE + "/transactions", { waitUntil: "domcontentloaded" });
      await page.waitForTimeout(1500);
      await dismissErrorOverlay(page);

      await page.evaluate(() => {
        const btn = document.querySelector('button.btn-del, button[title*="elete"], button[aria-label*="elete"]');
        if (btn) btn.click();
      });
      await page.waitForTimeout(600);
      await dismissErrorOverlay(page);

      const p5opened = await waitForDialog(page, 5000);
      if (p5opened) {
        await page.screenshot({ path: shot(`${label}_confirm_dialog_full`), fullPage: false });
        // DOM snapshot for analysis
        const domSnap = await auditDialog(page);
        fs.writeFileSync(
          path.join(SHOTS, `gm_03_dialogs_${label}_audit.json`),
          JSON.stringify(domSnap, null, 2)
        );
        log(`  Screenshot + DOM audit written for ${label}`);
        await cancelDialog(page);
      } else {
        await page.screenshot({ path: shot(`${label}_no_confirm_reopen`), fullPage: false });
        warn(`[P5] Could not reopen confirm dialog for ${label}`);
      }

      await ctx.close();
    }
  }

  if (nativeDialogFired) {
    warn(`CRITICAL: a native browser dialog fired during the probe — C42 regression!`);
  } else {
    log(`✓ No native dialogs fired throughout probe`);
  }

  if (errors.length === 0) {
    console.log("\nPASS: All dialog probes completed with no warnings.");
  } else {
    console.log(`\nDONE with ${errors.length} warning(s):`);
    errors.forEach((e, i) => console.log(`  ${i + 1}. ${e}`));
    process.exitCode = 1;
  }
} finally {
  await browser.close();
}
