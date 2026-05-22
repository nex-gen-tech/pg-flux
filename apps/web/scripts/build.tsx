/**
 * pg-flux docs site — static build.
 *
 *   1. Load + render all markdown pages (see src/lib/content.ts)
 *   2. Render React pages (landing, docs) to static HTML via renderToStaticMarkup
 *   3. Compile Tailwind v4 CSS through @tailwindcss/cli
 *   4. Bundle client interactivity (theme toggle, ⌘K search, mobile nav)
 *      with Bun's native bundler
 *   5. Emit dist/ with: index.html, docs/*.html, app.css, app.js,
 *      search-index.json
 *
 * Pure Bun: no Vite, Next, Astro, Webpack.
 */
import { mkdir, writeFile, rm } from "node:fs/promises";
import { join, dirname } from "node:path";
import { spawnSync } from "node:child_process";
import { renderToStaticMarkup } from "react-dom/server";
import { Landing } from "../src/pages/landing";
import { DocPage } from "../src/pages/doc";
import { loadAllPages, type Page } from "../src/lib/content";
import { BASE } from "../src/lib/base";

const ROOT = new URL("..", import.meta.url).pathname;
const OUT = join(ROOT, "dist");

const DOCTYPE = "<!doctype html>";

/** Inline theme-init script — prevents flash of incorrect theme before paint. */
const THEME_INIT_SCRIPT = `
(function () {
  try {
    var stored = localStorage.getItem('pgflux-theme');
    var prefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
    var dark = stored === 'dark' || (stored === null && prefersDark);
    if (dark) document.documentElement.classList.add('dark');
  } catch (e) {}
})();
`.trim();

function wrapHtml({
  title,
  description,
  bodyHtml,
}: {
  title: string;
  description?: string;
  bodyHtml: string;
}): string {
  return `${DOCTYPE}
<html lang="en" class="no-transitions">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
  <title>${escapeHtml(title)}</title>
  ${description ? `<meta name="description" content="${escapeHtml(description)}">` : ""}
  <link rel="preconnect" href="https://rsms.me/">
  <link rel="stylesheet" href="https://rsms.me/inter/inter.css">
  <link rel="stylesheet" href="${BASE}/app.css">
  <script>${THEME_INIT_SCRIPT}</script>
</head>
<body>
  ${bodyHtml}
  <script src="${BASE}/app.js" defer></script>
  <script>
    // Remove transition-blocker so theme changes animate after load.
    window.addEventListener('load', function () {
      document.documentElement.classList.remove('no-transitions');
    });
  </script>
</body>
</html>`;
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]!));
}

async function compileTailwind() {
  console.log(">> compiling Tailwind v4 → app.css");
  const result = spawnSync(
    "bun",
    [
      "x",
      "@tailwindcss/cli",
      "-i",
      "src/styles/globals.css",
      "-o",
      "dist/app.css",
      "--minify",
    ],
    { cwd: ROOT, stdio: "inherit" },
  );
  if (result.status !== 0) {
    throw new Error("tailwind CLI failed");
  }
}

async function bundleClient() {
  console.log(">> bundling client JS → app.js");
  const result = await Bun.build({
    entrypoints: [join(ROOT, "src/client/index.ts")],
    outdir: OUT,
    naming: "app.js",
    target: "browser",
    minify: true,
    format: "iife",
  });
  if (!result.success) {
    for (const log of result.logs) console.error(log);
    throw new Error("client bundle failed");
  }
}

async function emitPage(relPath: string, html: string) {
  const out = join(OUT, relPath);
  await mkdir(dirname(out), { recursive: true });
  await writeFile(out, html, "utf8");
}

async function emitLanding(pages: Page[]) {
  void pages;
  const body = renderToStaticMarkup(<Landing />);
  const html = wrapHtml({
    title: "pg-flux — declarative PostgreSQL migrations + codegen",
    description:
      "Declarative PostgreSQL migrations with safe apply, drift detection, schema dump, and end-to-end Go + TypeScript codegen.",
    bodyHtml: body,
  });
  await emitPage("index.html", html);
}

async function emitDocs(pages: Page[]) {
  for (const page of pages) {
    const body = renderToStaticMarkup(<DocPage page={page} pages={pages} />);
    const html = wrapHtml({
      title: `${page.title} — pg-flux docs`,
      description: page.description ?? `${page.title} — pg-flux documentation.`,
      bodyHtml: body,
    });
    await emitPage(page.slug + ".html", html);
  }
}

async function emitSearchIndex(pages: Page[]) {
  const index = pages.map((p) => ({
    title: p.title,
    href: p.href,
    group: p.group,
    body: p.raw.replace(/```[\s\S]*?```/g, "").replace(/\s+/g, " ").slice(0, 4000),
  }));
  await writeFile(join(OUT, "search-index.json"), JSON.stringify(index), "utf8");
}

async function main() {
  console.log(">> pg-flux docs build (v2)");
  await rm(OUT, { recursive: true, force: true });
  await mkdir(OUT, { recursive: true });

  const pages = await loadAllPages();
  console.log(`>> loaded ${pages.length} markdown pages`);

  await emitLanding(pages);
  await emitDocs(pages);
  await emitSearchIndex(pages);

  await compileTailwind();
  await bundleClient();

  console.log(`<< wrote landing + ${pages.length} doc pages to dist/`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
