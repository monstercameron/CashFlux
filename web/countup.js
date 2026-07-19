/**
 * countup.js — W-15 WONDER flourish: count-up animation for KPI stat figures.
 *
 * Public API:
 *   window.cashfluxCountUpScan()
 *     Finds all [data-countup] elements that haven't yet been animated to their
 *     current text value and runs a count-up tween on each one.
 *
 * Safety contract:
 *   - The FINAL displayed value is ALWAYS the exact original textContent.
 *   - If text isn't parseable as a number the element is left untouched.
 *   - Gated by --wonder-on (0 = off) and prefers-reduced-motion.
 *   - Tracks last-animated value in data-countup-last so re-renders with the
 *     same value don't re-trigger, and genuine value changes do re-animate.
 */
(function () {
  'use strict';

  /**
   * Parse the numeric value out of a formatted money/number string.
   * Returns { value: number, prefix: string, suffix: string, decimals: number }
   * or null if the string doesn't contain a parseable number.
   *
   * Handles: "$1,234.56", "-$1,234.56", "($1,234.56)", "1 234,56", "€-12.00",
   *          plain integers, negative amounts in parentheses (accounting style).
   */
  function parse(text) {
    var raw = text.trim();
    if (!raw) return null;

    // Detect accounting-negative: parentheses wrapping e.g. ($1,234.56)
    var accountingNeg = /^\((.+)\)$/.test(raw);
    if (accountingNeg) {
      raw = raw.slice(1, -1);
    }

    // Split into: optional leading non-digit/non-minus/non-dot prefix,
    // the numeric core, optional trailing suffix.
    // Regex: (prefix)(sign?)(digits/separators/decimal)(suffix)
    var m = raw.match(/^([^0-9\-]*)(\-?)([0-9][0-9 ,.]*)([^0-9]*)$/);
    if (!m) return null;

    var prefix = m[1];
    var sign = m[2];
    var numStr = m[3];
    var suffix = m[4];

    // Determine decimal separator: if there's a '.' followed by exactly 2 digits
    // at the end assume it's the decimal point; a ',' in that position means
    // European format where comma is decimal.
    var decimals = 0;
    var normalized;
    var dotIdx = numStr.lastIndexOf('.');
    var commaIdx = numStr.lastIndexOf(',');

    if (dotIdx > commaIdx) {
      // dot is decimal separator (e.g. 1,234.56)
      decimals = numStr.length - dotIdx - 1;
      normalized = numStr.replace(/,/g, '').replace('.', '.');
    } else if (commaIdx > dotIdx) {
      // comma is decimal separator (e.g. 1.234,56)
      decimals = numStr.length - commaIdx - 1;
      normalized = numStr.replace(/\./g, '').replace(',', '.');
    } else {
      // no decimal separator
      normalized = numStr.replace(/[, ]/g, '');
    }

    var value = parseFloat(normalized);
    if (isNaN(value)) return null;

    if (sign === '-' || accountingNeg) value = -value;

    return {
      value: value,
      prefix: prefix,
      sign: sign,
      suffix: suffix,
      decimals: decimals,
      accountingNeg: accountingNeg,
    };
  }

  /**
   * Format a numeric value back into the same style as the original parsed info.
   * This is a best-effort mid-tween approximation; the final frame always
   * restores the exact original string.
   */
  function format(value, info) {
    var abs = Math.abs(value);
    var negative = value < 0;

    // Format with the right number of decimal places
    var numStr = abs.toFixed(info.decimals);

    // Add thousands separators (always comma for simplicity — mid-tween only)
    var parts = numStr.split('.');
    parts[0] = parts[0].replace(/\B(?=(\d{3})+(?!\d))/g, ',');
    var formatted = parts.join('.');

    if (info.accountingNeg && negative) {
      return '(' + info.prefix + formatted + info.suffix + ')';
    }
    var signStr = negative ? '-' : '';
    return signStr + info.prefix + formatted + info.suffix;
  }

  /**
   * Ease-out cubic easing function.
   */
  function easeOut(t) {
    return 1 - Math.pow(1 - t, 3);
  }

  /**
   * Animate a single element count-up.
   * @param {Element} el
   * @param {string} finalText - the exact original textContent to restore at end
   * @param {number} durationMs
   */
  function animateEl(el, finalText, durationMs) {
    var info = parse(finalText);
    if (!info) return; // not a number — leave untouched

    var target = info.value;
    // Start from 0 (or a sensible non-confusing start for negatives)
    var start = 0;

    var startTime = null;

    // Cancellation token: a re-render can REUSE this DOM node as a different
    // element (the framework reconciler morphs nodes in place) or a newer scan
    // can start a fresh tween on it. Writing el.textContent from a stale
    // animation would then WIPE the node's new children (observed: a KPI's
    // .fig/caption replaced by bare text after switching builder presets).
    // The newest animation owns the node; anything else must stop silently.
    var token = {};
    el.__cfCountup = token;

    function step(ts) {
      if (el.__cfCountup !== token || !el.isConnected || !el.hasAttribute('data-countup')) {
        return; // superseded, detached, or morphed into a non-countup node
      }
      if (startTime === null) startTime = ts;
      var elapsed = ts - startTime;
      var progress = Math.min(elapsed / durationMs, 1);
      var eased = easeOut(progress);
      var current = start + (target - start) * eased;

      if (progress < 1) {
        el.textContent = format(current, info);
        requestAnimationFrame(step);
      } else {
        // Always restore exact original text at the end
        el.textContent = finalText;
        el.setAttribute('data-countup-last', finalText);
      }
    }

    requestAnimationFrame(step);
  }

  /**
   * Check whether WONDER animations are enabled.
   * Returns false if --wonder-on is 0 or prefers-reduced-motion is set.
   */
  function wonderEnabled() {
    // Check prefers-reduced-motion first (cheapest)
    if (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      return false;
    }
    // Check --wonder-on CSS variable
    var val = getComputedStyle(document.documentElement)
      .getPropertyValue('--wonder-on')
      .trim();
    var num = parseFloat(val);
    return !isNaN(num) && num > 0;
  }

  /**
   * Scan all [data-countup] elements and animate those whose value has changed
   * since last animation (or that have never been animated).
   */
  function cashfluxCountUpScan() {
    var els = document.querySelectorAll('[data-countup]');
    if (!els.length) return;

    var enabled = wonderEnabled();

    var durMs = 320; // fallback = the v1.2.3 Data token
    if (enabled) {
      // Read --motion-data from CSS (v1.2.3 spec: totals animate at the 320ms
      // Data token; e.g. "320ms" or "0.32s")
      var durStr = getComputedStyle(document.documentElement)
        .getPropertyValue('--motion-data')
        .trim();
      if (durStr) {
        if (durStr.endsWith('ms')) {
          durMs = parseFloat(durStr) || durMs;
        } else if (durStr.endsWith('s')) {
          durMs = (parseFloat(durStr) || 0.32) * 1000;
        }
      }
    }

    for (var i = 0; i < els.length; i++) {
      var el = els[i];
      var finalText = el.textContent;
      var lastAnimated = el.getAttribute('data-countup-last');

      // Skip if value hasn't changed since last animation
      if (lastAnimated === finalText) continue;

      if (!enabled || durMs <= 0) {
        // Off / reduced-motion: just record and leave value as-is
        el.setAttribute('data-countup-last', finalText);
        continue;
      }

      animateEl(el, finalText, durMs);
    }
  }

  window.cashfluxCountUpScan = cashfluxCountUpScan;
})();
