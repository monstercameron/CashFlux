// CashFlux background music ("muzak"): a calming, low-volume looping player with
// crossfaded track transitions and volume fading.
//
//   * Playlist  — the internal track data structure (list + cursor, advance/shuffle).
//   * Fader     — ramps an <audio> element's volume over time (cancelable, rAF).
//   * Player    — two <audio> elements crossfaded so tracks blend instead of cut;
//                 fade-in on enable, fade-out on disable, fade-in loop on a single
//                 track. Autoplay is blocked until a gesture, so a rejected play()
//                 arms a one-shot pointer/key listener.
//
// Drop Suno-generated tracks in web/audio/ matching DEFAULT_TRACKS (or call
// cashfluxMuzak.init([...urls])). Missing files are skipped; if every track fails
// the player backs off instead of busy-looping.
(function () {
  var DEFAULT_TRACKS = ["./audio/calm-01.mp3", "./audio/calm-02.mp3", "./audio/calm-03.mp3"];
  var DEFAULT_VOL = 0.12;
  var CROSSFADE_MS = 2600; // overlap when moving between tracks
  var TOGGLE_MS = 1200; // fade-in / fade-out on enable / disable

  function clamp01(v) { return Math.max(0, Math.min(1, v)); }

  // ---- Playlist: the internal track data structure -------------------------
  function Playlist(tracks) {
    this.tracks = [];
    this.index = 0;
    this.set(tracks);
  }
  Playlist.prototype.set = function (tracks) {
    this.tracks = Array.isArray(tracks) ? tracks.slice() : [];
    if (this.index >= this.tracks.length) this.index = 0;
  };
  Playlist.prototype.size = function () { return this.tracks.length; };
  Playlist.prototype.current = function () { return this.tracks.length ? this.tracks[this.index] : null; };
  Playlist.prototype.advance = function () {
    if (!this.tracks.length) return null;
    this.index = (this.index + 1) % this.tracks.length;
    return this.current();
  };
  Playlist.prototype.shuffle = function (rand) {
    rand = rand || Math.random;
    for (var i = this.tracks.length - 1; i > 0; i--) {
      var j = Math.floor(rand() * (i + 1));
      var t = this.tracks[i]; this.tracks[i] = this.tracks[j]; this.tracks[j] = t;
    }
    this.index = 0;
  };

  // ---- Fader: cancelable volume ramp on an <audio> element -----------------
  function fade(el, to, ms, done) {
    if (!el) return;
    if (el.__fadeRaf) { cancelAnimationFrame(el.__fadeRaf); el.__fadeRaf = null; }
    to = clamp01(to);
    if (ms <= 0 || !window.requestAnimationFrame) { el.volume = to; if (done) done(); return; }
    var from = el.volume, t0 = null;
    function step(ts) {
      if (t0 === null) t0 = ts;
      var k = Math.min(1, (ts - t0) / ms);
      el.volume = clamp01(from + (to - from) * k);
      if (k < 1) el.__fadeRaf = requestAnimationFrame(step);
      else { el.__fadeRaf = null; if (done) done(); }
    }
    el.__fadeRaf = requestAnimationFrame(step);
  }

  // ---- Player --------------------------------------------------------------
  var pl = new Playlist(DEFAULT_TRACKS);
  var vol = DEFAULT_VOL;
  var enabled = false;
  var els = null; // [audioA, audioB]
  var active = 0; // index into els currently audible
  var crossing = false; // a crossfade is in progress
  var armed = false; // a gesture listener is waiting
  var errStreak = 0; // consecutive load errors → back off

  function ensureEls() {
    if (els) return;
    els = [new Audio(), new Audio()];
    for (var i = 0; i < 2; i++) {
      var el = els[i];
      el.preload = "none";
      el.volume = 0;
      el.addEventListener("timeupdate", onTimeUpdate);
      el.addEventListener("ended", onEnded);
      el.addEventListener("error", onError);
      el.addEventListener("playing", function () { errStreak = 0; });
    }
  }

  function configureLoop() {
    // A lone track loops natively (seamless); multi-track uses crossfades.
    var single = pl.size() === 1;
    els[0].loop = single;
    els[1].loop = single;
  }

  function startTrack(el, src) {
    if (!src) return;
    el.src = src;
    try { el.currentTime = 0; } catch (e) { /* not seekable yet */ }
    el.volume = 0;
    var p = el.play();
    if (p && p.catch) p.catch(armGesture);
  }

  function crossfadeTo(src) {
    if (!src) { crossing = false; return; }
    var cur = els[active], nxt = els[active ^ 1];
    startTrack(nxt, src);
    fade(nxt, vol, CROSSFADE_MS);
    fade(cur, 0, CROSSFADE_MS, function () { try { cur.pause(); } catch (e) {} });
    active ^= 1;
    crossing = false;
  }

  function onTimeUpdate(e) {
    var el = e.target;
    if (!enabled || el !== els[active] || crossing || pl.size() < 2) return;
    if (!isFinite(el.duration) || el.duration <= 0) return;
    var remain = el.duration - el.currentTime;
    if (el.currentTime > 0 && remain <= CROSSFADE_MS / 1000 + 0.05) {
      crossing = true;
      crossfadeTo(pl.advance());
    }
  }

  function onEnded(e) {
    var el = e.target;
    if (!enabled || el !== els[active]) return;
    // Reached the end without a crossfade (e.g. a track shorter than the fade, or
    // duration unknown): advance and fade the next one in on the same element.
    var src = pl.size() > 1 ? pl.advance() : pl.current();
    startTrack(el, src);
    fade(el, vol, TOGGLE_MS);
  }

  function onError() {
    if (!enabled || pl.size() < 2) return;
    errStreak++;
    if (errStreak >= pl.size()) return; // every track failed — stop hammering 404s
    crossing = false;
    startTrack(els[active], pl.advance());
    fade(els[active], vol, 400);
  }

  function armGesture() {
    if (armed) return;
    armed = true;
    var start = function () {
      window.removeEventListener("pointerdown", start, true);
      window.removeEventListener("keydown", start, true);
      armed = false;
      if (enabled) enable();
    };
    window.addEventListener("pointerdown", start, true);
    window.addEventListener("keydown", start, true);
  }

  function enable() {
    ensureEls();
    if (!pl.size()) return;
    var el = els[active];
    if (!el.src) { configureLoop(); startTrack(el, pl.current()); }
    else if (el.paused) { var p = el.play(); if (p && p.catch) p.catch(armGesture); }
    errStreak = 0;
    fade(el, vol, TOGGLE_MS);
  }

  function disable() {
    if (!els) return;
    fade(els[0], 0, TOGGLE_MS, function () { try { els[0].pause(); } catch (e) {} });
    fade(els[1], 0, TOGGLE_MS, function () { try { els[1].pause(); } catch (e) {} });
  }

  window.cashfluxMuzak = {
    init: function (list, volume) {
      if (Array.isArray(list) && list.length) pl.set(list);
      if (typeof volume === "number") vol = clamp01(volume);
      ensureEls();
    },
    setEnabled: function (on) {
      enabled = !!on;
      ensureEls();
      if (enabled) enable(); else disable();
    },
    setVolume: function (v) {
      if (typeof v !== "number") return;
      vol = clamp01(v);
      if (enabled && els) fade(els[active], vol, 300);
    },
    isEnabled: function () { return enabled; },
    next: function () {
      if (pl.size() < 2) return;
      crossing = true;
      crossfadeTo(pl.advance());
    },
    shuffle: function () { pl.shuffle(); },
    // Debug/introspection (used by tests): the live playlist + player state.
    state: function () {
      return { enabled: enabled, size: pl.size(), index: pl.index, volume: vol, crossfadeMs: CROSSFADE_MS };
    },
  };
})();
