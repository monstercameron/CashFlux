import { test, expect } from "./fixtures.mjs";
import { startServer, createPasswordAccount, configureRemote, login, signOut, waitForSync, addUniqueTransaction, datasetHas, reloadReady, accountSnapshots, accessToken } from "./sv0_helpers.mjs";

let server;
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await server?.stop(); });

test("signing out of A and into B cannot route A's queued data to B", async ({ app, request }) => {
  test.setTimeout(180_000);
  const suffix = crypto.randomUUID().slice(0, 8);
  const a = { username: `sv0-a-${suffix}`, password: `CashFlux-A-${suffix}-Pass!` };
  const b = { username: `sv0-b-${suffix}`, password: `CashFlux-B-${suffix}-Pass!` };
  await createPasswordAccount(request, server.origin, a.username, a.password);
  await createPasswordAccount(request, server.origin, b.username, b.password);
  await configureRemote(app, server.origin);
  await login(app, a.username, a.password);
  await waitForSync(app);

  const marker = `SV0-ACCOUNT-A-${suffix}`;
  await addUniqueTransaction(app, marker);
  await waitForSync(app);
  expect(await datasetHas(app, marker)).toBe(true);
  const aToken = await accessToken(app);
  expect(aToken).not.toBe("");
  const aCopy = await accountSnapshots(request, server.origin, aToken);
  expect(aCopy.length, "A's server copy should be present after encrypted sync").toBeGreaterThan(0);
  expect(aCopy.some((snapshot) => snapshot.envelope.includes(marker)), "the server must not see A's plaintext transaction").toBe(false);

  await signOut(app);
  await login(app, b.username, b.password);
  await waitForSync(app);
  await reloadReady(app);
  expect(await datasetHas(app, marker), "B must not see A's local transaction after identity reload").toBe(false);
  const bCopy = await accountSnapshots(request, server.origin, await accessToken(app));
  expect(bCopy.some((snapshot) => snapshot.envelope.includes(marker)), "the server must not see A's plaintext transaction in B's scope").toBe(false);

  await signOut(app);
  await login(app, a.username, a.password);
  await waitForSync(app);
  await reloadReady(app);
  expect(await datasetHas(app, marker), "A's server copy must retain A's transaction").toBe(true);
  const restoredACopy = await accountSnapshots(request, server.origin, await accessToken(app));
  expect(restoredACopy.length, "A's encrypted server copy must remain available").toBeGreaterThan(0);
  expect(restoredACopy.some((snapshot) => snapshot.envelope.includes(marker)), "A's server copy must remain encrypted").toBe(false);
});
