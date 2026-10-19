// SPDX-License-Identifier: MIT

const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");

class MemoryCache {
  constructor(name, failAfter) {
    this.name = name;
    this.entries = new Map();
    this.failAfter = failAfter;
  }
  key(request) {
    return new URL(typeof request === "string" ? request : request.url, "https://cashflux.test/").href;
  }
  async match(request) {
    const response = this.entries.get(this.key(request));
    return response ? response.clone() : undefined;
  }
  async put(request, response) {
    if (this.failAfter !== undefined && this.entries.size >= this.failAfter) {
      throw new Error("simulated interrupted cache promotion");
    }
    this.entries.set(this.key(request), response.clone());
  }
  async delete(request) {
    return this.entries.delete(this.key(request));
  }
  async keys() {
    return [...this.entries.keys()].map((url) => new Request(new URL(url, "https://cashflux.test/")));
  }
}

class MemoryCacheStorage {
  constructor() {
    this.items = new Map();
    this.failNextBuildCache = false;
  }
  async open(name) {
    if (!this.items.has(name)) {
      const failAfter = name === "cashflux-new-build" && this.failNextBuildCache ? 1 : undefined;
      this.items.set(name, new MemoryCache(name, failAfter));
    }
    return this.items.get(name);
  }
  async keys() {
    return [...this.items.keys()];
  }
  async delete(name) {
    return this.items.delete(name);
  }
}

async function main() {
  const listeners = new Map();
  const caches = new MemoryCacheStorage();
  const self = {
    location: { href: "https://cashflux.test/sw.js?build=new-build", origin: "https://cashflux.test" },
    addEventListener(name, listener) { listeners.set(name, listener); },
    async skipWaiting() {},
    clients: { async claim() {}, async matchAll() { return []; } },
  };
  const context = {
    self,
    caches,
    URL,
    Request,
    Response,
    console,
    async fetch(input) {
      const url = typeof input === "string" ? input : input.url;
      if (url.endsWith("build.json")) {
        return new Response(JSON.stringify({ buildId: "new-build", schema: 1 }), {
          headers: { "Content-Type": "application/json" },
        });
      }
      const body = url.endsWith("index.html") || url.endsWith("/") ? "new shell" : "asset";
      return new Response(body, { headers: { "Content-Type": url.endsWith(".wasm") ? "application/wasm" : "text/plain" } });
    },
  };
  vm.runInNewContext(fs.readFileSync("web/sw.js", "utf8"), context, { filename: "web/sw.js" });

  const previous = await caches.open("cashflux-old-build");
  for (const key of ["./", "./index.html", "./wasm_exec.js", "./manifest.webmanifest", "./favicon.svg", "./fonts.css", "./bin/main.wasm"]) {
    await previous.put(key, new Response("old shell"));
  }
  await previous.put("./__cashflux_complete__.json", new Response(JSON.stringify({ buildId: "old-build", complete: true })));

  caches.failNextBuildCache = true;
  let installPromise;
  listeners.get("install")({ waitUntil(promise) { installPromise = promise; } });
  await assert.rejects(installPromise, /simulated interrupted cache promotion/);

  const staged = await caches.open("cashflux-new-build-staging");
  assert.ok((await staged.keys()).length > 0, "failed install should leave staging data for the test");
  const partial = await caches.open("cashflux-new-build");
  assert.equal(await partial.match("./__cashflux_complete__.json"), undefined, "partial cache must have no completion marker");

  context.fetch = async () => { throw new Error("offline"); };
  let navigation;
  listeners.get("fetch")({
    request: { url: "https://cashflux.test/deep-link", method: "GET", mode: "navigate" },
    respondWith(promise) { navigation = promise; },
  });
  const offlineShell = await navigation;
  assert.equal(await offlineShell.text(), "old shell", "offline boot must use the last complete cache");
  console.log("PASS: incomplete staging/promotion is ignored; offline boot uses the prior complete cache");
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
