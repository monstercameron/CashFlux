# CashFlux background music (muzak)

Drop your generated tracks here as MP3s with these exact names (the player loops
through them in order):

- `calm-01.mp3`
- `calm-02.mp3`
- `calm-03.mp3`

Want a different playlist? Edit `DEFAULT_TRACKS` in `web/muzak.js` (or call
`cashfluxMuzak.init([...urls])`).

Notes:
- Music is **on by default at low volume (0.12)** and toggled from the ♪ button in
  the top bar. The choice persists in `localStorage` (`cashflux:muzak`).
- Browsers block autoplay until the first click/keypress, so playback starts on
  your first interaction with the page.
- Keep files reasonably small (these aren't precached by the service worker).
