/**
 * Static-site build for the pg-flux docs.
 *
 * Pipeline:
 *   1. Read every Markdown file under content/docs/**.md
 *   2. Render to HTML with marked, syntax-highlight code blocks with shiki
 *   3. Generate page metadata (title, slug, sidebar group) from frontmatter
 *      or filename
 *   4. Wrap each page in the shared layout (sidebar nav, header, footer)
 *   5. Render the landing page from src/landing.tsx
 *   6. Copy public/ assets into dist/
 *   7. Write a tiny search-index.json for client-side fuzzy search
 *
 * Pure Bun: no framework, no Node, no third-party bundler. marked + shiki
 * are leaf dependencies (markdown + syntax highlight); everything else is
 * stdlib.
 */
import { mkdir, readdir, readFile, writeFile, rm, cp, stat } from "node:fs/promises";
import { join, relative, dirname, basename } from "node:path";
import { Marked } from "marked";
import { createHighlighter, type Highlighter } from "shiki";
import { renderLayout } from "../src/layout.tsx";
import { renderLanding } from "../src/landing.tsx";

const ROOT = new URL("..", import.meta.url).pathname;
const CONTENT = join(ROOT, "content");
const PUBLIC = join(ROOT, "public");
const OUT = join(ROOT, "dist");

interface Page {
  slug: string;       // "docs/quick-start"
  href: string;       // "/docs/quick-start.html"
  title: string;
  group: string;      // sidebar group label
  order: number;      // sort within group
  html: string;       // rendered body
  raw: string;        // original markdown (for search index)
}

async function findMarkdown(dir: string, prefix = ""): Promise<string[]> {
  const out: string[] = [];
  const entries = await readdir(dir, { withFileTypes: true });
  for (const e of entries) {
    const full = join(dir, e.name);
    const rel = prefix ? `${prefix}/${e.name}` : e.name;
    if (e.isDirectory()) {
      out.push(...(await findMarkdown(full, rel)));
    } else if (e.name.endsWith(".md")) {
      out.push(rel);
    }
  }
  return out;
}

// Front-matter parser: simple YAML-ish blocks at the top of a .md file.
//   ---
//   title: Quick start
//   group: Getting started
//   order: 2
//   ---
function parseFrontmatter(src: string): { meta: Record<string, string>; body: string } {
  const m = src.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/);
  if (!m) return { meta: {}, body: src };
  const meta: Record<string, string> = {};
  for (const line of m[1].split("\n")) {
    const kv = line.match(/^(\w+):\s*(.+)$/);
    if (kv) meta[kv[1]] = kv[2].trim();
  }
  return { meta, body: m[2] };
}

async function buildPages(highlighter: Highlighter): Promise<Page[]> {
  const md = new Marked({
    gfm: true,
    breaks: false,
    renderer: {
      code({ text, lang }) {
        const safeLang = lang && highlighter.getLoadedLanguages().includes(lang as never)
          ? (lang as never)
          : "text";
        return highlighter.codeToHtml(text, { lang: safeLang, theme: "github-dark" });
      },
      heading({ tokens, depth }) {
        const text = this.parser.parseInline(tokens);
        const slug = text.toLowerCase()
          .replace(/<[^>]+>/g, "")
          .replace(/[^\w\s-]/g, "")
          .replace(/\s+/g, "-");
        return `<h${depth} id="${slug}"><a class="anchor" href="#${slug}">#</a>${text}</h${depth}>`;
      },
    },
  });
  const files = await findMarkdown(join(CONTENT, "docs"));
  const pages: Page[] = [];
  for (const rel of files) {
    const src = await readFile(join(CONTENT, "docs", rel), "utf8");
    const { meta, body } = parseFrontmatter(src);
    const slug = "docs/" + rel.replace(/\.md$/, "").replace(/\\/g, "/");
    const title = meta.title || deriveTitle(body) || basename(rel, ".md");
    const html = md.parse(body) as string;
    pages.push({
      slug,
      href: "/" + slug + ".html",
      title,
      group: meta.group || "Reference",
      order: meta.order ? Number(meta.order) : 99,
      html,
      raw: body,
    });
  }
  return pages.sort((a, b) => a.order - b.order || a.title.localeCompare(b.title));
}

function deriveTitle(md: string): string {
  const m = md.match(/^#\s+(.+)$/m);
  return m ? m[1].trim() : "";
}

async function copyDirIfExists(src: string, dst: string) {
  try {
    const s = await stat(src);
    if (!s.isDirectory()) return;
  } catch {
    return;
  }
  await cp(src, dst, { recursive: true });
}

async function main() {
  console.log(">> pg-flux docs build");
  await rm(OUT, { recursive: true, force: true });
  await mkdir(OUT, { recursive: true });

  const highlighter = await createHighlighter({
    themes: ["github-dark"],
    langs: ["bash", "sql", "go", "typescript", "tsx", "yaml", "json", "javascript", "text", "diff"],
  });

  // Render pages.
  const pages = await buildPages(highlighter);
  for (const p of pages) {
    const html = renderLayout({ page: p, pages });
    const out = join(OUT, p.slug + ".html");
    await mkdir(dirname(out), { recursive: true });
    await writeFile(out, html, "utf8");
  }

  // Render landing index.
  const indexHtml = renderLanding({ pages });
  await writeFile(join(OUT, "index.html"), indexHtml, "utf8");

  // Copy static assets (CSS, fonts, images).
  await copyDirIfExists(PUBLIC, OUT);

  // Search index for client-side fuzzy search.
  const searchIndex = pages.map(p => ({
    title: p.title,
    href: p.href,
    group: p.group,
    body: p.raw.replace(/```[\s\S]*?```/g, "").replace(/\s+/g, " ").slice(0, 4000),
  }));
  await writeFile(join(OUT, "search-index.json"), JSON.stringify(searchIndex), "utf8");

  console.log(`<< wrote ${pages.length} doc pages + landing + search-index to ${relative(ROOT, OUT)}/`);
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});
