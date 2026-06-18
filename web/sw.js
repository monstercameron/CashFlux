// CashFlux service worker: a network-first cache so the app stays fresh online
// (live-reload friendly) yet loads offline from the last successful fetch.
// Only same-origin GETs are cached; cross-origin calls (e.g. OpenAI) pass
// straight through. Bump CACHE on release to evict stale assets.
const CACHE = "cashflux-v15";
const CORE = [
  "./", "./index.html", "./wasm_exec.js", "./bin/main.wasm", "./manifest.webmanifest",
  "./chart.js", "./flip.js", "https://cdn.jsdelivr.net/npm/d3@7.9.0/dist/d3.min.js",
];

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

// appShell resolves to the cached SPA entry document, trying index.html then "./".
const appShell = () => caches.match("./index.html").then((m) => m || caches.match("./"));

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;

  // SPA navigations (e.g. hard-refresh at /accounts): client-side routes don't
  // exist as files, so a static host or offline returns 404 / fails. Serve the
  // cached app shell instead — it boots and the router resolves the path. This
  // is the SW side of deep-link refresh (the static 404.html covers first load).
  if (req.mode === "navigate") {
    event.respondWith(
      fetch(req)
        .then((resp) => {
          if (resp.ok) {
            if (new URL(req.url).origin === self.location.origin) {
              const copy = resp.clone();
              caches.open(CACHE).then((c) => c.put("./index.html", copy));
            }
            return resp;
          }
          return appShell().then((m) => m || resp);
        })
        .catch(() => appShell())
    );
    return;
  }

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
