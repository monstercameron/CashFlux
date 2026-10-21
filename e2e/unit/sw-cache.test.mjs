import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";

const source = readFileSync(new URL("../../web/sw.js", import.meta.url), "utf8");
const origin = "https://cashflux.test";

function worker(initialEntries = [], { offline = false } = {}) {
  const listeners = new Map();
  const entries = new Set(initialEntries);
  const deletedCaches = [];
  const cached = [];
  const cache = {
    keys: async () => [...entries].map((url) => new Request(url)),
    delete: async (request) => entries.delete(request.url),
    match: async (key) => key === "./__cashflux_complete__.json"
      ? new Response(JSON.stringify({ buildId: "test", complete: true }))
      : typeof key === "string" && key.startsWith("./")
        ? new Response(key === "./erase-device.html" ? "ERASER" : "asset") : null,
    put: async (request) => { cached.push(request.url); },
  };
  const caches = {
    keys: async () => ["cashflux-fav1-test", "cashflux-fav1-old"],
    open: async () => cache,
    delete: async (name) => { deletedCaches.push(name); return true; },
  };
  const self = {
    location: { href: `${origin}/sw.js?build=test`, origin },
    registration: { scope: `${origin}/` },
    clients: { claim: async () => {}, matchAll: async () => [] },
    addEventListener: (name, listener) => listeners.set(name, listener),
  };
  const fetch = async () => {
    if (offline) throw new Error("offline");
    return new Response("asset");
  };
  const context = vm.createContext({ self, caches, URL, Request, Response, fetch, console });
  vm.runInContext(source, context);
  return { listeners, entries, deletedCaches, cached, context };
}

test("API paths cannot masquerade as precached files", () => {
  const { context } = worker();
  assert.equal(vm.runInContext(`coreKeyFor("${origin}/icon-192.png")`, context), "./icon-192.png");
  assert.equal(vm.runInContext(`coreKeyFor("${origin}/v1/icon-192.png")`, context), "");
  assert.equal(vm.runInContext(`coreKeyFor("${origin}/v1/bin/main.wasm")`, context), "");
});

test("offline device erase navigation serves the eraser instead of the app shell", async () => {
  const { context, listeners } = worker([], { offline: true });
  assert.equal(vm.runInContext(`CORE_REQUIRED.includes("./erase-device.html")`, context), true);
  assert.equal(vm.runInContext(`CORE_REQUIRED.includes("./device-erase.js")`, context), true);
  assert.equal(vm.runInContext(`coreKeyFor("${origin}/device-erase.js")`, context), "./device-erase.js");
  let response;
  listeners.get("fetch")({
    request: { method: "GET", mode: "navigate", url: `${origin}/erase-device.html` },
    respondWith: (value) => { response = value; },
  });
  assert.equal(await (await response).text(), "ERASER");
});

test("worker leaves API requests alone and caches only public static paths", async () => {
  const { listeners, cached } = worker();
  const fetchListener = listeners.get("fetch");
  for (const path of ["/v1/account/export", "/v1/me", "/grpc", "/private.json", "/v1/icon-192.png"]) {
    let intercepted = false;
    fetchListener({ request: new Request(origin + path), respondWith: () => { intercepted = true; } });
    assert.equal(intercepted, false, `${path} was intercepted`);
  }
  let navigatedAPI = false;
  fetchListener({
    request: { method: "GET", mode: "navigate", url: `${origin}/v1/account/export` },
    respondWith: () => { navigatedAPI = true; },
  });
  assert.equal(navigatedAPI, false, "top-level API navigation was intercepted");
  let response;
  fetchListener({ request: new Request(`${origin}/fonts/f1.woff2`), respondWith: (value) => { response = value; } });
  assert.equal((await response).status, 200);
  assert.deepEqual(cached, [`${origin}/fonts/f1.woff2`]);
  let authorized = false;
  fetchListener({
    request: new Request(`${origin}/fonts/f1.woff2`, { headers: { Authorization: "Bearer example" } }),
    respondWith: () => { authorized = true; },
  });
  assert.equal(authorized, false);
});

test("activation removes old private responses from the retained build cache", async () => {
  const { listeners, entries, deletedCaches } = worker([
    `${origin}/index.html`,
    `${origin}/__cashflux_complete__.json`,
    `${origin}/fonts/f1.woff2`,
    `${origin}/v1/account/export`,
    `${origin}/v1/me`,
    `${origin}/v1/icon-192.png`,
  ]);
  let complete;
  listeners.get("activate")({ waitUntil: (promise) => { complete = promise; } });
  await complete;
  assert.deepEqual(deletedCaches, ["cashflux-fav1-old"]);
  assert.deepEqual([...entries].sort(), [
    `${origin}/__cashflux_complete__.json`,
    `${origin}/fonts/f1.woff2`,
    `${origin}/index.html`,
  ].sort());
});
