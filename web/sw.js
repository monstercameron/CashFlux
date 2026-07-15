// CashFlux service worker: a network-first cache so the app stays fresh online
// (live-reload friendly) yet loads offline from the last successful fetch.
// Only same-origin GETs are cached; cross-origin calls (e.g. OpenAI) pass
// straight through. Bump CACHE on release to evict stale assets.
const CACHE = "cashflux-v314";
const CORE = [
  // Precache the gzip binary (the loader's primary path) — ~4× smaller than the raw
  // .wasm, so the offline install cache shrinks with it. DecompressionStream is
  // available offline, so a cached .gz boots without the network.
  "./", "./index.html", "./wasm_exec.js", "./bin/main.wasm.gz", "./manifest.webmanifest",
  "./favicon.svg", "./icon-192.png", "./icon-512.png", "./apple-touch-icon.png",
  "./chart.js", "./muzak.js", "./wonder.js", "./countup.js", "./mermaid.min.js", "./mermaid.js",
  "./marked.min.js", "./purify.min.js", "./d3.min.js",
  // Self-hosted font stylesheet (the @font-face decls for Fraunces + Inter). The
  // woff2 binaries under ./fonts/ are large (~1.4 MB across subsets); they cache via
  // the runtime fetch handler on first online load rather than bloating the install
  // precache. A never-been-online cold load falls back to system fonts gracefully.
  "./fonts.css",
];

self.addEventListener("install", (event) => {
  // Cache each core asset INDIVIDUALLY (not addAll, which is all-or-nothing: a
  // single 404 would leave the cache empty and break offline). One failed asset
  // no longer prevents the rest from being cached, so offline stays usable (L19).
  event.waitUntil(
    caches.open(CACHE)
      // C312: don't swallow precache failures silently. Log which asset failed (a
      // failed ./bin/main.wasm is the difference between an offline boot and a blank
      // page), so the gap is visible in the console / SW devtools rather than hidden.
      .then((c) => Promise.all(CORE.map((u) =>
        c.add(u).catch((e) => console.warn("[sw] precache failed:", u, e && e.message ? e.message : e))
      )))
      .catch((e) => console.warn("[sw] caches.open failed:", e && e.message ? e.message : e))
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
