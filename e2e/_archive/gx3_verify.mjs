// GX3 CSS verification — checks table header bg (light), select height, and save button (dark).
// Exits 0 if all pass, 1 if any fail.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8080";
const SCREENSHOT_DIR = path.join(__dirname);

let allPass = true;
const pass = (label, measured) => console.log(`PASS [${label}]: ${measured}`);
const fail = (label, measured, expected) => {
  console.error(`FAIL [${label}]: measured=${measured} | expected=${expected}`);
  allPass = false;
};

function rgbComponents(cssColor) {
  const m = cssColor.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
  if (!m) return null;
  return [parseInt(m[1]), parseInt(m[2]), parseInt(m[3])];
}

// Load the SPA root and wait for WASM to boot
async function loadApp(page) {
  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  // Wait for WASM app to mount (shell appears)
  await page.waitForSelector(".cf-shell", { timeout: 60000 });
}

// Navigate client-side to a route
async function goTo(page, route) {
  await page.evaluate((r) => {
    history.pushState({}, "", r);
    window.dispatchEvent(new PopStateEvent("popstate", { state: {} }));
  }, route);
  await page.waitForTimeout(800);
}

async function setLight(page) {
  await page.evaluate(() =>
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "light" }))
  );
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector(".cf-shell", { timeout: 60000 });
  await page.waitForFunction(
    () => document.documentElement.getAttribute("data-theme") === "light",
    { timeout: 20000 }
  );
}

async function setDark(page) {
  await page.evaluate(() =>
    localStorage.setItem("cashflux:prefs", JSON.stringify({ theme: "dark" }))
  );
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector(".cf-shell", { timeout: 60000 });
  await page.waitForFunction(
    () => document.documentElement.getAttribute("data-theme") === "dark",
    { timeout: 20000 }
  );
}

async function dismissOverlay(page) {
  await page.evaluate(() => {
    document.querySelectorAll('gwc-error-overlay,[id*="gwc-error"],[class*="gwc-error"]').forEach(n => n.remove());
  });
}

const browser = await chromium.launch({ headless: true });

try {
  // ── CHECK 1: Light mode /transactions — table header bg + cell text color ──
  {
    console.log("-- Check 1: table header bg + cell text color (light) --");
    const page = await browser.newPage();

    await loadApp(page);
    await dismissOverlay(page);
    await setLight(page);
    await dismissOverlay(page);

    // Navigate to transactions
    await goTo(page, "/transactions");
    await page.waitForTimeout(1500);
    await dismissOverlay(page);

    let thBg = null;
    let tdColor = null;

    try {
      await page.waitForSelector(".txn-table thead th", { timeout: 8000 });
      thBg = await page.evaluate(() => {
        const th = document.querySelector(".txn-table thead th");
        return th ? getComputedStyle(th).backgroundColor : null;
      });
      tdColor = await page.evaluate(() => {
        const td = document.querySelector(".txn-table tbody td");
        return td ? getComputedStyle(td).color : null;
      });
    } catch {
      console.log("  .txn-table not found — checking DOM for available tables:");
      const debug = await page.evaluate(() => {
        const tables = [...document.querySelectorAll("table,thead,tbody,[class*='table'],[class*='txn']")];
        return tables.map(e => e.tagName + "." + e.className).slice(0, 20);
      });
      console.log("  ", JSON.stringify(debug));
    }

    await page.screenshot({ path: path.join(SCREENSHOT_DIR, "gx3_verify_table_light.png") });

    if (!thBg) {
      fail("table-header-bg", "NOT FOUND (.txn-table thead th)", "≈rgb(247,246,243)");
    } else {
      const comps = rgbComponents(thBg);
      const isLight = comps && comps[0] > 200 && comps[1] > 200 && comps[2] > 200;
      if (isLight) {
        pass("table-header-bg", thBg);
      } else {
        fail("table-header-bg", thBg, "≈rgb(247,246,243) (light, not near-black)");
      }
    }

    if (!tdColor) {
      fail("table-cell-color", "NOT FOUND (.txn-table tbody td)", "≈rgb(28,28,30) dark text");
    } else {
      const comps = rgbComponents(tdColor);
      const isDark = comps && comps[0] < 80 && comps[1] < 80 && comps[2] < 80;
      if (isDark) {
        pass("table-cell-color", tdColor);
      } else {
        fail("table-cell-color", tdColor, "≈rgb(28,28,30) (dark text, not near-white)");
      }
    }

    await page.close();
  }

  // ── CHECK 2: Select height + backgroundColor (light mode) ──
  {
    console.log("-- Check 2: select height + bg (light) --");
    const page = await browser.newPage();

    await loadApp(page);
    await dismissOverlay(page);
    await setLight(page);
    await dismissOverlay(page);
    await goTo(page, "/transactions");
    await page.waitForTimeout(1500);
    await dismissOverlay(page);

    let selectHeight = null;
    let selectBg = null;
    let foundSelect = false;

    // First try: select already visible on page
    try {
      const allSelects = await page.$$("select");
      for (const sel of allSelects) {
        if (await sel.isVisible()) {
          selectHeight = await page.evaluate(el => getComputedStyle(el).height, sel);
          selectBg = await page.evaluate(el => getComputedStyle(el).backgroundColor, sel);
          foundSelect = true;
          console.log("  Found select directly on page");
          break;
        }
      }
    } catch {}

    if (!foundSelect) {
      // Try +Add button via JS
      try {
        const addClicked2 = await page.evaluate(() => {
          const btn = document.querySelector(".add-btn");
          if (btn) { btn.click(); return true; }
          return false;
        });
        if (addClicked2) {
          await page.waitForTimeout(800);
          await dismissOverlay(page);
          console.log("  Clicked .add-btn via JS");
        }

        // Look for "New transaction" / "Add transaction" menu item
        const txnClicked2 = await page.evaluate(() => {
          const items = document.querySelectorAll(".add-item,li,button,[role='menuitem'],[role='option']");
          for (const item of items) {
            if (item.textContent && item.textContent.toLowerCase().includes("transaction")) {
              item.click();
              return item.textContent.trim();
            }
          }
          return null;
        });
        if (txnClicked2) {
          await page.waitForTimeout(800);
          await dismissOverlay(page);
          console.log("  Clicked txn item:", txnClicked2.substring(0, 30));
        }

        const allSelects = await page.$$("select");
        for (const sel of allSelects) {
          if (await sel.isVisible()) {
            selectHeight = await page.evaluate(el => getComputedStyle(el).height, sel);
            selectBg = await page.evaluate(el => getComputedStyle(el).backgroundColor, sel);
            foundSelect = true;
            console.log("  Found select in modal");
            break;
          }
        }
      } catch (e) {
        console.log("  Error finding select:", e.message);
      }
    }

    await page.screenshot({ path: path.join(SCREENSHOT_DIR, "gx3_verify_select.png") });

    if (!foundSelect || !selectHeight) {
      fail("select-height", "NOT FOUND", "≥40px");
      fail("select-bg", "NOT FOUND", "elevated surface");
    } else {
      const h = parseFloat(selectHeight);
      if (h >= 40) {
        pass("select-height", selectHeight);
      } else {
        fail("select-height", selectHeight, "≥40px");
      }

      const comps = selectBg ? rgbComponents(selectBg) : null;
      const isPureWhite = comps && comps[0] === 255 && comps[1] === 255 && comps[2] === 255;
      const isTransparent = !selectBg || selectBg === "rgba(0, 0, 0, 0)" || selectBg === "transparent";
      if (isTransparent) {
        pass("select-bg", `${selectBg} (transparent — inherits surface)`);
      } else if (!isPureWhite) {
        pass("select-bg", selectBg);
      } else {
        fail("select-bg", selectBg, "elevated surface, not raw rgb(255,255,255)");
      }
    }

    await page.close();
  }

  // ── CHECK 3: Dark mode — .set-btn.save backgroundColor + color ──
  {
    console.log("-- Check 3: save button bg + color (dark) --");
    const page = await browser.newPage();

    await loadApp(page);
    await dismissOverlay(page);
    await setDark(page);
    await dismissOverlay(page);
    await goTo(page, "/transactions");
    await page.waitForTimeout(1500);
    await dismissOverlay(page);

    let saveBg = null;
    let saveColor = null;
    let foundSave = false;

    // Try to open +Add → Add transaction modal
    try {
      // Click .add-btn via JS to avoid overlay intercept issues
      const addClicked = await page.evaluate(() => {
        const btn = document.querySelector(".add-btn");
        if (btn) { btn.click(); return true; }
        return false;
      });
      if (addClicked) {
        await page.waitForTimeout(800);
        await dismissOverlay(page);
        console.log("  Clicked .add-btn via JS");
      }

      // Look for "New transaction" or any "transaction" add-item — click via JS
      const txnClicked = await page.evaluate(() => {
        const items = document.querySelectorAll(".add-item,li,button,[role='menuitem'],[role='option']");
        for (const item of items) {
          if (item.textContent && item.textContent.toLowerCase().includes("transaction")) {
            item.click();
            return item.textContent.trim();
          }
        }
        return null;
      });
      if (txnClicked) {
        await page.waitForTimeout(1000);
        await dismissOverlay(page);
        console.log("  Clicked txn item:", txnClicked.substring(0, 30));
      }

      await page.waitForTimeout(800);
      await dismissOverlay(page);

      const saveBtn = await page.$(".set-btn.save");
      if (saveBtn && await saveBtn.isVisible()) {
        saveBg = await page.evaluate(el => getComputedStyle(el).backgroundColor, saveBtn);
        saveColor = await page.evaluate(el => getComputedStyle(el).color, saveBtn);
        foundSave = true;
        console.log("  Found .set-btn.save");
      } else {
        // Debug: log all buttons
        const allBtns = await page.$$eval("button", els =>
          els.map(e => ({ text: e.textContent?.trim().substring(0,30), cls: e.className })).slice(0, 30)
        );
        console.log("  All buttons:", JSON.stringify(allBtns));
      }
    } catch (e) {
      console.error("  Error in check 3:", e.message);
    }

    await page.screenshot({ path: path.join(SCREENSHOT_DIR, "gx3_verify_save_dark.png") });

    if (!foundSave) {
      fail("save-btn-bg", "NOT FOUND (.set-btn.save)", "rgb(46,139,87)");
      fail("save-btn-color", "NOT FOUND (.set-btn.save)", "rgb(255,255,255)");
    } else {
      // rgb(46,139,87) = sea-green
      const bgComps = saveBg ? rgbComponents(saveBg) : null;
      const isSeaGreen =
        bgComps &&
        bgComps[0] >= 25 && bgComps[0] <= 90 &&
        bgComps[1] >= 100 && bgComps[1] <= 180 &&
        bgComps[2] >= 50 && bgComps[2] <= 130;
      if (isSeaGreen) {
        pass("save-btn-bg", saveBg);
      } else {
        fail("save-btn-bg", saveBg, "rgb(46,139,87)");
      }

      const colorComps = saveColor ? rgbComponents(saveColor) : null;
      const isWhite = colorComps && colorComps[0] > 200 && colorComps[1] > 200 && colorComps[2] > 200;
      if (isWhite) {
        pass("save-btn-color", saveColor);
      } else {
        fail("save-btn-color", saveColor, "rgb(255,255,255) (white)");
      }
    }

    await page.close();
  }
} finally {
  await browser.close();
}

if (!allPass) {
  console.error("\nSome checks FAILED.");
  process.exit(1);
} else {
  console.log("\nAll checks PASSED.");
  process.exit(0);
}
