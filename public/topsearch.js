(function () {
  var forms = document.querySelectorAll("[data-top-search]");
  if (!forms.length) return;

  function isSteamURL(q) {
    return /steamcommunity\.com/i.test(q) || /^https?:\/\//i.test(q);
  }

  function isSteamID64(q) {
    return /^\d{17}$/.test(q);
  }

  function looksLikeVanity(q) {
    return !/\s/.test(q) && /^[A-Za-z0-9_-]{2,64}$/.test(q);
  }

  function bind(form) {
    var input = form.querySelector("[data-top-search-input]");
    var box = form.querySelector("[data-top-search-results]");
    if (!input || !box) return;

    var timer = 0;
    var req = 0;
    var items = [];
    var active = -1;
    var profileQ = "";

    function hide() {
      box.hidden = true;
      box.replaceChildren();
      items = [];
      active = -1;
      profileQ = "";
    }

    function addProfileOption(q) {
      if (!looksLikeVanity(q) && !isSteamID64(q)) return;
      profileQ = q;
      var a = document.createElement("button");
      a.type = "button";
      a.className = "top-search-hit top-search-profile";
      a.textContent = "Open Steam profile · " + q;
      a.setAttribute("role", "option");
      a.addEventListener("mousedown", function (ev) {
        ev.preventDefault();
        goProfile(q);
      });
      box.append(a);
    }

    function render(games, q) {
      items = games || [];
      profileQ = "";
      active = items.length ? 0 : -1;
      box.replaceChildren();
      for (var i = 0; i < items.length; i++) {
        var a = document.createElement("a");
        a.className = "top-search-hit" + (i === active ? " active" : "");
        a.href = items[i].pageUrl || "/app/" + items[i].appid;
        a.textContent = items[i].name;
        a.setAttribute("role", "option");
        a.addEventListener("mousedown", function (ev) {
          ev.preventDefault();
          location.href = this.href;
        });
        box.append(a);
      }
      addProfileOption(q);
      var hits = box.querySelectorAll(".top-search-hit");
      if (!hits.length) {
        box.hidden = true;
        return;
      }
      if (active < 0 && profileQ) {
        active = items.length; // highlight profile row when no games
        hits[active].classList.add("active");
      }
      box.hidden = false;
    }

    function setActive(next) {
      var hits = box.querySelectorAll(".top-search-hit");
      if (!hits.length) return;
      active = (next + hits.length) % hits.length;
      for (var i = 0; i < hits.length; i++) {
        hits[i].classList.toggle("active", i === active);
      }
    }

    function fetchGames(q) {
      var id = ++req;
      fetch("/api/games?q=" + encodeURIComponent(q) + "&limit=8", { credentials: "same-origin" })
        .then(function (res) {
          return res.json();
        })
        .then(function (data) {
          if (id !== req) return;
          if ((input.value || "").trim() !== q) return;
          render((data && data.games) || [], q);
        })
        .catch(function () {
          if (id === req) {
            box.replaceChildren();
            addProfileOption(q);
            box.hidden = !box.childNodes.length;
            items = [];
            active = box.childNodes.length ? 0 : -1;
            if (active >= 0) box.querySelector(".top-search-hit").classList.add("active");
          }
        });
    }

    function goProfile(q) {
      var m = q.match(/steamcommunity\.com\/profiles\/(\d{17})/i);
      if (m) {
        location.href = "/u/" + m[1];
        return;
      }
      m = q.match(/steamcommunity\.com\/id\/([^/?#]+)/i);
      if (m) {
        location.href = "/u/" + encodeURIComponent(m[1]);
        return;
      }
      if (/^\d{17}$/.test(q) || looksLikeVanity(q)) {
        location.href = "/u/" + encodeURIComponent(q);
        return;
      }
      location.href = "/u/" + encodeURIComponent(q);
    }

    function exactGame(q) {
      var lower = q.toLowerCase();
      for (var i = 0; i < items.length; i++) {
        if ((items[i].name || "").toLowerCase() === lower) return items[i];
      }
      return null;
    }

    input.addEventListener("input", function () {
      var q = (input.value || "").trim();
      window.clearTimeout(timer);
      if (q.length < 2) {
        hide();
        return;
      }
      timer = window.setTimeout(function () {
        if (isSteamURL(q)) {
          box.replaceChildren();
          items = [];
          profileQ = q;
          var a = document.createElement("button");
          a.type = "button";
          a.className = "top-search-hit top-search-profile active";
          a.textContent = "Open Steam profile";
          a.addEventListener("mousedown", function (ev) {
            ev.preventDefault();
            goProfile(q);
          });
          box.append(a);
          active = 0;
          box.hidden = false;
          return;
        }
        fetchGames(q);
      }, 180);
    });

    input.addEventListener("keydown", function (ev) {
      if (box.hidden) return;
      var hits = box.querySelectorAll(".top-search-hit");
      if (!hits.length) return;
      if (ev.key === "ArrowDown") {
        ev.preventDefault();
        setActive(active + 1);
      } else if (ev.key === "ArrowUp") {
        ev.preventDefault();
        setActive(active - 1);
      } else if (ev.key === "Escape") {
        hide();
      }
    });

    input.addEventListener("blur", function () {
      window.setTimeout(hide, 120);
    });

    form.addEventListener("submit", function (ev) {
      ev.preventDefault();
      var q = (input.value || "").trim();
      if (!q) return;

      if (isSteamURL(q) || isSteamID64(q)) {
        goProfile(q);
        return;
      }

      var hits = box.querySelectorAll(".top-search-hit");
      if (!box.hidden && active >= 0 && hits[active]) {
        if (hits[active].classList.contains("top-search-profile")) {
          goProfile(profileQ || q);
          return;
        }
        if (items[active]) {
          location.href = items[active].pageUrl || "/app/" + items[active].appid;
          return;
        }
      }

      var exact = exactGame(q);
      if (exact) {
        location.href = exact.pageUrl || "/app/" + exact.appid;
        return;
      }

      if (looksLikeVanity(q)) {
        goProfile(q);
        return;
      }

      location.href = "/search?q=" + encodeURIComponent(q);
    });
  }

  for (var i = 0; i < forms.length; i++) bind(forms[i]);
})();
