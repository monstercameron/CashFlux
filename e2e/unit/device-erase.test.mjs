import test from "node:test";
import assert from "node:assert/strict";
import erase from "../../web/device-erase.js";

function fixture({ blocked = false } = {}) {
  const names = new Set([
    "cashflux", "cashflux-kv", "cashflux-kv-v2:other-account",
    "cashflux-atrest-keys-v1:other-workspace", "cashflux-refresh-v1:other-account",
    "cfv2:other-workspace", "unrelated-app",
  ]);
  const savedCaches = new Set(["cashflux-fav1-old", "cashflux-private-legacy", "unrelated-cache"]);
  const values = new Map([
    ["cashflux:workspaces", "two workspaces"],
    ["cashflux:sync-queue", "pending snapshot"],
    ["cf.artifactSalt", "legacy salt"],
    [erase.marker, "ticket"],
    ["unrelated:key", "keep"],
  ]);
  const localStorage = {
    get length() { return values.size; },
    key(index) { return Array.from(values.keys())[index] ?? null; },
    removeItem(key) { values.delete(key); },
  };
  const indexedDB = {
    async databases() { return Array.from(names, (name) => ({ name })); },
    deleteDatabase(name) {
      const request = {};
      setTimeout(() => {
        if (blocked) request.onblocked?.();
        else { names.delete(name); request.onsuccess?.(); }
      }, 0);
      return request;
    },
  };
  const caches = {
    async keys() { return Array.from(savedCaches); },
    async delete(name) { return savedCaches.delete(name); },
  };
  const env = { indexedDB, caches, localStorage, navigator: { serviceWorker: { async getRegistrations() { return []; } } } };
  return { env, names, savedCaches, values };
}

test("erase removes every CashFlux storage family while retaining unrelated origins and the retry marker", async () => {
  const { env, names, savedCaches, values } = fixture();
  const result = await erase.erase(env);
  assert.equal(result.before.databases.length, 6);
  assert.deepEqual(result.after, { databases: [], caches: [], localKeys: [] });
  assert.deepEqual(Array.from(names), ["unrelated-app"]);
  assert.deepEqual(Array.from(savedCaches), ["unrelated-cache"]);
  assert.deepEqual(Array.from(values.keys()), [erase.marker, "unrelated:key"]);
});

test("a blocked second tab never produces a success acknowledgement", async () => {
  const { env, names } = fixture({ blocked: true });
  await assert.rejects(erase.erase(env, null, 20), /Another CashFlux tab/);
  assert.ok(names.has("cashflux"));
});

test("verification fails closed when the browser cannot enumerate databases", async () => {
  const { env } = fixture();
  delete env.indexedDB.databases;
  await assert.rejects(erase.erase(env), /could not be verified/);
});
