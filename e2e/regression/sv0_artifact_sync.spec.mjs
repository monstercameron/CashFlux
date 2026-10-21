import { readFile } from "node:fs/promises";
import { test, expect, boot, nav } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, waitForSync, accountSnapshots, accessToken, addUniqueTransaction, unlockIfNeeded } from "./sv0_helpers.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

async function artifactRecordMIME(page, id) {
  return page.evaluate(async (key) => {
    const db = await new Promise((resolve, reject) => {
      const request = indexedDB.open("cashflux", 1);
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error);
    });
    try {
      const record = await new Promise((resolve, reject) => {
        const request = db.transaction("artifacts", "readonly").objectStore("artifacts").get(key);
        request.onsuccess = () => resolve(request.result);
        request.onerror = () => reject(request.error);
      });
      return record?.mime || "";
    } finally {
      db.close();
    }
  }, id);
}

async function artifactIDForName(page, name) {
  await nav(page, "/artifacts");
  const item = page.getByTestId("artifact-name").filter({ hasText: name });
  await expect(item).toBeVisible({ timeout: 30_000 });
  const row = page.locator(".row").filter({ has: item });
  const menuID = await row.locator('[data-testid^="artifact-menu-btn-"]').getAttribute("data-testid");
  return menuID.slice("artifact-menu-btn-".length);
}

async function closeTransactionEditor(page) {
  const dialog = page.getByTestId("txn-edit-dialog");
  await expect(page.getByTestId("txn-edit-form")).toHaveAttribute("aria-busy", "false", { timeout: 45_000 });
  await expect.poll(async () => {
    if (!(await dialog.isVisible())) return true;
    await page.getByTestId("txn-edit-cancel").click({ timeout: 2_000 }).catch(() => {});
    return !(await dialog.isVisible());
  }, { timeout: 25_000, intervals: [500, 1000] }).toBe(true);
}

async function connectWithPassword(page, username, password, expectExistingRemote = false) {
  await configureRemote(page, server.origin);
  try {
    await login(page, username, password);
  } catch (err) {
    console.log("sync login diagnostic", (await page.locator("body").innerText()).slice(-1000));
    throw err;
  }
  if (expectExistingRemote) await waitForRemoteReady(page);
}

async function waitForRemoteReady(page) {
  let previousGeneration = "";
  let stableChecks = 0;
  await expect.poll(async () => {
    await unlockIfNeeded(page);
    const state = await page.evaluate(() => ({
      owner: !!window.cashfluxStoreGet?.("cashflux:dataset-owner"),
      generation: window.cashfluxStoreGet?.("cashflux:dataset:gen") || "",
      ready: document.documentElement.getAttribute("data-app-ready") === "true",
    })).catch(() => ({ owner: false, generation: "", ready: false }));
    const locked = await page.locator("#cf-applock-gate").isVisible().catch(() => true);
    stableChecks = state.owner && state.ready && !locked && state.generation === previousGeneration ? stableChecks + 1 : 0;
    previousGeneration = state.generation;
    return stableChecks >= 4;
  }, { timeout: 120_000, intervals: [500] }).toBe(true);
}

async function waitForVisibleUnlocked(page, locator) {
  await expect.poll(async () => {
    await unlockIfNeeded(page);
    return locator.isVisible();
  }, { timeout: 90_000, intervals: [500, 1000, 2000] }).toBe(true);
}

test("encrypted file and later edits sync across both devices", async ({ app, browser, request }) => {
  test.setTimeout(600_000);
  // Server timestamps use the real clock; fixture time is intentionally pinned
  // months earlier for UI screenshots and would make this edit look stale.
  await app.clock.setFixedTime(new Date());
  const suffix = crypto.randomUUID().slice(0, 8);
  const filename = `private-receipt-${suffix}.png`;
  const pixels = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=", "base64");
  const user = { username: `sv0-file-${suffix}`, password: `CashFlux-F-${suffix}-Pass!` };
  await createPasswordAccount(request, server.origin, user.username, user.password);
  await expect(app.getByTestId("sample-start-fresh")).toBeVisible({ timeout: 30_000 });
  await Promise.all([app.waitForEvent("load", { timeout: 20_000 }), app.getByTestId("sample-start-fresh").click()]);
  await app.waitForFunction(() => document.documentElement.getAttribute("data-app-ready") === "true", null, { timeout: 45_000 });
  await nav(app, "/accounts");
  await app.getByTestId("accounts-add").click();
  const accountForm = app.getByTestId("account-add-form");
  await expect(accountForm).toBeVisible();
  await accountForm.locator('input[type="text"]').first().fill(`File sync ${suffix}`);
  await accountForm.locator('input[type="number"]').first().fill("10");
  await app.locator('button[form="account-add-form"]').click();
  await expect(accountForm).toBeHidden({ timeout: 20_000 });
  let localGeneration = "";
  let settledSaves = 0;
  await expect.poll(async () => {
    const generation = await app.evaluate(() => window.cashfluxStoreGet?.("cashflux:dataset:gen") || "");
    const failed = await app.getByText("Changes aren't saved yet").isVisible().catch(() => false);
    settledSaves = generation && generation === localGeneration && !failed ? settledSaves + 1 : 0;
    localGeneration = generation;
    return settledSaves >= 4;
  }, { timeout: 30_000, intervals: [500] }).toBe(true);
  await connectWithPassword(app, user.username, user.password);
  await expect.poll(async () => (await accountSnapshots(request, server.origin, await accessToken(app))).length, { timeout: 90_000 }).toBeGreaterThan(0);
  await waitForSync(app);
  let previousGeneration = "";
  let stableChecks = 0;
  await expect.poll(async () => {
    const state = await app.evaluate(() => ({
      generation: window.cashfluxStoreGet?.("cashflux:dataset:gen") || "",
      status: JSON.parse(window.cashfluxStoreGet?.("cashflux:sync-status") || "{}"),
    }));
    stableChecks = state.status.state === "synced" && !state.status.pending && state.generation === previousGeneration ? stableChecks + 1 : 0;
    previousGeneration = state.generation;
    return stableChecks >= 4;
  }, { timeout: 30_000, intervals: [500] }).toBe(true);
  await expect(app.getByTestId("stale-dataset-banner")).toHaveCount(0);
  const beforeUpload = await accountSnapshots(request, server.origin, await accessToken(app));

  await nav(app, "/artifacts");
  const [chooser] = await Promise.all([
    app.waitForEvent("filechooser", { timeout: 20_000 }),
    app.getByRole("button", { name: /upload image/i }).click({ timeout: 20_000 }),
  ]);
  await chooser.setFiles({ name: filename, mimeType: "image/png", buffer: pixels });
  const uploadedName = app.getByTestId("artifact-name").filter({ hasText: filename });
  await expect(uploadedName).toBeVisible();
  const uploadedRow = app.locator(".row").filter({ has: uploadedName });
  const menuID = await uploadedRow.locator('[data-testid^="artifact-menu-btn-"]').getAttribute("data-testid");
  const artifactID = menuID.slice("artifact-menu-btn-".length);
  await expect.poll(() => artifactRecordMIME(app, artifactID), { timeout: 30_000 }).toBe("application/x-cashflux-encrypted-artifact");
  await expect.poll(async () => {
    const found = await accountSnapshots(request, server.origin, await accessToken(app));
    return found.some((snapshot) => !beforeUpload.some((old) => old.workspaceID === snapshot.workspaceID && old.envelope === snapshot.envelope));
  }, { timeout: 90_000 }).toBe(true);
  const serverCopies = await accountSnapshots(request, server.origin, await accessToken(app));
  expect(serverCopies.every((snapshot) => !snapshot.envelope.includes(filename)), "server snapshot must not expose the filename").toBe(true);

  // Account documents accept PDFs too. A PDF must not bloat the protected
  // dataset, and its safe open link must recover the exact bytes after pull.
  const pdfName = `private-statement-${suffix}.pdf`;
  const pdfBytes = Buffer.from("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\n%%EOF\n");
  await nav(app, "/accounts");
  const detailsToggle = app.locator('[data-testid^="acct-details-toggle-"]').first();
  const accountID = (await detailsToggle.getAttribute("data-testid")).slice("acct-details-toggle-".length);
  await detailsToggle.click();
  await app.getByTestId(`acct-docs-toggle-${accountID}`).click();
  const [pdfChooser] = await Promise.all([
    app.waitForEvent("filechooser", { timeout: 20_000 }),
    app.getByTestId(`acct-doc-attach-btn-${accountID}`).click(),
  ]);
  await pdfChooser.setFiles({ name: pdfName, mimeType: "application/pdf", buffer: pdfBytes });
  const docRow = app.locator('[data-testid^="acct-doc-row-"]').first();
  await expect(docRow).toBeVisible({ timeout: 30_000 });
  const docID = (await docRow.getAttribute("data-testid")).slice("acct-doc-row-".length);
  await expect.poll(() => artifactRecordMIME(app, docID), { timeout: 30_000 }).toBe("application/x-cashflux-encrypted-artifact");
  const localDocLink = app.getByTestId(`acct-doc-open-${docID}`);
  await expect(localDocLink).toBeVisible({ timeout: 30_000 });
  expect(Buffer.from((await localDocLink.getAttribute("href")).split(",")[1], "base64")).toEqual(pdfBytes);
  await expect.poll(async () => {
    const snapshots = await accountSnapshots(request, server.origin, await accessToken(app));
    return snapshots.some((snapshot) => !serverCopies.some((old) => old.workspaceID === snapshot.workspaceID && old.envelope === snapshot.envelope));
  }, { timeout: 90_000 }).toBe(true);

  // Transaction receipts and goal photos use the same encrypted local store.
  // Check their actual UI paths, then reopen both from another browser.
  const receiptName = `private-receipt-transaction-${suffix}.png`;
  const receiptTxn = `receipt-owner-${suffix}`;
  await addUniqueTransaction(app, receiptTxn);
  const receiptRow = app.locator('tr[data-testid^="txn-row-"]').filter({ hasText: receiptTxn }).first();
  await receiptRow.getByTestId("txn-rowedit").click();
  await expect(app.getByTestId("txn-edit-amount")).toBeVisible({ timeout: 20_000 });
  await app.getByTestId("txn-edit-more-actions").click();
  const [receiptChooser] = await Promise.all([
    app.waitForEvent("filechooser", { timeout: 20_000 }),
    app.getByTestId("txn-edit-attach").click(),
  ]);
  await receiptChooser.setFiles({ name: receiptName, mimeType: "image/png", buffer: pixels });
  const receiptThumb = app.getByTestId("txn-receipt-thumb").first().locator("img");
  await expect(receiptThumb).toHaveAttribute("src", /^data:image\/png;base64,/, { timeout: 30_000 });
  expect(Buffer.from((await receiptThumb.getAttribute("src")).split(",")[1], "base64")).toEqual(pixels);
  await closeTransactionEditor(app);
  const receiptID = await artifactIDForName(app, receiptName);
  await expect.poll(() => artifactRecordMIME(app, receiptID), { timeout: 30_000 }).toBe("application/x-cashflux-encrypted-artifact");

  const goalName = `photo-goal-${suffix}`;
  const goalPhotoName = `private-goal-${suffix}.png`;
  await nav(app, "/goals");
  await app.getByTestId("goals-add").click();
  const goalForm = app.getByTestId("goal-add-form");
  await expect(goalForm).toBeVisible({ timeout: 20_000 });
  await goalForm.locator("#goal-add").fill(goalName);
  await goalForm.locator('input[type="number"]').first().fill("250");
  await goalForm.locator('button[type="submit"]').click();
  await expect(goalForm).toBeHidden({ timeout: 20_000 });
  const goalRow = app.locator('[data-testid^="goal-row-"]').filter({ hasText: goalName }).first();
  await expect(goalRow).toBeVisible();
  const goalID = (await goalRow.getAttribute("data-testid")).slice("goal-row-".length);
  const expandGoal = app.getByTestId(`goal-expand-${goalID}`);
  if (await expandGoal.count()) await expandGoal.click();
  await app.getByTestId(`goal-edit-btn-${goalID}`).click();
  const [goalChooser] = await Promise.all([
    app.waitForEvent("filechooser", { timeout: 20_000 }),
    app.getByTestId(`goal-add-photo-${goalID}`).click(),
  ]);
  await goalChooser.setFiles({ name: goalPhotoName, mimeType: "image/png", buffer: pixels });
  const goalImage = app.getByTestId(`goal-photo-${goalID}`).locator("img").first();
  await expect(goalImage).toHaveAttribute("src", /^data:image\/png;base64,/, { timeout: 30_000 });
  expect(Buffer.from((await goalImage.getAttribute("src")).split(",")[1], "base64")).toEqual(pixels);
  await app.locator('[role="dialog"]').getByRole("button", { name: "Cancel" }).last().click();
  const goalPhotoID = await artifactIDForName(app, goalPhotoName);
  await expect.poll(() => artifactRecordMIME(app, goalPhotoID), { timeout: 30_000 }).toBe("application/x-cashflux-encrypted-artifact");
  await waitForSync(app);
  const appOrigin = new URL(app.url()).origin;
  await app.close(); // release the first WebAssembly runtime before booting another profile

  const secondContext = await browser.newContext({ baseURL: appOrigin, reducedMotion: "reduce" });
  const second = await secondContext.newPage();
  try {
    await boot(second);
    await second.clock.setFixedTime(new Date());
    await connectWithPassword(second, user.username, user.password, true);
    await unlockIfNeeded(second);
    await nav(second, "/artifacts");
    await waitForVisibleUnlocked(second, second.getByTestId("artifact-name").filter({ hasText: filename }));
    const row = second.locator(".row").filter({ has: second.getByTestId("artifact-name").filter({ hasText: filename }) });
    await row.locator('[data-testid^="artifact-menu-btn-"]').click({ timeout: 20_000 });
    const [download] = await Promise.all([
      second.waitForEvent("download", { timeout: 25_000 }),
      row.locator('[data-testid^="artifact-download-"]').click({ timeout: 20_000 }),
    ]);
    expect(await readFile(await download.path())).toEqual(pixels);
    await nav(second, "/accounts");
    await second.getByTestId(`acct-details-toggle-${accountID}`).click();
    await second.getByTestId(`acct-docs-toggle-${accountID}`).click();
    const remoteDocLink = second.getByTestId(`acct-doc-open-${docID}`);
    await waitForVisibleUnlocked(second, remoteDocLink);
    expect(Buffer.from((await remoteDocLink.getAttribute("href")).split(",")[1], "base64")).toEqual(pdfBytes);
    await expect.poll(() => artifactRecordMIME(second, docID), { timeout: 30_000 }).toBe("application/x-cashflux-encrypted-artifact");
    await nav(second, "/goals");
    await waitForVisibleUnlocked(second, second.getByTestId(`goal-row-${goalID}`));
    const remoteExpand = second.getByTestId(`goal-expand-${goalID}`);
    if (await remoteExpand.count()) await remoteExpand.click();
    const remoteGoal = second.getByTestId(`goal-photo-${goalID}`).locator("img").first();
    await waitForVisibleUnlocked(second, remoteGoal);
    expect(Buffer.from((await remoteGoal.getAttribute("src")).split(",")[1], "base64")).toEqual(pixels);
    await expect.poll(() => artifactRecordMIME(second, goalPhotoID), { timeout: 30_000 }).toBe("application/x-cashflux-encrypted-artifact");
    await nav(second, "/transactions");
    const remoteReceiptRow = second.locator('tr[data-testid^="txn-row-"]').filter({ hasText: receiptTxn }).first();
    await waitForVisibleUnlocked(second, remoteReceiptRow);
    await remoteReceiptRow.getByTestId("txn-rowedit").click();
    const remoteReceipt = second.getByTestId("txn-receipt-thumb").first().locator("img");
    await expect(remoteReceipt).toHaveAttribute("src", /^data:image\/png;base64,/, { timeout: 30_000 });
    expect(Buffer.from((await remoteReceipt.getAttribute("src")).split(",")[1], "base64")).toEqual(pixels);
    await expect.poll(() => artifactRecordMIME(second, receiptID), { timeout: 30_000 }).toBe("application/x-cashflux-encrypted-artifact");
    await closeTransactionEditor(second);
    const beforeReturn = await accountSnapshots(request, server.origin, await accessToken(second));
    const edit = `second-device-edit-${suffix}`;
    await addUniqueTransaction(second, edit);
    try {
      await expect.poll(async () => {
        const snapshots = await accountSnapshots(request, server.origin, await accessToken(second));
        return snapshots.some((snapshot) => !beforeReturn.some((old) => old.workspaceID === snapshot.workspaceID && old.envelope === snapshot.envelope));
      }, { timeout: 30_000 }).toBe(true);
    } catch (err) {
      console.log("second browser sync state", await second.evaluate(() => {
        const registry = JSON.parse(window.cashfluxStoreGet?.("cashflux:workspaces") || "{}");
        return {
          status: window.cashfluxStoreGet?.("cashflux:sync-status"),
          queue: !!window.cashfluxStoreGet?.("cashflux:sync-queue"),
          owner: !!window.cashfluxStoreGet?.("cashflux:dataset-owner"),
          generation: !!window.cashfluxStoreGet?.("cashflux:dataset:gen"),
          head: !!window.cashfluxStoreGet?.("cashflux:dataset:head"),
          auth: !!window.cashfluxStoreGet?.("cashflux:auth:user-id"),
          meta: window.cashfluxStoreGet?.(`cashflux:sync-meta:${registry.activeId}`)?.slice(0, 200),
          saveBanner: document.querySelector('[data-testid="save-failure-banner"]')?.textContent,
        };
      }));
      throw err;
    }
    await waitForSync(second);
    let stableSyncChecks = 0;
    const cleanState = async () => {
      const state = await second.evaluate(() => {
        const get = window.cashfluxStoreGet;
        const registry = JSON.parse(get?.("cashflux:workspaces") || "{}");
        return {
          status: JSON.parse(get?.("cashflux:sync-status") || "{}"),
          queued: JSON.parse(get?.("cashflux:sync-queue") || "[]").length > 0,
          conflict: !!get?.(`cashflux:sync-conflict:${registry.activeId}`),
        };
      });
      stableSyncChecks = state.status.state === "synced" && !state.queued && !state.conflict ? stableSyncChecks + 1 : 0;
      return stableSyncChecks >= 6;
    };
    try {
      await expect.poll(cleanState, { timeout: 30_000, intervals: [500] }).toBe(true);
    } catch (err) {
      console.log("second browser unsettled sync", await second.evaluate(() => {
        const get = window.cashfluxStoreGet;
        const registry = JSON.parse(get?.("cashflux:workspaces") || "{}");
        const conflict = JSON.parse(get?.(`cashflux:sync-conflict:${registry.activeId}`) || "null");
        const meta = JSON.parse(get?.(`cashflux:sync-meta:${registry.activeId}`) || "null");
        return { status: get?.("cashflux:sync-status"), queue: get?.("cashflux:sync-queue")?.slice(0, 300), conflict: conflict && { baseVersion: conflict.baseVersion, hash: conflict.hash, hasEdit: conflict.dataset?.includes("second-device-edit") }, meta: meta && { version: meta.version, hash: meta.hash } };
      }));
      throw err;
    }
    const returned = await accountSnapshots(request, server.origin, await accessToken(second));
    expect(returned.every((snapshot) => !snapshot.envelope.includes(edit)), "second-device edit must remain encrypted on the server").toBe(true);
    await second.close(); // each profile is independent; only one runtime needs to stay loaded
    const thirdContext = await browser.newContext({ baseURL: appOrigin, reducedMotion: "reduce" });
    try {
      const third = await thirdContext.newPage();
      await boot(third);
      await third.clock.setFixedTime(new Date());
      await connectWithPassword(third, user.username, user.password, true);
      await unlockIfNeeded(third);
      await nav(third, "/transactions");
      await waitForVisibleUnlocked(third, third.getByText(edit, { exact: true }));
    } finally {
      await thirdContext.close();
    }
  } finally {
    await secondContext.close();
  }
});
