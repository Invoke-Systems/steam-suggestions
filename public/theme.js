(function () {
  var key = "playsift-theme";
  var root = document.documentElement;

  function current() {
    return root.getAttribute("data-theme") === "light" ? "light" : "dark";
  }

  function label(theme) {
    return theme === "light" ? "Dark" : "Light";
  }

  function apply(theme) {
    theme = theme === "light" ? "light" : "dark";
    root.setAttribute("data-theme", theme);
    try {
      localStorage.setItem(key, theme);
    } catch (e) {}
    var buttons = document.querySelectorAll("[data-theme-toggle]");
    for (var i = 0; i < buttons.length; i++) {
      buttons[i].textContent = label(theme);
      buttons[i].setAttribute("aria-label", "Switch to " + label(theme).toLowerCase() + " mode");
    }
  }

  var saved = "";
  try {
    saved = localStorage.getItem(key) || "";
  } catch (e) {}
  if (!saved && window.matchMedia && window.matchMedia("(prefers-color-scheme: light)").matches) {
    saved = "light";
  }
  apply(saved || "dark");

  document.addEventListener("click", function (event) {
    var btn = event.target.closest("[data-theme-toggle]");
    if (!btn) return;
    apply(current() === "light" ? "dark" : "light");
  });
})();
