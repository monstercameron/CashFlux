// CashFlux service worker (SV7a): one cache per stamped build, filled
// atomically, activated only when the user accepts the update.
//
// How a build is identified:
//   - index.html fetches ./build.json with cache:"no-store" and registers
//     this worker as ./sw.js?build=<buildId>. A new deploy therefore
//     registers a NEW script URL, which is what makes the browser install
//     a new worker at all (an unchanged sw.js body never re-installs).
//   - At install the worker also fetches ./build.json itself
//     (cache:"no-store") and prefers it when no ?build= was given. The URL
//     parameter wins when both exist: it is the one value this worker can
//     still read after a restart, so the cache name it derives can never
//     drift over the worker's life.
//   - No build.json (gwc dev, plain static serving) means build "dev":
//     precache stays tolerant per-asset and the worker activates
//     immediately, exactly as live reload expects.
//
// Update flow for stamped builds: install does NOT skipWaiting. The new
// worker waits, index.html notices (registration.waiting / updatefound),
// the wasm app shows "An update is ready — Reload", and only the user's
// click posts {type:"SKIP_WAITING"} here. Activation then claims clients
// and the page reloads once on controllerchange. An old tab is never
// silently swapped onto new code mid-session, and never left believing it
// is current either: it keeps being told an update is waiting.
//
// Only same-origin GETs are cached; cross-origin calls (e.g. OpenAI) pass
// straight through. build.json is never cached or intercepted: every
// freshness decision depends on reading the server's current copy.

const CACHE_PREFIX = "cashflux-fav1-";
const LEGACY_CACHE_PREFIX = "cashflux-";
const DEV_BUILD = "dev";
const COMPLETE_ENTRY = "./__cashflux_complete__.json";

// Boot-critical assets. For a stamped build these precache atomically: all
// of them land in the new cache, or the install fails and the previous
// build's worker and cache keep serving a consistent old app. A partial
// shell — new index.html over an old wasm — must never become the offline
// boot path.
const CORE_REQUIRED = [
  "./", "./index.html", "./wasm_exec.js", "./manifest.webmanifest",
  "./favicon.ico", "./favicon.svg", "./favicon-32.png", "./fonts.css",
];
// The wasm binary is boot-critical too. Prefer the loader's negotiated URL:
// the browser transparently decodes its Content-Encoding, so that response
// is also suitable for offline instantiation. Static hosts may instead have
// only .gz for the loader's DecompressionStream fallback. At least one must land.
const CORE_WASM_ALTERNATIVES = ["./bin/main.wasm", "./bin/main.wasm.gz"];
// Everything else the old worker precached: cached best-effort into the
// same build cache, failures logged. A missing chart library degrades one
// view offline; it must not block the whole update.
const CORE_OPTIONAL = [
  "./services-worker.js", "./bin/services.wasm.gz", "./bin/services.wasm",
  "./icon-192.png", "./icon-512.png", "./icon-maskable-192.png",
  "./icon-maskable-512.png", "./apple-touch-icon.png", "./favicon-16.png",
  "./chart.js", "./muzak.js", "./wonder.js", "./countup.js", "./mermaid.min.js", "./mermaid.js",
  "./marked.min.js", "./purify.min.js", "./d3.min.js",
];
const CORE_ALL = new Set([...CORE_REQUIRED, ...CORE_WASM_ALTERNATIVES, ...CORE_OPTIONAL]);

// sanitize mirrors internal/buildinfo.Sanitize so both sides derive the
// same cache name from the same build ID.
function sanitize(s) {
  return String(s || "").replace(/[^A-Za-z0-9._-]/g, "-");
}

function cacheNameFor(buildId) {
  return CACHE_PREFIX + (sanitize(buildId) || DEV_BUILD);
}

function stagingCacheNameFor(buildId) {
  return cacheNameFor(buildId) + "-staging";
}

async function cacheIsComplete(cache, buildId) {
  try {
    const marker = await cache.match(COMPLETE_ENTRY);
    if (!marker) return false;
    const info = await marker.json();
    if (!info || info.buildId !== buildId || info.complete !== true) return false;
    for (const url of CORE_REQUIRED) {
      if (!(await cache.match(url))) return false;
    }
    for (const url of CORE_WASM_ALTERNATIVES) {
      if (await cache.match(url)) return true;
    }
    return false;
  } catch (e) {
    return false;
  }
}

async function findCompleteCache() {
  try {
    const keys = await caches.keys();
    for (const key of keys) {
      if (!key.startsWith(CACHE_PREFIX) || key.endsWith("-staging")) continue;
      const buildId = key.slice(CACHE_PREFIX.length);
      const cache = await caches.open(key);
      if (await cacheIsComplete(cache, buildId)) return cache;
    }
  } catch (e) {}
  return null;
}

// buildIdFromURL reads the ?build= this worker was registered with. It is
// available in every event for the worker's whole life, restarts included.
function buildIdFromURL() {
  try {
    return new URL(self.location.href).searchParams.get("build") || "";
  } catch (e) {
    return "";
  }
}

// fetchBuildJSON reads the server's current build identity, bypassing all
// caches. Resolves to null when there is no readable build.json (offline,
// dev serving, an auth gate answering HTML).
async function fetchBuildJSON() {
  try {
    const resp = await fetch("./build.json", { cache: "no-store", credentials: "same-origin" });
    if (!resp.ok) return null;
    const type = resp.headers.get("content-type") || "";
    if (type.includes("text/html")) return null;
    const info = await resp.json();
    if (info && typeof info.buildId === "string" && info.buildId.trim() !== "") return info;
  } catch (e) {}
  return null;
}

let resolvedBuildId = "";

// resolveBuildId settles this worker's build identity once per worker
// lifetime: URL parameter first (restart-stable), then build.json, then
// an already-existing build cache (a restarted parameterless worker),
// then "dev".
async function resolveBuildId() {
  if (resolvedBuildId) return resolvedBuildId;
  const fromURL = buildIdFromURL();
  if (fromURL) {
    resolvedBuildId = fromURL;
    return resolvedBuildId;
  }
  const info = await fetchBuildJSON();
  if (info) {
    resolvedBuildId = info.buildId;
    return resolvedBuildId;
  }
  try {
    const keys = await caches.keys();
    const mine = keys.filter((k) => k.startsWith(CACHE_PREFIX) && !k.endsWith("-staging"));
    for (const key of mine) {
      const candidate = key.slice(CACHE_PREFIX.length);
      if (await cacheIsComplete(await caches.open(key), candidate)) {
        resolvedBuildId = candidate;
        return resolvedBuildId;
      }
    }
  } catch (e) {}
  resolvedBuildId = DEV_BUILD;
  return resolvedBuildId;
}

async function ownCache() {
  const buildId = await resolveBuildId();
  if (buildId === DEV_BUILD) return caches.open(cacheNameFor(buildId));
  const cache = await caches.open(cacheNameFor(buildId));
  if (await cacheIsComplete(cache, buildId)) return cache;
  return findCompleteCache();
}

// fetchCoreAsset fetches one precache asset and returns the Response ONLY
// if it really is that asset. The status check alone is not enough: a
// login/interstitial page answering 200 with HTML for "./bin/main.wasm.gz"
// would cache a web page under the wasm's URL, and the next boot would try
// to instantiate HTML as WebAssembly and hang. That is not hypothetical —
// a host gate answering every unauthenticated request with its lock page
// did exactly this. So an HTML body for a non-document asset is a failure.
async function fetchCoreAsset(url) {
  const resp = await fetch(url, { credentials: "same-origin", cache: "no-store" });
  if (!resp.ok) throw new Error("status " + resp.status);
  const type = resp.headers.get("content-type") || "";
  const isDocument = url === "./" || url.endsWith(".html");
  if (!isDocument && type.includes("text/html")) {
    throw new Error("got HTML for a non-document asset (auth gate or error page?)");
  }
  return resp;
}

// precacheStamped fills a per-build staging cache, then promotes its entries
// into the build cache and writes a completion manifest last. CacheStorage
// has no multi-entry transaction, so an interrupted promotion has no marker
// and is never eligible for offline boot. The prior complete cache remains
// untouched until this worker activates.
async function precacheStamped(buildId) {
  const fetched = await Promise.all(CORE_REQUIRED.map(async (url) => [url, await fetchCoreAsset(url)]));
  let wasm = null;
  for (const url of CORE_WASM_ALTERNATIVES) {
    try {
      wasm = [url, await fetchCoreAsset(url)];
      break;
    } catch (e) {
      console.warn("[sw] wasm precache candidate failed:", url, e && e.message ? e.message : e);
    }
  }
  if (!wasm) throw new Error("no wasm binary could be precached");
  fetched.push(wasm);

  const staging = await caches.open(stagingCacheNameFor(buildId));
  for (const key of await staging.keys()) await staging.delete(key);
  for (const [url, resp] of fetched) {
    await staging.put(url, resp);
  }
  await Promise.all(CORE_OPTIONAL.map(async (url) => {
    try {
      await staging.put(url, await fetchCoreAsset(url));
    } catch (e) {
      console.warn("[sw] optional precache failed:", url, e && e.message ? e.message : e);
    }
  }));
  const required = [...CORE_REQUIRED, wasm[0]];
  for (const url of required) {
    if (!(await staging.match(url))) throw new Error("required staged asset is missing: " + url);
  }

  const cache = await caches.open(cacheNameFor(buildId));
  await cache.delete(COMPLETE_ENTRY);
  for (const request of await staging.keys()) {
    const response = await staging.match(request);
    if (!response) throw new Error("staged asset disappeared: " + request.url);
    await cache.put(request, response);
  }
  for (const url of required) {
    if (!(await cache.match(url))) throw new Error("required promoted asset is missing: " + url);
  }
  await cache.put(COMPLETE_ENTRY, new Response(JSON.stringify({ buildId, complete: true }), {
    headers: { "Content-Type": "application/json" },
  }));
}

// precacheDev is the pre-SV7a tolerant behaviour for unstamped serving
// (gwc dev): each asset lands if it can, gaps are logged, nothing is
// fatal. Dev servers legitimately lack the .gz siblings, and blocking
// live reload on them would be strictly worse than a thin offline cache.
async function precacheDev(cache) {
  const all = [...CORE_REQUIRED, ...CORE_WASM_ALTERNATIVES, ...CORE_OPTIONAL];
  await Promise.all(all.map(async (url) => {
    try {
      await cache.put(url, await fetchCoreAsset(url));
    } catch (e) {
      console.warn("[sw] precache failed:", url, e && e.message ? e.message : e);
    }
  }));
}

self.addEventListener("install", (event) => {
  event.waitUntil((async () => {
    // Re-read build.json at install (SV7a): if a deploy landed between the
    // page's fetch and this install, the fetched ID is used when the URL
    // carried none, and a mismatch with the URL is logged — this worker
    // is already superseded and the next page load will register the
    // newer script URL.
    const info = await fetchBuildJSON();
    const fromURL = buildIdFromURL();
    if (fromURL && (!info || info.buildId !== fromURL)) {
      // This registration describes a build that is no longer the one the
      // server publishes. Installing it against the current in-place asset
      // URLs could label a mixed deployment with the wrong cache ID.
      throw new Error("build.json changed during install for " + fromURL);
    }
    const buildId = fromURL || (info && info.buildId) || DEV_BUILD;
    resolvedBuildId = buildId;

    if (buildId === DEV_BUILD) {
      await precacheDev(await caches.open(cacheNameFor(buildId)));
      await self.skipWaiting(); // live reload: never strand a dev tab on a waiting worker
      return;
    }
    // Stamped build: atomic precache or no install. A rejection here marks
    // this worker redundant and leaves the previous build fully in
    // control — the user keeps a consistent old app instead of a mixed
    // new one. Deliberately NO skipWaiting: activation waits for the
    // user's Reload click (see the message handler).
    try {
      await precacheStamped(buildId);
    } catch (error) {
      // A partial staging or promoted cache has no completion manifest and
      // is ignored by offline boot. Keep the previous complete cache intact.
      throw error;
    }
  })());
});

self.addEventListener("activate", (event) => {
  event.waitUntil((async () => {
    const buildId = await resolveBuildId();
    const keep = cacheNameFor(buildId);
    const keys = await caches.keys();
    // Only this app's caches are evicted. Caches are origin-wide: other
    // apps on the same origin keep theirs.
    await Promise.all(
      keys.filter((k) => (k.startsWith(CACHE_PREFIX) || k.startsWith(LEGACY_CACHE_PREFIX)) && k !== keep)
        .map((k) => caches.delete(k))
    );
    await self.clients.claim();
    // Tell open pages which build now controls them; pages that asked for
    // the update reload on controllerchange, the rest learn they are stale.
    const clients = await self.clients.matchAll({ type: "window" });
    for (const client of clients) {
      client.postMessage({ type: "SW_ACTIVATED", buildId: buildId });
    }
  })());
});

// The page posts SKIP_WAITING only from the user's Reload click. Accept
// the object and bare-string shapes so older cached pages can still apply
// an update.
self.addEventListener("message", (event) => {
  const data = event.data;
  if (data === "SKIP_WAITING" || (data && data.type === "SKIP_WAITING")) {
    event.waitUntil(self.skipWaiting());
  }
});

// A guaranteed Response so respondWith() is NEVER handed undefined (which throws
// "Failed to convert value to 'Response'") and never a rejected promise (ERR_FAILED).
const offlineResp = () => new Response("", { status: 504, statusText: "offline" });

// appShell always resolves to a real Response: THIS build's cached SPA
// entry document, then a network fetch of it, then a synthetic 504. It
// never borrows another build's shell from a neighbouring cache.
async function appShell() {
  try {
    const cache = await ownCache();
    const cached = (await cache.match("./index.html")) || (await cache.match("./"));
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
  const buildId = await resolveBuildId();
  if (buildId !== DEV_BUILD) {
    // A stamped worker always serves its verified shell. Returning the
    // just-deployed network document here could pair its loader with this
    // worker's older wasm while the new worker is still waiting.
    try {
      const cache = await ownCache();
      const cached = (await cache.match("./index.html")) || (await cache.match("./"));
      if (cached) return cached;
    } catch (e) {}
    return offlineResp();
  }
  try {
    const resp = await fetch(req, { cache: "no-store" });
    if (resp && resp.ok) return resp;
  } catch (e) {}
  return appShell(); // 404/offline on a deep link → serve the shell, not the error
}

// coreKeyFor maps a request URL to its precache key when it names a core
// asset, so the runtime handler never overwrites the verified precache
// set with whatever the network happens to serve mid-deploy.
function coreKeyFor(url) {
  const path = new URL(url).pathname;
  for (const key of CORE_ALL) {
    if (key !== "./" && path.endsWith(key.slice(1))) return key;
  }
  return "";
}

// handleAsset is network-first with a same-build cache fallback; never returns
// undefined. Non-core same-origin responses (font files, images) runtime-cache
// into this build's cache; core assets are served from the network online and
// from the precache offline, but never re-written at runtime.
async function handleAsset(req, sameOrigin) {
  const key = sameOrigin ? coreKeyFor(req.url) : "";
  if (key && (await resolveBuildId()) !== DEV_BUILD) {
    // Core assets are an immutable set for a stamped worker. This keeps an
    // online deploy from replacing only the wasm or loader underneath an
    // already-open build; a new worker owns the new set after Reload.
    try {
      const cached = await (await ownCache()).match(key);
      if (cached) return cached;
    } catch (e) {}
  }
  try {
    const resp = await fetch(req, sameOrigin ? { cache: "no-store" } : undefined);
    if (resp) {
      if (resp.ok && sameOrigin && !coreKeyFor(req.url)) {
        try { (await ownCache()).put(req, resp.clone()); } catch (e) {}
      }
      return resp;
    }
  } catch (e) {}
  try {
    const cache = await ownCache();
    const cached = (key && await cache.match(key)) || (await cache.match(req));
    if (cached) return cached;
  } catch (e) {}
  return offlineResp(); // e.g. favicon.ico on a server that has none
}

// isBuildJSON: build.json is the freshness oracle. It is never intercepted,
// never cached, never served stale — the request bypasses this worker so
// the page's own cache:"no-store" fetch decides.
function isBuildJSON(url) {
  return new URL(url).pathname.endsWith("/build.json");
}

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return;
  if (isBuildJSON(req.url)) return;
  if (req.mode === "navigate") {
    event.respondWith(handleNavigate(req));
    return;
  }
  const sameOrigin = new URL(req.url).origin === self.location.origin;
  event.respondWith(handleAsset(req, sameOrigin));
});
