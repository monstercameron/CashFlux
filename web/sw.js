// CashFlux service worker: a network-first cache so the app stays fresh online
// (live-reload friendly) yet loads offline from the last successful fetch.
// Only same-origin GETs are cached; cross-origin calls (e.g. OpenAI) pass
// straight through. Bump CACHE on release to evict stale assets.
const CACHE = "cashflux-v1";
const CORE = ["./", "./index.html", "./wasm_exec.js", "./bin/main.wasm", "./manifest.webmanifest"];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE).then((c) => c.addAll(CORE)).catch(() => {}).then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  event.respondWith(
    fetch(req)
      .then((resp) => {
        if (resp.ok && new URL(req.url).origin === self.location.origin) {
          const copy = resp.clone();
          caches.open(CACHE).then((c) => c.put(req, copy));
        }
        return resp;
      })
      .catch(() => caches.match(req))
  );
});
