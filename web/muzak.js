// CashFlux background music ("muzak"): a tiny looping ambient player. Plays a
// playlist of calming tracks at a low volume, advancing on end and looping. On by
// default, but browsers block autoplay until the first user gesture, so when a
// play() is rejected we arm a one-shot pointer/key listener and start then.
//
// Drop your Suno-generated tracks in web/audio/ matching DEFAULT_TRACKS (or call
// cashfluxMuzak.init([...urls]) with your own list). Missing files just stay
// silent — the toggle still works.
(function () {
  var DEFAULT_TRACKS = [
    "./audio/calm-01.mp3",
    "./audio/calm-02.mp3",
    "./audio/calm-03.mp3",
  ];
  var audio = null;
  var tracks = DEFAULT_TRACKS.slice();
  var idx = 0;
  var enabled = false;
  var vol = 0.12; // low by default
  var armed = false;

  function ensure() {
    if (audio) return;
    audio = new Audio();
    audio.preload = "none";
    audio.volume = vol;
    audio.addEventListener("ended", function () {
      if (!tracks.length) return;
      idx = (idx + 1) % tracks.length;
      audio.src = tracks[idx];
      tryPlay();
    });
    // If a source 404s or fails, skip to the next track rather than stalling.
    audio.addEventListener("error", function () {
      if (!enabled || tracks.length < 2) return;
      idx = (idx + 1) % tracks.length;
      audio.src = tracks[idx];
      tryPlay();
    });
  }

  function tryPlay() {
    if (!enabled || !tracks.length) return;
    ensure();
    if (!audio.src) audio.src = tracks[idx];
    var p = audio.play();
    if (p && p.catch) p.catch(function () { armGesture(); });
  }

  // Autoplay is blocked until the user interacts; start on the first gesture.
  function armGesture() {
    if (armed) return;
    armed = true;
    var start = function () {
      window.removeEventListener("pointerdown", start, true);
      window.removeEventListener("keydown", start, true);
      armed = false;
      if (enabled) tryPlay();
    };
    window.addEventListener("pointerdown", start, true);
    window.addEventListener("keydown", start, true);
  }

  window.cashfluxMuzak = {
    init: function (list, volume) {
      if (Array.isArray(list) && list.length) tracks = list.slice();
      if (typeof volume === "number") vol = volume;
      ensure();
      audio.volume = vol;
    },
    setEnabled: function (on) {
      enabled = !!on;
      ensure();
      if (enabled) tryPlay();
      else audio.pause();
    },
    setVolume: function (v) {
      if (typeof v !== "number") return;
      vol = Math.max(0, Math.min(1, v));
      if (audio) audio.volume = vol;
    },
    isEnabled: function () { return enabled; },
    next: function () {
      if (!tracks.length) return;
      idx = (idx + 1) % tracks.length;
      ensure();
      audio.src = tracks[idx];
      tryPlay();
    },
  };
})();
