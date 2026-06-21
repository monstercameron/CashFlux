# CashFlux background music (muzak)

The calming playlist (`calm-01.mp3` … `calm-08.mp3`) the player loops through and
crossfades between. To change the set, edit `DEFAULT_TRACKS` in `web/muzak.js` (or
call `cashfluxMuzak.init([...urls])`).

Notes:
- Music is **on by default at low volume (0.12)** and toggled from the speaker
  button in the top bar (next to the + Add menu). The choice persists in
  `localStorage` (`cashflux:muzak`).
- Browsers block autoplay until the first click/keypress, so playback starts on
  your first interaction with the page.
- Tracks crossfade into each other near the end (see `CROSSFADE_MS` in muzak.js).
- These files are not precached by the service worker (they're large); they stream
  on demand.
