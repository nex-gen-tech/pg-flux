/**
 * Page layout shared by every docs page. Returns a complete HTML document
 * string — no client-side React/hydration required.
 */
import type { Page } from "./types";

interface LayoutInput {
  page: Page;
  pages: Page[];
}

export function renderLayout({ page, pages }: LayoutInput): string {
  const groups = groupPages(pages);
  return /* html */ `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>${escape(page.title)} — pg-flux</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="description" content="pg-flux — declarative PostgreSQL migrations + bidirectional codegen.">
<link rel="stylesheet" href="/style.css">
</head>
<body class="docs">
${renderHeader(page)}
<main class="layout">
  <aside class="sidebar">
    ${renderSidebar(groups, page)}
  </aside>
  <article class="content">
    <header class="page-header">
      <span class="kicker">${escape(page.group)}</span>
      <h1>${escape(page.title)}</h1>
    </header>
    <div class="prose">${page.html}</div>
    ${renderPagination(page, pages)}
  </article>
</main>
${renderFooter()}
<script src="/search.js" defer></script>
</body>
</html>`;
}

function renderHeader(currentPage?: Page): string {
  return /* html */ `<header class="topnav">
  <a class="brand" href="/">pg-flux</a>
  <nav>
    <a href="/" class="${currentPage ? "" : "active"}">Home</a>
    <a href="/docs/quick-start.html" class="${currentPage ? "active" : ""}">Docs</a>
    <a href="https://github.com/nexg/pg-flux" target="_blank" rel="noopener">GitHub</a>
  </nav>
  <div class="search">
    <input id="search-input" type="search" placeholder="Search docs..." aria-label="Search docs">
    <div id="search-results" class="hidden"></div>
  </div>
</header>`;
}

function renderSidebar(groups: ReturnType<typeof groupPages>, current: Page): string {
  return Object.entries(groups).map(([group, items]) => /* html */ `
    <div class="nav-group">
      <h4>${escape(group)}</h4>
      <ul>
        ${items.map(p => `<li><a href="${p.href}" class="${p.slug === current.slug ? "active" : ""}">${escape(p.title)}</a></li>`).join("\n")}
      </ul>
    </div>`).join("\n");
}

function renderPagination(current: Page, pages: Page[]): string {
  const idx = pages.findIndex(p => p.slug === current.slug);
  const prev = idx > 0 ? pages[idx - 1] : null;
  const next = idx < pages.length - 1 ? pages[idx + 1] : null;
  if (!prev && !next) return "";
  return /* html */ `<nav class="pagination">
    ${prev ? `<a class="prev" href="${prev.href}"><span>← Previous</span><strong>${escape(prev.title)}</strong></a>` : "<span></span>"}
    ${next ? `<a class="next" href="${next.href}"><span>Next →</span><strong>${escape(next.title)}</strong></a>` : "<span></span>"}
  </nav>`;
}

function renderFooter(): string {
  return /* html */ `<footer class="bottom">
  <p>pg-flux — declarative PostgreSQL migrations + codegen. <a href="https://github.com/nexg/pg-flux">GitHub</a></p>
</footer>`;
}

function groupPages(pages: Page[]): Record<string, Page[]> {
  const out: Record<string, Page[]> = {};
  for (const p of pages) {
    (out[p.group] ||= []).push(p);
  }
  // Sort the group order: known groups first, others alphabetical.
  const order = ["Getting started", "Migrations", "Dump & sync", "Codegen", "Configuration", "Reference"];
  const sorted: Record<string, Page[]> = {};
  for (const g of order) if (out[g]) sorted[g] = out[g];
  for (const g of Object.keys(out)) if (!sorted[g]) sorted[g] = out[g];
  return sorted;
}

function escape(s: string): string {
  return s.replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]!));
}

export { renderHeader, renderFooter };
