/**
 * Tiny client-side search. Loads search-index.json once and runs a substring
 * + token-match scorer on every keystroke. No external dependencies.
 */
(async function () {
  const input = document.getElementById("search-input");
  const results = document.getElementById("search-results");
  if (!input || !results) return;

  let index = [];
  try {
    const res = await fetch("/search-index.json");
    index = await res.json();
  } catch (e) {
    return;
  }

  function search(q) {
    q = q.trim().toLowerCase();
    if (!q) return [];
    const tokens = q.split(/\s+/);
    return index
      .map(p => {
        const title = p.title.toLowerCase();
        const body = p.body.toLowerCase();
        let score = 0;
        for (const t of tokens) {
          if (title.includes(t)) score += 10;
          if (body.includes(t)) score += 1;
        }
        return { ...p, score };
      })
      .filter(p => p.score > 0)
      .sort((a, b) => b.score - a.score)
      .slice(0, 8);
  }

  function render(hits) {
    if (hits.length === 0) {
      results.classList.add("hidden");
      results.innerHTML = "";
      return;
    }
    results.innerHTML = hits
      .map(h => `<a href="${h.href}"><strong>${escapeHtml(h.title)}</strong><div class="group">${escapeHtml(h.group)}</div></a>`)
      .join("");
    results.classList.remove("hidden");
  }

  function escapeHtml(s) {
    return s.replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]));
  }

  input.addEventListener("input", () => render(search(input.value)));
  input.addEventListener("focus", () => render(search(input.value)));
  input.addEventListener("blur", () => setTimeout(() => results.classList.add("hidden"), 200));
  document.addEventListener("keydown", e => {
    if (e.key === "/" && document.activeElement !== input) {
      e.preventDefault();
      input.focus();
    }
    if (e.key === "Escape") results.classList.add("hidden");
  });
})();
