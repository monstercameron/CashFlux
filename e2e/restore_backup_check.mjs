// L9 gate — "restore from a backup". Export a full backup, tamper its workspace
// registry with a sentinel name, restore it via the palette (file picker), and
// assert the sentinel survives the reload — proving the restore wrote the backup's
// contents back into localStorage. Exits non-zero on any failure.
import { createRequire } from "module";
import { fileURLToPath } from "url";
import path from "path";
import fs from "fs";
import os from "os";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(__dirname, "..", ".tools", "package.json"));
const { chromium } = require("playwright");

const BASE = process.env.E2E_URL || "http://127.0.0.1:8099";
const SENTINEL = "SentinelWS_" + "restorecheck";

const browser = await chromium.launch({ headless: true });
const fail = (m) => {
  console.error("FAIL: " + m);
  process.exitCode = 1;
};

const runCommand = async (page, text, query) => {
  await page.keyboard.press("Control+k");
  await page.waitForSelector("#cf-cmd-input", { timeout: 10000, state: "visible" });
  await page.fill("#cf-cmd-input", query);
  await page.waitForTimeout(150);
  return page.locator("[data-cmd-row]").filter({ hasText: text }).first();
};

try {
  const page = await browser.newPage();
  const errors = [];
  page.on("pageerror", (e) => {
    const s = String(e);
    if (/Go program has already exited/.test(s)) return; // known headless-download artifact
    errors.push(s);
  });
  page.on("dialog", (d) => d.accept()); // accept the "replace all data?" confirm

  await page.goto(BASE + "/", { waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });

  // 1) Export a full backup and read it.
  const backupRow = await runCommand(page, /back up everything/i, "back up everything");
  const [download] = await Promise.all([page.waitForEvent("download", { timeout: 10000 }), backupRow.click()]);
  const backup = JSON.parse(fs.readFileSync(await download.path(), "utf8"));
  if (typeof backup.workspaceRegistry !== "string" || !backup.workspaceRegistry.includes("Default")) {
    fail("backup workspaceRegistry missing the default workspace name to tamper: " + backup.workspaceRegistry);
  }

  // 2) Tamper the registry with a sentinel workspace name, write a fixture file.
  backup.workspaceRegistry = backup.workspaceRegistry.replace("Default", SENTINEL);
  const fixture = path.join(os.tmpdir(), "cashflux-restore-fixture.json");
  fs.writeFileSync(fixture, JSON.stringify(backup));

  // The download tears down the wasm runtime (known headless artifact), so reload
  // to get a fresh one before driving the restore command.
  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector("#app", { timeout: 60000 });

  // 3) Restore it: the command opens a file picker; feed it our fixture.
  page.once("filechooser", (fc) => fc.setFiles(fixture));
  const restoreRow = await runCommand(page, /restore from a backup/i, "restore from a backup");
  await restoreRow.click();

  // 4) After the confirm + reload, the sentinel registry name should be persisted.
  await page
    .waitForFunction(
      (s) => (localStorage.getItem("cashflux:workspaces") || "").includes(s),
      SENTINEL,
      { timeout: 15000 },
    )
    .catch(() => fail("the restored sentinel workspace name was not persisted after reload"));

  if (errors.length) fail("page errors: " + errors.join(" | "));
  if (!process.exitCode) console.log("PASS: restore-from-backup wrote the backup's registry back and survived reload.");
} finally {
  await browser.close();
}
