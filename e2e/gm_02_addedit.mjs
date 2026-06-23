// GLAMOR GM2 — Add/Edit entity modals UX review.
// Opens the global "+Add" menu and exercises:
//   - Add transaction (QuickAdd panel)
//   - Add account    (FlipPanel modal via AddHost)
//   - Add budget     (FlipPanel modal)
//   - Add goal       (FlipPanel modal)
//   - Inline edit    (transaction row .row-edit form)
// Screenshots at 1280 + 768 in BOTH themes.
// Writes into e2e/screenshots/gm_02_addedit_*.png and gm_02_addedit_dom.json.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require   = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE  = process.env.E2E_URL || "http://127.0.0.1:8099";
const SHOTS = path.join(__dirname, "screenshots");
if (!fs.existsSync(SHOTS)) fs.mkdirSync(SHOTS, { recursive: true });

const shot  = (name) => path.join(SHOTS, `gm_02_addedit_${name}.png`);
const browser = await chromium.launch({ headless: true });
const errors  = [];
const warn    = (m) => { errors.push("WARN: " + m); console.warn("WARN: " + m); };

// ── Helpers ───────────────────────────────────────────────────────────────────

async function boot(page, theme) {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
  await page.waitForTimeout(800);

  // Reset member filter
  await page.evaluate(() => {
    const raw = localStorage.getItem("cashflux:prefs");
    if (raw) {
      try {
        const p = JSON.parse(raw);
        delete p.viewAsMember;
        localStorage.setItem("cashflux:prefs", JSON.stringify(p));
      } catch (_) {}
    }
  });

  if (theme === "light") {
    await page.evaluate(() => {
      const raw = localStorage.getItem("cashflux:prefs");
      const p = raw ? JSON.parse(raw) : {};
      p.theme = "light";
      localStorage.setItem("cashflux:prefs", JSON.stringify(p));
    });
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
    await page.waitForTimeout(800);
    // Toggle via Settings panel to make it stick against WASM prefs overwrite
    const isDark = await page.evaluate(() => document.documentElement.getAttribute("data-theme") !== "light");
    if (isDark) {
      // Try toggling via settings
      const settingsBtn = page.locator('button[title="Settings"], button[aria-label="Settings"]').first();
      if (await settingsBtn.count() > 0) {
        await settingsBtn.click();
        await page.waitForTimeout(400);
        const themeToggle = page.locator('button, label').filter({ hasText: /light|dark|theme/i }).first();
        if (await themeToggle.count() > 0) {
          await themeToggle.click();
          await page.waitForTimeout(400);
        }
        await page.keyboard.press("Escape");
        await page.waitForTimeout(300);
      }
    }
  } else {
    // Ensure dark (default)
    await page.evaluate(() => {
      const raw = localStorage.getItem("cashflux:prefs");
      const p = raw ? JSON.parse(raw) : {};
      p.theme = "dark";
      localStorage.setItem("cashflux:prefs", JSON.stringify(p));
    });
    await page.reload({ waitUntil: "domcontentloaded" });
    await page.waitForSelector('nav[aria-label="Main navigation"] a[title]', { timeout: 60000 });
    await page.waitForTimeout(600);
  }
}

async function closeAddMenu(page) {
  // Close the add menu if it's open — the backdrop intercepts clicks (OnClick(closeMenu))
  // so we click it directly. Escape is not wired to the add menu, only to FlipPanel.
  const backdrop = page.locator('.add-backdrop');
  if (await backdrop.count() > 0) {
    const cls = await backdrop.getAttribute("class").catch(() => "");
    if (!cls || !cls.includes("hidden-menu")) {
      await backdrop.click({ force: true });
      await page.waitForTimeout(200);
    }
  }
}

async function openAddMenu(page) {
  // Close any open menu first to avoid backdrop-intercept
  await closeAddMenu(page);
  // The add button has aria-label from addmenu.go — "topbar.add" → "Add something new"
  const btn = page.locator('button[aria-label="Add something new"], button[title="Add something new"], .add-btn').first();
  if (await btn.count() === 0) {
    warn("Add button not found — checking all buttons for + icon");
    return false;
  }
  await btn.click();
  await page.waitForTimeout(300);
  return true;
}

async function closeFlipPanel(page) {
  const closeBtn = page.locator('.set-close, button[title="Close"]').first();
  if (await closeBtn.count() > 0) {
    await closeBtn.click();
  } else {
    await page.keyboard.press("Escape");
  }
  await page.waitForTimeout(400);
}

async function auditAddModal(page, entityName) {
  return page.evaluate((entity) => {
    const backdrop = document.querySelector(".flip-backdrop");
    const wrap     = document.querySelector(".flip-wrap");
    const back     = document.querySelector(".flip-back");
    const setH     = document.querySelector(".set-h");
    const setBody  = document.querySelector(".set-body");
    const setFoot  = document.querySelector(".set-foot");

    // Labels vs placeholders audit
    const labels   = [...(back || document).querySelectorAll("label")].map(l => l.textContent.trim()).filter(Boolean);
    const inputs   = [...(back || document).querySelectorAll("input, select, textarea")];
    const placeholderOnly = inputs.filter(i => {
      const hasLabel = i.id && document.querySelector(`label[for="${i.id}"]`);
      const hasAriaLabel = i.getAttribute("aria-label");
      const hasWrappingLabel = i.closest("label");
      const hasLabeledField = i.closest(".labeled-field");
      return !hasLabel && !hasAriaLabel && !hasWrappingLabel && !hasLabeledField;
    }).map(i => ({ tag: i.tagName, type: i.type, placeholder: i.placeholder, id: i.id }));

    const inputTypes = inputs.map(i => ({ tag: i.tagName, type: i.type, placeholder: i.placeholder, id: i.id }));

    // Buttons in footer
    const footBtns = setFoot ? [...setFoot.querySelectorAll("button")].map(b => b.textContent.trim()) : [];

    // Primary submit in body
    const submitBtns = back ? [...back.querySelectorAll('button[type="submit"], .btn-primary')].map(b => ({
      text: b.textContent.trim(), cls: b.className
    })) : [];

    // Sizing
    const wrapRect = wrap ? wrap.getBoundingClientRect() : null;
    const bodyRect = setBody ? setBody.getBoundingClientRect() : null;

    // Colors (theming)
    const bodyBg  = setBody ? getComputedStyle(setBody).backgroundColor : "N/A";
    const faceBg  = back    ? getComputedStyle(back).backgroundColor    : "N/A";
    const titleEl = setH    ? setH.querySelector("h3")                  : null;
    const titleColor = titleEl ? getComputedStyle(titleEl).color : "N/A";

    // form-grid present?
    const hasFormGrid = !!(back && back.querySelector(".form-grid"));
    // labeled-field count
    const labeledFieldCount = back ? back.querySelectorAll(".labeled-field").length : 0;

    // role=dialog?
    const hasDialogRole = !!(wrap && wrap.getAttribute("role") === "dialog");
    const hasAriaModal  = !!(wrap && wrap.getAttribute("aria-modal") === "true");
    const hasAriaLabel  = !!(wrap && wrap.getAttribute("aria-label"));

    // backdrop classes
    const backdropCls = backdrop ? backdrop.className : "N/A";
    const backdropBg  = backdrop ? getComputedStyle(backdrop).backgroundColor : "N/A";

    // field input styling
    const firstInput = back ? back.querySelector("input.field, select.field") : null;
    const inputBg    = firstInput ? getComputedStyle(firstInput).backgroundColor : "N/A";
    const inputBorder= firstInput ? getComputedStyle(firstInput).borderColor : "N/A";
    const inputColor = firstInput ? getComputedStyle(firstInput).color : "N/A";

    // error text
    const errEls = back ? [...back.querySelectorAll(".err-text, [role=alert]")].map(e => e.textContent.trim()) : [];

    return {
      entity,
      hasBackdrop: !!backdrop,
      backdropCls,
      backdropBg,
      hasWrap: !!wrap,
      hasBack: !!back,
      wrapW: wrapRect ? Math.round(wrapRect.width) : null,
      wrapH: wrapRect ? Math.round(wrapRect.height) : null,
      bodyH: bodyRect ? Math.round(bodyRect.height) : null,
      faceBg,
      bodyBg,
      titleColor,
      hasDialogRole,
      hasAriaModal,
      hasAriaLabel,
      hasFormGrid,
      labeledFieldCount,
      labels,
      inputTypes,
      placeholderOnly,
      footBtns,
      submitBtns,
      inputBg,
      inputBorder,
      inputColor,
      errEls,
    };
  }, entityName);
}

// ── Run at both viewports × both themes ──────────────────────────────────────

const results = {};

for (const theme of ["dark", "light"]) {
  for (const vw of [1280, 768]) {
    const ctx  = await browser.newContext({ viewport: { width: vw, height: 900 } });
    const page = await ctx.newPage();
    page.on("console", m => { if (/panic/i.test(m.text())) warn(`[${theme}/${vw}] console panic: ${m.text()}`); });

    console.log(`\n── ${theme.toUpperCase()} @ ${vw}px ────────────────────────────────`);
    await boot(page, theme);

    const actualTheme = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
    console.log(`  data-theme confirmed: ${actualTheme}`);

    // ── 1. BASELINE: Add menu closed ──────────────────────────────────────────
    await page.screenshot({ path: shot(`${theme}_${vw}_baseline`), fullPage: false });

    // ── 2. Add menu open ──────────────────────────────────────────────────────
    const opened = await openAddMenu(page);
    if (!opened) {
      warn(`[${theme}/${vw}] Could not open Add menu`);
    } else {
      await page.screenshot({ path: shot(`${theme}_${vw}_addmenu_open`), fullPage: false });

      // Capture add-menu DOM
      const menuAudit = await page.evaluate(() => {
        const menu    = document.querySelector(".add-menu");
        const items   = menu ? [...menu.querySelectorAll("[role=menuitem]")].map(i => i.textContent.trim()) : [];
        const menuBg  = menu ? getComputedStyle(menu).backgroundColor : "N/A";
        const itemColor = items.length > 0
          ? getComputedStyle(document.querySelector("[role=menuitem]")).color
          : "N/A";
        const menuVisible = !!(menu && !menu.classList.contains("hidden-menu"));
        const addBtn  = document.querySelector(".add-btn");
        const addBtnBg = addBtn ? getComputedStyle(addBtn).backgroundColor : "N/A";
        return { items, menuBg, itemColor, menuVisible, addBtnBg };
      });
      console.log(`  Add menu items (${menuAudit.items.length}):`, menuAudit.items.join(", "));
      console.log(`  Menu bg: ${menuAudit.menuBg} | Item color: ${menuAudit.itemColor}`);
      if (!results[`${theme}_${vw}`]) results[`${theme}_${vw}`] = {};
      results[`${theme}_${vw}`].menu = menuAudit;

      // Close menu via backdrop click (Escape is not wired to add-menu)
      await closeAddMenu(page);
    }

    // ── 3. Add Account modal ──────────────────────────────────────────────────
    await openAddMenu(page);
    const accMenuItem = page.locator('[role="menuitem"]').filter({ hasText: /account/i }).first();
    if (await accMenuItem.count() > 0) {
      await accMenuItem.click();
      await page.waitForTimeout(600);
      // Wait for flip-back to appear
      await page.waitForSelector(".flip-back, .flip-backdrop", { timeout: 5000 }).catch(() => warn(`[${theme}/${vw}] Account modal did not open`));
      await page.waitForTimeout(400);

      await page.screenshot({ path: shot(`${theme}_${vw}_add_account`), fullPage: false });
      const acctAudit = await auditAddModal(page, "account");
      results[`${theme}_${vw}`].account = acctAudit;
      console.log(`  Account modal: wrap=${acctAudit.wrapW}×${acctAudit.wrapH}, labeled=${acctAudit.labeledFieldCount}, placeholderOnly=${acctAudit.placeholderOnly.length}, footBtns=[${acctAudit.footBtns}], role=${acctAudit.hasDialogRole}`);

      await closeFlipPanel(page);
    } else {
      warn(`[${theme}/${vw}] "New account" menu item not found`);
    }

    // ── 4. Add Budget modal ───────────────────────────────────────────────────
    await openAddMenu(page);
    const budgetItem = page.locator('[role="menuitem"]').filter({ hasText: /budget/i }).first();
    if (await budgetItem.count() > 0) {
      await budgetItem.click();
      await page.waitForTimeout(600);
      await page.waitForSelector(".flip-back, .flip-backdrop", { timeout: 5000 }).catch(() => warn(`[${theme}/${vw}] Budget modal did not open`));
      await page.waitForTimeout(400);

      await page.screenshot({ path: shot(`${theme}_${vw}_add_budget`), fullPage: false });
      const budgetAudit = await auditAddModal(page, "budget");
      results[`${theme}_${vw}`].budget = budgetAudit;
      console.log(`  Budget modal: wrap=${budgetAudit.wrapW}×${budgetAudit.wrapH}, labeled=${budgetAudit.labeledFieldCount}, placeholderOnly=${budgetAudit.placeholderOnly.length}, footBtns=[${budgetAudit.footBtns}]`);

      await closeFlipPanel(page);
    } else {
      warn(`[${theme}/${vw}] "New budget" menu item not found`);
    }

    // ── 5. Add Goal modal ─────────────────────────────────────────────────────
    await openAddMenu(page);
    const goalItem = page.locator('[role="menuitem"]').filter({ hasText: /goal/i }).first();
    if (await goalItem.count() > 0) {
      await goalItem.click();
      await page.waitForTimeout(600);
      await page.waitForSelector(".flip-back, .flip-backdrop", { timeout: 5000 }).catch(() => warn(`[${theme}/${vw}] Goal modal did not open`));
      await page.waitForTimeout(400);

      await page.screenshot({ path: shot(`${theme}_${vw}_add_goal`), fullPage: false });
      const goalAudit = await auditAddModal(page, "goal");
      results[`${theme}_${vw}`].goal = goalAudit;
      console.log(`  Goal modal: wrap=${goalAudit.wrapW}×${goalAudit.wrapH}, labeled=${goalAudit.labeledFieldCount}, placeholderOnly=${goalAudit.placeholderOnly.length}, submitBtns=[${goalAudit.submitBtns.map(b=>b.text).join(",")}]`);

      await closeFlipPanel(page);
    } else {
      warn(`[${theme}/${vw}] "New goal" menu item not found`);
    }

    // ── 6. Add Transaction (QuickAdd panel) ───────────────────────────────────
    await openAddMenu(page);
    const txnItem = page.locator('[role="menuitem"]').filter({ hasText: /transaction/i }).first();
    if (await txnItem.count() > 0) {
      await txnItem.click();
      await page.waitForTimeout(600);
      // QuickAdd might use flip-panel or an inline panel
      await page.screenshot({ path: shot(`${theme}_${vw}_add_transaction`), fullPage: false });

      const qaAudit = await page.evaluate(() => {
        // QuickAdd is a different panel type — look for the quick-add wrapper
        const qa      = document.querySelector(".quick-add, .qa-panel, [data-testid='quick-add'], .flip-backdrop");
        const qaForm  = document.querySelector(".qa-form, form[data-testid], .flip-back form, .flip-back .form-grid");
        const labels  = [...document.querySelectorAll(".flip-back label, .quick-add label")].map(l => l.textContent.trim());
        const inputs  = [...document.querySelectorAll(".flip-back input, .flip-back select, .quick-add input, .quick-add select")];
        const inputInfo = inputs.map(i => ({ tag: i.tagName, type: i.type, placeholder: i.placeholder, hasLabel: !!(i.closest("label") || i.closest(".labeled-field") || (i.id && document.querySelector(`label[for="${i.id}"]`)) || i.getAttribute("aria-label")) }));
        return {
          hasPanel: !!qa,
          hasForm: !!qaForm,
          labels,
          inputInfo,
        };
      });
      results[`${theme}_${vw}`].transaction = qaAudit;
      console.log(`  Transaction panel: hasPanel=${qaAudit.hasPanel}, inputs=${qaAudit.inputInfo.length}, labels=${qaAudit.labels.length}`);

      // Close
      await page.keyboard.press("Escape");
      await page.waitForTimeout(400);
      // If escape didn't close, try click outside or close button
      const stillOpen = await page.locator(".flip-backdrop").count() > 0 && await page.locator(".flip-backdrop").isVisible().catch(() => false);
      if (stillOpen) {
        await closeFlipPanel(page);
      }
    } else {
      warn(`[${theme}/${vw}] "New transaction" menu item not found`);
    }

    // ── 7. Inline EDIT form on Transactions page ──────────────────────────────
    const txnLink = page.locator('nav a[title="Transactions"]').first();
    if (await txnLink.count() > 0) {
      await txnLink.click();
      await page.waitForTimeout(1000);
      await page.waitForSelector(".txn-table, table", { timeout: 10000 }).catch(() => warn(`[${theme}/${vw}] Transactions page: no table found`));
      await page.waitForTimeout(500);

      // Click the Edit button on the first row — after G2 glamor fixes it's icon-only with title
      const editBtn = page.locator('button[title="Edit this transaction"], .txn-table .btn-icon[title*="Edit"], .txn-table button[title*="Edit"]').first();
      if (await editBtn.count() > 0) {
        await editBtn.click();
        await page.waitForTimeout(600);
        await page.waitForSelector(".row-edit", { timeout: 5000 }).catch(() => warn(`[${theme}/${vw}] Inline edit form did not open`));
        await page.waitForTimeout(400);

        await page.screenshot({ path: shot(`${theme}_${vw}_inline_edit`), fullPage: false });

        const editAudit = await page.evaluate(() => {
          const rowEdit  = document.querySelector(".row-edit");
          const grid     = rowEdit ? rowEdit.querySelector(".form-grid") : null;
          const labels   = rowEdit ? [...rowEdit.querySelectorAll("label")].map(l => l.textContent.trim()).filter(Boolean) : [];
          const lf       = rowEdit ? rowEdit.querySelectorAll(".labeled-field").length : 0;
          const inputs   = rowEdit ? [...rowEdit.querySelectorAll("input, select, textarea")] : [];
          const placeholderOnly = inputs.filter(i => {
            return !i.closest("label") && !i.closest(".labeled-field") && !i.getAttribute("aria-label") && !(i.id && document.querySelector(`label[for="${i.id}"]`));
          }).map(i => ({ tag: i.tagName, type: i.type, ph: i.placeholder }));
          const btns     = rowEdit ? [...rowEdit.querySelectorAll("button")].map(b => ({ text: b.textContent.trim(), cls: b.className })) : [];
          const bg       = rowEdit ? getComputedStyle(rowEdit).backgroundColor : "N/A";
          const inputEl  = rowEdit ? rowEdit.querySelector("input.field") : null;
          const inputBg  = inputEl ? getComputedStyle(inputEl).backgroundColor : "N/A";
          const inputColor = inputEl ? getComputedStyle(inputEl).color : "N/A";
          return { hasRowEdit: !!rowEdit, hasGrid: !!grid, labels, labeledFieldCount: lf, inputs: inputs.length, placeholderOnly, btns, bg, inputBg, inputColor };
        });
        results[`${theme}_${vw}`].inlineEdit = editAudit;
        console.log(`  Inline edit: hasRowEdit=${editAudit.hasRowEdit}, labeledFields=${editAudit.labeledFieldCount}, placeholderOnly=${editAudit.placeholderOnly.length}, btns=[${editAudit.btns.map(b=>b.text).join(",")}]`);
      } else {
        warn(`[${theme}/${vw}] No Edit button found on transaction row`);
        await page.screenshot({ path: shot(`${theme}_${vw}_inline_edit`), fullPage: false });
      }
    }

    // ── 8. Add modal at 768px: check for overflow / layout collapse ───────────
    if (vw === 768) {
      await openAddMenu(page);
      const accItem2 = page.locator('[role="menuitem"]').filter({ hasText: /account/i }).first();
      if (await accItem2.count() > 0) {
        await accItem2.click();
        await page.waitForTimeout(600);
        await page.waitForSelector(".flip-back, .flip-backdrop", { timeout: 5000 }).catch(() => {});
        await page.waitForTimeout(400);
        await page.screenshot({ path: shot(`${theme}_${vw}_add_account_responsive`), fullPage: false });

        const overflowAudit = await page.evaluate(() => {
          const wrap = document.querySelector(".flip-wrap");
          if (!wrap) return { overflow: false };
          const vw = window.innerWidth;
          const rect = wrap.getBoundingClientRect();
          const overflow = rect.width > vw + 4 || rect.right > vw + 4;
          const widthPct = Math.round((rect.width / vw) * 100);
          return { overflow, wrapW: Math.round(rect.width), vw, widthPct };
        });
        console.log(`  768px account modal overflow: ${overflowAudit.overflow}, wrap=${overflowAudit.wrapW}px (${overflowAudit.widthPct}% of vw)`);
        results[`${theme}_${vw}`].responsive = overflowAudit;

        await closeFlipPanel(page);
      }
    }

    await ctx.close();
  }
}

// ── Write JSON audit ──────────────────────────────────────────────────────────
const jsonPath = path.join(__dirname, "screenshots", "gm_02_addedit_dom.json");
fs.writeFileSync(jsonPath, JSON.stringify(results, null, 2));
console.log(`\nDOM audit written: ${jsonPath}`);

// ── Report warnings ───────────────────────────────────────────────────────────
if (errors.length > 0) {
  console.warn("\nWARNINGS:");
  errors.forEach(e => console.warn("  " + e));
}

await browser.close();
console.log("\nDONE. Screenshots in e2e/screenshots/gm_02_addedit_*.png");
