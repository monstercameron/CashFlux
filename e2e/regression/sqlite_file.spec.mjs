// sqlite_file.spec.mjs — exporting and importing the household as a real
// SQLite database file, driven through the actual UI in a real browser.
//
// The unit tests prove the bytes round-trip. What only a browser can prove is
// the part between them: that the download is triggered and arrives as binary
// (a Blob built from a Uint8Array, not text mangled by an encoding), that the
// file picker hands those exact bytes back to wasm, and that importing replaces
// the household the user is looking at.
import { test, expect, nav } from "./fixtures.mjs";
import fs from "node:fs/promises";

const SQLITE_MAGIC = "SQLite format 3\0";

/** Open Settings → Data, where the export/import buttons live. */
async function openDataTab(app) {
  await nav(app, "/settings");
  await app
    .locator(".settings-page .set-tab-strip button", { hasText: "Data" })
    .first()
    .click();
  await expect(
    app.getByRole("button", { name: "Export database (.sqlite)" }),
  ).toBeVisible();
}

/** Click a data-tab button and return the bytes it downloads. */
async function downloadBytes(app, buttonName) {
  const [download] = await Promise.all([
    app.waitForEvent("download"),
    app.getByRole("button", { name: buttonName }).click(),
  ]);
  const path = await download.path();
  return { name: download.suggestedFilename(), bytes: await fs.readFile(path) };
}

/** Wait for the app to finish booting (used after any reload). */
async function waitForAppReady(app) {
  await app.waitForFunction(
    () => document.documentElement.getAttribute("data-app-ready") === "true",
    null,
    { timeout: 45_000 },
  );
}

/**
 * Click an import button and hand the picker a file.
 *
 * pickFile creates its <input type=file> detached and clicks it without ever
 * appending it to the document, so there is no element to locate — the file
 * chooser event is the only handle on it.
 */
async function importFile(app, buttonName, file) {
  const [chooser] = await Promise.all([
    app.waitForEvent("filechooser"),
    app.getByRole("button", { name: buttonName }).click(),
  ]);
  await chooser.setFiles(file);
}

/**
 * How many transactions the household holds, read from the persisted dataset.
 *
 * Deliberately not counted off the ledger screen. What this spec is testing is
 * data moving in and out of a file, and routing a heavyweight screen in and out
 * on every check would make the test's slowest, flakiest part the one thing it
 * is not about.
 */
async function transactionCount(app) {
  return app.evaluate(() => {
    const raw = window.cashfluxStoreGet?.("cashflux:dataset");
    if (!raw) return -1;
    try {
      return (JSON.parse(raw).transactions || []).length;
    } catch {
      return -1;
    }
  });
}

test.describe("SQLite database file export and import", () => {
  test.describe.configure({ mode: "serial" });

  test("exports a real SQLite file and imports it back through the UI", async ({
    app,
  }) => {
    test.setTimeout(180_000);

    const before = await transactionCount(app);
    expect(before).toBeGreaterThan(0); // the sample household seeds rows

    // ── Export ────────────────────────────────────────────────────────────
    await openDataTab(app);
    const { name, bytes } = await downloadBytes(
      app,
      "Export database (.sqlite)",
    );

    expect(name).toBe("cashflux.sqlite");
    // The bytes must be a genuine database, not a JSON blob with a new
    // extension — this is the whole promise of the feature.
    expect(bytes.subarray(0, SQLITE_MAGIC.length).toString("binary")).toBe(
      SQLITE_MAGIC,
    );
    // A Blob built from text rather than a Uint8Array would arrive mangled and
    // truncated; a real database of a seeded household is comfortably past this.
    expect(bytes.length).toBeGreaterThan(4096);
    // And it carries the household, not just an empty schema.
    expect(bytes.includes(Buffer.from("transactions"))).toBe(true);

    // ── Wipe, so the import has something to prove ────────────────────────
    // The wipe reloads the page once the clean slate is durable, so wait for the
    // app to come back up before touching it again — anything else races a
    // destroyed execution context rather than testing the import.
    await openDataTab(app);
    await app.getByRole("button", { name: "Wipe data" }).click();
    const erase = app.getByRole("button", { name: "Erase everything" });
    await expect(erase).toBeVisible();
    await Promise.all([app.waitForLoadState("load"), erase.click()]);
    await waitForAppReady(app);
    await expect
      .poll(async () => transactionCount(app), { timeout: 30_000 })
      .toBe(0);

    // ── Import it back ────────────────────────────────────────────────────
    await openDataTab(app);
    await importFile(app, "Import database (.sqlite)…", {
      name: "cashflux.sqlite",
      mimeType: "application/vnd.sqlite3",
      buffer: bytes,
    });
    const replace = app.getByRole("button", { name: "Replace all data" });
    await expect(replace).toBeVisible();
    await replace.click();

    await expect
      .poll(async () => transactionCount(app), { timeout: 30_000 })
      .toBe(before);
  });

  test("refuses a file that is not one of our databases, and keeps the data", async ({
    app,
  }) => {
    test.setTimeout(120_000);

    const before = await transactionCount(app);
    expect(before).toBeGreaterThan(0);

    await openDataTab(app);
    await importFile(app, "Import database (.sqlite)…", {
      name: "notes.sqlite",
      mimeType: "application/vnd.sqlite3",
      buffer: Buffer.from('{"members":[],"schemaVersion":1}'), // a JSON export, renamed
    });
    const replace = app.getByRole("button", { name: "Replace all data" });
    await expect(replace).toBeVisible();
    await replace.click();

    // It says so plainly, and — the part that matters — the household is intact.
    await expect(
      app.getByText(/Couldn't import that database/i).first(),
    ).toBeVisible({
      timeout: 15_000,
    });
    expect(await transactionCount(app)).toBe(before);
  });
});
