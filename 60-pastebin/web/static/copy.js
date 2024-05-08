// Copy-to-clipboard button for the paste content.
(function () {
  "use strict";
  document.addEventListener("click", function (ev) {
    var btn = ev.target.closest("[data-copy-target]");
    if (!btn) return;
    var sel = btn.getAttribute("data-copy-target");
    var el = document.querySelector(sel);
    if (!el) return;
    var text = el.innerText !== undefined ? el.innerText : el.textContent;
    var done = function () {
      var original = btn.textContent;
      btn.textContent = "Copied!";
      setTimeout(function () { btn.textContent = original; }, 1500);
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, fallback);
    } else {
      fallback();
    }
    function fallback() {
      var ta = document.createElement("textarea");
      ta.value = text;
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      try { document.execCommand("copy"); done(); } catch (e) { /* ignore */ }
      document.body.removeChild(ta);
    }
  });
})();
