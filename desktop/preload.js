// CashFlux desktop — preload (runs before the renderer, in an isolated context).
//
// The WASM app is fully self-contained and needs no privileged bridge, so this
// preload deliberately exposes nothing — it exists to (a) satisfy the sandboxed
// webPreferences.preload contract and (b) tag the environment so the renderer can
// detect it's running in the desktop shell (e.g. to hide PWA "install" prompts).
const { contextBridge } = require("electron");

contextBridge.exposeInMainWorld("cashfluxDesktop", {
  isDesktop: true,
  platform: process.platform,
});
