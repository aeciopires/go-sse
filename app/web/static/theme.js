// Applies the saved color theme before the page is painted (avoids a flash of
// the wrong theme). Loaded synchronously in <head>. Default theme: dark.
(function () {
    "use strict";
    var theme = "dark";
    try {
        var saved = window.localStorage.getItem("go-sse-theme");
        if (saved === "light" || saved === "dark") {
            theme = saved;
        }
    } catch (e) {
        // Storage may be unavailable (private mode, blocked cookies): keep default.
    }
    document.documentElement.setAttribute("data-theme", theme);
})();
