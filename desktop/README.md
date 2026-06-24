# CashFlux Desktop (Electron)

A native desktop wrapper around the existing local-first `web/` WASM build. There
is **no separate UI codebase** — the production `web/` directory is the renderer
payload, so the desktop app matches the web/PWA experience exactly and runs fully
offline.

## Architecture
- `main.js` — Electron main process: creates the `BrowserWindow`, loads
  `web/index.html` (dev: `../web`; packaged: bundled `resources/web`), sets a
  minimal native menu, and routes external links to the system browser.
- `preload.js` — sandboxed bridge that exposes only `window.cashfluxDesktop`
  (`isDesktop`, `platform`) so the renderer can hide PWA install prompts in-app.
- `package.json` — Electron + `electron-builder` config with per-OS targets.

## Develop / run
```bash
cd desktop
npm install
npm start          # rebuilds main.wasm from Go source, then launches Electron
```
`npm start` runs `build:wasm` first (`GOOS=js GOARCH=wasm go build`), so the
desktop renderer always uses a fresh build of the same Go source — never a
hand-copied artifact.

## Package installers
```bash
npm run dist        # current OS
npm run dist:win    # Windows NSIS installer
npm run dist:mac    # macOS .dmg
npm run dist:linux  # Linux AppImage + .deb
```
Each `dist` script rebuilds the wasm first, then runs `electron-builder` with the
config in `package.json` → `build`. macOS targets must be built on macOS.

## Remaining
- **`icon.png`** — add a 512×512 app icon here (referenced by `main.js` and the
  builder config). Until then Electron falls back to its default icon.
- **Verify** — install + launch the produced artifact on each OS and confirm
  offline load + PWA parity (requires the Electron/`electron-builder` toolchain).
