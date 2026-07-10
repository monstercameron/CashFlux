/**
 * wonder.js — W-21 Scroll-reveal
 *
 * Observes .card elements inside #cf-page-view with an IntersectionObserver so
 * they fade + rise into view as the user scrolls. The reveal is a pure
 * enhancement: content is always visible when:
 *   • JS is disabled or this file fails to load (no .wonder-reveal class added)
 *   • [data-wonder="off"] is set on <html> (early-exit + CSS hard-override)
 *   • prefers-reduced-motion: reduce (early-exit + CSS hard-override)
 *   • IntersectionObserver is unsupported (feature-detected; all skipped silently)
 *
 * The Go side calls window.cashfluxWonder.observe() after each route change so
 * newly rendered cards on the incoming page are picked up.
 */
(function () {
  "use strict";

  /** True when motion should be completely suppressed. */
  function motionOff() {
    if (document.documentElement.getAttribute("data-wonder") === "off") return true;
    if (typeof matchMedia === "function" &&
        matchMedia("(prefers-reduced-motion: reduce)").matches) return true;
    return false;
  }

  /**
   * Reveal all .wonder-reveal elements immediately (no animation). Used when
   * motion is off or IO is unsupported so content is never left hidden.
   */
  function revealAll(root) {
    var els = (root || document).querySelectorAll(".wonder-reveal");
    for (var i = 0; i < els.length; i++) {
      els[i].classList.add("in-view");
    }
  }

  // If no IntersectionObserver, degrade gracefully: expose a no-op API.
  if (typeof IntersectionObserver === "undefined") {
    window.cashfluxWonder = { observe: function () { revealAll(); } };
    return;
  }

  var io = new IntersectionObserver(
    function (entries) {
      for (var i = 0; i < entries.length; i++) {
        var entry = entries[i];
        if (entry.isIntersecting) {
          entry.target.classList.add("in-view");
          io.unobserve(entry.target); // fire once per element
        }
      }
    },
    { threshold: 0.08 } // trigger when ≥8% of the card is visible
  );

  /**
   * observe() stamps .wonder-reveal on unprocessed .card elements inside
   * #cf-page-view, then registers them with the IntersectionObserver. Cards
   * already in the viewport are revealed synchronously so the top-of-page
   * content is never hidden even for a single frame.
   *
   * Call after each route change (Go shell invokes this via UseEffect).
   * If motion is off, reveal everything immediately and skip the observer.
   */
  function observe() {
    var pageView = document.getElementById("cf-page-view");
    if (!pageView) return;

    if (motionOff()) {
      revealAll(pageView);
      return;
    }

    var vh = window.innerHeight || document.documentElement.clientHeight || 0;
    var cards = pageView.querySelectorAll(".card:not([data-wonder-observed])");
    for (var i = 0; i < cards.length; i++) {
      var card = cards[i];
      card.setAttribute("data-wonder-observed", "1");
      card.classList.add("wonder-reveal");
      // Already in the viewport? Reveal it synchronously so above-the-fold content
      // is never gated on the async IntersectionObserver callback, which can be
      // missed on a cold deep-link load (the element is sampled once, while a
      // page-enter transform still has it out of place, and never re-checked). This
      // is the behavior this function's contract has always described.
      var r = card.getBoundingClientRect();
      if (r.top < vh && r.bottom > 0) {
        card.classList.add("in-view");
      } else {
        io.observe(card);
      }
    }
  }

  /**
   * crossFade(applyFn) — W-10 View Transitions API helper.
   *
   * Wraps applyFn in document.startViewTransition() when:
   *   • the API is available (Chrome 111+, Safari 18+)
   *   • [data-wonder] is not "off"
   *   • prefers-reduced-motion: reduce is not set
   *
   * Falls back to calling applyFn() directly when any condition is not met,
   * so callers never need to feature-detect the API themselves.
   *
   * The Go side (pageenter.go) uses this to wrap the W-9 class-toggle so the
   * browser can optionally manage the animation via its view-transition machinery.
   */
  function crossFade(applyFn) {
    var off = document.documentElement.getAttribute("data-wonder") === "off";
    var reduced = typeof matchMedia === "function" &&
        matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (!off && !reduced && typeof document.startViewTransition === "function") {
      // startViewTransition returns a ViewTransition whose .ready/.finished promises REJECT with
      // an AbortError ("Transition was skipped") when a rapid subsequent navigation starts a new
      // transition before this one finishes. That's expected during fast route-switching, but the
      // rejections otherwise surface as unhandled-promise console errors. Swallow them — the
      // applyFn (DOM swap) has already run regardless of whether the visual transition completes.
      var t = document.startViewTransition(applyFn);
      if (t) {
        var hush = function () {};
        if (t.ready && t.ready.catch) t.ready.catch(hush);
        if (t.finished && t.finished.catch) t.finished.catch(hush);
        if (t.updateCallbackDone && t.updateCallbackDone.catch) t.updateCallbackDone.catch(hush);
      }
    } else {
      applyFn();
    }
  }

  window.cashfluxWonder = { observe: observe, crossFade: crossFade };
})();
