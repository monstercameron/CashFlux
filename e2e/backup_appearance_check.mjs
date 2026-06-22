// L22 gate — appearance is included in the "back up everything" envelope and
// survives a restore cycle.
//
// Strategy: directly write a sentinel value into the cashflux:theme localStorage
// key (bypassing the theme editor UI, which is complex to drive headlessly), run
// "Back up everything", wipe the key, restore the backup, and assert the sentinel
// comes back. The same pattern is repeated for the cashflux:banner and
// cashflux:fonts keys to confirm all three appearance slots are round-tripped.
//
// localStorage keys under test:
//   cashflux:theme  — JSON theme token object (verbatim stored value)
//   cashflux:fonts  — JSON array of FontAsset objects (verbatim stored value)
//   cashflux:banner — banner data URL / JSON (verbatim stored value)
//
// Selectors used:
//   #app            — GoWebComponents mount root; present when the WASM app is ready
//   #cf-cmd-input   — command palette text input (opened by Ctrl+K)
//   [data-cmd-row]  — a single command palette row
//
// Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";
import os from "os";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";

// Sentinel values written into localStorage before the backup.  Each is a valid
// stored blob for its key so the app can parse it after restore — but with
// unmistakeable sentinel markers so we can assert they came back exactly.
const SENTINEL_THEME = JSON.stringify({ __sentinel: "L22-theme", accent: "#badbad" });
const SENTINEL_FONTS = JSON.stringify([{ __sentinel: "L22-fonts", family: "SentinelFont", data: "data:font/woff2;base64,AA==" }]);
const SENTINEL_BANNER = "data:image/png;base64,L22sentinelBanner==";

const THEME_KEY  = "cashflux:theme";
const FONTS_KEY  = "cashflux:fonts";
const BANNER_KEY = "cashflux:banner";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const openPalette = async (page) => {
  await page.keyboard.press("Control+k");
  await page.waitForSelector("#cf-cmd-input", { timeout: 10000, state: "visible" });
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => {
    const s = String(e);
    // "Go program has already exited" is a known headless-download artifact —
    // the same error fires on the existing Export JSON command, so it is not
    // specific to this flow. Ignore it; surface everything else.
    if (/Go program has already exited/.test(s)) return;
    errors.push(s);
  });
  page.on("dialog", (d) => d.accept()); // accept the "replace all data?" confirm

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });

  // 1) Write sentinel appearance values directly into localStorage.
  await page.evaluate(
    ({ themeKey, fontsKey, bannerKey, theme, fonts, banner }) => {
      localStorage.setItem(themeKey,  theme);
      localStorage.setItem(fontsKey,  fonts);
      localStorage.setItem(bannerKey, banner);
    },
    {
      themeKey:  THEME_KEY,
      fontsKey:  FONTS_KEY,
      bannerKey: BANNER_KEY,
      theme:     SENTINEL_THEME,
      fonts:     SENTINEL_FONTS,
      banner:    SENTINEL_BANNER,
    },
  );

  // 2) Run "Back up everything" and capture the downloaded file.
  await openPalette(page);
  await page.fill("#cf-cmd-input", "back up everything");
  await page.waitForTimeout(150);

  const backupRow = page.locator("[data-cmd-row]").filter({ hasText: /back up everything/i }).first();
  if ((await backupRow.count()) === 0) {
    fail('the "Back up everything" command did not surface in the palette');
  }

  const [download] = await Promise.all([
    page.waitForEvent("download", { timeout: 10000 }),
    backupRow.click(),
  ]);

  const downloadPath = await download.path();
  const envelope = JSON.parse(fs.readFileSync(downloadPath, "utf8"));

  // 3) Assert the appearance block is present in the envelope.
  if (!envelope.appearance) {
    fail("backup envelope missing appearance field");
  } else {
    if (envelope.appearance.theme !== SENTINEL_THEME) {
      fail(`envelope.appearance.theme mismatch: got ${envelope.appearance.theme}`);
    }
    if (envelope.appearance.fonts !== SENTINEL_FONTS) {
      fail(`envelope.appearance.fonts mismatch: got ${envelope.appearance.fonts}`);
    }
    if (envelope.appearance.banner !== SENTINEL_BANNER) {
      fail(`envelope.appearance.banner mismatch: got ${envelope.appearance.banner}`);
    }
  }

  // 4) Wipe the appearance keys so we can tell whether the restore puts them back.
  await page.evaluate(
    ({ themeKey, fontsKey, bannerKey }) => {
      localStorage.removeItem(themeKey);
      localStorage.removeItem(fontsKey);
      localStorage.removeItem(bannerKey);
    },
    { themeKey: THEME_KEY, fontsKey: FONTS_KEY, bannerKey: BANNER_KEY },
  );

  // Write the fixture for the restore command (use the unmodified backup).
  const fixture = path.join(os.tmpdir(), "cashflux-backup-appearance-fixture.json");
  fs.writeFileSync(fixture, JSON.stringify(envelope));

  // The download tears down the wasm runtime (known headless artifact), so reload
  // to get a fresh page before driving the restore command.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });

  // 5) Restore the backup: feed our fixture file via the file-chooser event.
  page.once("filechooser", (fc) => fc.setFiles(fixture));
  await openPalette(page);
  await page.fill("#cf-cmd-input", "restore from a backup");
  await page.waitForTimeout(150);

  const restoreRow = page.locator("[data-cmd-row]").filter({ hasText: /restore from a backup/i }).first();
  if ((await restoreRow.count()) === 0) {
    fail('the "Restore from a backup" command did not surface in the palette');
  }
  await restoreRow.click();

  // 6) After the confirm + page reload, assert all three appearance sentinels are
  //    back in localStorage.
  const check = async (key, sentinel) => {
    await page
      .waitForFunction(
        ({ k, v }) => localStorage.getItem(k) === v,
        { k: key, v: sentinel },
        { timeout: 15000 },
      )
      .catch(() => fail(`appearance key "${key}" was not restored to its sentinel value after backup restore`));
  };

  await check(THEME_KEY,  SENTINEL_THEME);
  await check(FONTS_KEY,  SENTINEL_FONTS);
  await check(BANNER_KEY, SENTINEL_BANNER);

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) {
    console.log("PASS: backup includes appearance (theme/fonts/banner) and restore puts them back.");
  }
} finally {
  await browser.close();
}
