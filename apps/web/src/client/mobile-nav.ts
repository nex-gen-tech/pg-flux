/**
 * Mobile sidebar — slide-in drawer from the left. Backdrop click + Escape
 * close it; closes automatically when a nav link is clicked.
 */
function init() {
  const root = document.querySelector<HTMLDivElement>("[data-mobile-nav]");
  if (!root) return;
  const backdrop = root.querySelector<HTMLDivElement>("[data-mobile-nav-backdrop]");
  const aside = root.querySelector<HTMLElement>("aside");

  function open() {
    root.classList.remove("hidden");
    root.classList.add("flex");
    document.body.style.overflow = "hidden";
    if (aside) {
      aside.style.transform = "translateX(0)";
    }
  }
  function close() {
    root.classList.add("hidden");
    root.classList.remove("flex");
    document.body.style.overflow = "";
  }
  for (const t of document.querySelectorAll<HTMLButtonElement>("[data-mobile-nav-trigger]")) {
    t.addEventListener("click", open);
  }
  backdrop?.addEventListener("click", close);
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") close();
  });
  for (const link of root.querySelectorAll<HTMLAnchorElement>("a")) {
    link.addEventListener("click", close);
  }
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
else init();
