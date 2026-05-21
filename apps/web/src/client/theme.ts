/**
 * Theme toggle. The inline <script> in build.ts already applies the right
 * class before paint to avoid FOUC; this module just wires the toggle button
 * once the page is interactive.
 */
type Theme = "light" | "dark";

const KEY = "pgflux-theme";

function getCurrent(): Theme {
  return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

function set(theme: Theme) {
  const root = document.documentElement;
  if (theme === "dark") root.classList.add("dark");
  else root.classList.remove("dark");
  try {
    localStorage.setItem(KEY, theme);
  } catch {
    /* ignore quota / private-mode errors */
  }
}

function init() {
  for (const btn of document.querySelectorAll<HTMLButtonElement>("[data-theme-toggle]")) {
    btn.addEventListener("click", () => set(getCurrent() === "dark" ? "light" : "dark"));
  }
  // Listen to system preference changes when no explicit choice is stored.
  if (typeof window.matchMedia === "function") {
    const mql = window.matchMedia("(prefers-color-scheme: dark)");
    mql.addEventListener?.("change", (e) => {
      if (localStorage.getItem(KEY) === null) set(e.matches ? "dark" : "light");
    });
  }
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
