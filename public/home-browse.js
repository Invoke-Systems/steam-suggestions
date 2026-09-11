(function () {
  function gamePageUrl(item) {
    if (item && item.pageUrl) return item.pageUrl;
    if (item && item.appid) return "/app/" + item.appid;
    return (item && item.steamUrl) || "#";
  }

  function thumbSrc(item) {
    if (item && item.appid) return "/images/" + item.appid + ".jpg";
    return (item && item.header) || "";
  }

  function renderList(list, items, sale) {
    list.replaceChildren();
    var n = Math.min(items.length, 24);
    for (var i = 0; i < n; i++) {
      var item = items[i];
      var a = document.createElement("a");
      a.className = "home-row";
      a.href = gamePageUrl(item);

      var thumb = document.createElement("span");
      thumb.className = "home-row-thumb";
      thumb.setAttribute("aria-hidden", "true");
      var src = thumbSrc(item);
      if (src) {
        var img = document.createElement("img");
        img.alt = "";
        img.loading = "lazy";
        img.src = src;
        img.addEventListener("error", function () {
          this.remove();
        });
        thumb.append(img);
      }

      var name = document.createElement("span");
      name.className = "home-row-name";
      name.textContent = item.name || "Game";

      var meta = document.createElement("span");
      meta.className = "home-row-meta";
      if (sale && item.discount) {
        meta.textContent = item.formatted
          ? "−" + item.discount + "% · " + item.formatted
          : "−" + item.discount + "%";
      } else {
        meta.textContent = item.meta || "";
      }

      a.append(thumb, name);
      if (meta.textContent) a.append(meta);
      list.append(a);
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
        ["hot", data.hot || [], false],
        ["topPlayers", data.topPlayers || [], false],
        ["newWithPlayers", data.newWithPlayers || [], false],
        ["onSale", data.onSale || [], true],
      ];
      var any = false;
      for (var i = 0; i < rails.length; i++) {
        var key = rails[i][0];
        var items = rails[i][1];
        var sale = rails[i][2];
        var section = browse.querySelector('[data-home-rail="' + key + '"]');
        if (!section) continue;
        var list = section.querySelector("[data-home-list]");
        if (!items.length || !list) {
          section.hidden = true;
          continue;
        }
        any = true;
        section.hidden = false;
        renderList(list, items, sale);
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
  window.__sipLoadHome = load;
})();
