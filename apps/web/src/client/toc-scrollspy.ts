/**
 * TOC scrollspy — highlights the currently-visible heading in the right rail.
 * Uses IntersectionObserver with a top-margin offset so the active link
 * matches what the user's eye is reading, not literally the topmost element.
 */
function init() {
  const links = Array.from(document.querySelectorAll<HTMLAnchorElement>("[data-toc-link]"));
  if (links.length === 0) return;
  const linkByTarget = new Map<string, HTMLAnchorElement>();
  const headings: HTMLElement[] = [];
  for (const l of links) {
    const target = l.getAttribute("data-toc-target");
    if (!target) continue;
    const el = document.getElementById(target);
    if (el) {
      linkByTarget.set(target, l);
      headings.push(el);
    }
  }
  if (headings.length === 0) return;

  function setActive(id: string | null) {
    for (const l of links) {
      const active = l.getAttribute("data-toc-target") === id;
      l.classList.toggle("text-[--color-primary]", active);
      l.classList.toggle("font-medium", active);
      l.classList.toggle("border-[--color-primary]", active);
      l.style.borderLeftColor = active ? "var(--color-primary)" : "";
    }
  }

  const observer = new IntersectionObserver(
    (entries) => {
      const visible = entries
        .filter((e) => e.isIntersecting)
        .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
      if (visible[0]) setActive(visible[0].target.id);
    },
    { rootMargin: "-15% 0px -75% 0px", threshold: 0 },
  );
  for (const h of headings) observer.observe(h);
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", init);
else init();
