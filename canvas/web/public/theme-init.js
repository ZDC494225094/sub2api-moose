try {
    var persistedTheme = JSON.parse(localStorage.getItem("infinite-canvas:theme_store") || "{}");
    var initialTheme = persistedTheme.state && persistedTheme.state.theme === "light" ? "light" : "dark";
    document.documentElement.classList.toggle("dark", initialTheme === "dark");
    document.documentElement.style.colorScheme = initialTheme;
} catch (_error) {
    document.documentElement.classList.add("dark");
    document.documentElement.style.colorScheme = "dark";
}
