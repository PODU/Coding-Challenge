/* Slide navigation and live reload in the browser.
 * The index helpers are copies of src/client/navigation.ts, which has the tests. */
(function () {
  "use strict";

  function clampIndex(i, count) {
    if (count <= 0) return 0;
    if (isNaN(i)) return 0;
    var n = Math.trunc(i); // Infinity is handled by the checks below
    if (n < 0) return 0;
    if (n > count - 1) return count - 1;
    return n;
  }
  function nextIndex(i, count) { return clampIndex(clampIndex(i, count) + 1, count); }
  function prevIndex(i, count) { return clampIndex(clampIndex(i, count) - 1, count); }

  var slides = Array.prototype.slice.call(document.querySelectorAll(".slide"));
  var count = slides.length;
  var current = 0;

  var currentEl = document.getElementById("current");
  var totalEl = document.getElementById("total");
  var prevBtn = document.getElementById("prev");
  var nextBtn = document.getElementById("next");

  function show(i) {
    current = clampIndex(i, count);
    for (var k = 0; k < slides.length; k++) {
      slides[k].classList.toggle("active", k === current);
    }
    if (currentEl) currentEl.textContent = String(current + 1);
    if (totalEl) totalEl.textContent = String(count);
    if (prevBtn) prevBtn.disabled = current === 0;
    if (nextBtn) nextBtn.disabled = current >= count - 1;
    var hash = "#" + (current + 1);
    if (location.hash !== hash) {
      history.replaceState(null, "", hash);
    }
  }

  function fromHash() {
    var n = parseInt((location.hash || "").replace(/^#/, ""), 10);
    return isNaN(n) ? 0 : clampIndex(n - 1, count);
  }

  document.addEventListener("keydown", function (e) {
    if (e.key === "ArrowRight" || e.key === "PageDown" || e.key === " ") {
      show(nextIndex(current, count));
      e.preventDefault();
    } else if (e.key === "ArrowLeft" || e.key === "PageUp") {
      show(prevIndex(current, count));
      e.preventDefault();
    } else if (e.key === "Home") {
      show(0);
    } else if (e.key === "End") {
      show(count - 1);
    }
  });

  if (prevBtn) prevBtn.addEventListener("click", function () { show(prevIndex(current, count)); });
  if (nextBtn) nextBtn.addEventListener("click", function () { show(nextIndex(current, count)); });
  window.addEventListener("hashchange", function () { show(fromHash()); });

  show(fromHash());

  /* The dev server sets this; static builds don't. */
  if (window.__MDP_LIVERELOAD__) {
    try {
      var ws = new WebSocket(window.__MDP_LIVERELOAD__);
      ws.onmessage = function (ev) {
        var msg;
        try { msg = JSON.parse(ev.data); } catch (_) { return; }
        if (msg && msg.type === "reload") location.reload();
      };
    } catch (_) { /* live reload is best-effort */ }
  }
})();
