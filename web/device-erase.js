// CashFlux device erasure runs on a separate page so the Wasm app has released
// its IndexedDB connections and cannot save or sync while deletion is in flight.
(function (root, factory) {
  const api = factory();
  if (typeof module === "object" && module.exports) module.exports = api;
  else root.CashFluxDeviceErase = api;
})(typeof window === "object" ? window : globalThis, function () {
  "use strict";

  const marker = "cashflux:erase-in-progress";
  const erasedFlag = "cf-device-erased-v1"; // Nonpersonal; prevents demo data on the next boot.

  function cashFluxDatabase(name) {
    return name === "cashflux" || name === "cashflux-kv" ||
      name.startsWith("cashflux-kv-v2:") ||
      name.startsWith("cashflux-atrest-keys-v1:") ||
      name.startsWith("cashflux-refresh-v1:") || name.startsWith("cfv2:");
  }

  function cashFluxCache(name) {
    return name.startsWith("cashflux-");
  }

  function cashFluxLocalKey(name) {
    return (name.startsWith("cashflux:") && name !== marker) ||
      name === "cf.artifactSalt";
  }

  function localKeys(storage) {
    const keys = [];
    for (let i = 0; i < storage.length; i++) keys.push(storage.key(i));
    return keys.filter(cashFluxLocalKey);
  }

  async function inventory(env) {
    if (!env.indexedDB || typeof env.indexedDB.databases !== "function") {
      throw new Error("This browser cannot list saved databases. Erasure could not be verified.");
    }
    if (!env.caches || typeof env.caches.keys !== "function") {
      throw new Error("This browser cannot check saved caches. Erasure could not be verified.");
    }
    const databases = await env.indexedDB.databases();
    const cacheNames = await env.caches.keys();
    return {
      databases: databases.map((entry) => entry.name).filter((name) => name && cashFluxDatabase(name)),
      caches: cacheNames.filter(cashFluxCache),
      localKeys: localKeys(env.localStorage),
    };
  }

  function deleteDatabase(factory, name, timeoutMs) {
    return new Promise((resolve, reject) => {
      let request;
      try { request = factory.deleteDatabase(name); }
      catch (error) { reject(error); return; }
      let settled = false;
      let blocked = false;
      const finish = (error) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        if (error) reject(error);
        else resolve();
      };
      const timer = setTimeout(() => finish(new Error(blocked
        ? "Another CashFlux tab still has saved data open. Close it and retry."
        : "Deleting saved data took too long. Retry erasure.")), timeoutMs);
      request.onsuccess = () => finish();
      request.onerror = () => finish(new Error("Deleting " + name + " failed: " + (request.error?.message || "browser error")));
      request.onblocked = () => { blocked = true; };
    });
  }

  async function unregisterWorkers(env) {
    if (!env.navigator?.serviceWorker) return;
    const registrations = await env.navigator.serviceWorker.getRegistrations();
    const ownScript = new URL("./sw.js", env.document.baseURI);
    for (const registration of registrations) {
      const script = registration.active?.scriptURL || registration.waiting?.scriptURL || registration.installing?.scriptURL;
      if (!script) continue;
      const registeredScript = new URL(script);
      if (registeredScript.origin !== ownScript.origin || registeredScript.pathname !== ownScript.pathname) continue;
      if (!(await registration.unregister())) throw new Error("CashFlux's offline worker could not be removed. Retry erasure.");
    }
  }

  async function erase(env, progress, timeoutMs = 10000) {
    const before = await inventory(env);
    progress?.("Stopping offline storage");
    await unregisterWorkers(env);
    progress?.("Removing saved databases");
    for (const name of before.databases) await deleteDatabase(env.indexedDB, name, timeoutMs);
    progress?.("Removing offline caches");
    for (const name of before.caches) {
      if (!(await env.caches.delete(name))) throw new Error("Deleting offline cache " + name + " failed. Retry erasure.");
    }
    progress?.("Removing legacy browser records");
    for (const name of before.localKeys) env.localStorage.removeItem(name);
    const after = await inventory(env);
    if (after.databases.length || after.caches.length || after.localKeys.length) {
      throw new Error("Some saved records remain. Close other CashFlux tabs and retry erasure.");
    }
    return { before, after };
  }

  return { marker, erasedFlag, cashFluxDatabase, cashFluxCache, cashFluxLocalKey, inventory, erase };
});
