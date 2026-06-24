// CashFlux desktop — Electron main process.
//
// Wraps the existing local-first web/ build as a native desktop app: no separate
// UI codebase, the production WASM bundle is the renderer payload (§5.1). The app
// is fully offline — everything is served from the bundled web/ directory via a
// custom file:// load, so it matches the PWA's offline behavior.
const { app, BrowserWindow, Menu, shell } = require("electron");
const path = require("path");

// The renderer payload is the repo's production web/ build. When packaged,
// electron-builder includes ../web as extra resources (see package.json build.files),
// resolved relative to the app root; in dev it's one level up from desktop/.
function webRoot() {
  // Packaged: resources/web ; Dev: ../web
  const packaged = path.join(process.resourcesPath || "", "web", "index.html");
  const dev = path.join(__dirname, "..", "web", "index.html");
  return app.isPackaged ? packaged : dev;
}

function createWindow() {
  const win = new BrowserWindow({
    width: 1280,
    height: 860,
    minWidth: 720,
    minHeight: 560,
    backgroundColor: "#0e0e0f", // matches the app's dark base so there's no white flash
    title: "CashFlux",
    icon: path.join(__dirname, "icon.png"),
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
      // The WASM app needs no Node access; keep the renderer sandboxed.
      sandbox: true,
    },
  });

  win.loadFile(webRoot());

  // Open external links in the user's browser, not a new app window.
  win.webContents.setWindowOpenHandler(({ url }) => {
    if (url.startsWith("http")) {
      shell.openExternal(url);
      return { action: "deny" };
    }
    return { action: "allow" };
  });

  return win;
}

// Minimal native menu: app/edit/view/window, plus a link to the changelog. Keeps
// platform-standard shortcuts (copy/paste/zoom/devtools) without bespoke chrome.
function buildMenu() {
  const isMac = process.platform === "darwin";
  const template = [
    ...(isMac ? [{ role: "appMenu" }] : []),
    { role: "fileMenu" },
    { role: "editMenu" },
    { role: "viewMenu" },
    { role: "windowMenu" },
    {
      role: "help",
      submenu: [
        {
          label: "CashFlux Changelog",
          click: () =>
            shell.openExternal(
              "https://github.com/monstercameron/CashFlux/blob/main/CHANGELOG.md"
            ),
        },
      ],
    },
  ];
  Menu.setApplicationMenu(Menu.buildFromTemplate(template));
}

app.whenReady().then(() => {
  buildMenu();
  createWindow();

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") app.quit();
});
