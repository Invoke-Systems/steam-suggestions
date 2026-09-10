(function () {
  function gamePageUrl(item) {
    if (item && item.pageUrl) return item.pageUrl;
    if (item && item.appid) return "/app/" + item.appid;
    return (item && item.steamUrl) || "#";
  }

  function decorateCover(cover, url, saleText, appid) {
    cover.replaceChildren();
    cover.hidden = false;
    var fallbacks = [];
    if (url) fallbacks.push(url);
    if (appid) {
      fallbacks.push("https://cdn.akamai.steamstatic.com/steam/apps/" + appid + "/header.jpg");
      fallbacks.push(
        "https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/" + appid + "/capsule_616x353.jpg"
      );
    }
    if (!fallbacks.length) {
      cover.hidden = true;
    } else {
      var img = document.createElement("img");
      img.className = "cover-photo";
      img.alt = "";
      img.loading = "lazy";
      var idx = 0;
      img.src = fallbacks[0];
      img.addEventListener("error", function () {
        idx += 1;
        if (idx < fallbacks.length) img.src = fallbacks[idx];
        else cover.hidden = true;
      });
      cover.append(img);
    }
    if (saleText) {
      var badge = document.createElement("span");
      badge.className = "sale-badge";
      badge.textContent = saleText;
      cover.append(badge);
    }
  }

  function renderRail(grid, items, sale) {
    grid.replaceChildren();
    for (var i = 0; i < Math.min(items.length, 8); i++) {
      var item = items[i];
      var card = document.createElement("article");
      card.className = "game-card";
      var cover = document.createElement("a");
      cover.className = "cover";
      cover.href = gamePageUrl(item);
      cover.setAttribute("aria-label", item.name || "Game");
      var saleText = "";
      if (sale && item.discount) {
        saleText = item.formatted ? "−" + item.discount + "% · " + item.formatted : "−" + item.discount + "%";
      }
      decorateCover(cover, item.header, saleText, item.appid);
      var body = document.createElement("div");
      body.className = "card-body";
      var name = document.createElement("h3");
      var link = document.createElement("a");
      link.href = gamePageUrl(item);
      link.textContent = item.name || "Game";
      name.append(link);
      var meta = document.createElement("div");
      meta.className = "hours";
      meta.textContent = item.meta || (sale && item.discount ? item.discount + "% off" : "");
      body.append(name, meta);
      card.append(cover, body);
      grid.append(card);
    }
  }

  async function load() {
    var browse = document.getElementById("browse");
    if (!browse || browse.dataset.loaded === "1") return;
    try {
      var res = await fetch("/api/home", { credentials: "same-origin" });
      var data = await res.json().catch(function () {
        return {};
      });
      if (!res.ok) throw new Error("home");
      var rails = [
        ["risingReviews", data.risingReviews || [], false],
        ["newWithPlayers", data.newWithPlayers || [], false],
        ["onSale", data.onSale || [], true],
        ["topPlayers", data.topPlayers || [], false],
      ];
      var any = false;
      for (var i = 0; i < rails.length; i++) {
        var key = rails[i][0];
        var items = rails[i][1];
        var sale = rails[i][2];
        var section = browse.querySelector('[data-home-rail="' + key + '"]');
        if (!section) continue;
        var grid = section.querySelector("[data-home-grid]");
        if (!items.length || !grid) {
          section.hidden = true;
          continue;
        }
        any = true;
        section.hidden = false;
        renderRail(grid, items, sale);
      }
      browse.hidden = !any;
      if (any) browse.dataset.loaded = "1";
    } catch (err) {
      browse.hidden = true;
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", load);
  } else {
    load();
  }
  window.__playsiftLoadHome = load;
})();
