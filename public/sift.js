(function () {
  var active = "";
  var bars = document.querySelectorAll("[data-taste]");
  var cards = document.querySelectorAll("#library-grid .game-card[data-tags], #library-more-grid .game-card[data-tags]");
  var hint = document.getElementById("library-filter-hint");

  function apply() {
    var shown = 0;
    for (var i = 0; i < cards.length; i++) {
      var tags = cards[i].getAttribute("data-tags") || "";
      var match = !active || tags.indexOf("|" + active + "|") !== -1;
      cards[i].hidden = !match;
      if (match) shown += 1;
    }
    var more = document.querySelector(".library-more");
    if (more && active) more.open = true;
    for (var j = 0; j < bars.length; j++) {
      bars[j].classList.toggle("include", active !== "" && bars[j].getAttribute("data-taste") === active);
    }
    if (hint) {
      hint.hidden = !active;
      hint.textContent = active ? "Library filtered to " + active + " · " + shown + " games." : "";
    }
  }

  for (var b = 0; b < bars.length; b++) {
    bars[b].addEventListener("click", function () {
      var tag = this.getAttribute("data-taste") || "";
      active = active === tag ? "" : tag;
      apply();
    });
  }

  var photos = document.querySelectorAll("img.cover-photo");
  for (var p = 0; p < photos.length; p++) {
    var img = photos[p];
    img.addEventListener("error", hideBrokenCover);
    if (img.complete && img.naturalWidth === 0) hideBrokenCover.call(img);
  }

  function hideBrokenCover() {
    var cover = this.closest(".cover");
    if (cover) cover.hidden = true;
  }
})();
