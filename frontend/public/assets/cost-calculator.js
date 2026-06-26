/* TokenProvider — LLM API cost calculator (vanilla, CSP-safe: external + addEventListener) */
(function () {
  "use strict";

  // Editable list-price defaults (USD per 1M tokens), approximate as of June 2026.
  // Users can edit every rate inline; these are conveniences, not assertions.
  var MODELS = [
    { name: "Claude Opus 4.x",   vendor: "Anthropic", inp: 15,   out: 75 },
    { name: "Claude Sonnet 4.x", vendor: "Anthropic", inp: 3,    out: 15 },
    { name: "Claude Haiku 4.x",  vendor: "Anthropic", inp: 0.8,  out: 4 },
    { name: "GPT-4o",            vendor: "OpenAI",    inp: 2.5,  out: 10 },
    { name: "GPT-4o mini",       vendor: "OpenAI",    inp: 0.15, out: 0.6 },
    { name: "Gemini 2.0 Flash",  vendor: "Google",    inp: 0.1,  out: 0.4 },
    { name: "Gemini 1.5 Pro",    vendor: "Google",    inp: 1.25, out: 5 }
  ];

  // Cached input is typically billed at ~10% of the base input rate on cache reads.
  var CACHE_READ_FACTOR = 0.1;

  function $(id) { return document.getElementById(id); }
  function num(v, d) { var n = parseFloat(v); return (isFinite(n) && n >= 0) ? n : d; }

  function money(x) {
    if (!isFinite(x)) return "—";
    if (x >= 100) return "$" + x.toFixed(0);
    if (x >= 1)   return "$" + x.toFixed(2);
    if (x >= 0.01) return "$" + x.toFixed(3);
    return "$" + x.toFixed(5);
  }

  function buildRows() {
    var tb = $("calcRows");
    if (!tb) return;
    tb.innerHTML = "";
    MODELS.forEach(function (m, i) {
      var tr = document.createElement("tr");
      tr.innerHTML =
        '<td data-label="Model"><strong>' + m.name + '</strong>' +
          '<span class="calc-vendor">' + m.vendor + '</span></td>' +
        '<td data-label="Input $/M"><input class="calc-rate" type="number" inputmode="decimal" ' +
          'min="0" step="0.01" data-i="' + i + '" data-k="inp" value="' + m.inp + '" ' +
          'aria-label="' + m.name + ' input price per million tokens"></td>' +
        '<td data-label="Output $/M"><input class="calc-rate" type="number" inputmode="decimal" ' +
          'min="0" step="0.01" data-i="' + i + '" data-k="out" value="' + m.out + '" ' +
          'aria-label="' + m.name + ' output price per million tokens"></td>' +
        '<td data-label="$ / request" class="calc-out" id="req' + i + '">—</td>' +
        '<td data-label="$ / day" class="calc-out" id="day' + i + '">—</td>' +
        '<td data-label="$ / month" class="calc-out calc-strong" id="mon' + i + '">—</td>';
      tb.appendChild(tr);
    });
  }

  function recompute() {
    var inTok  = num($("inTok")  && $("inTok").value, 0);
    var outTok = num($("outTok") && $("outTok").value, 0);
    var reqDay = num($("reqDay") && $("reqDay").value, 0);
    var cachePct = num($("cachePct") && $("cachePct").value, 0);

    var cv = $("cacheVal");
    if (cv) cv.textContent = Math.round(cachePct) + "%";

    var cacheFrac = Math.min(Math.max(cachePct / 100, 0), 1);
    var inMult = (1 - cacheFrac) + cacheFrac * CACHE_READ_FACTOR;

    // Pull current (possibly edited) rates back into the model list.
    var rates = document.querySelectorAll(".calc-rate");
    Array.prototype.forEach.call(rates, function (el) {
      var i = +el.getAttribute("data-i");
      var k = el.getAttribute("data-k");
      if (MODELS[i]) MODELS[i][k] = num(el.value, MODELS[i][k]);
    });

    var best = Infinity, bestIdx = -1;
    MODELS.forEach(function (m, i) {
      var costIn  = inTok * inMult * m.inp / 1e6;
      var costOut = outTok * m.out / 1e6;
      var perReq  = costIn + costOut;
      var perDay  = perReq * reqDay;
      var perMon  = perDay * 30;

      var r = $("req" + i), d = $("day" + i), mo = $("mon" + i);
      if (r)  r.textContent  = money(perReq);
      if (d)  d.textContent  = money(perDay);
      if (mo) mo.textContent = money(perMon);

      if (perMon < best) { best = perMon; bestIdx = i; }
    });

    MODELS.forEach(function (m, i) {
      var mo = $("mon" + i);
      if (mo) mo.classList.toggle("calc-best", i === bestIdx && reqDay > 0);
    });
  }

  function init() {
    buildRows();
    ["inTok", "outTok", "reqDay", "cachePct"].forEach(function (id) {
      var el = $(id);
      if (el) el.addEventListener("input", recompute);
    });
    // Rows are built dynamically — delegate rate edits at the document level.
    document.addEventListener("input", function (e) {
      var t = e.target;
      if (t && t.classList && t.classList.contains("calc-rate")) recompute();
    });
    recompute();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
