// Progressive enhancement for OpenIntern pages. Everything here is optional:
// with JavaScript off the filters submit as a normal GET form, the apply
// picker is a <details>, and the Applied and Copy buttons stay hidden.
(function () {
  "use strict";

  // Theme: follows the system until the visitor picks one; the choice is kept
  // in this browser. The inline script in <head> applies it before paint.
  (function () {
    var btn = document.querySelector("[data-theme-toggle]");
    if (!btn) return;
    var KEY = "openintern:theme";
    var root = document.documentElement;
    var media = window.matchMedia ? window.matchMedia("(prefers-color-scheme: dark)") : null;
    function current() {
      return root.dataset.theme || (media && media.matches ? "dark" : "light");
    }
    function label() {
      var next = current() === "dark" ? "light" : "dark";
      btn.textContent = next === "dark" ? "Dark" : "Light";
      btn.setAttribute("aria-label", "Switch to " + next + " theme");
    }
    btn.addEventListener("click", function () {
      var next = current() === "dark" ? "light" : "dark";
      root.dataset.theme = next;
      try { localStorage.setItem(KEY, next); } catch (e) {}
      label();
    });
    if (media && media.addEventListener) media.addEventListener("change", label);
    label();
    btn.hidden = false;
  })();

  var STORAGE_KEY = "openintern:applied";
  var HIDE_KEY = "openintern:hide-applied";

  function aliases() {
    var el = document.getElementById("job-aliases");
    if (!el) return {};
    try {
      return JSON.parse(el.textContent) || {};
    } catch (e) {
      return {};
    }
  }

  // Applied marks live only in this browser. Jobs replaced after a company
  // merge keep their mark: old ids are rewritten to the new ones on read.
  function readApplied() {
    try {
      var raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return [];
      var parsed = JSON.parse(raw);
      if (!Array.isArray(parsed)) return [];
      var ids = parsed.filter(function (x) { return typeof x === "string"; });
      var map = aliases();
      var moved = false;
      var out = [];
      ids.forEach(function (id) {
        var next = map[id] || id;
        if (next !== id) moved = true;
        if (out.indexOf(next) === -1) out.push(next);
      });
      if (moved) localStorage.setItem(STORAGE_KEY, JSON.stringify(out));
      return moved ? out : ids;
    } catch (e) {
      return [];
    }
  }

  function writeApplied(ids) {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(ids));
    } catch (e) {}
  }

  function hideApplied() {
    try {
      return localStorage.getItem(HIDE_KEY) === "true";
    } catch (e) {
      return false;
    }
  }

  function paintApplied() {
    var ids = readApplied();
    document.querySelectorAll("[data-applied]").forEach(function (btn) {
      var on = ids.indexOf(btn.getAttribute("data-applied")) !== -1;
      var title = btn.getAttribute("data-title");
      btn.hidden = false;
      btn.setAttribute("aria-pressed", on ? "true" : "false");
      btn.textContent = on ? "Applied" : "Mark applied";
      btn.setAttribute("aria-label", (on ? "Mark not applied" : "Mark applied") + (title ? ": " + title : ""));
    });
    var hide = hideApplied();
    var total = 0;
    var shown = 0;
    document.querySelectorAll("[data-job-ids]").forEach(function (row) {
      var jobIds = row.getAttribute("data-job-ids").split(" ").filter(Boolean);
      var all = jobIds.length > 0 && jobIds.every(function (id) { return ids.indexOf(id) !== -1; });
      row.classList.toggle("is-applied", all);
      if (row.tagName === "LI") {
        total++;
        row.hidden = hide && all;
        if (!row.hidden) shown++;
      }
    });
    var toggle = document.querySelector("[data-hide-applied]");
    if (toggle) {
      toggle.checked = hide;
      toggle.closest("label").hidden = false;
    }
    var note = document.querySelector("[data-hidden-note]");
    if (note) note.textContent = hide && shown !== total ? ", showing " + shown + " of " + total + " on this page" : "";
    var allApplied = document.querySelector("[data-all-applied]");
    if (allApplied) allApplied.hidden = !(hide && total > 0 && shown === 0);
  }

  document.addEventListener("click", function (event) {
    var btn = event.target.closest("[data-applied]");
    if (!btn) return;
    var id = btn.getAttribute("data-applied");
    var ids = readApplied();
    var i = ids.indexOf(id);
    if (i === -1) ids.push(id);
    else ids.splice(i, 1);
    writeApplied(ids);
    paintApplied();
  });

  window.addEventListener("storage", function (event) {
    if (event.key === STORAGE_KEY || event.key === HIDE_KEY) paintApplied();
  });

  // Copy buttons, e.g. the curl command.
  document.addEventListener("click", function (event) {
    var btn = event.target.closest("[data-copy]");
    if (!btn) return;
    var source = document.getElementById(btn.getAttribute("data-copy"));
    if (!source || !navigator.clipboard) return;
    navigator.clipboard.writeText(source.textContent.trim()).then(function () {
      btn.textContent = "Copied";
      window.setTimeout(function () { btn.textContent = "Copy"; }, 1800);
    }, function () {});
  });

  function showCopyButtons() {
    if (!navigator.clipboard) return;
    document.querySelectorAll("[data-copy]").forEach(function (btn) { btn.hidden = false; });
  }

  // Board filters apply as soon as they change. The page is fetched as HTML
  // and the results swapped in, so focus stays on the control just used.
  var form = document.getElementById("filters");

  function boardURL() {
    var data = new FormData(form);
    var params = new URLSearchParams();
    ["q", "company", "role", "region", "term", "duration", "sort"].forEach(function (key) {
      data.getAll(key).forEach(function (value) {
        value = String(value).trim();
        if (!value) return;
        if (key === "sort" && value === "first_seen") return;
        params.append(key, value);
      });
    });
    var qs = params.toString();
    return qs ? "/jobs?" + qs : "/jobs";
  }

  var pending = null;

  function load(url, push) {
    var results = document.getElementById("results");
    if (!results || !window.fetch || !window.DOMParser) {
      window.location.href = url;
      return;
    }
    if (pending) pending.abort();
    var controller = window.AbortController ? new AbortController() : null;
    pending = controller;
    results.setAttribute("aria-busy", "true");
    fetch(url, { signal: controller ? controller.signal : undefined, headers: { Accept: "text/html" } })
      .then(function (response) {
        if (!response.ok) throw new Error("status " + response.status);
        return response.text();
      })
      .then(function (html) {
        var doc = new DOMParser().parseFromString(html, "text/html");
        document.querySelectorAll("[data-sync]").forEach(function (el) {
          var next = el.id && doc.getElementById(el.id);
          if (next) el.innerHTML = next.innerHTML;
        });
        if (push) history.pushState({ board: true }, "", url);
        document.title = doc.title;
        results.removeAttribute("aria-busy");
        paintApplied();
        pending = null;
      })
      .catch(function (err) {
        if (err && err.name === "AbortError") return;
        window.location.href = url;
      });
  }

  function syncFormFromURL() {
    var params = new URLSearchParams(window.location.search);
    form.querySelectorAll("input[type=checkbox]").forEach(function (box) {
      box.checked = params.getAll(box.name).indexOf(box.value) !== -1;
    });
    var q = form.querySelector("[name=q]");
    if (q) q.value = params.get("q") || "";
    var company = form.querySelector("[name=company]");
    if (company) company.value = params.get("company") || "";
  }

  if (form) {
    form.querySelectorAll("[data-nojs]").forEach(function (el) { el.hidden = true; });

    document.addEventListener("change", function (event) {
      var el = event.target;
      if (!el.form || el.form !== form || el.name === "q") return;
      load(boardURL(), true);
    });

    form.addEventListener("submit", function (event) {
      event.preventDefault();
      load(boardURL(), true);
    });

    document.addEventListener("click", function (event) {
      var link = event.target.closest("a[data-board-link]");
      if (!link || event.metaKey || event.ctrlKey || event.shiftKey || event.button !== 0) return;
      event.preventDefault();
      var href = link.getAttribute("href");
      if (href === "/jobs") {
        form.reset();
        form.querySelectorAll("input[type=checkbox]").forEach(function (box) { box.checked = false; });
        var q = form.querySelector("[name=q]");
        if (q) q.value = "";
        var company = form.querySelector("[name=company]");
        if (company) company.value = "";
      }
      load(href, true);
      if (link.closest(".pager")) {
        var top = document.getElementById("results");
        if (top) top.scrollIntoView({ block: "start" });
      }
    });

    window.addEventListener("popstate", function () {
      syncFormFromURL();
      load(window.location.pathname + window.location.search, false);
    });

    // On a phone the filters start folded so the listings come first.
    var panel = document.getElementById("filters-panel");
    if (panel && window.matchMedia("(max-width: 759px)").matches) panel.open = false;
  }

  document.addEventListener("change", function (event) {
    if (!event.target.matches("[data-hide-applied]")) return;
    try {
      localStorage.setItem(HIDE_KEY, String(event.target.checked));
    } catch (e) {}
    paintApplied();
  });

  paintApplied();
  showCopyButtons();
})();
