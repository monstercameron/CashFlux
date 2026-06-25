// CashFlux service worker: a network-first cache so the app stays fresh online
// (live-reload friendly) yet loads offline from the last successful fetch.
// Only same-origin GETs are cached; cross-origin calls (e.g. OpenAI) pass
// straight through. Bump CACHE on release to evict stale assets.
const CACHE = "cashflux-v268";
const CORE = [
  "./", "./index.html", "./wasm_exec.js", "./bin/main.wasm", "./manifest.webmanifest",
  "./chart.js", "./flip.js", "./muzak.js", "./wonder.js", "./countup.js", "./mermaid.min.js", "./mermaid.js",
  "./marked.min.js", "./purify.min.js", "./d3.min.js",
];

self.addEventListener("install", (event) => {
  // Cache each core asset INDIVIDUALLY (not addAll, which is all-or-nothing: a
  // single 404 would leave the cache empty and break offline). One failed asset
  // no longer prevents the rest from being cached, so offline stays usable (L19).
  event.waitUntil(
    caches.open(CACHE)
      .then((c) => Promise.all(CORE.map((u) => c.add(u).catch(() => {}))))
      .catch(() => {})
      .then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

// A guaranteed Response so respondWith() is NEVER handed undefined (which throws
// "Failed to convert value to 'Response'") and never a rejected promise (ERR_FAILED).
const offlineResp = () => new Response("", { status: 504, statusText: "offline" });

// appShell always resolves to a real Response: the cached SPA entry document, then a
// network fetch of it, then a synthetic 504. Each step is guarded so it can't reject.
async function appShell() {
  try {
    const cached = (await caches.match("./index.html")) || (await caches.match("./"));
    if (cached) return cached;
  } catch (e) {}
  try {
    const net = await fetch("./index.html");
    if (net) return net;
  } catch (e) {}
  return offlineResp();
}

// handleNavigate serves real files, else the SPA shell — so a deep link / refresh at
// /p/<slug> works on a static host with no history fallback (e.g. gwc dev returns 404
// for the route). It can never reject: every branch resolves to a Response.
async function handleNavigate(req) {
  try {
    const resp = await fetch(req, { cache: "no-store" });
    if (resp && resp.ok) {
      if (new URL(req.url).origin === self.location.origin) {
        try { (await caches.open(CACHE)).put("./index.html", resp.clone()); } catch (e) {}
      }
      return resp;
    }
  } catch (e) {}
  return appShell(); // 404/offline on a deep link → serve the shell, not the error
}

// handleAsset is network-first with a cache fallback; never returns undefined.
async function handleAsset(req, sameOrigin) {
  try {
    const resp = await fetch(req, sameOrigin ? { cache: "no-store" } : undefined);
    if (resp) {
      if (resp.ok && sameOrigin) {
        try { (await caches.open(CACHE)).put(req, resp.clone()); } catch (e) {}
      }
      return resp;
    }
  } catch (e) {}
  try {
    const cached = await caches.match(req);
    if (cached) return cached;
  } catch (e) {}
  return offlineResp(); // e.g. favicon.ico on a server that has none
}

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  if (req.mode === "navigate") {
    event.respondWith(handleNavigate(req));
    return;
  }
  const sameOrigin = new URL(req.url).origin === self.location.origin;
  event.respondWith(handleAsset(req, sameOrigin));
});
